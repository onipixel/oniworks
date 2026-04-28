package validation_test

import (
	"testing"

	"github.com/onipixel/oniworks/framework/validation"
)

type RegisterInput struct {
	Name     string `validate:"required,min=2,max=50"`
	Email    string `validate:"required,email"`
	Password string `validate:"required,min=8"`
	Age      int    `validate:"min=18"`
}

func TestValidateStructPass(t *testing.T) {
	v := validation.New()
	in := &RegisterInput{
		Name:     "Alice",
		Email:    "alice@example.com",
		Password: "securepassword",
		Age:      25,
	}
	if err := v.Validate(in); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateRequired(t *testing.T) {
	v := validation.New()
	in := &RegisterInput{Email: "alice@example.com", Password: "password1"}
	err := v.Validate(in)
	if err == nil {
		t.Fatal("expected validation error for missing Name")
	}
	errs, ok := err.(validation.Errors)
	if !ok {
		t.Fatalf("expected validation.Errors, got %T", err)
	}
	// Validator uses json tag name or lowercase field name as key
	if _, has := errs["name"]; !has {
		t.Errorf("expected error for 'name' field, got keys: %v", errs)
	}
}

func TestValidateEmail(t *testing.T) {
	v := validation.New()
	in := &struct {
		Email string `json:"email" validate:"required,email"`
	}{Email: "not-an-email"}
	err := v.Validate(in)
	if err == nil {
		t.Fatal("expected email validation error")
	}
	errs := err.(validation.Errors)
	if _, has := errs["email"]; !has {
		t.Errorf("expected email error, got keys: %v", errs)
	}
}

func TestValidateMinMax(t *testing.T) {
	v := validation.New()
	in := &struct {
		Name string `validate:"min=5,max=10"`
	}{Name: "Hi"}
	err := v.Validate(in)
	if err == nil {
		t.Fatal("expected min length error")
	}
}

func TestValidateMap(t *testing.T) {
	v := validation.New()
	data := map[string]any{
		"email": "bad-email",
		"name":  "",
	}
	rules := validation.Rules{
		"email": "required,email",
		"name":  "required",
	}
	errs := v.ValidateMap(data, rules)
	if !errs.Any() {
		t.Fatal("expected validation errors")
	}
	if _, has := errs["email"]; !has {
		t.Error("expected email error")
	}
	if _, has := errs["name"]; !has {
		t.Error("expected name required error")
	}
}

func TestCustomRule(t *testing.T) {
	v := validation.New()
	v.Register("startswith", func(value any, param string) string {
		s, ok := value.(string)
		if !ok || len(s) == 0 || string(s[0]) != param {
			return "must start with " + param
		}
		return ""
	})

	// Use = for param separator (validator uses = syntax)
	in := &struct {
		Code string `validate:"startswith=X"`
	}{Code: "ABC"}
	err := v.Validate(in)
	if err == nil {
		t.Fatal("expected custom rule error")
	}

	in.Code = "XYZ"
	if err := v.Validate(in); err != nil {
		t.Errorf("expected pass for XYZ: %v", err)
	}
}

func TestValidateMultipleErrors(t *testing.T) {
	v := validation.New()
	in := &RegisterInput{} // all zero values
	err := v.Validate(in)
	if err == nil {
		t.Fatal("expected multiple validation errors")
	}
	errs := err.(validation.Errors)
	if len(errs) < 2 {
		t.Errorf("expected at least 2 field errors, got %d", len(errs))
	}
}
