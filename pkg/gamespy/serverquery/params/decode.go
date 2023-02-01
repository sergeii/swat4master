package params

import (
	"reflect"
	"strconv"

	"github.com/rs/zerolog/log"
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
	for i := 0; i < st.NumField(); i++ {
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
			log.Error().
				Err(err).Str("field", field.Name).Stringer("value", fieldValue).
				Msg("Unable to marshal field value")
			return nil, err
		}
		params[paramName] = paramValue
	}
	return params, nil
}

func getParamValue(field reflect.StructField, value reflect.Value) (string, error) {
	switch field.Type.Kind() { // nolint: exhaustive
	case reflect.Int:
		return getIntValue(value)
	case reflect.Bool:
		return getBoolValue(value)
	case reflect.String:
		return getStringValue(value)
	default:
		return "", ErrUnknownType
	}
}

func getIntValue(value reflect.Value) (string, error) {
	if !value.CanInt() {
		return "", ErrInvalidValue
	}
	return strconv.FormatInt(value.Int(), 10), nil
}

func getBoolValue(value reflect.Value) (string, error) {
	if value.Bool() {
		return "1", nil
	}
	return "0", nil
}

func getStringValue(value reflect.Value) (string, error) {
	return value.String(), nil
}
