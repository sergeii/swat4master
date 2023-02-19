package servers

import (
	"time"

	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
)

type FilterSet struct {
	withStatus    ds.DiscoveryStatus
	noStatus      ds.DiscoveryStatus
	updatedBefore time.Time
	updatedAfter  time.Time
	activeBefore  time.Time
	activeAfter   time.Time
}

func NewFilterSet() FilterSet {
	return FilterSet{}
}

func (fs FilterSet) UpdatedAfter(after time.Time) FilterSet {
	fs.updatedAfter = after
	return fs
}

func (fs FilterSet) UpdatedBefore(before time.Time) FilterSet {
	fs.updatedBefore = before
	return fs
}

func (fs FilterSet) ActiveAfter(after time.Time) FilterSet {
	fs.activeAfter = after
	return fs
}

func (fs FilterSet) ActiveBefore(before time.Time) FilterSet {
	fs.activeBefore = before
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

func (fs FilterSet) GetUpdatedAfter() (time.Time, bool) {
	if fs.updatedAfter.IsZero() {
		return fs.updatedAfter, false
	}
	return fs.updatedAfter, true
}

func (fs FilterSet) GetUpdatedBefore() (time.Time, bool) {
	if fs.updatedBefore.IsZero() {
		return fs.updatedBefore, false
	}
	return fs.updatedBefore, true
}

func (fs FilterSet) GetActiveAfter() (time.Time, bool) {
	if fs.activeAfter.IsZero() {
		return fs.activeAfter, false
	}
	return fs.activeAfter, true
}

func (fs FilterSet) GetActiveBefore() (time.Time, bool) {
	if fs.activeBefore.IsZero() {
		return fs.activeBefore, false
	}
	return fs.activeBefore, true
}
