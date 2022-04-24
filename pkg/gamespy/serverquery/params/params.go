package params

import (
	"errors"
)

var ErrValueMustBePointer = errors.New("value must be a pointer to a struct")
var ErrInvalidValue = errors.New("unable to parse value with given type")
var ErrUnknownType = errors.New("unable to unmarshal to unknown type")
