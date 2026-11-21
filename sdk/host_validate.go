package sdk

import (
	"errors"
	"fmt"
	"reflect"
	"regexp/syntax"
	"unicode/utf8"

	"github.com/go-playground/validator/v10"

	ut "github.com/go-playground/universal-translator"
)

// emptyPassRenameTag is the tag of the combination of an empty-file policy and a
// move mode the host connector refuses.
const emptyPassRenameTag = "empty_pass_with_rename"

// HostEmptyPassWithRenameMessage is what the host connector says of the
// combination; the console says the same when the configuration is saved.
const HostEmptyPassWithRenameMessage = "unanalyzable_policy.empty: pass delivers an empty file as a new empty file, which needs move.mode copy: it is refused with rename while the move action is on (leave the key out to skip empty files in that mode)"

// filepathPatternTag is the tag of the rules on the patterns of filepath_filter.
const filepathPatternTag = "filepath_pattern"

const (
	// maxFilepathPatternLen bounds a pattern, in bytes.
	maxFilepathPatternLen = 1024
	// maxEchoedPattern is how much of a refused pattern a message repeats.
	maxEchoedPattern = 64
)

// echoPattern quotes at most maxEchoedPattern bytes of a pattern, cut on a
// character boundary: a pattern is the user's, and may be huge or binary.
func echoPattern(p string) string {
	if len(p) <= maxEchoedPattern {
		return fmt.Sprintf("%q", p)
	}
	cut := maxEchoedPattern
	for cut > 0 && !utf8.RuneStart(p[cut]) {
		cut--
	}
	return fmt.Sprintf("%q...", p[:cut])
}

const (
	// maxFilepathPatterns bounds each list of filepath_filter.
	maxFilepathPatterns = 256
	// maxFilepathTotalBytes bounds the patterns of both lists together: parsing a
	// pattern of Unicode classes costs milliseconds, so the input is bounded before
	// any parsing.
	maxFilepathTotalBytes = 32 << 10
)

// parseRegexp is syntax.Parse; tests replace it to count the calls.
var parseRegexp = syntax.Parse

// filepathProblem is the first thing wrong in a filepath_filter.
type filepathProblem struct {
	field, structField, reason string
}

// checkFilepathFilter finds the first problem of the section, in an order that
// bounds the work before any of it is done: the count of each list, the bytes of
// all the patterns, then each pattern (not empty, its size, not in both lists)
// and only then the syntax of each (not compiled: the host connector compiles
// once when it applies the configuration, and also refuses what only the
// compilation sees, a pattern too large or one that matches the empty string).
func checkFilepathFilter(f HostFilepathFilter) *filepathProblem {
	lists := []struct {
		name, structField string
		patterns          []string
	}{
		{"forbidden_paths", "ForbiddenPaths", f.ForbiddenPaths},
		{"skipped_paths", "SkippedPaths", f.SkippedPaths},
	}
	for _, l := range lists {
		if len(l.patterns) > maxFilepathPatterns {
			return &filepathProblem{l.name, l.structField, fmt.Sprintf("the list has %d patterns, at most %d", len(l.patterns), maxFilepathPatterns)}
		}
	}
	total := 0
	for _, l := range lists {
		for _, p := range l.patterns {
			total += len(p)
		}
	}
	if total > maxFilepathTotalBytes {
		return &filepathProblem{"patterns", "ForbiddenPaths", fmt.Sprintf("the patterns of both lists hold %d bytes, at most %d in all", total, maxFilepathTotalBytes)}
	}
	inForbidden := make(map[string]struct{}, len(f.ForbiddenPaths))
	for _, p := range f.ForbiddenPaths {
		inForbidden[p] = struct{}{}
	}
	for _, l := range lists {
		for i, p := range l.patterns {
			field := fmt.Sprintf("%s[%d]", l.name, i)
			switch {
			case p == "":
				return &filepathProblem{field, l.structField, "an empty pattern matches every path"}
			case len(p) > maxFilepathPatternLen:
				return &filepathProblem{field, l.structField, fmt.Sprintf("the pattern is %d bytes, at most %d", len(p), maxFilepathPatternLen)}
			}
			if _, both := inForbidden[p]; both && l.name == "skipped_paths" {
				return &filepathProblem{field, l.structField, fmt.Sprintf("the pattern %s is also in forbidden_paths", echoPattern(p))}
			}
		}
	}
	for _, l := range lists {
		for i, p := range l.patterns {
			if _, err := parseRegexp(p, syntax.Perl); err != nil {
				// The error of Parse quotes the whole pattern: only its code is said.
				reason := "invalid syntax"
				if se, ok := errors.AsType[*syntax.Error](err); ok {
					reason = string(se.Code)
				}
				return &filepathProblem{fmt.Sprintf("%s[%d]", l.name, i), l.structField, fmt.Sprintf("the pattern %s is not a valid regular expression (%s)", echoPattern(p), reason)}
			}
		}
	}
	return nil
}

// registerFilepathFilterValidation refuses, in filepath_filter, an empty pattern,
// one over 1024 bytes, one that is not a regular expression, and one that is in
// both lists. The count of each list is the tag max=256 of its field. It is
// registered on the section, so that it runs when the section is validated on its
// own and when the whole configuration is.
func registerFilepathFilterValidation(validate *validator.Validate, trans ut.Translator) error {
	validate.RegisterStructValidation(func(sl validator.StructLevel) {
		f, ok := reflect.TypeAssert[HostFilepathFilter](sl.Current())
		if !ok {
			return
		}
		if problem := checkFilepathFilter(f); problem != nil {
			sl.ReportError("", problem.field, problem.structField, filepathPatternTag, problem.reason)
		}
	}, HostFilepathFilter{})
	return validate.RegisterTranslation(filepathPatternTag, trans,
		func(ut.Translator) error { return nil },
		func(_ ut.Translator, fe validator.FieldError) string {
			return "filepath_filter." + fe.Field() + ": " + fe.Param()
		},
	)
}

// registerHostConfigValidation adds to validate the rules that bind several
// keys of the host configuration. An empty policy of pass is refused with the
// move action on and its mode rename: the empty file is made new, which only the
// move by copy does. The policy left out with rename is the connector's to
// settle (it skips), and with the move action off the mode means nothing.
func registerHostConfigValidation(validate *validator.Validate, trans ut.Translator) error {
	if err := registerFilepathFilterValidation(validate, trans); err != nil {
		return err
	}
	validate.RegisterStructValidation(func(sl validator.StructLevel) {
		c, ok := sl.Current().Interface().(HostConfig)
		if !ok {
			return
		}
		if c.UnanalyzablePolicy.Empty == "pass" && c.Actions.Move && c.Move.Mode == "rename" {
			sl.ReportError(c.UnanalyzablePolicy.Empty, "unanalyzable_policy.empty", "Empty", emptyPassRenameTag, "")
		}
	}, HostConfig{})
	return validate.RegisterTranslation(emptyPassRenameTag, trans,
		func(ut.Translator) error { return nil },
		func(ut.Translator, validator.FieldError) string { return HostEmptyPassWithRenameMessage },
	)
}
