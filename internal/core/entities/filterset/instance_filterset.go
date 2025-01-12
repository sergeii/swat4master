package filterset

import (
	"time"
)

type InstanceFilterSet struct {
	updatedBefore time.Time
}

func NewInstanceFilterSet() InstanceFilterSet {
	return InstanceFilterSet{}
}

func (fs InstanceFilterSet) UpdatedBefore(before time.Time) InstanceFilterSet {
	fs.updatedBefore = before
	return fs
}

func (fs InstanceFilterSet) GetUpdatedBefore() (time.Time, bool) {
	if fs.updatedBefore.IsZero() {
		return fs.updatedBefore, false
	}
	return fs.updatedBefore, true
}
