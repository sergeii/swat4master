package params

import (
	"fmt"
	"reflect"
	"strconv"
)

func Marshal(v any) (map[string]string, error) {
	rv := reflect.ValueOf(v)
	if err := validatePointer(rv); err != nil {
		return nil, err
	}
	return marshal(rv.Elem())
}

func marshal(sv reflect.Value) (map[string]string, error) {
	params := make(map[string]string)
	st := reflect.TypeOf(sv.Interface())
	for i := range st.NumField() {
		field := st.Field(i)
		// ignore private fields
		if !field.IsExported() {
			continue
		}
		paramName, ok := GetParamName(field)
		// certain public fields may be ignored
		if !ok {
			continue
		}
		fieldValue := sv.FieldByName(field.Name)
		paramValue, err := getParamValue(field, fieldValue)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal value '%v' for field '%s' (%w)", fieldValue, field.Name, err)
		}
		params[paramName] = paramValue
	}
	return params, nil
}

func getParamValue(field reflect.StructField, value reflect.Value) (string, error) {
	kind := field.Type.Kind()
	switch kind { // nolint: exhaustive
	case reflect.Int:
		return getIntValue(value), nil
	case reflect.Bool:
		return getBoolValue(value), nil
	case reflect.String:
		return getStringValue(value), nil
	default:
		return "", fmt.Errorf("unknown field type '%s'", kind)
	}
}

func getIntValue(value reflect.Value) string {
	return strconv.FormatInt(value.Int(), 10)
}

func getBoolValue(value reflect.Value) string {
	switch value.Bool() {
	case true:
		return "1"
	default:
		return "0"
	}
}

func getStringValue(value reflect.Value) string {
	return value.String()
}
