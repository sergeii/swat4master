package validators

import (
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
)

func ValidateRatio(fl validator.FieldLevel) bool {
	value := fl.Field().String()

	// don't validate empty value
	if value == "" {
		return true
	}

	left, right, ok := strings.Cut(value, "/")
	if !ok {
		return false
	}

	return isNonNegativeNumber(left) && isNonNegativeNumber(right)
}

func isNonNegativeNumber(maybeNumber string) bool {
	number, err := strconv.Atoi(maybeNumber)
	if err != nil {
		return false
	}
	return number >= 0
}
