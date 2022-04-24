package params

import (
	"reflect"
	"strconv"
)

func Unmarshal(params map[string]string, v any) error {
	rv := reflect.ValueOf(v)
	if err := validatePointer(rv); err != nil {
		return err
	}
	if err := unmarshal(params, rv.Elem()); err != nil {
		return err
	}

	return nil
}

func unmarshal(params map[string]string, sv reflect.Value) error {
	st := reflect.TypeOf(sv.Interface())
	for i := 0; i < st.NumField(); i++ {
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
	switch field.Type.Kind() { // nolint: exhaustive
	case reflect.Int:
		intValue, err := parseIntValue(value)
		if err != nil {
			return err
		}
		sv.FieldByName(field.Name).SetInt(intValue)
	case reflect.Bool:
		boolValue, err := parseBoolValue(value)
		if err != nil {
			return err
		}
		sv.FieldByName(field.Name).SetBool(boolValue)
	case reflect.String:
		sv.FieldByName(field.Name).SetString(value)
	default:
		return ErrUnknownType
	}
	return nil
}

func parseIntValue(maybeInt string) (int64, error) {
	intValue, err := strconv.ParseInt(maybeInt, 10, 0)
	if err != nil {
		return 0, ErrInvalidValue
	}
	return intValue, nil
}

func parseBoolValue(maybeBool string) (bool, error) {
	switch maybeBool {
	case "1", "true":
		return true, nil
	case "0", "false":
		return false, nil
	default:
		return false, ErrInvalidValue
	}
}
