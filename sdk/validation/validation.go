// Package validation provides reusable helpers to build custom
// validations for github.com/go-playground/validator.
package validation

import (
	"fmt"
	"slices"
	"strings"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
)

// EnumValidation couples a tag's validation function with its allowed value set,
// used both to validate a field and to build a readable translation message.
type EnumValidation struct {
	Fn      validator.Func
	Allowed []string
}

// NewEnumValidation builds an EnumValidation accepting only values present in the
// given enum value set. The field is matched on its string representation.
func NewEnumValidation[T ~string](values []T) EnumValidation {
	allowed := make([]string, len(values))
	for i, v := range values {
		allowed[i] = string(v)
	}
	return EnumValidation{
		Fn: func(fl validator.FieldLevel) bool {
			return slices.Contains(allowed, fl.Field().String())
		},
		Allowed: allowed,
	}
}

// RegisterEnumTranslation registers an English message listing the allowed
// values for an enum tag, e.g. "status must be one of [a b c]".
func RegisterEnumTranslation(validate *validator.Validate, trans ut.Translator, tag string, allowed []string) (err error) {
	message := fmt.Sprintf("{0} must be one of [%s]", strings.Join(allowed, " "))
	registerFn := func(ut ut.Translator) error {
		return ut.Add(tag, message, false)
	}
	translationFn := func(ut ut.Translator, fe validator.FieldError) string {
		t, transErr := ut.T(tag, fe.Field())
		if transErr != nil {
			return fe.(error).Error()
		}
		return t
	}
	err = validate.RegisterTranslation(tag, trans, registerFn, translationFn)
	return
}

// Register registers every validation and its translation on the given
// validator, so structs using these tags can be validated by any validator.
// trans may be nil to skip translation registration.
func Register(validate *validator.Validate, trans ut.Translator, validations map[string]EnumValidation) (err error) {
	for tag, ev := range validations {
		err = validate.RegisterValidation(tag, ev.Fn)
		if err != nil {
			return
		}
		if trans == nil {
			continue
		}
		err = RegisterEnumTranslation(validate, trans, tag, ev.Allowed)
		if err != nil {
			return
		}
	}
	return
}
