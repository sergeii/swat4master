package memory

import (
	"sync"
	"sync/atomic"
)

type Unlocker struct {
	Locker   sync.Locker
	unlocked atomic.Bool
}

func NewUnlocker(locker sync.Locker) *Unlocker {
	return &Unlocker{
		Locker: locker,
	}
}

func (u *Unlocker) Unlock() {
	if u.unlocked.CompareAndSwap(false, true) {
		u.Locker.Unlock()
	}
}
