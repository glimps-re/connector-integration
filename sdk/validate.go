package sdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
	"github.com/labstack/echo/v4"
)

// StrictJSONSerializer implements JSON encoding using encoding/json with DisallowUnknownFields
type StrictJSONSerializer struct{}

// Serialize converts an interface into a json and writes it to the response.
// You can optionally use the indent parameter to produce pretty JSONs.
func (d StrictJSONSerializer) Serialize(c echo.Context, i any, indent string) error {
	enc := json.NewEncoder(c.Response())
	if indent != "" {
		enc.SetIndent("", indent)
	}
	return enc.Encode(i)
}

// Deserialize reads a JSON from a request body and converts it into an interface.
func (d StrictJSONSerializer) Deserialize(c echo.Context, i any) (err error) {
	dec := json.NewDecoder(c.Request().Body)
	dec.DisallowUnknownFields()
	err = dec.Decode(i)
	if err != nil {
		return newValidationError(err)
	}
	return
}

// Return a default echo.Validator with basic validation and translation
func DefaultValidator() (v *defaultValidator, err error) {
	en := en.New()
	uni := ut.New(en, en)
	trans, _ := uni.GetTranslator("en")
	validate := validator.New()
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
	err = validate.RegisterValidation("connector_type", ValidateConnectorType)
	if err != nil {
		return
	}
	err = en_translations.RegisterDefaultTranslations(validate, trans)
	if err != nil {
		return
	}
	v = &defaultValidator{Validator: validate, Trans: trans}
	return
}

var validConnectorTypes = map[string]bool{
	DummyKey:      true,
	SharepointKey: true,
	ICAPKey:       true,
	M365Key:       true,
	HostKey:       true,
}

func ValidateConnectorType(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	valid, ok := validConnectorTypes[value]
	return valid && ok
}

type defaultValidator struct {
	Trans     ut.Translator
	Validator *validator.Validate
}

func (v *defaultValidator) Validate(i any) error {
	if err := v.Validator.Struct(i); err != nil {
		return err
	}
	return nil
}

func BindAndValidate(c echo.Context, model any) (err error) {
	// Bind check params in given order: route, request (get and delete only), body
	if err = c.Bind(model); err != nil {
		return newValidationError(err)
	}

	if err = c.Validate(model); err != nil {
		return newValidationError(err)
	}
	return nil
}

func BindAndValidateConfig(connectorType string, payload json.RawMessage) (config any, err error) {
	config, err = InitDefault(connectorType)
	if err != nil {
		return
	}
	err = BindAndValidateRaw(config, payload)
	if err != nil {
		return
	}
	return
}

func BindRaw(model any, raw []byte) (err error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	err = dec.Decode(model)
	if err != nil {
		return newValidationError(err)
	}
	return
}

// Can be used to validate raw data. Useful to validate embed json.RawMessage for instance.
func BindAndValidateRaw(model any, raw []byte) (err error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	err = dec.Decode(model)
	if err != nil {
		return newValidationError(err)
	}
	v, err := DefaultValidator()
	if err != nil {
		return newValidationError(err)
	}
	if err := v.Validate(model); err != nil {
		return newValidationError(err)
	}
	return
}

func newValidationError(err error) (newError ValidationError) {
	reg := regexp.MustCompile("json: unknown field \"(.*)\"")
	field := reg.FindStringSubmatch(err.Error())
	switch {
	case len(field) > 0:
		newError.Details = []echo.Map{{field[1]: "unknown field"}}
	default:
		newError.Err = err
	}
	return
}

// MUST BE used for validation error. Will be return as ResponseError with a http code 400 (bad request)
type ValidationError struct {
	Details []echo.Map `json:"details"`
	Err     error      `json:"error"`
}

func (e ValidationError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	if e.Details != nil {
		return fmt.Sprint(e.Details)
	}
	return "ValidationError"
}

func (e ValidationError) Unwrap() error {
	return e.Err
}
