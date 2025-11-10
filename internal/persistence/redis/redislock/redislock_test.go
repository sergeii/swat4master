package redislock_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/internal/persistence/redis/redislock"
	"github.com/sergeii/swat4master/internal/testutils/testredis"
)

func TestRedisLockManager_Guard_OK(t *testing.T) {
	ctx := context.TODO()
	rdb := testredis.MakeClient(t)
	logger := zerolog.Nop()

	// Given a lock manager
	m := redislock.NewManager(rdb, &logger)

	// When a redis operation is guarded by a lock
	err := m.Guard(ctx, "lock:foo", time.Minute, func(tx *redis.Tx) error {
		tx.Set(ctx, "foo", "bar", 0)
		return nil
	})
	require.NoError(t, err)

	// Then the operation is executed
	val := rdb.Get(ctx, "foo").Val()
	require.Equal(t, "bar", val)

	// And the lock is released
	err = rdb.Get(ctx, "lock:foo").Err()
	require.ErrorIs(t, err, redis.Nil)
}

func TestRedisLockManager_Guard_ConcurrentAccessGuarded(t *testing.T) {
	ctx := context.TODO()
	rdb := testredis.MakeClient(t)
	logger := zerolog.Nop()

	// Given a lock manager
	m := redislock.NewManager(rdb, &logger)

	// And a function that performs a guarded operation
	errCh := make(chan error)
	do := func() {
		// When a redis operation is guarded by a lock
		err := m.Guard(ctx, "lock:foo", time.Minute, func(tx *redis.Tx) error {
			tx.Incr(ctx, "foo")
			time.Sleep(time.Millisecond * 10)
			return nil
		})
		if err != nil {
			errCh <- err
		}
	}

	// When multiple redis clients try to execute an operation using the same lock
	go func() {
		wg := &sync.WaitGroup{}
		for range 10 {
			wg.Go(do)
		}
		wg.Wait()
		close(errCh)
	}()

	errors := make([]error, 0)
	for guardErr := range errCh {
		errors = append(errors, guardErr)
	}

	// Then only one operation is executed
	val := rdb.Get(ctx, "foo").Val()
	require.Equal(t, "1", val)

	// And the rest of the operations are not
	require.Len(t, errors, 9)
}

func TestRedisLockManager_Guard_NoConflictOnIndependentAccess(t *testing.T) {
	ctx := context.TODO()
	rdb := testredis.MakeClient(t)
	logger := zerolog.Nop()

	// Given a lock manager
	m := redislock.NewManager(rdb, &logger)

	// And a function that performs independent guarded operations using different locks
	do := func(i int) {
		// When a redis operation is guarded by a lock
		err := m.Guard(ctx, fmt.Sprintf("lock:foo:%d", i), time.Minute, func(tx *redis.Tx) error {
			tx.Set(ctx, fmt.Sprintf("foo:%d", i), fmt.Sprintf("bar:%d", i), 0)
			time.Sleep(time.Millisecond * 10)
			return nil
		})
		if err != nil {
			panic(err)
		}
	}

	// When multiple redis clients try to execute independent operations
	wg := &sync.WaitGroup{}
	for i := range 10 {
		wg.Go(func() {
			do(i)
		})
	}
	wg.Wait()

	// Then all operations are executed
	for i := range 10 {
		val := rdb.Get(ctx, fmt.Sprintf("foo:%d", i)).Val()
		require.Equal(t, fmt.Sprintf("bar:%d", i), val)
	}

	// And all locks are released
	for i := range 10 {
		err := rdb.Get(ctx, fmt.Sprintf("lock:foo:%d", i)).Err()
		require.ErrorIs(t, err, redis.Nil)
	}
}

func TestRedisLockManager_Guard_LockNotObtained(t *testing.T) {
	ctx := context.TODO()
	rdb := testredis.MakeClient(t)
	logger := zerolog.Nop()

	// Given a lock manager
	m := redislock.NewManager(rdb, &logger)

	// And a lock that is already held by another client
	acquired := make(chan struct{})
	ready := make(chan struct{})
	released := make(chan struct{})
	go func() {
		err := m.Guard(ctx, "lock:foo", time.Minute, func(tx *redis.Tx) error {
			close(acquired)
			tx.Incr(ctx, "foo")
			<-ready
			return nil
		})
		require.NoError(t, err)
		close(released)
	}()

	<-acquired
	// When a guarded operation is attempted while the lock is still held
	err := m.Guard(ctx, "lock:foo", time.Minute, func(tx *redis.Tx) error {
		tx.Incr(ctx, "foo")
		return nil
	})
	// Then the operation is not executed and an error is returned
	require.ErrorIs(t, err, redislock.ErrNotAcquired)

	close(ready)
	<-released

	// And only one operation is executed
	val := rdb.Get(ctx, "foo").Val()
	require.Equal(t, "1", val)

	// And the lock is released
	err = rdb.Get(ctx, "lock:foo").Err()
	require.ErrorIs(t, err, redis.Nil)
}

func TestRedisLockManager_Guard_LockOwnershipLost(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()
	mr := miniredis.RunT(t)
	rdb := testredis.MakeClientFromMini(t, mr)

	// Given a lock manager
	m := redislock.NewManager(rdb, &logger)

	// When a guarded operation that takes a longer time than the lock TTL is executed
	err := m.Guard(ctx, "lock:foo", time.Millisecond*50, func(tx *redis.Tx) error {
		_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Set(ctx, "foo", "bar", 0)
			// expire the lock
			mr.FastForward(time.Millisecond * 100)
			return nil
		})
		return err
	})
	// Then the operation is aborted and an error is returned
	require.ErrorIs(t, err, redislock.ErrNotAcquired)

	// And the conflicting operation is executed instead
	fooErr := rdb.Get(ctx, "foo").Err()
	require.ErrorIs(t, fooErr, redis.Nil)

	// And the lock is released
	err = rdb.Get(ctx, "lock:foo").Err()
	require.ErrorIs(t, err, redis.Nil)
}

func TestRedisLockManager_Guard_LockOwnershipLostAndReclaimed(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()
	mr := miniredis.RunT(t)

	// Given a concurrent guarded operation that uses the same lock
	lost := make(chan struct{})
	released := make(chan struct{})
	go func(mr *miniredis.Miniredis) {
		rdb := testredis.MakeClientFromMini(t, mr)
		m := redislock.NewManager(rdb, &logger)
		<-lost
		err := m.Guard(ctx, "lock:foo", time.Second, func(tx *redis.Tx) error {
			_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, "foo", "bar", 0)
				pipe.Incr(ctx, "counter")
				return nil
			})
			return err
		})
		if err != nil {
			panic(err)
		}
		close(released)
	}(mr)

	// And a lock manager used by the main client
	rdb := testredis.MakeClientFromMini(t, mr)
	m := redislock.NewManager(rdb, &logger)

	// When a guarded operation that takes a longer time than the lock TTL is executed
	err := m.Guard(ctx, "lock:foo", time.Millisecond*50, func(tx *redis.Tx) error {
		_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Set(ctx, "baz", "ham", 0)
			pipe.Incr(ctx, "counter")
			// expire the lock
			mr.FastForward(time.Millisecond * 100)
			close(lost)
			<-released
			return nil
		})
		return err
	})
	// Then the operation is aborted and an error is returned
	require.ErrorIs(t, err, redislock.ErrNotAcquired)

	// And the conflicting operation is executed instead
	foo := rdb.Get(ctx, "foo").Val()
	require.Equal(t, "bar", foo)

	bazErr := rdb.Get(ctx, "baz").Err()
	require.ErrorIs(t, bazErr, redis.Nil)

	times := rdb.Get(ctx, "counter").Val()
	require.Equal(t, "1", times)

	// And the lock is released
	err = rdb.Get(ctx, "lock:foo").Err()
	require.ErrorIs(t, err, redis.Nil)
}
