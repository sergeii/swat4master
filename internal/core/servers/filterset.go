package servers

import (
	"time"

	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
)

type FilterSet struct {
	withStatus ds.DiscoveryStatus
	noStatus   ds.DiscoveryStatus
	before     time.Time
	after      time.Time
}

func NewFilterSet() FilterSet {
	return FilterSet{}
}

func (fs FilterSet) After(after time.Time) FilterSet {
	fs.after = after
	return fs
}

func (fs FilterSet) Before(before time.Time) FilterSet {
	fs.before = before
	return fs
}

func (fs FilterSet) ResetStatus() FilterSet {
	fs.withStatus = ds.NoStatus
	fs.noStatus = ds.NoStatus
	return fs
}

func (fs FilterSet) WithStatus(status ds.DiscoveryStatus) FilterSet {
	fs.withStatus |= status
	return fs
}

func (fs FilterSet) NoStatus(status ds.DiscoveryStatus) FilterSet {
	fs.noStatus |= status
	return fs
}

func (fs FilterSet) GetWithStatus() (ds.DiscoveryStatus, bool) {
	if !fs.withStatus.HasStatus() {
		return fs.withStatus, false
	}
	return fs.withStatus, true
}

func (fs FilterSet) GetNoStatus() (ds.DiscoveryStatus, bool) {
	if !fs.noStatus.HasStatus() {
		return fs.noStatus, false
	}
	return fs.noStatus, true
}

func (fs FilterSet) GetAfter() (time.Time, bool) {
	if fs.after.IsZero() {
		return fs.after, false
	}
	return fs.after, true
}

func (fs FilterSet) GetBefore() (time.Time, bool) {
	if fs.before.IsZero() {
		return fs.before, false
	}
	return fs.before, true
}
