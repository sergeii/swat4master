package params

import (
	"fmt"
	"reflect"
	"strconv"
)

func Unmarshal(params map[string]string, v any) error {
	rv := reflect.ValueOf(v)
	if err := validatePointer(rv); err != nil {
		return err
	}
	return unmarshal(params, rv.Elem())
}

func unmarshal(params map[string]string, sv reflect.Value) error {
	st := reflect.TypeOf(sv.Interface())
	for i := range st.NumField() {
		field := st.Field(i)
		// ignore private fields
		if !field.IsExported() {
			continue
		}
		paramName, exists := GetParamName(field)
		// certain public fields may be ignored
		if !exists {
			continue
		}
		paramValue, ok := params[paramName]
		if !ok {
			continue
		}
		if err := setStructValue(sv, field, paramValue); err != nil {
			return err
		}
	}
	return nil
}

func setStructValue(sv reflect.Value, field reflect.StructField, value string) error {
	kind := field.Type.Kind()
	switch kind { // nolint: exhaustive
	case reflect.Int:
		intValue, parsed := parseIntValue(value)
		if !parsed {
			return fmt.Errorf("invalid value '%v' for integer field '%s'", value, field.Name)
		}
		sv.FieldByName(field.Name).SetInt(intValue)
	case reflect.Bool:
		boolValue, parsed := parseBoolValue(value)
		if !parsed {
			return fmt.Errorf("invalid value '%v' for boolean field '%s'", value, field.Name)
		}
		sv.FieldByName(field.Name).SetBool(boolValue)
	case reflect.String:
		sv.FieldByName(field.Name).SetString(value)
	default:
		return fmt.Errorf("unsupported type '%s' for field '%s'", kind, field.Name)
	}
	return nil
}

func parseIntValue(maybeInt string) (int64, bool) {
	intValue, err := strconv.ParseInt(maybeInt, 10, 0)
	if err != nil {
		return 0, false
	}
	return intValue, true
}

func parseBoolValue(maybeBool string) (bool, bool) {
	switch maybeBool {
	case "1", "true":
		return true, true
	case "0", "false":
		return false, true
	default:
		return false, false
	}
}
