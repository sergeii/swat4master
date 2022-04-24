package params

import (
	"reflect"
	"strings"
)

func validatePointer(rv reflect.Value) error {
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return ErrValueMustBePointer
	}
	return nil
}

func GetParamName(field reflect.StructField) (string, bool) {
	tag, exists := field.Tag.Lookup("param")
	if exists {
		// don't look into this field
		if tag == "-" {
			return "", false
		}
		return tag, true
	}
	return strings.ToLower(field.Name), true
}
