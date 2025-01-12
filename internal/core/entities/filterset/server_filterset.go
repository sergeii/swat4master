package filterset

import (
	"time"

	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
)

type ServerFilterSet struct {
	withStatus    ds.DiscoveryStatus
	noStatus      ds.DiscoveryStatus
	updatedBefore time.Time
	updatedAfter  time.Time
	activeBefore  time.Time
	activeAfter   time.Time
}

func NewServerFilterSet() ServerFilterSet {
	return ServerFilterSet{}
}

func (fs ServerFilterSet) UpdatedAfter(after time.Time) ServerFilterSet {
	fs.updatedAfter = after
	return fs
}

func (fs ServerFilterSet) UpdatedBefore(before time.Time) ServerFilterSet {
	fs.updatedBefore = before
	return fs
}

func (fs ServerFilterSet) ActiveAfter(after time.Time) ServerFilterSet {
	fs.activeAfter = after
	return fs
}

func (fs ServerFilterSet) ActiveBefore(before time.Time) ServerFilterSet {
	fs.activeBefore = before
	return fs
}

func (fs ServerFilterSet) ResetStatus() ServerFilterSet {
	fs.withStatus = ds.NoStatus
	fs.noStatus = ds.NoStatus
	return fs
}

func (fs ServerFilterSet) WithStatus(status ds.DiscoveryStatus) ServerFilterSet {
	fs.withStatus |= status
	return fs
}

func (fs ServerFilterSet) NoStatus(status ds.DiscoveryStatus) ServerFilterSet {
	fs.noStatus |= status
	return fs
}

func (fs ServerFilterSet) GetWithStatus() (ds.DiscoveryStatus, bool) {
	if !fs.withStatus.HasStatus() {
		return fs.withStatus, false
	}
	return fs.withStatus, true
}

func (fs ServerFilterSet) GetNoStatus() (ds.DiscoveryStatus, bool) {
	if !fs.noStatus.HasStatus() {
		return fs.noStatus, false
	}
	return fs.noStatus, true
}

func (fs ServerFilterSet) GetUpdatedAfter() (time.Time, bool) {
	if fs.updatedAfter.IsZero() {
		return fs.updatedAfter, false
	}
	return fs.updatedAfter, true
}

func (fs ServerFilterSet) GetUpdatedBefore() (time.Time, bool) {
	if fs.updatedBefore.IsZero() {
		return fs.updatedBefore, false
	}
	return fs.updatedBefore, true
}

func (fs ServerFilterSet) GetActiveAfter() (time.Time, bool) {
	if fs.activeAfter.IsZero() {
		return fs.activeAfter, false
	}
	return fs.activeAfter, true
}

func (fs ServerFilterSet) GetActiveBefore() (time.Time, bool) {
	if fs.activeBefore.IsZero() {
		return fs.activeBefore, false
	}
	return fs.activeBefore, true
}
