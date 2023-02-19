package params

import (
	"errors"
)

var ErrValueMustBePointer = errors.New("value must be a pointer to a struct")
