package sdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp/syntax"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/go-playground/validator/v10"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gopkg.in/yaml.v3"
)

func newHostConfigWithNewKeys() HostConfig {
	return HostConfig{
		WorkDir: "/var/lib/gmhost/work",
		UnanalyzablePolicy: HostUnanalyzablePolicy{
			TooBig: "pass", Empty: "mitigate", Encrypted: "error", ExtractError: "as_file", AnalysisError: "pass",
		},
		FilepathFilter: HostFilepathFilter{
			ForbiddenPaths: []string{`^/srv/in/[^/]+/\.git/`, `\.exe$`},
			SkippedPaths:   []string{`^/srv/in/trusted/`},
		},
		Monitoring: HostMonitoringConfig{
			DisableFSWatcher: true, MaxTrackedMemory: "256MiB", MaxTrackedDirPercent: 25,
		},
		Move: HostMoveConfig{Mode: "copy", MinThroughput: "10MiB"},
	}
}

func TestHostConfig_NewKeysRoundTrip(t *testing.T) {
	want := newHostConfigWithNewKeys()

	t.Run("json", func(t *testing.T) {
		b, err := json.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		var got HostConfig
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("json round trip diff = %s", diff)
		}
	})

	t.Run("yaml", func(t *testing.T) {
		b, err := yaml.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		var got HostConfig
		if err := yaml.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("yaml round trip diff = %s", diff)
		}
	})
}

func TestHostConfig_NewKeysValidation(t *testing.T) {
	validate := validator.New()
	tests := []struct {
		name    string
		mutate  func(c *HostConfig)
		wantErr bool
	}{
		{name: "zero values accepted", mutate: func(c *HostConfig) {}},
		{name: "all valid", mutate: func(c *HostConfig) { *c = newHostConfigWithNewKeys() }},
		{name: "too_big invalid", mutate: func(c *HostConfig) { c.UnanalyzablePolicy.TooBig = "skip" }, wantErr: true},
		{name: "empty invalid", mutate: func(c *HostConfig) { c.UnanalyzablePolicy.Empty = "error" }, wantErr: true},
		{name: "encrypted invalid", mutate: func(c *HostConfig) { c.UnanalyzablePolicy.Encrypted = "skip" }, wantErr: true},
		{name: "extract_error invalid", mutate: func(c *HostConfig) { c.UnanalyzablePolicy.ExtractError = "pass" }, wantErr: true},
		{name: "analysis_error invalid", mutate: func(c *HostConfig) { c.UnanalyzablePolicy.AnalysisError = "skip" }, wantErr: true},
		{name: "analysis_error mitigate", mutate: func(c *HostConfig) { c.UnanalyzablePolicy.AnalysisError = "mitigate" }},
		{name: "analysis_error error", mutate: func(c *HostConfig) { c.UnanalyzablePolicy.AnalysisError = "error" }},
		{name: "256 forbidden patterns", mutate: func(c *HostConfig) { c.FilepathFilter.ForbiddenPaths = patterns(256, 10) }},
		{name: "257 forbidden patterns", mutate: func(c *HostConfig) { c.FilepathFilter.ForbiddenPaths = patterns(257, 10) }, wantErr: true},
		{name: "257 skipped patterns", mutate: func(c *HostConfig) { c.FilepathFilter.SkippedPaths = patterns(257, 10) }, wantErr: true},
		{name: "move mode invalid", mutate: func(c *HostConfig) { c.Move.Mode = "link" }, wantErr: true},
		{name: "dir percent negative", mutate: func(c *HostConfig) { c.Monitoring.MaxTrackedDirPercent = -1 }, wantErr: true},
		{name: "quarantine format invalid", mutate: func(c *HostConfig) { c.Quarantine.Format = "gz" }, wantErr: true},
		{name: "dir percent above 100", mutate: func(c *HostConfig) { c.Monitoring.MaxTrackedDirPercent = 101 }, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c HostConfig
			tt.mutate(&c)
			err := errors.Join(
				validate.Struct(c.UnanalyzablePolicy),
				validate.Struct(c.FilepathFilter),
				validate.Struct(c.Monitoring),
				validate.Struct(c.Move),
				validate.Struct(c.Quarantine),
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate.Struct() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInitDefault_HostNewKeys(t *testing.T) {
	config, err := InitDefault(HostKey)
	if err != nil {
		t.Fatal(err)
	}
	c, ok := config.(*HostConfig)
	if !ok {
		t.Fatalf("InitDefault returned %T", config)
	}
	want := HostUnanalyzablePolicy{TooBig: "error", Empty: "pass", AnalysisError: "error"}
	if c.UnanalyzablePolicy != want || c.Move.Mode != "copy" {
		t.Errorf("defaults = %+v, move.mode %q; want %+v, copy", c.UnanalyzablePolicy, c.Move.Mode, want)
	}
	if c.FilepathFilter.ForbiddenPaths == nil || len(c.FilepathFilter.ForbiddenPaths) != 0 || c.FilepathFilter.SkippedPaths == nil || len(c.FilepathFilter.SkippedPaths) != 0 {
		t.Errorf("filepath_filter defaults = %+v, want two empty lists (not null)", c.FilepathFilter)
	}
	if c.WorkDir != "" || c.Monitoring.DisableFSWatcher {
		t.Errorf("other new keys must default to zero (connector default), got %+v", c)
	}
	if c.Move.MinThroughput != "10MiB" {
		t.Errorf("move.min_throughput = %q, want 10MiB", c.Move.MinThroughput)
	}
	if c.Monitoring.MaxTrackedMemory != "256MiB" || c.Monitoring.MaxTrackedDirPercent != 25 {
		t.Errorf("monitoring defaults = %q, %d; want 256MiB, 25", c.Monitoring.MaxTrackedMemory, c.Monitoring.MaxTrackedDirPercent)
	}
}

func TestGetConfigFields_HostExposesNewKeys(t *testing.T) {
	fields, err := getConfigFields(HostConfig{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]ConfigField{}
	collectFields(fields, "", got)
	for _, k := range []string{"work_dir", "unanalyzable_policy", "unanalyzable_policy.too_big", "unanalyzable_policy.extract_error", "unanalyzable_policy.analysis_error", "filepath_filter", "filepath_filter.forbidden_paths", "filepath_filter.skipped_paths", "monitoring.disable_fs_watcher", "monitoring.max_tracked_memory", "monitoring.max_tracked_dir_percent", "move.mode", "move.min_throughput"} {
		f, ok := got[k]
		if !ok || f.Description == "" {
			t.Errorf("field %q missing or without description", k)
		}
	}
}

func collectFields(fields []ConfigField, prefix string, out map[string]ConfigField) {
	for _, f := range fields {
		key := prefix + f.Key
		out[key] = f
		collectFields(f.Properties, key+".", out)
	}
}

func TestHostConfig_DescMentionsDefault(t *testing.T) {
	raw, err := InitDefault(HostKey)
	if err != nil {
		t.Fatal(err)
	}
	c, ok := raw.(*HostConfig)
	if !ok {
		t.Fatalf("InitDefault returned %T", raw)
	}
	fields, err := getConfigFields(HostConfig{})
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]ConfigField{}
	collectFields(fields, "", byKey)
	for key, def := range map[string]string{
		"unanalyzable_policy.too_big": c.UnanalyzablePolicy.TooBig,
		"unanalyzable_policy.empty":   c.UnanalyzablePolicy.Empty,
		"move.mode":                   c.Move.Mode,
		"move.min_throughput":         c.Move.MinThroughput,
	} {
		want := "'" + def + "'"
		if d := byKey[key].Description; !strings.Contains(d, want) || !strings.Contains(d, "default") {
			t.Errorf("%s description %q does not state default %s", key, d, want)
		}
	}
}

// empty: pass needs the move by copy: it is refused with the move action on in
// rename mode, and only then.
func TestHostConfig_emptyPassWithRename(t *testing.T) {
	tests := []struct {
		name    string
		empty   string
		move    bool
		mode    string
		wantErr bool
	}{
		{"pass, move on, rename", "pass", true, "rename", true},
		{"pass, move on, copy", "pass", true, "copy", false},
		{"pass, move on, no mode", "pass", true, "", false},
		{"pass, move off, rename", "pass", false, "rename", false},
		{"absent, move on, rename", "", true, "rename", false},
		{"skip, move on, rename", "skip", true, "rename", false},
		{"mitigate, move on, rename", "mitigate", true, "rename", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := DefaultValidator()
			if err != nil {
				t.Fatal(err)
			}
			base, err := InitDefault(HostKey)
			if err != nil {
				t.Fatal(err)
			}
			c, ok := base.(*HostConfig)
			if !ok {
				t.Fatalf("InitDefault returned %T", base)
			}
			c.GMalwareAPIURL, c.GMalwareAPIToken, c.Paths = "https://gmalware.example", "00000000-00000000-00000000-00000000-00000000", []string{"/in"}
			c.UnanalyzablePolicy.Empty, c.Actions.Move, c.Move.Mode = tt.empty, tt.move, tt.mode
			err = v.Validate(c)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil {
				return
			}
			var verrs validator.ValidationErrors
			if !errors.As(err, &verrs) || len(verrs) != 1 {
				t.Fatalf("error = %v, want one validation error", err)
			}
			if got := verrs[0].Translate(v.Trans); got != HostEmptyPassWithRenameMessage {
				t.Errorf("message = %q, want the connector's wording", got)
			}
		})
	}
}

// patterns is n patterns of size bytes each.
func patterns(n, size int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = padded("a", "", size)
	}
	return out
}

// padded is head, then flags that match nothing (they cost no instruction, so a
// pattern of any size stays within the bounds of the program), then tail, in size
// bytes.
func padded(head, tail string, size int) string {
	pad := size - len(head) - len(tail)
	if pad < 0 {
		return strings.Repeat(head, size-len(tail)) + tail
	}
	return head + strings.Repeat("(?s)", pad/4) + strings.Repeat(head, pad%4) + tail
}

func TestHostConfig_FilepathFilterAndAnalysisErrorDescriptions(t *testing.T) {
	fields, err := getConfigFields(HostConfig{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]ConfigField{}
	collectFields(fields, "", got)
	for key, want := range map[string][]string{
		"unanalyzable_policy.analysis_error": {"error", "mitigate", "pass", "detection"},
		"filepath_filter.forbidden_paths":    {"256", "1024", "<archive path>/<member path>"},
		"filepath_filter.skipped_paths":      {"policy-skipped", "Risk", "anchor"},
	} {
		f, ok := got[key]
		if !ok {
			t.Errorf("key %q missing", key)
			continue
		}
		for _, w := range want {
			if !strings.Contains(f.Description, w) {
				t.Errorf("description of %q = %q, want it to mention %q", key, f.Description, w)
			}
		}
	}
}

func TestHostConfig_FilepathFilterKeepsItsListsThroughPatchConfig(t *testing.T) {
	actual, err := InitDefault(HostKey)
	if err != nil {
		t.Fatal(err)
	}
	const base = `"gmalware_api_url":"http://gmalware.test","gmalware_api_token":"token","paths":["/in"],`
	patched, err := PatchConfig(HostKey, actual, []byte(`{`+base+`"filepath_filter":{"forbidden_paths":["a$"],"skipped_paths":["^/ok/"]},"unanalyzable_policy":{"analysis_error":"mitigate"}}`))
	if err != nil {
		t.Fatal(err)
	}
	c, ok := patched.(*HostConfig)
	if !ok {
		t.Fatalf("PatchConfig returned %T", patched)
	}
	if diff := cmp.Diff(HostFilepathFilter{ForbiddenPaths: []string{"a$"}, SkippedPaths: []string{"^/ok/"}}, c.FilepathFilter); diff != "" {
		t.Errorf("filepath_filter diff (-want +got) = %s", diff)
	}
	if c.UnanalyzablePolicy.AnalysisError != "mitigate" {
		t.Errorf("analysis_error = %q, want mitigate", c.UnanalyzablePolicy.AnalysisError)
	}
	if _, err = PatchConfig(HostKey, actual, []byte(`{`+base+`"filepath_filter":{"forbidden_paths":["`+strings.Repeat("a", 1025)+`"]}}`)); err == nil {
		t.Error("a pattern over 1024 bytes must be refused when the configuration is patched")
	}
	if _, err = PatchConfig(HostKey, actual, []byte(`{`+base+`"unanalyzable_policy":{"analysis_error":"skip"}}`)); err == nil {
		t.Error("analysis_error: skip must be refused")
	}
}

func TestHostConfig_FilepathPatternRules(t *testing.T) {
	v, err := DefaultValidator()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		filter    HostFilepathFilter
		wantError []string // fragments the translated messages must hold; nil: accepted
	}{
		{name: "a valid pattern in each list", filter: HostFilepathFilter{ForbiddenPaths: []string{`\.exe$`}, SkippedPaths: []string{`^/srv/in/trusted/`}}},
		{name: "no pattern", filter: HostFilepathFilter{}},
		{name: "two spaces match only a path with two spaces", filter: HostFilepathFilter{SkippedPaths: []string{"  "}}},
		{name: "an empty forbidden pattern", filter: HostFilepathFilter{ForbiddenPaths: []string{"a", ""}}, wantError: []string{"filepath_filter.forbidden_paths[1]", "empty"}},
		{name: "an empty skipped pattern delivers everything", filter: HostFilepathFilter{SkippedPaths: []string{""}}, wantError: []string{"filepath_filter.skipped_paths[0]", "empty"}},
		{name: "an unbalanced group", filter: HostFilepathFilter{SkippedPaths: []string{"ok", "("}}, wantError: []string{"filepath_filter.skipped_paths[1]", "not a valid regular expression"}},
		{name: "a bracket never closed", filter: HostFilepathFilter{ForbiddenPaths: []string{"(["}}, wantError: []string{"forbidden_paths[0]", "not a valid regular expression"}},
		{name: "a repeat of 100", filter: HostFilepathFilter{ForbiddenPaths: []string{`a{100}b`}}},
		{name: "a repeat of 101", filter: HostFilepathFilter{ForbiddenPaths: []string{`a{101}b`}}, wantError: []string{"forbidden_paths[0]", "repeats 101 times, at most 100"}},
		{name: "a range that ends at 101", filter: HostFilepathFilter{SkippedPaths: []string{`ok`, `a{1,101}`}}, wantError: []string{"skipped_paths[1]", "repeats 101 times"}},
		{name: "a repeat of 101 nested in a group", filter: HostFilepathFilter{SkippedPaths: []string{`(?:x(?:a{101})y)+z`}}, wantError: []string{"repeats 101 times"}},
		{name: "an estimate under 512 instructions", filter: HostFilepathFilter{ForbiddenPaths: []string{`(?:a{50}){5}`}}},
		{name: "an estimate over 512 instructions", filter: HostFilepathFilter{ForbiddenPaths: []string{`(?:a{50}){10}`}}, wantError: []string{"forbidden_paths[0]", "estimated at 526 instructions, at most 512"}},
		{name: "written-out optional characters", filter: HostFilepathFilter{SkippedPaths: []string{strings.Repeat("a?", 500) + "b"}}, wantError: []string{"skipped_paths[0]", "at most 512"}},
		{name: "classes of Unicode letters", filter: HostFilepathFilter{ForbiddenPaths: []string{`\pL`, `\pL`, `\pL`, `\pL`, `\pL`, `\pL`, `\pL`}}, wantError: []string{"more than 4096 ranges in all"}},
		{name: "1024 ASCII bytes", filter: HostFilepathFilter{SkippedPaths: patterns(1, 1024)}},
		{name: "1025 ASCII bytes", filter: HostFilepathFilter{ForbiddenPaths: patterns(1, 1025)}, wantError: []string{"forbidden_paths[0]", "1025 bytes, at most 1024"}},
		{name: "1024 bytes of two-byte runes and flags", filter: HostFilepathFilter{SkippedPaths: []string{strings.Repeat("(?s)", 200) + strings.Repeat("é", 112)}}},
		{name: "513 two-byte runes are 1026 bytes", filter: HostFilepathFilter{SkippedPaths: []string{strings.Repeat("é", 513)}}, wantError: []string{"skipped_paths[0]", "1026 bytes"}},
		{name: "a pattern in both lists", filter: HostFilepathFilter{ForbiddenPaths: []string{"x", "same"}, SkippedPaths: []string{"y", "same"}}, wantError: []string{"filepath_filter.skipped_paths[1]", "also in forbidden_paths"}},
		{name: "257 patterns", filter: HostFilepathFilter{SkippedPaths: patterns(257, 3)}, wantError: []string{"skipped_paths"}},
	}
	for _, tt := range tests {
		for scope, value := range map[string]any{"section": tt.filter, "whole configuration": filterInConfig(tt.filter)} {
			t.Run(tt.name+"/"+scope, func(t *testing.T) {
				err := v.Validate(value)
				if tt.wantError == nil {
					if err != nil {
						t.Fatalf("Validate() error = %v, want it accepted", err)
					}
					return
				}
				var verrs validator.ValidationErrors
				if !errors.As(err, &verrs) {
					t.Fatalf("Validate() error = %v, want validation errors", err)
				}
				var msgs []string
				for _, fe := range verrs {
					if scope == "whole configuration" && !strings.Contains(fe.Namespace(), "filepath_filter") {
						continue
					}
					msgs = append(msgs, fe.Translate(v.Trans))
				}
				all := strings.Join(msgs, " | ")
				for _, frag := range tt.wantError {
					if !strings.Contains(all, frag) {
						t.Errorf("messages = %q, want one holding %q", all, frag)
					}
				}
			})
		}
	}
}

// filterInConfig is a host configuration valid but for its filepath_filter.
func filterInConfig(f HostFilepathFilter) HostConfig {
	c := HostConfig{Paths: []string{"/in"}, Workers: 1, ExtractWorkers: 1, FilepathFilter: f}
	c.GMalwareAPIURL, c.GMalwareAPIToken = "https://gmalware.example", "token"
	return c
}

// A refused pattern is echoed for at most 64 bytes, cut on a character boundary.
func TestHostConfig_FilepathPatternMessageEchoesLittle(t *testing.T) {
	v, err := DefaultValidator()
	if err != nil {
		t.Fatal(err)
	}
	huge := "(" + strings.Repeat("é", 400)
	err = v.Validate(HostFilepathFilter{ForbiddenPaths: []string{huge}})
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) || len(verrs) != 1 {
		t.Fatalf("Validate() error = %v, want one validation error", err)
	}
	msg := verrs[0].Translate(v.Trans)
	if strings.Count(msg, "é") > 32 || !utf8.ValidString(msg) {
		t.Errorf("message echoes %d characters, valid UTF-8 %t: want at most 64 bytes of the pattern, cut on a character", strings.Count(msg, "é"), utf8.ValidString(msg))
	}
	both := HostFilepathFilter{ForbiddenPaths: []string{huge[:200]}, SkippedPaths: []string{huge[:200]}}
	if err = v.Validate(both); err == nil {
		t.Error("an invalid pattern in both lists must be refused")
	}
}

// distinctPatterns is n different patterns of size bytes each.
func distinctPatterns(n, size int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = padded("a", fmt.Sprintf("%06d", i), size)
	}
	return out
}

// otherPatterns is the patterns of distinctPatterns with another letter: none of
// them is in the first lot.
func otherPatterns(n, size int) []string {
	out := distinctPatterns(n, size)
	for i, p := range out {
		out[i] = strings.ReplaceAll(p, "a", "c")
	}
	return out
}

// countParses replaces the parser of the patterns by one that counts its calls.
func countParses(t *testing.T) *int {
	t.Helper()
	prev := parseRegexp
	n := 0
	parseRegexp = func(s string, flags syntax.Flags) (*syntax.Regexp, error) {
		n++
		return prev(s, flags)
	}
	t.Cleanup(func() { parseRegexp = prev })
	return &n
}

func TestHostConfig_FilepathPatternBudgetIsCheckedBeforeAnyParsing(t *testing.T) {
	v, err := DefaultValidator()
	if err != nil {
		t.Fatal(err)
	}
	half := func(extra string) HostFilepathFilter {
		f := HostFilepathFilter{ForbiddenPaths: distinctPatterns(16, 1024), SkippedPaths: otherPatterns(15, 1024)}
		f.SkippedPaths = append(f.SkippedPaths, padded("b", extra, 1024))
		return f
	}
	tests := []struct {
		name       string
		filter     HostFilepathFilter
		wantErr    string
		wantParses int
	}{
		{name: "32 KiB exactly", filter: half("b"), wantParses: 32},
		{name: "32 KiB and one byte", filter: HostFilepathFilter{ForbiddenPaths: distinctPatterns(16, 1024), SkippedPaths: append(otherPatterns(15, 1024), padded("b", "", 1024), "d")}, wantErr: "32768 in all"},
		{name: "257 entries", filter: HostFilepathFilter{SkippedPaths: distinctPatterns(257, 6)}, wantErr: "257 patterns, at most 256"},
		{name: "20000 entries", filter: HostFilepathFilter{ForbiddenPaths: distinctPatterns(20_000, 6)}, wantErr: "20000 patterns, at most 256"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parses := countParses(t)
			err := v.Validate(tt.filter)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want it accepted", err)
				}
			} else {
				var verrs validator.ValidationErrors
				if !errors.As(err, &verrs) {
					t.Fatalf("Validate() error = %v, want a refusal", err)
				}
				found := false
				for _, fe := range verrs {
					found = found || strings.Contains(fe.Translate(v.Trans), tt.wantErr)
				}
				if !found {
					t.Errorf("Validate() error = %v, want a message holding %q", err, tt.wantErr)
				}
			}
			if *parses != tt.wantParses {
				t.Errorf("%d patterns parsed, want %d: a refused budget is refused before any parsing", *parses, tt.wantParses)
			}
		})
	}
}

func TestCheckFilepathFilter_stopsAtTheFirstProblemInTheOrderOfTheChecks(t *testing.T) {
	tests := []struct {
		name   string
		filter HostFilepathFilter
		want   string
	}{
		{"count before size", HostFilepathFilter{ForbiddenPaths: distinctPatterns(257, 200), SkippedPaths: []string{""}}, "257 patterns"},
		{"size before the content of the patterns", HostFilepathFilter{ForbiddenPaths: append(distinctPatterns(32, 1024), ""), SkippedPaths: []string{"("}}, "in all"},
		{"an empty pattern before a bad syntax", HostFilepathFilter{ForbiddenPaths: []string{"(", ""}}, "forbidden_paths[1]"},
		{"overlap before a bad syntax", HostFilepathFilter{ForbiddenPaths: []string{"x", "("}, SkippedPaths: []string{"x"}}, "also in forbidden_paths"},
		{"a bad syntax last", HostFilepathFilter{SkippedPaths: []string{"ok", "(["}}, "skipped_paths[1]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := checkFilepathFilter(tt.filter)
			if p == nil || !strings.Contains(p.field+" "+p.reason, tt.want) {
				t.Errorf("checkFilepathFilter() = %+v, want the first problem to hold %q", p, tt.want)
			}
		})
	}
}

// worstCasePatterns is 32 KiB of patterns whose parsing is the dearest: a class
// of many Unicode categories, 1 KiB each.
func worstCasePatterns() []string {
	class := `\p{L}\p{Lu}\p{Ll}\p{Lt}\p{Lm}\p{Lo}\p{M}\p{Mn}\p{Mc}\p{Me}\p{N}\p{Nd}\p{Nl}\p{No}\p{P}\p{S}\p{Z}\p{C}`
	out := make([]string, 32)
	for i := range out {
		p := ""
		for len(p)+len(class)+2 <= 1020 {
			p += "[" + class + "]"
		}
		// Filled to 1 KiB, so that the 32 patterns are exactly 32 KiB.
		out[i] = p + strings.Repeat("a", 1020-len(p)) + fmt.Sprintf("%04d", i)
	}
	return out
}

// The work is asserted, not the clock: the number of patterns parsed. The timings
// are in the two benchmarks below.
func TestHostConfig_FilepathPatternWorkIsBounded(t *testing.T) {
	v, err := DefaultValidator()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		filter     HostFilepathFilter
		wantErr    bool
		wantParses int
	}{
		{"32 KiB of Unicode classes: each pattern parsed once", HostFilepathFilter{SkippedPaths: worstCasePatterns()}, false, 32},
		{"257 entries: nothing parsed", HostFilepathFilter{SkippedPaths: distinctPatterns(257, 6)}, true, 0},
		{"20000 entries: nothing parsed", HostFilepathFilter{ForbiddenPaths: distinctPatterns(20_000, 6)}, true, 0},
		{"over 32 KiB: nothing parsed", HostFilepathFilter{ForbiddenPaths: append(worstCasePatterns(), strings.Repeat("a", 1024))}, true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parses := countParses(t)
			err := v.Validate(tt.filter)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %t", err, tt.wantErr)
			}
			if *parses != tt.wantParses {
				t.Errorf("%d patterns parsed, want %d", *parses, tt.wantParses)
			}
		})
	}
}

func TestHostConfig_FilepathBudgetMessageNamesNoSingleList(t *testing.T) {
	v, err := DefaultValidator()
	if err != nil {
		t.Fatal(err)
	}
	f := HostFilepathFilter{ForbiddenPaths: distinctPatterns(1, 1024), SkippedPaths: otherPatterns(32, 1024)}
	var verrs validator.ValidationErrors
	if err = v.Validate(f); !errors.As(err, &verrs) || len(verrs) != 1 {
		t.Fatalf("Validate() error = %v, want one refusal", err)
	}
	msg := verrs[0].Translate(v.Trans)
	if strings.Contains(msg, "forbidden_paths") || !strings.Contains(msg, "both lists") {
		t.Errorf("message = %q, want the total named, not one list", msg)
	}
}

func BenchmarkHostConfig_FilepathPatternWorstCase(b *testing.B) {
	f := HostFilepathFilter{SkippedPaths: worstCasePatterns()}
	b.ReportAllocs()
	for b.Loop() {
		if p := checkFilepathFilter(f); p != nil {
			b.Fatalf("worst case refused: %+v", p)
		}
	}
}

func BenchmarkHostConfig_FilepathPatternTooManyEntries(b *testing.B) {
	f := HostFilepathFilter{ForbiddenPaths: distinctPatterns(20_000, 6)}
	b.ReportAllocs()
	for b.Loop() {
		if checkFilepathFilter(f) == nil {
			b.Fatal("20000 entries accepted")
		}
	}
}
