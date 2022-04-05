package filter

import (
	"errors"
	"fmt"
	"strconv"
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

var ErrInvalidFilterFormat = errors.New("invalid filter format")
var ErrUnknownFieldName = errors.New("unknown field name")
var ErrUnsupportedOperatorType = errors.New("unknown operator name")
var ErrInvalidValueFormat = errors.New("unknown field value format")

var ErrFieldNotFound = errors.New("field was not found")
var ErrFieldInvalidValueType = errors.New("field value contains invalid type")
var ErrFieldUnsupportedOperatorType = errors.New("field value does not support provided operator")

type FieldValue struct {
	field string
}

// Evaluate obtains the actual value from the provided fieldset
// The obtained value is then used for comparison, such as numplayers!=maxplayers
func (fv FieldValue) Evaluate(fieldset map[string]string) (string, error) {
	val, ok := fieldset[fv.field]
	if !ok {
		return "", ErrFieldNotFound
	}
	return val, nil
}

func NewFieldValue(field string) FieldValue {
	return FieldValue{field}
}

type Filter struct {
	field string
	op    Operator
	opStr string
	value interface{}
}

func (f Filter) String() string {
	return fmt.Sprintf("%s%s%v", f.field, f.opStr, f.value)
}

// Match checks whether this filter instance matches either of the provided field set
func (f Filter) Match(fields map[string]string) (bool, error) {
	// the field does not exist in the set, so no match
	fieldValue, exists := fields[f.field]
	if !exists {
		return false, nil
	}
	switch wantedValue := f.value.(type) {
	case int:
		// password=0, numplayers>0
		// since we already know that the value that holds the current filter instance is numeric
		// we assume that the tested value is also an int
		fieldValueInt, err := strconv.Atoi(fieldValue)
		if err != nil {
			return false, err
		}
		return f.compareToInt(fieldValueInt, wantedValue)
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
		return f.compareToString(fieldValue, evaluatedVal)
	default:
		return false, ErrFieldInvalidValueType
	}
}

func (f Filter) compareToInt(this, other int) (bool, error) {
	switch f.op {
	case EQ:
		return eq(this, other), nil
	case NE:
		return ne(this, other), nil
	case LT:
		return lt(this, other), nil
	case GT:
		return gt(this, other), nil
	default:
		return false, ErrFieldUnsupportedOperatorType
	}
}

func (f Filter) compareToString(this, other string) (bool, error) {
	// nolint: exhaustive
	switch f.op {
	case EQ:
		return eq(this, other), nil
	case NE:
		return ne(this, other), nil
	default:
		return false, ErrFieldUnsupportedOperatorType
	}
}

func NewFilter(field, rawOp string, value interface{}) (Filter, error) {
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

// ParseFilter accepts a string in the form of "<field><op><value>" that represents
// a single filter value for the specified server field.
// Examples: numplayers!=maxplayers, password=0, gamevariant='SWAT 4'
// Returns an instance of Filter
func ParseFilter(filter string) (Filter, error) { // nolint: cyclop
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
	return NewFilter(fieldName, op, filterValue)
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
