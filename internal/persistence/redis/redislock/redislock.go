package redislock

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

var ErrNotAcquired = errors.New("lock: not acquired")

type Manager struct {
	client *redis.Client
	logger *zerolog.Logger
}

func NewManager(client *redis.Client, logger *zerolog.Logger) *Manager {
	return &Manager{
		client: client,
		logger: logger,
	}
}

func (m *Manager) Guard(ctx context.Context, key string, ttl time.Duration, op func(tx *redis.Tx) error) error {
	token := uuid.NewString()

	acquired, err := m.client.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return fmt.Errorf("guard: take lock ownership: %w", err)
	}
	if !acquired {
		return ErrNotAcquired
	}
	defer func() {
		if err := m.release(ctx, key, token); err != nil {
			// The lock ownership has been lost while trying to release it
			// Because that's what we wanted in the first place, this is absolutely acceptable
			if !errors.Is(err, redis.TxFailedErr) {
				m.logger.Warn().Str("key", key).Msg("lock ownership lost while releasing")
			} else {
				m.logger.Error().Err(err).Str("key", key).Msg("failed to release lock")
			}
		}
	}()

	// Make sure we own the lock for the entire duration of the operation
	err = m.client.Watch(ctx, func(tx *redis.Tx) error {
		// Make sure the lock is still ours,
		// as it could have been expired between SETNX and WATCH calls and acquired by someone else
		currToken, err := tx.Get(ctx, key).Result()
		if err != nil {
			return fmt.Errorf("guard: check lock ownership: %w", err)
		}
		if currToken != token {
			return ErrNotAcquired
		}
		return op(tx)
	}, key)
	if err != nil {
		if errors.Is(err, redis.TxFailedErr) || errors.Is(err, ErrNotAcquired) {
			return ErrNotAcquired
		}
		return fmt.Errorf("guard: redis watch: %w", err)
	}

	return nil
}

func (m *Manager) release(ctx context.Context, key, token string) error {
	// Release the lock but only if we still own it
	return m.client.Watch(ctx, func(tx *redis.Tx) error {
		currToken, err := tx.Get(ctx, key).Result()
		if err != nil {
			return fmt.Errorf("release: check lock ownership: %w", err)
		}
		if currToken == token {
			tx.Del(ctx, key)
		}
		return nil
	}, key)
}
