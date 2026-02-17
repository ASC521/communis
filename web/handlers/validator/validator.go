package validator

import (
	"slices"
	"strings"
	"unicode/utf8"
)

type Validator struct {
	FieldErrors map[string]string
}

func (v *Validator) Valid() bool {
	return len(v.FieldErrors) == 0
}

func (v *Validator) AddFieldError(key, message string) {
	if v.FieldErrors == nil {
		v.FieldErrors = map[string]string{}
	}

	if _, exists := v.FieldErrors[key]; !exists {
		v.FieldErrors[key] = message
	}
}

func (v *Validator) CheckField(ok bool, key, message string) {
	if !ok {
		v.AddFieldError(key, message)
	}
}

func NotBlank(v string) bool {
	return strings.TrimSpace(v) != ""
}

func MaxChars(v string, n int) bool {
	return utf8.RuneCountInString(v) <= n
}

func MinChars(v string, n int) bool {
	return utf8.RuneCountInString(v) >= n
}

func PermittedValue[T comparable](v T, permittedValues ...T) bool {
	return slices.Contains(permittedValues, v)
}

func Equals[T comparable](v, o T) bool {
	return v == o
}
