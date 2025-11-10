package probes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/redis/go-redis/v9"

	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/pkg/redisutils"
)

const (
	queueKey = "probes:queue"
	dataKey  = "probes:items"
)

type Repository struct {
	client *redis.Client
	clock  clockwork.Clock
}

type qItem struct {
	Probe   probe.Probe `json:"probe"`
	Expires time.Time   `json:"expires"`
}

func New(client *redis.Client, c clockwork.Clock) *Repository {
	return &Repository{
		client: client,
		clock:  c,
	}
}

func (r *Repository) Add(ctx context.Context, prb probe.Probe) error {
	return r.enqueue(ctx, prb, repositories.NC, repositories.NC)
}

func (r *Repository) AddBetween(ctx context.Context, prb probe.Probe, after time.Time, before time.Time) error {
	return r.enqueue(ctx, prb, after, before)
}

func (r *Repository) enqueue(ctx context.Context, prb probe.Probe, after time.Time, before time.Time) error {
	// ignore items with ready time set after or equal to the expiration time
	if !after.IsZero() && !before.IsZero() && (after.After(before) || after.Equal(before)) {
		return nil
	}

	item, err := json.Marshal(qItem{
		Probe:   prb,
		Expires: before,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal probe: %w", err)
	}

	itemID := uuid.NewString()
	// unless specified, the probe is ready to be processed immediately
	itemReadyAt := after
	if itemReadyAt.IsZero() {
		itemReadyAt = r.clock.Now()
	}

	_, err = r.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctx, dataKey, itemID, item)
		// add the probe to the queue
		pipe.ZAdd(ctx, queueKey, redis.Z{
			Score:  float64(itemReadyAt.UnixNano()),
			Member: itemID,
		})
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to enqueue probe: %w", err)
	}

	return nil
}

func (r *Repository) Pop(ctx context.Context) (probe.Probe, error) {
	probes, _, err := r.PopMany(ctx, 1)
	if err != nil {
		return probe.Blank, err
	}
	if len(probes) == 0 {
		return probe.Blank, repositories.ErrProbeQueueIsEmpty
	}
	return probes[0], nil
}

func (r *Repository) Peek(ctx context.Context) (probe.Probe, error) {
	keys, err := r.client.ZRange(ctx, queueKey, 0, 1).Result()
	if err != nil {
		return probe.Blank, fmt.Errorf("failed to peek probe: %w", err)
	}

	if len(keys) == 0 {
		return probe.Blank, repositories.ErrProbeQueueIsEmpty
	}

	value, err := r.client.HGet(ctx, dataKey, keys[0]).Result()
	if err != nil {
		return probe.Blank, fmt.Errorf("failed to fetch peeked probe: %w", err)
	}

	item, err := asQueuedItem(value)
	if err != nil {
		return probe.Blank, fmt.Errorf("failed to unmarshal probe: %w", err)
	}

	return item.Probe, nil
}

func (r *Repository) PopMany(ctx context.Context, count int) ([]probe.Probe, int, error) {
	if count <= 0 {
		return nil, 0, nil
	}

	expired := 0
	probes := make([]probe.Probe, 0, count)

	// fetch the first n probes from the queue that are ready to be processed
	for len(probes) < count {
		items, err := r.pop(ctx, count-len(probes))
		if err != nil {
			if errors.Is(err, repositories.ErrProbeQueueIsEmpty) {
				break
			}
			return nil, 0, err
		}
		for _, item := range items {
			if isItemExpired(item.Expires, r.clock.Now()) {
				expired++
				continue
			}
			probes = append(probes, item.Probe)
		}
	}

	return probes, expired, nil
}

func (r *Repository) pop(ctx context.Context, count int) ([]qItem, error) {
	// fetch the first n probes from the queue that are ready to be processed
	keys, err := r.client.ZRangeArgs(
		ctx,
		redis.ZRangeArgs{
			Key:     queueKey,
			ByScore: true,
			Start:   "-inf",
			Stop:    strconv.FormatInt(r.clock.Now().UnixNano(), 10), // inclusive
			Count:   int64(count),
		},
	).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch probes: %w", err)
	}

	// queue is empty
	if len(keys) == 0 {
		return nil, repositories.ErrProbeQueueIsEmpty
	}

	// pop the ready-to-process probes from the items set and the queue atomically
	var result *redis.SliceCmd
	if _, err = r.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.ZRem(ctx, queueKey, redisutils.KeysToMembers(keys)...)
		result = pipe.HMGet(ctx, dataKey, keys...)
		pipe.HDel(ctx, dataKey, keys...)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to pop probes: %w", err)
	}

	items := make([]qItem, 0, len(keys))
	var item qItem
	for _, val := range result.Val() {
		if val == nil {
			continue
		}
		if item, err = asQueuedItem(val); err != nil {
			return nil, fmt.Errorf("failed to unmarshal probe: %w", err)
		}
		items = append(items, item)
	}

	// By the time we obtained the probes, some of them might have popped by other consumers
	if len(items) == 0 {
		return nil, repositories.ErrProbeQueueIsEmpty
	}

	return items, nil
}

func (r *Repository) Count(ctx context.Context) (int, error) {
	count, err := r.client.ZCard(ctx, queueKey).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to count probes: %w", err)
	}
	return int(count), nil
}

func isItemExpired(expires time.Time, now time.Time) bool {
	return !expires.IsZero() && expires.Before(now)
}

func asQueuedItem(val any) (qItem, error) {
	var item qItem
	encoded, ok := val.(string)
	if !ok {
		return qItem{}, fmt.Errorf("unexpected type %T, %v", val, val)
	}
	if err := json.Unmarshal([]byte(encoded), &item); err != nil {
		return qItem{}, fmt.Errorf("failed to unmarshal probe item: %w", err)
	}
	return item, nil
}
