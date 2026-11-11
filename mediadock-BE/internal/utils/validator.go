package utils

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Validator wraps go-playground/validator for request DTO validation.
type Validator struct {
	engine *validator.Validate
}

// NewValidator initializes a reusable Validator instance.
func NewValidator() *Validator {
	return &Validator{
		engine: validator.New(validator.WithRequiredStructEnabled()),
	}
}

// Validate validates a struct and returns a map of field-level validation messages.
func (v *Validator) Validate(payload interface{}) map[string]string {
	err := v.engine.Struct(payload)
	if err == nil {
		return nil
	}

	var valErrs validator.ValidationErrors
	if !errors.As(err, &valErrs) {
		return map[string]string{"_error": "invalid request payload"}
	}

	out := make(map[string]string, len(valErrs))
	for _, fe := range valErrs {
		field := strings.ToLower(fe.Field())
		out[field] = formatFieldError(fe)
	}
	return out
}

func formatFieldError(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return fmt.Sprintf("must be at least %s characters", fe.Param())
	case "max":
		return fmt.Sprintf("must be at most %s characters", fe.Param())
	case "url":
		return "must be a valid URL"
	case "oneof":
		return fmt.Sprintf("must be one of: %s", fe.Param())
	default:
		return fmt.Sprintf("failed validation rule %q", fe.Tag())
	}
}

