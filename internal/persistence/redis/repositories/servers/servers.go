package servers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/redis/go-redis/v9"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/filterset"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/persistence/redis/redislock"
	"github.com/sergeii/swat4master/pkg/slice"
)

const (
	itemsKey     = "servers:items"
	updatesKey   = "servers:updated"
	refreshesKey = "servers:refreshed"
	statusKeyFmt = "servers:status:%s"
	lockKeyFmt   = "servers:lock:%s"
)

type LockOpts struct {
	LeaseDuration time.Duration
	RetryBackoff  time.Duration
	MaxAttempts   int
}
type filterBuilder func(
	redis.Pipeliner,
	[]*redis.StringSliceCmd,
	[]*redis.StringSliceCmd,
) ([]*redis.StringSliceCmd, []*redis.StringSliceCmd)

type Repository struct {
	client   *redis.Client
	clock    clockwork.Clock
	locker   *redislock.Manager
	lockOpts LockOpts
}

func New(client *redis.Client, locker *redislock.Manager, c clockwork.Clock) *Repository {
	return &Repository{
		client: client,
		clock:  c,
		locker: locker,
		lockOpts: LockOpts{
			LeaseDuration: time.Second,
			RetryBackoff:  100 * time.Millisecond,
			MaxAttempts:   5,
		},
	}
}

func (r *Repository) Get(ctx context.Context, addr addr.Addr) (server.Server, error) {
	return r.get(ctx, addr)
}

func (r *Repository) Add(
	ctx context.Context,
	svr server.Server,
	onConflict func(*server.Server) bool,
) (server.Server, error) {
	var added server.Server
	err := r.updateExclusive(ctx, svr, func(tx *redis.Tx) error {
		var err error
		added, err = r.add(ctx, tx, svr, onConflict)
		return err
	})
	if err != nil {
		return server.Blank, err
	}
	return added, nil
}

func (r *Repository) add(
	ctx context.Context,
	tx *redis.Tx,
	svr server.Server,
	onConflict func(*server.Server) bool,
) (server.Server, error) {
	existing, err := r.get(ctx, svr.Addr)
	if err != nil {
		// the server does not exist, we can safely add it
		if errors.Is(err, repositories.ErrServerNotFound) {
			return r.save(ctx, tx, svr)
		}
		return server.Blank, err
	}

	// in case the server already exists,
	// let the caller decide whether the server should be added on conflict or not
	resolved := existing
	if !onConflict(&resolved) {
		return server.Blank, repositories.ErrServerExists
	}
	svr = resolved

	return r.save(ctx, tx, svr)
}

func (r *Repository) Update(
	ctx context.Context,
	svr server.Server,
	onConflict func(*server.Server) bool,
) (server.Server, error) {
	var updated server.Server
	err := r.updateExclusive(ctx, svr, func(tx *redis.Tx) error {
		var err error
		updated, err = r.update(ctx, tx, svr, onConflict)
		return err
	})
	if err != nil {
		return server.Blank, err
	}
	return updated, nil
}

func (r *Repository) update(
	ctx context.Context,
	tx *redis.Tx,
	svr server.Server,
	onConflict func(*server.Server) bool,
) (server.Server, error) {
	existing, err := r.get(ctx, svr.Addr)
	if err != nil {
		// the server does not exist, nothing to update
		if errors.Is(err, repositories.ErrServerNotFound) {
			return server.Blank, repositories.ErrServerNotFound
		}
		return server.Blank, err
	}

	// the server can be updated only if the provided version is greater than the existing one.
	// Otherwise, the caller has to resolve the conflict
	if existing.Version > svr.Version {
		resolved := existing
		if !onConflict(&resolved) {
			// return the newer version of the server
			// in case the caller has decided not to resolve the conflict
			return existing, nil
		}
		// replace the updated server object in case of successful conflict resolution
		svr = resolved
	}

	return r.save(ctx, tx, svr)
}

func (r *Repository) Remove(
	ctx context.Context,
	svr server.Server,
	onConflict func(*server.Server) bool,
) error {
	return r.updateExclusive(ctx, svr, func(tx *redis.Tx) error {
		return r.remove(ctx, tx, svr, onConflict)
	})
}

func (r *Repository) remove(
	ctx context.Context,
	tx *redis.Tx,
	svr server.Server,
	onConflict func(*server.Server) bool,
) error {
	existing, err := r.get(ctx, svr.Addr)
	if err != nil {
		// the removed server does not exist, nothing to remove
		if errors.Is(err, repositories.ErrServerNotFound) {
			return nil
		}
		return err
	}

	// in case the server already exists but the version is greater than the provided one,
	// let the caller decide whether to remove the server or not
	if existing.Version > svr.Version {
		resolved := existing
		if !onConflict(&resolved) {
			return nil
		}
		svr = resolved
	}

	_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		svrAddr := svr.Addr.String()
		pipe.HDel(ctx, itemsKey, svrAddr)
		pipe.ZRem(ctx, updatesKey, svrAddr)
		pipe.ZRem(ctx, refreshesKey, svrAddr)
		for _, status := range ds.Members() {
			pipe.SRem(ctx, fmt.Sprintf(statusKeyFmt, status), svrAddr)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("remove: redis pipeline: %w", err)
	}

	return nil
}

func (r *Repository) Filter(ctx context.Context, fs filterset.ServerFilterSet) ([]server.Server, error) {
	keys, err := r.filterServerKeys(ctx, fs)
	if err != nil {
		return nil, fmt.Errorf("filter: %w", err)
	}

	if len(keys) == 0 {
		return nil, nil
	}

	items, err := r.client.HMGet(ctx, itemsKey, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("filter: get items: %w", err)
	}

	servers := make([]server.Server, 0, len(items))
	for _, item := range items {
		svr, err := decodeServer(item)
		if err != nil {
			return nil, fmt.Errorf("filter: decode server: %w", err)
		}
		servers = append(servers, svr)
	}

	return servers, nil
}

func (r *Repository) filterServerKeys(ctx context.Context, fs filterset.ServerFilterSet) ([]string, error) {
	includeCmds := make([]*redis.StringSliceCmd, 0)
	excludeCmds := make([]*redis.StringSliceCmd, 0)

	_, err := r.client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		builders := []filterBuilder{
			r.buildTimestampFilters(ctx, fs),
			r.buildStatusFilters(ctx, fs),
		}

		for _, builder := range builders {
			includeCmds, excludeCmds = builder(pipe, includeCmds, excludeCmds)
		}

		// If no other inclusion filters are applied, include all servers as a fallback
		if len(includeCmds) == 0 {
			cmd := pipe.ZRange(ctx, updatesKey, 0, -1)
			includeCmds = append(includeCmds, cmd)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("redis pipeline: %w", err)
	}

	includeKeySets, err := r.resolveFilterKeys(includeCmds)
	if err != nil {
		return nil, fmt.Errorf("filter keys: include: %w", err)
	}
	excludeKeySets, err := r.resolveFilterKeys(excludeCmds)
	if err != nil {
		return nil, fmt.Errorf("filter keys: exclude: %w", err)
	}

	filteredKeys := slice.Difference(
		slice.Intersection(includeKeySets...),
		excludeKeySets...,
	)

	return filteredKeys, nil
}

func (r *Repository) buildTimestampFilters(
	ctx context.Context,
	fs filterset.ServerFilterSet,
) filterBuilder {
	return func(
		pipe redis.Pipeliner,
		includeCmds []*redis.StringSliceCmd,
		excludeCmds []*redis.StringSliceCmd,
	) ([]*redis.StringSliceCmd, []*redis.StringSliceCmd) {
		if activeBefore, ok := fs.GetActiveBefore(); ok {
			cmd := pipe.ZRangeArgs(
				ctx,
				redis.ZRangeArgs{
					Key:     refreshesKey,
					ByScore: true,
					Start:   "-inf",
					Stop:    fmt.Sprintf("(%d", activeBefore.UnixNano()), // exclusive
				},
			)
			includeCmds = append(includeCmds, cmd)
		}
		if activeAfter, ok := fs.GetActiveAfter(); ok {
			cmd := pipe.ZRangeArgs(
				ctx,
				redis.ZRangeArgs{
					Key:     refreshesKey,
					ByScore: true,
					Start:   strconv.FormatInt(activeAfter.UnixNano(), 10), // inclusive
					Stop:    "+inf",
				},
			)
			includeCmds = append(includeCmds, cmd)
		}
		if updatedBefore, ok := fs.GetUpdatedBefore(); ok {
			cmd := pipe.ZRangeArgs(
				ctx,
				redis.ZRangeArgs{
					Key:     updatesKey,
					ByScore: true,
					Start:   "-inf",
					Stop:    fmt.Sprintf("(%d", updatedBefore.UnixNano()), // exclusive
				},
			)
			includeCmds = append(includeCmds, cmd)
		}
		if updatedAfter, ok := fs.GetUpdatedAfter(); ok {
			cmd := pipe.ZRangeArgs(
				ctx,
				redis.ZRangeArgs{
					Key:     updatesKey,
					ByScore: true,
					Start:   strconv.FormatInt(updatedAfter.UnixNano(), 10), // inclusive
					Stop:    "+inf",
				},
			)
			includeCmds = append(includeCmds, cmd)
		}
		return includeCmds, excludeCmds
	}
}

func (r *Repository) buildStatusFilters(
	ctx context.Context,
	fs filterset.ServerFilterSet,
) filterBuilder {
	return func(
		pipe redis.Pipeliner,
		includeCmds []*redis.StringSliceCmd,
		excludeCmds []*redis.StringSliceCmd,
	) ([]*redis.StringSliceCmd, []*redis.StringSliceCmd) {
		if withStatus, ok := fs.GetWithStatus(); ok {
			keys := make([]string, 0)
			for status := range withStatus.Bits() {
				keys = append(keys, fmt.Sprintf(statusKeyFmt, status))
			}
			if len(keys) > 0 {
				cmd := pipe.SInter(ctx, keys...)
				includeCmds = append(includeCmds, cmd)
			}
		}

		if withNoStatus, ok := fs.GetNoStatus(); ok {
			keys := make([]string, 0)
			for status := range withNoStatus.Bits() {
				keys = append(keys, fmt.Sprintf(statusKeyFmt, status))
			}
			if len(keys) > 0 {
				cmd := pipe.SUnion(ctx, keys...)
				excludeCmds = append(excludeCmds, cmd)
			}
		}

		return includeCmds, excludeCmds
	}
}

func (r *Repository) resolveFilterKeys(cmds []*redis.StringSliceCmd) ([][]string, error) {
	sets := make([][]string, len(cmds))
	for i, cmd := range cmds {
		if cmd != nil {
			keys, err := cmd.Result()
			if err != nil {
				return nil, err
			}
			sets[i] = keys
		}
	}
	return sets, nil
}

func (r *Repository) Count(ctx context.Context) (int, error) {
	count, err := r.client.HLen(ctx, itemsKey).Result()
	if err != nil {
		return 0, fmt.Errorf("count: %w", err)
	}
	return int(count), nil
}

func (r *Repository) CountByStatus(ctx context.Context) (map[ds.DiscoveryStatus]int, error) {
	cmds := make(map[ds.DiscoveryStatus]*redis.IntCmd)
	_, err := r.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, status := range ds.Members() {
			cmds[status] = pipe.SCard(ctx, fmt.Sprintf(statusKeyFmt, status))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("count by status: redis pipeline: %w", err)
	}

	counts := make(map[ds.DiscoveryStatus]int)

	for status, cmd := range cmds {
		count, err := cmd.Result()
		if err != nil {
			return nil, fmt.Errorf("count by status: scard for '%s': %w", status, err)
		}
		counts[status] = int(count)
	}

	return counts, nil
}

func (r *Repository) get(ctx context.Context, svrAddr addr.Addr) (server.Server, error) {
	item, err := r.client.HGet(ctx, itemsKey, svrAddr.String()).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return server.Blank, repositories.ErrServerNotFound
		}
		return server.Blank, fmt.Errorf("get: %w", err)
	}
	return decodeServer(item)
}

func (r *Repository) save(ctx context.Context, tx *redis.Tx, svr server.Server) (server.Server, error) {
	// before the server is saved, its version has to be incremented
	svr.Version++

	item, err := json.Marshal(svr) // nolint:musttag
	if err != nil {
		return server.Blank, fmt.Errorf("save: marshal: %w", err)
	}

	_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		svrAddr := svr.Addr.String()
		pipe.HSet(ctx, itemsKey, svrAddr, item)
		pipe.ZAdd(ctx, updatesKey, redis.Z{
			Score:  float64(r.clock.Now().UnixNano()),
			Member: svrAddr,
		})
		if svr.RefreshedAt.IsZero() {
			pipe.ZRem(ctx, refreshesKey, svrAddr)
		} else {
			pipe.ZAdd(ctx, refreshesKey, redis.Z{
				Score:  float64(svr.RefreshedAt.UnixNano()),
				Member: svrAddr,
			})
		}

		// for each available status, add or remove the server from the corresponding set
		// based on the fact that the server has the status or not
		for _, status := range ds.Members() {
			if svr.HasDiscoveryStatus(status) {
				pipe.SAdd(ctx, fmt.Sprintf(statusKeyFmt, status), svrAddr)
			} else {
				pipe.SRem(ctx, fmt.Sprintf(statusKeyFmt, status), svrAddr)
			}
		}
		return nil
	})
	if err != nil {
		return server.Blank, fmt.Errorf("save: redis pipeline: %w", err)
	}

	return svr, nil
}

func (r *Repository) updateExclusive(
	ctx context.Context,
	svr server.Server,
	op func(tx *redis.Tx) error,
) error {
	lockKey := fmt.Sprintf(lockKeyFmt, svr.Addr.String())
	for range r.lockOpts.MaxAttempts {
		err := r.locker.Guard(ctx, lockKey, r.lockOpts.LeaseDuration, func(tx *redis.Tx) error {
			return op(tx)
		})
		if err != nil {
			if errors.Is(err, redislock.ErrNotAcquired) {
				time.Sleep(r.lockOpts.RetryBackoff)
				continue
			}
			return err
		}
		return nil
	}
	return fmt.Errorf("update exclusive: lock not acquired after %d attempts", r.lockOpts.MaxAttempts)
}

func decodeServer(val any) (server.Server, error) {
	var svr server.Server
	encoded, ok := val.(string)
	if !ok {
		return server.Blank, fmt.Errorf("unmashal: unexpected type: %T", val)
	}
	if err := json.Unmarshal([]byte(encoded), &svr); err != nil { // nolint:musttag
		return server.Blank, fmt.Errorf("unmashal: %w", err)
	}
	return svr, nil
}
