package filter

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"

	"github.com/sergeii/swat4master/pkg/gamespy/serverquery/params"
)

type Operator int

const (
	EQ Operator = iota
	NE
	LT
	GT
)

const (
	fieldParserStageName = iota
	fieldParserStageOp
	fieldParserStageValue
)

var (
	ErrInvalidFilterFormat     = errors.New("invalid filter format")
	ErrUnknownFieldName        = errors.New("unknown field name")
	ErrUnsupportedOperatorType = errors.New("unknown operator name")
	ErrInvalidValueFormat      = errors.New("unknown field value format")
)

var (
	ErrMustBePointer                = errors.New("fields value must be a pointer")
	ErrFieldNotFound                = errors.New("field was not found")
	ErrFieldInvalidValueType        = errors.New("field value contains invalid type")
	ErrFieldUnsupportedOperatorType = errors.New("field value does not support provided operator")
)

type FieldValue struct {
	field string
}

// Evaluate obtains the actual value from the provided fieldset
// The obtained value is then used for comparison, such as numplayers!=maxplayers
func (fv FieldValue) Evaluate(fields any) (interface{}, error) {
	value, err := getStructField(fields, fv.field)
	if err != nil {
		return "", err
	}
	return value, nil
}

func NewFieldValue(field string) FieldValue {
	return FieldValue{field}
}

type Filter struct {
	field string
	op    Operator
	rawop string
	value interface{}
}

func (f Filter) String() string {
	return fmt.Sprintf("%s%s%v", f.field, f.rawop, f.value)
}

// Match checks whether this filter instance matches either of the provided field set
func (f Filter) Match(fields any) (bool, error) {
	fieldValue, err := getStructField(fields, f.field)
	if err != nil {
		// the field does not exist in the set, so no match
		if errors.Is(err, ErrFieldNotFound) {
			return false, nil
		}
		return false, err
	}

	switch wantedValue := f.value.(type) {
	case int:
		// password=0, numplayers>0
		// since we already know that the value that holds the current filter instance is numeric
		// we assume that the tested value is also an int
		return f.compareToInt(fieldValue, wantedValue)
	case string:
		// gamevariant='SWAT 4'
		return f.compareToString(fieldValue, wantedValue)
	case FieldValue:
		// numplayers!=maxplayers
		// Although tested fields may hold numeric values,
		// we treat them as strings since no other operator except for "=" and "!=" is supported here
		evaluatedVal, err := wantedValue.Evaluate(fields)
		if err != nil {
			return false, err
		}
		return f.compare(fieldValue, evaluatedVal)
	default:
		return false, ErrFieldInvalidValueType
	}
}

func (f Filter) compare(this, other interface{}) (bool, error) {
	switch otherTyped := other.(type) {
	case int:
		return f.compareToInt(this, otherTyped)
	case string:
		return f.compareToString(this, otherTyped)
	default:
		return false, ErrFieldInvalidValueType
	}
}

func (f Filter) compareToInt(this interface{}, other int) (bool, error) {
	var thisInt int

	switch anyInt := this.(type) {
	case int:
		thisInt = anyInt
	case bool:
		if anyInt {
			thisInt = 1
		}
	default:
		return false, ErrFieldInvalidValueType
	}

	switch f.op {
	case EQ:
		return eq(thisInt, other), nil
	case NE:
		return ne(thisInt, other), nil
	case LT:
		return lt(thisInt, other), nil
	case GT:
		return gt(thisInt, other), nil
	default:
		return false, ErrFieldUnsupportedOperatorType
	}
}

func (f Filter) compareToString(this interface{}, other string) (bool, error) {
	thisStr, ok := this.(string)
	if !ok {
		return false, ErrFieldInvalidValueType
	}
	// nolint: exhaustive
	switch f.op {
	case EQ:
		return eq(thisStr, other), nil
	case NE:
		return ne(thisStr, other), nil
	default:
		return false, ErrFieldUnsupportedOperatorType
	}
}

func New(field, rawOp string, value interface{}) (Filter, error) {
	var op Operator
	if !IsQueryField(field) {
		return Filter{}, ErrUnknownFieldName
	}
	switch rawOp {
	case "=":
		op = EQ
	case "!=":
		op = NE
	case "<":
		op = LT
	case ">":
		op = GT
	default:
		return Filter{}, ErrUnsupportedOperatorType
	}
	return Filter{field, op, rawOp, value}, nil
}

func MustNew(field, rawOp string, value interface{}) Filter {
	f, err := New(field, rawOp, value)
	if err != nil {
		panic(err)
	}
	return f
}

// Parse accepts a string in the form of "<field><op><value>" that represents
// a single filter value for the specified server field.
// Examples: numplayers!=maxplayers, password=0, gamevariant='SWAT 4'
// Returns an instance of Filter
func Parse(filter string) (Filter, error) { // nolint: cyclop
	var fieldName, op string
	filterBytes := []byte(filter)
	i, j := 0, 0
	stage := fieldParserStageName
	for _, char := range filterBytes {
		if char == '!' || char == '=' || char == '<' || char == '>' {
			// stage transition: name -> operator
			if stage == fieldParserStageName {
				fieldName = string(filterBytes[i:j])
				stage = fieldParserStageOp
				i = j
			}
		} else if stage == fieldParserStageOp {
			// stage transition: operator -> value
			op = string(filterBytes[i:j])
			stage = fieldParserStageValue
			i = j
		}
		j++
	}
	if fieldName == "" || stage != fieldParserStageValue {
		return Filter{}, ErrInvalidFilterFormat
	}
	// the remaining bytes make up the field value
	filterValue, err := parseRawFilterValue(string(filterBytes[i:]))
	if err != nil {
		return Filter{}, err
	}
	return New(fieldName, op, filterValue)
}

func parseRawFilterValue(rawVal string) (interface{}, error) {
	// password=0, numplayers>0
	if numericVal, err := strconv.Atoi(rawVal); err == nil {
		return numericVal, nil
	}
	// gamevariant='SWAT 4', gametype='VIP Escort', gamever='1.1'
	if len(rawVal) > 2 && rawVal[0] == '\'' && rawVal[len(rawVal)-1] == '\'' {
		return rawVal[1 : len(rawVal)-1], nil
	}
	// numplayers!=maxplayers
	if IsQueryField(rawVal) {
		return NewFieldValue(rawVal), nil
	}
	return nil, ErrInvalidValueFormat
}

func IsQueryField(field string) bool {
	switch field {
	case
		"gamename",
		"hostname",
		"numplayers",
		"maxplayers",
		"gametype",
		"gamevariant",
		"mapname",
		"hostport",
		"password",
		"statsenabled",
		"gamever":
		return true
	}
	return false
}

func getStructField(v any, fieldName string) (any, error) {
	rv := reflect.ValueOf(v)

	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return nil, ErrMustBePointer
	}

	sv := rv.Elem()
	st := reflect.TypeOf(sv.Interface())

	for i := range st.NumField() {
		field := st.Field(i)
		// ignore private fields
		if !field.IsExported() {
			continue
		}
		// compare field names or their "param" aliases
		param, ok := params.GetParamName(field)
		if !ok || param != fieldName {
			continue
		}
		return sv.FieldByName(field.Name).Interface(), nil
	}

	return nil, ErrFieldNotFound
}
