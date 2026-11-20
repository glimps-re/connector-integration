package sdk

import (
	"github.com/go-playground/validator/v10"

	ut "github.com/go-playground/universal-translator"
)

// emptyPassRenameTag is the tag of the combination of an empty-file policy and a
// move mode the host connector refuses.
const emptyPassRenameTag = "empty_pass_with_rename"

// HostEmptyPassWithRenameMessage is what the host connector says of the
// combination; the console says the same when the configuration is saved.
const HostEmptyPassWithRenameMessage = "unanalyzable_policy.empty: pass delivers an empty file as a new empty file, which needs move.mode copy: it is refused with rename while the move action is on (leave the key out to skip empty files in that mode)"

// registerHostConfigValidation adds to validate the rules that bind several
// keys of the host configuration. An empty policy of pass is refused with the
// move action on and its mode rename: the empty file is made new, which only the
// move by copy does. The policy left out with rename is the connector's to
// settle (it skips), and with the move action off the mode means nothing.
func registerHostConfigValidation(validate *validator.Validate, trans ut.Translator) error {
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
