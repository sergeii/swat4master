package memory

import (
	"container/list"
	"context"
	"errors"
	"sync"
	"time"

	"github.com/sergeii/swat4master/internal/core/probes"
)

type enqueued struct {
	target probes.Target
	before time.Time
	after  time.Time
}

type Repository struct {
	queue  *list.List
	length int
	mutex  sync.RWMutex
}

func New() *Repository {
	repo := &Repository{
		queue: list.New(),
	}
	return repo
}

func (r *Repository) Add(_ context.Context, target probes.Target) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.enqueue(target, probes.NC, probes.NC)
	return nil
}

func (r *Repository) AddBetween(_ context.Context, target probes.Target, after time.Time, before time.Time) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.enqueue(target, before, after)
	return nil
}

func (r *Repository) Pop(_ context.Context) (probes.Target, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	passes := r.length
	for {
		next, err := r.next()
		if err == nil {
			r.queue.Remove(next)
			r.length--
			return next.Value.(enqueued).target, nil // nolint: forcetypeassert
		}
		passes--
		if passes > 0 {
			continue
		}
		switch {
		case errors.Is(err, probes.ErrTargetHasExpired):
			return probes.Blank, probes.ErrQueueIsEmpty
		default:
			return probes.Blank, err
		}
	}
}

func (r *Repository) PopAny(_ context.Context) (probes.Target, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	last := r.queue.Front()
	// queue is empty
	if last == nil {
		return probes.Blank, probes.ErrQueueIsEmpty
	}
	r.queue.Remove(last)
	r.length--
	return last.Value.(enqueued).target, nil // nolint: forcetypeassert
}

func (r *Repository) PopMany(_ context.Context, count int) ([]probes.Target, int, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// queue is empty
	if r.length == 0 {
		return nil, 0, nil
	}

	targets := make([]probes.Target, 0, count)
	seenItems := make([]*list.Element, 0, count)
	futureItems := make([]*list.Element, 0)
	expiredCount := 0

	for next := r.queue.Front(); next != nil && len(targets) < count; next = next.Next() {
		item := next.Value.(enqueued) // nolint: forcetypeassert
		// target's time hasn't come yet
		if !item.after.IsZero() && item.after.After(time.Now()) {
			futureItems = append(futureItems, next)
			continue
		}
		seenItems = append(seenItems, next)
		// target's time has not expired, or the expiration time hasn't been set
		if item.before.IsZero() || item.before.After(time.Now()) {
			targets = append(targets, item.target)
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

	return targets, expiredCount, nil
}

func (r *Repository) Count(context.Context) (int, error) {
	return r.length, nil
}

func (r *Repository) enqueue(
	target probes.Target, before time.Time, after time.Time,
) {
	item := enqueued{target, before, after}
	r.queue.PushBack(item)
	r.length++
}

func (r *Repository) next() (*list.Element, error) {
	next := r.queue.Front()
	// queue is empty
	if next == nil {
		return nil, probes.ErrQueueIsEmpty
	}
	item := next.Value.(enqueued) // nolint: forcetypeassert
	// the target's time has expired
	if !item.before.IsZero() && item.before.Before(time.Now()) {
		r.queue.Remove(next)
		r.length--
		return nil, probes.ErrTargetHasExpired
	}
	// the target's time hasn't come yet
	if !item.after.IsZero() && item.after.After(time.Now()) {
		r.queue.MoveToBack(next)
		return nil, probes.ErrTargetIsNotReady
	}
	return next, nil
}
