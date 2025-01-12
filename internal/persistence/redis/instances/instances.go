package instances

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/jonboulle/clockwork"
	"github.com/redis/go-redis/v9"

	"github.com/sergeii/swat4master/internal/core/entities/filterset"
	"github.com/sergeii/swat4master/internal/core/entities/instance"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/pkg/redisutils"
)

const (
	itemsKey   = "instances:items"
	updatesKey = "instances:updated"
)

type Repository struct {
	client *redis.Client
	clock  clockwork.Clock
}

func New(client *redis.Client, c clockwork.Clock) *Repository {
	return &Repository{
		client: client,
		clock:  c,
	}
}

func (r *Repository) Add(ctx context.Context, ins instance.Instance) error {
	item, err := encodeInstance(ins)
	if err != nil {
		return err
	}
	_, err = r.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		hexID := encodeID(ins.ID)
		// Add or update the instance in the hash set
		pipe.HSet(ctx, itemsKey, hexID, item)
		// Update the timestamp in the sorted set
		pipe.ZAdd(ctx, updatesKey, redis.Z{
			Score:  float64(r.clock.Now().UnixNano()),
			Member: hexID,
		})
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to add instance: %w", err)
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, id string) (instance.Instance, error) {
	item, err := r.client.HGet(ctx, itemsKey, encodeID(id)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return instance.Blank, repositories.ErrInstanceNotFound
		}
		return instance.Blank, fmt.Errorf("failed to retrieve instance by id: %w", err)
	}
	return decodeInstance(item)
}

func (r *Repository) Remove(ctx context.Context, id string) error {
	_, err := r.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		hexID := encodeID(id)
		pipe.HDel(ctx, itemsKey, hexID)
		pipe.ZRem(ctx, updatesKey, hexID)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to remove instance: %w", err)
	}
	return nil
}

func (r *Repository) Clear(ctx context.Context, fs filterset.InstanceFilterSet) (int, error) {
	stop := "+inf"
	if updatedBefore, ok := fs.GetUpdatedBefore(); ok {
		stop = strconv.FormatInt(updatedBefore.UnixNano(), 10)
	}

	// Fetch IDs of instances to remove
	keys, err := r.client.ZRangeArgs(
		ctx,
		redis.ZRangeArgs{
			Key:     updatesKey,
			ByScore: true,
			Start:   "-inf",
			Stop:    stop,
		},
	).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to fetch instances to remove: %w", err)
	}

	if len(keys) == 0 {
		return 0, nil
	}

	var affected *redis.IntCmd
	_, err = r.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.ZRem(ctx, updatesKey, redisutils.KeysToMembers(keys)...)
		affected = pipe.HDel(ctx, itemsKey, keys...)
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("failed to remove instances: %w", err)
	}

	return int(affected.Val()), nil
}

func (r *Repository) Count(ctx context.Context) (int, error) {
	count, err := r.client.HLen(ctx, itemsKey).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to count instances: %w", err)
	}
	return int(count), nil
}

func encodeID(id string) string {
	return fmt.Sprintf("%x", id)
}

func decodeID(hexID string) (string, error) {
	id, err := hex.DecodeString(hexID)
	if err != nil {
		return "", fmt.Errorf("failed to decode hex id: %w", err)
	}
	return string(id), nil
}

func encodeInstance(ins instance.Instance) ([]byte, error) {
	item := newStoredItem(ins.ID, ins.Addr)
	encoded, err := json.Marshal(item)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal instance item: %w", err)
	}
	return encoded, nil
}

func decodeInstance(val interface{}) (instance.Instance, error) {
	var item storedItem
	encoded, ok := val.(string)
	if !ok {
		return instance.Blank, fmt.Errorf("unexpected type %T, %v", val, val)
	}
	if err := json.Unmarshal([]byte(encoded), &item); err != nil {
		return instance.Blank, fmt.Errorf("failed to unmarshal instance item: %w", err)
	}
	return item.convert()
}
