package validation

import (
	"errors"
	"slices"
	"testing"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
)

func Test_NewEnumValidation(t *testing.T) {
	type args struct {
		values []string
		field  string
	}
	tests := []struct {
		name        string
		args        args
		wantAllowed []string
		want        bool
	}{
		{
			name: "ko value not in enum",
			args: args{
				values: []string{"a", "b", "c"},
				field:  "z",
			},
			wantAllowed: []string{"a", "b", "c"},
		},
		{
			name: "ko empty field against non-empty enum",
			args: args{
				values: []string{"a", "b"},
				field:  "",
			},
			wantAllowed: []string{"a", "b"},
		},
		{
			name: "ko any field against empty enum",
			args: args{
				values: []string{},
				field:  "a",
			},
			wantAllowed: []string{},
		},
		{
			name: "ok value in enum",
			args: args{
				values: []string{"a", "b", "c"},
				field:  "b",
			},
			wantAllowed: []string{"a", "b", "c"},
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := NewEnumValidation(tt.args.values)

			if diff := slices.Compare(ev.Allowed, tt.wantAllowed); diff != 0 {
				t.Errorf("NewEnumValidation() allowed = %v, want %v", ev.Allowed, tt.wantAllowed)
			}

			validate := validator.New()
			err := validate.RegisterValidation(tt.name, ev.Fn)
			if err != nil {
				t.Fatalf("RegisterValidation() error = %v", err)
			}

			got := validate.Var(tt.args.field, tt.name) == nil
			if got != tt.want {
				t.Errorf("NewEnumValidation() fn valid = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_RegisterEnumTranslation(t *testing.T) {
	type args struct {
		tag     string
		allowed []string
	}
	tests := []struct {
		name string
		args args
		// alreadyRegistered registers the tag once before the call under test,
		// so the second registration conflicts.
		alreadyRegistered bool
		wantMessage       string
		wantErr           bool
	}{
		{
			name: "ko tag already registered",
			args: args{
				tag:     "my_enum",
				allowed: []string{"a", "b"},
			},
			alreadyRegistered: true,
			wantErr:           true,
		},
		{
			name: "ok single allowed value",
			args: args{
				tag:     "my_enum",
				allowed: []string{"a"},
			},
			wantMessage: "Field must be one of [a]",
		},
		{
			name: "ok multiple allowed values",
			args: args{
				tag:     "my_enum",
				allowed: []string{"a", "b", "c"},
			},
			wantMessage: "Field must be one of [a b c]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enLocale := en.New()
			uni := ut.New(enLocale, enLocale)
			trans, _ := uni.GetTranslator("en")

			validate := validator.New()
			err := validate.RegisterValidation(tt.args.tag, func(fl validator.FieldLevel) bool {
				return false
			})
			if err != nil {
				t.Fatalf("RegisterValidation() error = %v", err)
			}

			if tt.alreadyRegistered {
				err = RegisterEnumTranslation(validate, trans, tt.args.tag, tt.args.allowed)
				if err != nil {
					t.Fatalf("RegisterEnumTranslation() setup error = %v", err)
				}
			}

			err = RegisterEnumTranslation(validate, trans, tt.args.tag, tt.args.allowed)

			if (err != nil) != tt.wantErr {
				t.Errorf("RegisterEnumTranslation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			// Validate a named struct field so the {0} placeholder resolves to a field name.
			type sample struct {
				Field string `validate:"my_enum"`
			}
			validationErr := validate.Struct(sample{Field: "x"})
			var validationErrs validator.ValidationErrors
			if !errors.As(validationErr, &validationErrs) {
				t.Fatalf("expected validator.ValidationErrors, got %T", validationErr)
			}
			got := validationErrs[0].Translate(trans)
			if got != tt.wantMessage {
				t.Errorf("translated message = %q, want %q", got, tt.wantMessage)
			}
		})
	}
}
