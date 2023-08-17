package details

import (
	"fmt"
)

type ObjectiveStatus int

const (
	ObjInProgress ObjectiveStatus = iota
	ObjCompleted
	ObjFailed
)

func (os ObjectiveStatus) String() string {
	switch os { // nolint: exhaustive
	case ObjInProgress:
		return "In Progress"
	case ObjCompleted:
		return "Completed"
	case ObjFailed:
		return "Failed"
	}
	return fmt.Sprintf("%d", os)
}

type Objective struct {
	Name   string          `validate:"required"`
	Status ObjectiveStatus `validate:"oneof=0 1 2"`
}
