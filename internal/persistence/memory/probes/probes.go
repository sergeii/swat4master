package probes

import (
	"container/list"
	"context"
	"errors"
	"sync"
	"time"

	"github.com/benbjohnson/clock"

	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/repositories"
)

type enqueued struct {
	probe  probe.Probe
	before time.Time
	after  time.Time
}

type Repository struct {
	queue  *list.List
	length int
	clock  clock.Clock
	mutex  sync.RWMutex
}

func New(c clock.Clock) *Repository {
	repo := &Repository{
		queue: list.New(),
		clock: c,
	}
	return repo
}

func (r *Repository) Add(_ context.Context, prb probe.Probe) error {
	r.enqueue(prb, repositories.NC, repositories.NC)
	return nil
}

func (r *Repository) AddBetween(_ context.Context, prb probe.Probe, after time.Time, before time.Time) error {
	r.enqueue(prb, before, after)
	return nil
}

func (r *Repository) Pop(_ context.Context) (probe.Probe, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	passes := r.length
	for {
		next, err := r.next()
		if err == nil {
			r.queue.Remove(next)
			r.length--
			return next.Value.(enqueued).probe, nil // nolint: forcetypeassert
		}
		passes--
		if passes > 0 {
			continue
		}
		switch {
		case errors.Is(err, repositories.ErrProbeHasExpired):
			return probe.Blank, repositories.ErrProbeQueueIsEmpty
		default:
			return probe.Blank, err
		}
	}
}

func (r *Repository) PopAny(_ context.Context) (probe.Probe, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	last := r.queue.Front()
	// queue is empty
	if last == nil {
		return probe.Blank, repositories.ErrProbeQueueIsEmpty
	}
	r.queue.Remove(last)
	r.length--
	return last.Value.(enqueued).probe, nil // nolint: forcetypeassert
}

func (r *Repository) PopMany(_ context.Context, count int) ([]probe.Probe, int, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// queue is empty
	if r.length == 0 {
		return nil, 0, nil
	}

	probes := make([]probe.Probe, 0, count)
	seenItems := make([]*list.Element, 0, count)
	futureItems := make([]*list.Element, 0)
	expiredCount := 0

	now := r.clock.Now()

	for next := r.queue.Front(); next != nil && len(probes) < count; next = next.Next() {
		item := next.Value.(enqueued) // nolint: forcetypeassert
		// target's time hasn't come yet
		if !item.after.IsZero() && item.after.After(now) {
			futureItems = append(futureItems, next)
			continue
		}
		seenItems = append(seenItems, next)
		// target's time has not expired, or the expiration time hasn't been set
		if item.before.IsZero() || item.before.After(now) {
			probes = append(probes, item.probe)
		} else {
			expiredCount++
		}
	}

	// remove expired and obtained targets from the queue
	for _, seen := range seenItems {
		r.queue.Remove(seen)
		r.length--
	}

	// move future targets to the end of the queue
	for _, future := range futureItems {
		r.queue.MoveToBack(future)
	}

	return probes, expiredCount, nil
}

func (r *Repository) Count(context.Context) (int, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.length, nil
}

func (r *Repository) enqueue(
	prb probe.Probe, before time.Time, after time.Time,
) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	item := enqueued{prb, before, after}
	r.queue.PushBack(item)
	r.length++
}

func (r *Repository) next() (*list.Element, error) {
	now := r.clock.Now()
	next := r.queue.Front()
	// queue is empty
	if next == nil {
		return nil, repositories.ErrProbeQueueIsEmpty
	}
	item := next.Value.(enqueued) // nolint: forcetypeassert
	// the target's time has expired
	if !item.before.IsZero() && item.before.Before(now) {
		r.queue.Remove(next)
		r.length--
		return nil, repositories.ErrProbeHasExpired
	}
	// the target's time hasn't come yet
	if !item.after.IsZero() && item.after.After(now) {
		r.queue.MoveToBack(next)
		return nil, repositories.ErrProbeIsNotReady
	}
	return next, nil
}
