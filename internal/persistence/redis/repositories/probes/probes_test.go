package probes_test

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jonboulle/clockwork"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/persistence/redis/repositories/probes"
	tu "github.com/sergeii/swat4master/internal/testutils"
	"github.com/sergeii/swat4master/internal/testutils/factories/probefactory"
	"github.com/sergeii/swat4master/internal/testutils/testredis"
	"github.com/sergeii/swat4master/pkg/slice"
)

type qItem struct {
	Probe   probe.Probe `json:"probe"`
	Expires time.Time   `json:"expires"`
}

type qMember struct {
	ID   string
	Time float64
}

type qState struct {
	Queue        []qMember
	QueueMembers map[string]float64
	Items        map[string]qItem
}

func ids(items []qMember) []string {
	itemKeys := make([]string, 0, len(items))
	for _, item := range items {
		itemKeys = append(itemKeys, item.ID)
	}
	return itemKeys
}

func micro(t time.Time) time.Time {
	return t.Truncate(time.Microsecond)
}

func collectQueueState(ctx context.Context, rdb *redis.Client) qState {
	zQueueMembers := tu.Must(rdb.ZRangeWithScores(ctx, "probes:queue", 0, -1).Result())
	hItems := tu.Must(rdb.HGetAll(ctx, "probes:items").Result())

	queue := make([]qMember, 0, len(zQueueMembers))
	queueMembers := make(map[string]float64)
	for _, m := range zQueueMembers {
		queue = append(queue, qMember{ID: m.Member.(string), Time: m.Score}) // nolint:forcetypeassert
		queueMembers[m.Member.(string)] = m.Score                            // nolint:forcetypeassert
	}

	items := make(map[string]qItem)
	for k, v := range hItems {
		var item qItem
		tu.MustNoErr(json.Unmarshal([]byte(v), &item))
		items[k] = item
	}

	return qState{
		Queue:        queue,
		QueueMembers: queueMembers,
		Items:        items,
	}
}

func TestProbesRedisRepo_Add_OK(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	rdb := testredis.MakeClient(t)

	repo := probes.New(rdb, c)
	now := c.Now()

	// Given a probe
	prb1 := probefactory.Build(probefactory.WithRandomServerAddress())

	// When the probe is added using the Add method
	err := repo.Add(ctx, prb1)
	require.NoError(t, err)

	// Then the probe should be placed in the queue and have no time constraints
	state := collectQueueState(ctx, rdb)
	assert.Len(t, state.Queue, 1)
	assert.Len(t, state.Items, 1)
	itemID := slice.First(ids(state.Queue))
	item := state.Items[itemID]
	assert.Equal(t, prb1, item.Probe)
	assert.True(t, item.Expires.IsZero())
	assert.Equal(t, float64(now.UnixNano()), state.QueueMembers[itemID])

	// When the clock is advanced by a small amount of time
	c.Advance(time.Millisecond)
	// and another probe is added to the repository
	prb2 := probefactory.Build(probefactory.WithRandomServerAddress())
	err = repo.Add(ctx, prb2)
	require.NoError(t, err)

	// Then the first probe should still be in the queue and the second probe should be added after it
	state = collectQueueState(ctx, rdb)
	assert.Len(t, state.Queue, 2)
	assert.Len(t, state.Items, 2)
	// the first probe should still be there and be the first one in the queue
	qKeys := ids(state.Queue)
	assert.Equal(t, itemID, qKeys[0])
	// the second probe should be the second one in the queue and have a later expiration time
	otherItemID := slice.First(qKeys[1:])
	otherItem := state.Items[otherItemID]
	assert.Equal(t, state.QueueMembers[otherItemID], float64(now.Add(time.Millisecond).UnixNano()))
	assert.Equal(t, prb2, otherItem.Probe)
	assert.True(t, otherItem.Expires.IsZero())
}

func TestProbesRedisRepo_AddBetween_After(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	rdb := testredis.MakeClient(t)

	repo := probes.New(rdb, c)
	now := c.Now()

	// Given a probe with After time constraint
	prb1 := probefactory.Build(probefactory.WithRandomServerAddress())
	// When the probe is added using the AddBetween method
	err := repo.AddBetween(
		ctx,
		prb1,
		now.Add(time.Millisecond*50),
		repositories.NC,
	)
	require.NoError(t, err)
	// Then the probe should be placed in the queue with the given time constraints
	state := collectQueueState(ctx, rdb)
	assert.Len(t, state.Queue, 1)
	assert.Len(t, state.Items, 1)
	itemID := slice.First(ids(state.Queue))
	item := state.Items[itemID]
	assert.Equal(t, float64(now.Add(time.Millisecond*50).UnixNano()), state.QueueMembers[itemID])
	assert.Equal(t, prb1, item.Probe)
	assert.True(t, item.Expires.IsZero())

	// When more probes are added with After time constraints
	prb2 := probefactory.Build(probefactory.WithRandomServerAddress())
	err = repo.AddBetween(
		ctx,
		prb2,
		now.Add(time.Millisecond*100),
		repositories.NC,
	)
	require.NoError(t, err)

	prb3 := probefactory.Build(probefactory.WithRandomServerAddress())
	err = repo.AddBetween(
		ctx,
		prb3,
		now.Add(time.Millisecond*10),
		repositories.NC,
	)
	require.NoError(t, err)

	// Then the probes should be placed in the queue sorted by their readiness
	state = collectQueueState(ctx, rdb)
	assert.Len(t, state.Queue, 3)
	assert.Len(t, state.Items, 3)

	qKeys := ids(state.Queue)
	assert.Equal(t, prb3, state.Items[qKeys[0]].Probe)
	assert.Equal(t, float64(now.Add(time.Millisecond*10).UnixNano()), state.QueueMembers[qKeys[0]])
	assert.Equal(t, state.Items[qKeys[1]].Probe, prb1)
	assert.Equal(t, float64(now.Add(time.Millisecond*50).UnixNano()), state.QueueMembers[qKeys[1]])
	assert.Equal(t, state.Items[qKeys[2]].Probe, prb2)
	assert.Equal(t, float64(now.Add(time.Millisecond*100).UnixNano()), state.QueueMembers[qKeys[2]])
}

func TestProbesRedisRepo_AddBetween_Before(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	rdb := testredis.MakeClient(t)

	repo := probes.New(rdb, c)
	now := c.Now().UTC()

	// Given a probe with Before time constraint
	prb1 := probefactory.Build(probefactory.WithRandomServerAddress())
	// When the probe is added using the AddBetween method
	err := repo.AddBetween(
		ctx,
		prb1,
		repositories.NC,
		now.Add(time.Millisecond*50),
	)
	require.NoError(t, err)
	// Then the probe should be placed in the queue with the given time constraint
	state := collectQueueState(ctx, rdb)
	assert.Len(t, state.Queue, 1)
	assert.Len(t, state.Items, 1)
	itemID := slice.First(ids(state.Queue))
	item := state.Items[itemID]
	assert.Equal(t, prb1, item.Probe)
	assert.Equal(t, micro(now.Add(time.Millisecond*50)), micro(item.Expires))

	c.Advance(time.Millisecond)

	// When more probes are added with Before time constraints
	prb2 := probefactory.Build(probefactory.WithRandomServerAddress())
	err = repo.AddBetween(
		ctx,
		prb2,
		repositories.NC,
		now.Add(-time.Millisecond*50),
	)
	require.NoError(t, err)

	c.Advance(time.Millisecond)

	prb3 := probefactory.Build(probefactory.WithRandomServerAddress())
	err = repo.AddBetween(
		ctx,
		prb3,
		repositories.NC,
		now.Add(-time.Second),
	)
	require.NoError(t, err)

	// Then the probes should be placed in the expired set sorted by their expiration time
	state = collectQueueState(ctx, rdb)
	assert.Len(t, state.Queue, 3)
	assert.Len(t, state.Items, 3)

	qKeys := ids(state.Queue)
	assert.Equal(t, prb1, state.Items[qKeys[0]].Probe)
	assert.Equal(t, micro(now.Add(time.Millisecond*50)), micro(state.Items[qKeys[0]].Expires))
	assert.Equal(t, prb2, state.Items[qKeys[1]].Probe)
	assert.Equal(t, micro(now.Add(-time.Millisecond*50)), micro(state.Items[qKeys[1]].Expires))
	assert.Equal(t, prb3, state.Items[qKeys[2]].Probe)
	assert.Equal(t, micro(now.Add(-time.Second)), micro(state.Items[qKeys[2]].Expires))
}

func TestProbesRedisRepo_AddBetween_Both(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	rdb := testredis.MakeClient(t)

	repo := probes.New(rdb, c)
	now := c.Now().UTC()

	// Given a probe with time constraints
	prb1 := probefactory.Build(probefactory.WithRandomServerAddress())
	// When the probe is added using the AddBetween method
	err := repo.AddBetween(
		ctx,
		prb1,
		now.Add(time.Millisecond*10),
		now.Add(time.Millisecond*49),
	)
	require.NoError(t, err)
	// Then the probe should be placed in the queue with the given time constraints
	state := collectQueueState(ctx, rdb)
	assert.Len(t, state.Queue, 1)
	assert.Len(t, state.Items, 1)
	itemID := slice.First(ids(state.Queue))
	item := state.Items[itemID]
	assert.Equal(t, state.QueueMembers[itemID], float64(now.Add(time.Millisecond*10).UnixNano()))
	assert.Equal(t, prb1, item.Probe)
	assert.Equal(t, micro(now.Add(time.Millisecond*49)), micro(item.Expires))

	// When more probes are added with time constraints
	prb2 := probefactory.Build(probefactory.WithRandomServerAddress())
	err = repo.AddBetween(
		ctx,
		prb2,
		now.Add(time.Millisecond*15),
		now.Add(time.Millisecond*100),
	)
	require.NoError(t, err)

	prb3 := probefactory.Build(probefactory.WithRandomServerAddress())
	err = repo.AddBetween(
		ctx,
		prb3,
		now.Add(time.Millisecond*1),
		now.Add(time.Millisecond*50),
	)
	require.NoError(t, err)

	prb4 := probefactory.Build(probefactory.WithRandomServerAddress())
	err = repo.AddBetween(
		ctx,
		prb4,
		now.Add(-time.Millisecond*600),
		now.Add(-time.Millisecond*300),
	)
	require.NoError(t, err)

	// Then the probes should be placed in the queue with the given time constraints
	state = collectQueueState(ctx, rdb)
	assert.Len(t, state.Queue, 4)
	assert.Len(t, state.Items, 4)

	// the probes should be in the queue in the order of their readiness,
	// and they should have the correct expiration times
	qKeys := ids(state.Queue)
	assert.Equal(t, state.QueueMembers[qKeys[0]], float64(now.Add(-time.Millisecond*600).UnixNano()))
	assert.Equal(t, prb4, state.Items[qKeys[0]].Probe)
	assert.Equal(t, micro(now.Add(-time.Millisecond*300)), micro(state.Items[qKeys[0]].Expires))
	assert.Equal(t, state.QueueMembers[qKeys[1]], float64(now.Add(time.Millisecond*1).UnixNano()))
	assert.Equal(t, prb3, state.Items[qKeys[1]].Probe)
	assert.Equal(t, micro(now.Add(time.Millisecond*50)), micro(state.Items[qKeys[1]].Expires))
	assert.Equal(t, state.QueueMembers[qKeys[2]], float64(now.Add(time.Millisecond*10).UnixNano()))
	assert.Equal(t, prb1, state.Items[qKeys[2]].Probe)
	assert.Equal(t, micro(now.Add(time.Millisecond*49)), micro(state.Items[qKeys[2]].Expires))
	assert.Equal(t, state.QueueMembers[qKeys[3]], float64(now.Add(time.Millisecond*15).UnixNano()))
	assert.Equal(t, prb2, state.Items[qKeys[3]].Probe)
	assert.Equal(t, micro(now.Add(time.Millisecond*100)), micro(state.Items[qKeys[3]].Expires))
}

func TestProbesRedisRepo_AddBetween_AfterGreaterThanBefore(t *testing.T) {
	tests := []struct {
		name   string
		after  func(now time.Time) time.Time
		before func(now time.Time) time.Time
	}{
		{
			name: "after greater than before",
			after: func(now time.Time) time.Time {
				return now.Add(time.Millisecond * 100)
			},
			before: func(now time.Time) time.Time {
				return now.Add(time.Millisecond * 50)
			},
		},
		{
			name: "after equal to before",
			after: func(now time.Time) time.Time {
				return now.Add(time.Millisecond * 50)
			},
			before: func(now time.Time) time.Time {
				return now.Add(time.Millisecond * 50)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			c := clockwork.NewFakeClock()
			rdb := testredis.MakeClient(t)

			repo := probes.New(rdb, c)
			now := c.Now()

			// Given a probe
			prb := probefactory.Build(probefactory.WithRandomServerAddress())
			// When the probe is added using the AddBetween method
			// with the After time constraint greater than the Before time constraint
			err := repo.AddBetween(ctx, prb, tt.after(now), tt.before(now))
			// Then the probe should not be added to the queue
			require.NoError(t, err)
			state := collectQueueState(ctx, rdb)
			assert.Len(t, state.Queue, 0)
			assert.Len(t, state.Items, 0)
		})
	}
}

func TestProbesRedisRepo_Pop_OK(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	rdb := testredis.MakeClient(t)

	repo := probes.New(rdb, c)
	now := c.Now()

	// Given the repository contains a probe with no time constraints
	prb1 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.Add(ctx, prb1))

	// And another probe that has expired
	prb2 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.AddBetween(ctx, prb2, now.Add(-time.Millisecond*100), now.Add(-time.Millisecond*50)))

	// And another probe added slightly later
	c.Advance(time.Millisecond)
	prb3 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.Add(ctx, prb3))

	// And another probe set to be ready far in the future
	prb4 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.AddBetween(ctx, prb4, now.Add(time.Minute), repositories.NC))

	// And the queue state should be as expected
	state := collectQueueState(ctx, rdb)
	assert.Len(t, state.Queue, 4)
	assert.Len(t, state.Items, 4)

	// When the Pop method is called
	got, err := repo.Pop(ctx)
	require.NoError(t, err)
	// Then the probe with the earliest readiness should be returned
	assert.Equal(t, prb1, got)
	// And the queue should contain the remaining non-expired probes
	state = collectQueueState(ctx, rdb)
	assert.Len(t, state.Queue, 2)
	assert.Len(t, state.Items, 2)

	// When the Pop method is called again
	got, err = repo.Pop(ctx)
	require.NoError(t, err)
	// Then the probe with the next earliest readiness should be returned
	assert.Equal(t, prb3, got)
	// And the queue should contain the remaining non-expired probes
	state = collectQueueState(ctx, rdb)
	assert.Len(t, state.Queue, 1)
	assert.Len(t, state.Items, 1)

	// When the Pop method is called again
	_, err = repo.Pop(ctx)
	// Then it should return an error as the last probe is not yet ready
	require.ErrorIs(t, err, repositories.ErrProbeQueueIsEmpty)
	// And the queue should contain the non-ready probe
	state = collectQueueState(ctx, rdb)
	assert.Len(t, state.Queue, 1)
	assert.Len(t, state.Items, 1)

	// When the time has passed so the last probe is ready to be popped
	c.Advance(time.Minute)
	got, err = repo.Pop(ctx)
	require.NoError(t, err)
	// Then the last probe should be returned
	assert.Equal(t, prb4, got)
	// And the queue should be empty
	state = collectQueueState(ctx, rdb)
	assert.Len(t, state.Queue, 0)
	assert.Len(t, state.Items, 0)

	// When the Pop method is called again
	_, err = repo.Pop(ctx)
	// Then it should return an error as the queue is empty
	require.ErrorIs(t, err, repositories.ErrProbeQueueIsEmpty)
}

func TestProbesRedisRepo_Pop_Empty(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	rdb := testredis.MakeClient(t)

	// Given an empty repository
	repo := probes.New(rdb, c)

	// When the Pop method is called
	_, err := repo.Pop(ctx)
	// Then it should return an error
	assert.ErrorIs(t, err, repositories.ErrProbeQueueIsEmpty)
}

func TestProbesRedisRepo_Pop_Expired(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	rdb := testredis.MakeClient(t)

	repo := probes.New(rdb, c)
	now := c.Now()

	// Given the repository contains expires probes
	prb1 := probefactory.Build(probefactory.WithRandomServerAddress())
	prb2 := probefactory.Build(probefactory.WithRandomServerAddress())

	tu.MustNoErr(repo.AddBetween(ctx, prb1, repositories.NC, now.Add(-time.Millisecond*50)))
	tu.MustNoErr(repo.AddBetween(ctx, prb2, repositories.NC, now.Add(-time.Millisecond)))

	// When the Pop method is called
	_, err := repo.Pop(ctx)
	// Then it should return the same error as when the queue is empty
	assert.ErrorIs(t, err, repositories.ErrProbeQueueIsEmpty)
}

func TestProbesRedisRepo_Pop_NotReady(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	rdb := testredis.MakeClient(t)

	repo := probes.New(rdb, c)
	now := c.Now()

	// Given the repository contains a probe that is not yet ready
	prv := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.AddBetween(ctx, prv, now.Add(time.Millisecond*50), repositories.NC))

	// When the Pop method is called
	_, err := repo.Pop(ctx)
	// Then it should return an error indicating that the probe is not ready
	assert.ErrorIs(t, err, repositories.ErrProbeQueueIsEmpty)
}

func TestProbesRedisRepo_Peek(t *testing.T) {
	tests := []struct {
		name   string
		after  func(now time.Time) time.Time
		before func(now time.Time) time.Time
	}{
		{
			name: "no constraints",
			after: func(_ time.Time) time.Time {
				return repositories.NC
			},
			before: func(_ time.Time) time.Time {
				return repositories.NC
			},
		},
		{
			name: "almost ready",
			after: func(now time.Time) time.Time {
				return now.Add(time.Millisecond)
			},
			before: func(_ time.Time) time.Time {
				return repositories.NC
			},
		},
		{
			name: "soon to be ready",
			after: func(now time.Time) time.Time {
				return now.Add(time.Millisecond * 50)
			},
			before: func(_ time.Time) time.Time {
				return repositories.NC
			},
		},
		{
			name: "expired some time ago",
			after: func(now time.Time) time.Time {
				return now.Add(-time.Second * 600)
			},
			before: func(now time.Time) time.Time {
				return now.Add(-time.Second * 300)
			},
		},
		{
			name: "expired just now",
			after: func(_ time.Time) time.Time {
				return repositories.NC
			},
			before: func(now time.Time) time.Time {
				return now
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			c := clockwork.NewFakeClock()
			rdb := testredis.MakeClient(t)

			repo := probes.New(rdb, c)
			now := c.Now()

			// Given the repository contains a probe with various time constraints
			prb := probefactory.Build(probefactory.WithRandomServerAddress())
			tu.MustNoErr(repo.AddBetween(ctx, prb, tt.after(now), tt.before(now)))

			// And another probe that will be ready far in the future
			other := probefactory.Build(probefactory.WithRandomServerAddress())
			tu.MustNoErr(repo.AddBetween(ctx, other, now.Add(time.Hour*24), repositories.NC))

			// When the Peek method is called
			got, err := repo.Peek(ctx)
			// Then the first available probe should be returned
			require.NoError(t, err)
			assert.Equal(t, prb, got)
			// And the probe should still be in the queue
			state := collectQueueState(ctx, rdb)
			assert.Len(t, state.Queue, 2)
			assert.Len(t, state.Items, 2)
		})
	}
}

func TestProbesRedisRepo_Peek_Empty(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	rdb := testredis.MakeClient(t)

	repo := probes.New(rdb, c)

	// When the Peek method is called
	_, err := repo.Peek(ctx)
	// Then it should return an error
	assert.ErrorIs(t, err, repositories.ErrProbeQueueIsEmpty)
}

func TestProbesRedisRepo_PopMany_OK(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	rdb := testredis.MakeClient(t)

	repo := probes.New(rdb, c)
	now := c.Now()

	// Given the repository contains a probe with no time constraints
	prb1 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.Add(ctx, prb1))

	// And another probe that has expired
	prb2 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.AddBetween(ctx, prb2, repositories.NC, now.Add(-time.Millisecond*50)))

	// And another probe added slightly later
	c.Advance(time.Millisecond)
	prb3 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.Add(ctx, prb3))

	// And another probe set to be ready far in the future
	prb4 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.AddBetween(ctx, prb4, now.Add(time.Minute), repositories.NC))

	// And the queue state should be as expected
	state := collectQueueState(ctx, rdb)
	assert.Len(t, state.Queue, 4)
	assert.Len(t, state.Items, 4)

	// When the PopMany method is called
	popped, expired, err := repo.PopMany(ctx, 3)
	require.NoError(t, err)
	// Then the probes with the earliest readiness should be returned
	assert.Equal(t, []probe.Probe{prb1, prb3}, popped)
	assert.Equal(t, 1, expired)

	// And the queue should contain the remaining non-expired probes
	state = collectQueueState(ctx, rdb)
	assert.Len(t, state.Queue, 1)
	assert.Len(t, state.Items, 1)

	// When the PopMany method is called again in the future
	c.Advance(time.Minute)
	popped, expired, err = repo.PopMany(ctx, 3)
	require.NoError(t, err)
	// Then the last probe should be returned
	assert.Equal(t, []probe.Probe{prb4}, popped)
	assert.Equal(t, 0, expired)
	// And the queue should be empty
	state = collectQueueState(ctx, rdb)
	assert.Len(t, state.Queue, 0)
	assert.Len(t, state.Items, 0)
}

func TestProbesRedisRepo_PopManyAddRace(t *testing.T) { // nolint:gocognit
	ctx := context.TODO()
	c := clockwork.NewRealClock()
	mr := miniredis.RunT(t)

	type produceable struct {
		Probe   probe.Probe
		Expired bool
	}

	testSize := 1000

	// Given a sizeable number of probes to be added and consumed from the repository
	initial := make([]probe.Probe, testSize)
	for i := range initial {
		initial[i] = probefactory.Build(probefactory.WithRandomServerAddress())
	}

	stop := make(chan struct{})
	todo := make(chan produceable, testSize)
	popped := make(chan probe.Probe, testSize)
	var expiredCnt int64

	// And a highly concurrent environment
	// where Add and PopMany operations are performed simultaneously
	produce := func(repo *probes.Repository, todo <-chan produceable) {
		select {
		case <-stop:
			return
		case item := <-todo:
			if item.Expired {
				expiredSecondsAgo := rand.IntN(31) // nolint:gosec
				tu.MustNoErr(
					repo.AddBetween(
						ctx,
						item.Probe,
						repositories.NC,
						c.Now().Add(-time.Millisecond*time.Duration(expiredSecondsAgo)),
					),
				)
			} else {
				tu.MustNoErr(repo.Add(ctx, item.Probe))
			}
		}
	}

	consume := func(repo *probes.Repository, popped chan<- probe.Probe) {
		select {
		case <-stop:
			return
		default:
			popCount := rand.IntN(5) // nolint:gosec
			readyProbes, expired, err := repo.PopMany(ctx, popCount)
			if err != nil {
				panic(err)
			}
			for _, prb := range readyProbes {
				popped <- prb
			}
			atomic.AddInt64(&expiredCnt, int64(expired))
		}
	}

	for range 10 {
		go func() {
			interval := rand.IntN(10) + 1 // nolint:gosec
			ticker := time.NewTicker(time.Millisecond * time.Duration(interval))
			defer ticker.Stop()

			rdb := testredis.MakeClientFromMini(t, mr)
			repo := probes.New(rdb, c)

			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					produce(repo, todo)
				}
			}
		}()
	}

	for range 10 {
		go func() {
			interval := rand.IntN(10) + 1 // nolint:gosec
			ticker := time.NewTicker(time.Millisecond * time.Duration(interval))
			defer ticker.Stop()

			rdb := testredis.MakeClientFromMini(t, mr)
			repo := probes.New(rdb, c)

			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					consume(repo, popped)
				}
			}
		}()
	}

	// When many probes are produced and consumed by the repository from multiple concurrent clients
	go func(itemsToDo []probe.Probe) {
		for idx, prb := range itemsToDo {
			todo <- produceable{
				Probe:   prb,
				Expired: idx%10 == 0, // every 10th probe is expired
			}
		}
	}(initial)

	go func() {
		// The worst case for time to wait is calculated as follows:
		// 10 producers producing every 10ms will produce 1000 probes in 1 second
		// 10 consumers consuming every 10ms will consume 1000 probes in 1 second as well but with some delay,
		// as the queue is not full at the beginning, and the consumers need to wait
		// for the producers to produce the last available probe which may take up to 1 second
		<-time.After(time.Second * 2)
		close(stop)
		close(popped)
	}()

	// And enough time has passed for the producers and consumers to finish
	consumed := make([]probe.Probe, 0, testSize)
	for prb := range popped {
		consumed = append(consumed, prb)
	}

	// Then the non-expired probes should be consumed and the number of expired probes should be as expected
	wantConsumed := make([]probe.Probe, 0, testSize)
	wantExpiredCnt := 0
	for i, prb := range initial {
		if i%10 == 0 {
			wantExpiredCnt++
		} else {
			wantConsumed = append(wantConsumed, prb)
		}
	}
	assert.ElementsMatch(t, wantConsumed, consumed)
	assert.Equal(t, atomic.LoadInt64(&expiredCnt), expiredCnt)

	// and the queue should be exhausted
	rdb := testredis.MakeClientFromMini(t, mr)
	state := collectQueueState(ctx, rdb)
	assert.Len(t, state.Queue, 0)
	assert.Len(t, state.Items, 0)
}

func TestProbesRedisRepo_PopMany_Zero(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	rdb := testredis.MakeClient(t)

	repo := probes.New(rdb, c)
	now := c.Now()

	// Given the repository contains some probes with different time constraints
	prb1 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.Add(ctx, prb1))

	prb2 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.AddBetween(ctx, prb2, now.Add(time.Millisecond), now.Add(time.Millisecond*50)))

	prb3 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.AddBetween(ctx, prb3, now.Add(time.Millisecond*100), repositories.NC))

	prb4 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.AddBetween(ctx, prb4, now.Add(-time.Millisecond*50), repositories.NC))

	prb5 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.AddBetween(ctx, prb5, now.Add(-time.Millisecond*100), now.Add(-time.Millisecond)))

	// When the PopMany method is called with a zero count
	popped, expired, err := repo.PopMany(ctx, 0)
	require.NoError(t, err)
	// Then it should return no probes
	assert.Len(t, popped, 0)
	// And indicate that 1 probe has expired
	assert.Equal(t, 0, expired)

	// And the queue should contain all the probes except the one that has expired
	state := collectQueueState(ctx, rdb)
	assert.Len(t, state.Queue, 5)
	assert.Len(t, state.Items, 5)
	qKeys := ids(state.Queue)
	assert.Equal(t, prb5, state.Items[qKeys[0]].Probe)
	assert.Equal(t, prb4, state.Items[qKeys[1]].Probe)
	assert.Equal(t, prb1, state.Items[qKeys[2]].Probe)
	assert.Equal(t, prb2, state.Items[qKeys[3]].Probe)
	assert.Equal(t, prb3, state.Items[qKeys[4]].Probe)
}

func TestProbesRedisRepo_PopMany_NotReady(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	rdb := testredis.MakeClient(t)

	repo := probes.New(rdb, c)
	now := c.Now()

	// Given the repository contains probes that are not yet ready
	prb1 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.AddBetween(ctx, prb1, now.Add(time.Millisecond*50), repositories.NC))

	prb2 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.AddBetween(ctx, prb2, now.Add(time.Millisecond*100), repositories.NC))

	// When the PopMany method is called
	popped, expired, err := repo.PopMany(ctx, 3)
	require.NoError(t, err)
	// Then it should return no probes
	assert.Len(t, popped, 0)
	assert.Equal(t, 0, expired)

	// And the queue should still contain both non-ready probes
	state := collectQueueState(ctx, rdb)
	assert.Len(t, state.Queue, 2)
	assert.Len(t, state.Items, 2)

	// And when the time has passed so the first probe is ready
	c.Advance(time.Millisecond * 51)
	popped, expired, err = repo.PopMany(ctx, 3)
	require.NoError(t, err)
	// Then it should return the first probe
	assert.Len(t, popped, 1)
	assert.Equal(t, prb1, popped[0])
	assert.Equal(t, 0, expired)
	// And the queue should contain the remaining non-ready probe
	state = collectQueueState(ctx, rdb)
	assert.Len(t, state.Queue, 1)
	assert.Len(t, state.Items, 1)

	// And when the time has passed so the second probe is ready
	c.Advance(time.Millisecond * 50)
	popped, expired, err = repo.PopMany(ctx, 3)
	require.NoError(t, err)
	// Then it should return the second probe
	assert.Len(t, popped, 1)
	assert.Equal(t, prb2, popped[0])
	assert.Equal(t, 0, expired)
	// And the queue should be empty
	state = collectQueueState(ctx, rdb)
	assert.Len(t, state.Queue, 0)
	assert.Len(t, state.Items, 0)
}

func TestProbesRedisRepo_PopMany_Expired(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	rdb := testredis.MakeClient(t)

	repo := probes.New(rdb, c)
	now := c.Now()

	// Given the repository contains only probes that have expired
	prb1 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.AddBetween(ctx, prb1, repositories.NC, now.Add(-time.Millisecond*50)))

	prb2 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.AddBetween(ctx, prb2, repositories.NC, now.Add(-time.Second)))

	// When the PopMany method is called
	popped, expired, err := repo.PopMany(ctx, 3)
	require.NoError(t, err)
	// Then it should return the expired probe count but no probes
	assert.Len(t, popped, 0)
	assert.Equal(t, 2, expired)

	// And the queue should be empty
	state := collectQueueState(ctx, rdb)
	assert.Len(t, state.Queue, 0)
	assert.Len(t, state.Items, 0)
}

func TestProbesRedisRepo_PopMany_Mixed(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	rdb := testredis.MakeClient(t)

	repo := probes.New(rdb, c)
	now := c.Now()

	// Given the repository contains probes that both have ready, expired, and not yet ready
	prb1 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.Add(ctx, prb1))

	prb2 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.AddBetween(ctx, prb2, repositories.NC, now.Add(-time.Millisecond*50)))

	prb3 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.AddBetween(ctx, prb3, now.Add(time.Millisecond*50), repositories.NC))

	prb4 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.AddBetween(ctx, prb4, now.Add(time.Millisecond), repositories.NC))

	prb5 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.AddBetween(ctx, prb5, now.Add(time.Millisecond*2), repositories.NC))

	prb6 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.AddBetween(ctx, prb6, now.Add(time.Millisecond*3), repositories.NC))

	c.Advance(time.Millisecond * 10)

	// When the PopMany method is called
	popped, expired, err := repo.PopMany(ctx, 3)
	require.NoError(t, err)
	// Then it should return the ready probes skipping the expired one
	assert.Len(t, popped, 3)
	assert.Equal(t, []probe.Probe{prb1, prb4, prb5}, popped)
	assert.Equal(t, 1, expired)

	// And the queue should contain the non-ready probe
	state := collectQueueState(ctx, rdb)
	assert.Len(t, state.Queue, 2)
	assert.Len(t, state.Items, 2)
	qKeys := ids(state.Queue)
	assert.Equal(t, prb6, state.Items[qKeys[0]].Probe)
	assert.Equal(t, prb3, state.Items[qKeys[1]].Probe)

	// When the time has passed so the last probes are ready to be popped
	c.Advance(time.Millisecond * 41)
	popped, expired, err = repo.PopMany(ctx, 3)
	require.NoError(t, err)
	// Then it should return the last probes
	assert.Len(t, popped, 2)
	assert.Equal(t, []probe.Probe{prb6, prb3}, popped)
	assert.Equal(t, 0, expired)
}

func TestProbesRedisRepo_PopMany_Empty(t *testing.T) {
	tests := []struct {
		name  string
		count int
	}{
		{"pop nothing", 0},
		{"pop 1 probe", 1},
		{"pop 5 probes", 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			c := clockwork.NewFakeClock()
			rdb := testredis.MakeClient(t)

			repo := probes.New(rdb, c)

			popped, expired, err := repo.PopMany(ctx, tt.count)
			require.NoError(t, err)
			assert.Equal(t, 0, len(popped))
			assert.Equal(t, 0, expired)
		})
	}
}

func TestProbesRedisRepo_Count(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	rdb := testredis.MakeClient(t)

	repo := probes.New(rdb, c)
	now := c.Now()

	assertCount := func(expected int) {
		cnt, err := repo.Count(ctx)
		assert.NoError(t, err)
		assert.Equal(t, expected, cnt)
	}

	// Given no probes in the repository yet
	// When the Count method is called
	// Then the count should be 0
	assertCount(0)

	// When a probe is added to the repository
	prb1 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.Add(ctx, prb1))
	// Then the count should be 1
	assertCount(1)

	// When another probe is added to the repository
	prb2 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.Add(ctx, prb2))
	// Then the count should be 2
	assertCount(2)

	// When the repository contains an expired probe
	prb3 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.AddBetween(ctx, prb3, repositories.NC, now.Add(-time.Second*600)))
	// Then the count should account for the expired probe
	assertCount(3)

	// When the repository contains a probe that is not yet ready
	prb4 := probefactory.Build(probefactory.WithRandomServerAddress())
	tu.MustNoErr(repo.AddBetween(ctx, prb4, now.Add(time.Second*600), now.Add(time.Second*700)))
	// Then the count should account for the not ready probe
	assertCount(4)

	// When multiple probes are popped from the repository
	items, expired, _ := repo.PopMany(ctx, 5)
	assert.Len(t, items, 2)
	assert.Equal(t, 1, expired)
	// Then the count should be decremented by the number of popped and expired probes
	assertCount(1)

	// When the time has passed so the last probe is ready to be popped
	c.Advance(time.Second * 601)
	tu.Must(repo.Pop(ctx))
	// Then the count should be 0
	assertCount(0)
}
