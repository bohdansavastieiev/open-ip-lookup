package config

import "fmt"

type Code string

const (
	CodeRequired    Code = "required"
	CodeWhitespace  Code = "whitespace"
	CodeInvalid     Code = "invalid"
	CodeRange       Code = "range"
	CodeDuplicate   Code = "duplicate"
	CodeUnsupported Code = "unsupported"
)

type FieldError struct {
	Field  string
	Code   Code
	Detail string
	Value  any
}

func (e *FieldError) Error() string {
	if e == nil {
		return "<nil>"
	}

	if e.Detail == "" {
		return fmt.Sprintf("field %q failed %s validation", e.Field, e.Code)
	}

	return fmt.Sprintf("field %q failed %s validation: %s", e.Field, e.Code, e.Detail)
}

func New(field string, code Code, detail string, value any) *FieldError {
	return &FieldError{
		Field:  field,
		Code:   code,
		Detail: detail,
		Value:  value,
	}
}
