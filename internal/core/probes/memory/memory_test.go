package memory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/internal/core/probes"
	"github.com/sergeii/swat4master/internal/core/probes/memory"
	"github.com/sergeii/swat4master/internal/entity/addr"
)

func makeRepo() (*memory.Repository, *clock.Mock) {
	clockMock := clock.NewMock()
	return memory.New(clockMock), clockMock
}

func TestProbesMemoryRepo_Add(t *testing.T) {
	repo, _ := makeRepo()
	ctx := context.Background()

	err := repo.Add(ctx, probes.New(addr.MustNewFromString("1.1.1.1", 10480), 10480, probes.GoalDetails))
	assert.NoError(t, err)
	cnt, _ := repo.Count(ctx)
	assert.Equal(t, 1, cnt)

	err = repo.Add(ctx, probes.New(addr.MustNewFromString("2.2.2.2", 10480), 10480, probes.GoalDetails))
	assert.NoError(t, err)
	cnt, _ = repo.Count(ctx)
	assert.Equal(t, 2, cnt)

	t1, err := repo.Pop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "1.1.1.1", t1.GetDottedIP())

	t2, err := repo.Pop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "2.2.2.2", t2.GetDottedIP())

	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, probes.ErrQueueIsEmpty)
}

func TestProbesMemoryRepo_AddBetween(t *testing.T) {
	repo, clockMock := makeRepo()
	ctx := context.Background()
	now := clockMock.Now()

	err := repo.AddBetween(
		ctx,
		probes.New(addr.MustNewFromString("1.1.1.1", 10480), 10480, probes.GoalDetails),
		now.Add(time.Millisecond*10),
		now.Add(time.Millisecond*50),
	)
	assert.NoError(t, err)

	cnt, _ := repo.Count(ctx)
	assert.Equal(t, 1, cnt)
	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, probes.ErrTargetIsNotReady)

	err = repo.AddBetween(
		ctx,
		probes.New(addr.MustNewFromString("2.2.2.2", 10480), 10480, probes.GoalDetails),
		now.Add(time.Millisecond*10),
		now.Add(time.Millisecond*15),
	)
	assert.NoError(t, err)

	err = repo.AddBetween(
		ctx,
		probes.New(addr.MustNewFromString("3.3.3.3", 10480), 10480, probes.GoalDetails),
		now.Add(time.Millisecond*25),
		now.Add(time.Millisecond*50),
	)
	assert.NoError(t, err)

	cnt, _ = repo.Count(ctx)
	assert.Equal(t, 3, cnt)
	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, probes.ErrTargetIsNotReady)

	clockMock.Add(time.Millisecond * 5)

	// not ready yet
	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, probes.ErrTargetIsNotReady)

	clockMock.Add(time.Millisecond * 15)

	// 1st item is ready
	t1, err := repo.Pop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "1.1.1.1", t1.GetDottedIP())

	// 2nd item has expired
	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, probes.ErrTargetIsNotReady)

	// 3rd item is not ready
	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, probes.ErrTargetIsNotReady)

	clockMock.Add(time.Millisecond * 5)

	// 3rd item is now ready
	t3, err := repo.Pop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "3.3.3.3", t3.GetDottedIP())

	// queue is empty now
	cnt, _ = repo.Count(ctx)
	assert.Equal(t, 0, cnt)
	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, probes.ErrQueueIsEmpty)
}

func TestProbesMemoryRepo_AddBetween_After(t *testing.T) {
	repo, clockMock := makeRepo()
	ctx := context.Background()
	now := clockMock.Now()

	err := repo.AddBetween(
		ctx,
		probes.New(addr.MustNewFromString("1.1.1.1", 10480), 10480, probes.GoalDetails),
		now.Add(time.Millisecond*50),
		probes.NC,
	)
	assert.NoError(t, err)

	cnt, _ := repo.Count(ctx)
	assert.Equal(t, 1, cnt)
	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, probes.ErrTargetIsNotReady)

	err = repo.AddBetween(
		ctx,
		probes.New(addr.MustNewFromString("2.2.2.2", 10480), 10480, probes.GoalDetails),
		now.Add(time.Millisecond*100),
		probes.NC,
	)
	assert.NoError(t, err)
	cnt, _ = repo.Count(ctx)
	assert.Equal(t, 2, cnt)

	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, probes.ErrTargetIsNotReady)

	clockMock.Add(time.Millisecond * 5)
	// not ready yet
	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, probes.ErrTargetIsNotReady)

	// 1st item is ready
	clockMock.Add(time.Millisecond * 50)
	t1, err := repo.Pop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "1.1.1.1", t1.GetDottedIP())

	// 2nd item still not ready
	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, probes.ErrTargetIsNotReady)

	// 2nd item is now ready
	clockMock.Add(time.Millisecond * 50)
	t1, err = repo.Pop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "2.2.2.2", t1.GetDottedIP())

	// queue is empty now
	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, probes.ErrQueueIsEmpty)
}

func TestProbesMemoryRepo_AddBetween_AddBefore(t *testing.T) {
	repo, clockMock := makeRepo()
	ctx := context.Background()
	now := clockMock.Now()

	err := repo.AddBetween(
		ctx,
		probes.New(addr.MustNewFromString("1.1.1.1", 10480), 10480, probes.GoalDetails),
		probes.NC,
		now.Add(time.Millisecond*50),
	)
	assert.NoError(t, err)

	err = repo.AddBetween(
		ctx,
		probes.New(addr.MustNewFromString("2.2.2.2", 10480), 10480, probes.GoalDetails),
		probes.NC,
		now.Add(time.Millisecond*50),
	)
	assert.NoError(t, err)

	cntBeforeSleep, _ := repo.Count(ctx)
	assert.Equal(t, 2, cntBeforeSleep)

	clockMock.Add(time.Millisecond * 10)

	cntAfterSleep, _ := repo.Count(ctx)
	assert.Equal(t, 2, cntAfterSleep)

	t1, err := repo.Pop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "1.1.1.1", t1.GetDottedIP())

	cntAfterPop, _ := repo.Count(ctx)
	assert.Equal(t, 1, cntAfterPop)

	clockMock.Add(time.Millisecond * 41)

	cntAfterPopSleep, _ := repo.Count(ctx)
	assert.Equal(t, 1, cntAfterPopSleep)

	// other target is now expired
	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, probes.ErrQueueIsEmpty)

	cntAfterEmptyPop, _ := repo.Count(ctx)
	assert.Equal(t, 0, cntAfterEmptyPop)
}

func TestProbesMemoryRepo_PopExpired(t *testing.T) {
	repo, clockMock := makeRepo()
	ctx := context.Background()
	now := clockMock.Now()

	repo.AddBetween( // nolint:errcheck
		ctx,
		probes.New(addr.MustNewFromString("1.1.1.1", 10480), 10480, probes.GoalDetails),
		probes.NC,
		now.Add(-time.Millisecond*50),
	)
	repo.AddBetween( // nolint:errcheck
		ctx,
		probes.New(addr.MustNewFromString("2.2.2.2", 10480), 10480, probes.GoalDetails),
		probes.NC,
		now.Add(-time.Millisecond*10),
	)
	cnt, _ := repo.Count(ctx)
	assert.Equal(t, 2, cnt)

	_, err := repo.Pop(ctx)
	assert.ErrorIs(t, err, probes.ErrQueueIsEmpty)
	cnt, _ = repo.Count(ctx)
	assert.Equal(t, 0, cnt)
}

func TestProbesMemoryRepo_Pop(t *testing.T) {
	repo, clockMock := makeRepo()
	ctx := context.Background()
	now := clockMock.Now()

	repo.AddBetween( // nolint:errcheck
		ctx,
		probes.New(addr.MustNewFromString("1.1.1.1", 10480), 10480, probes.GoalDetails),
		now.Add(time.Millisecond*75),
		probes.NC,
	)
	repo.AddBetween( // nolint:errcheck
		ctx,
		probes.New(addr.MustNewFromString("2.2.2.2", 10480), 10480, probes.GoalDetails),
		now.Add(time.Millisecond*50),
		probes.NC,
	)
	repo.Add(ctx, probes.New(addr.MustNewFromString("3.3.3.3", 10480), 10480, probes.GoalDetails)) // nolint:errcheck
	repo.Add(ctx, probes.New(addr.MustNewFromString("4.4.4.4", 10480), 10480, probes.GoalDetails)) // nolint:errcheck
	repo.AddBetween(                                                                               // nolint:errcheck
		ctx,
		probes.New(addr.MustNewFromString("5.5.5.5", 10480), 10480, probes.GoalDetails),
		now.Add(time.Millisecond*100),
		probes.NC,
	)

	popped := make([]string, 0)
	started := make(chan struct{})
	ready := make(chan struct{})

	go func() {
		ticker := clockMock.Ticker(time.Millisecond * 5)
		close(started)
		for range ticker.C {
			tgt, err := repo.Pop(ctx)
			if errors.Is(err, probes.ErrTargetIsNotReady) {
				continue
			} else if errors.Is(err, probes.ErrQueueIsEmpty) {
				return
			}
			popped = append(popped, tgt.GetDottedIP())
		}
	}()

	go func() {
		<-clockMock.After(time.Millisecond * 50)
		close(ready)
	}()

	<-started
	clockMock.Add(time.Second)
	<-ready

	assert.Equal(t, []string{"3.3.3.3", "4.4.4.4", "2.2.2.2", "1.1.1.1", "5.5.5.5"}, popped)

	// queue is empty now
	_, err := repo.Pop(ctx)
	assert.ErrorIs(t, err, probes.ErrQueueIsEmpty)
}

func TestProbesMemoryRepo_PopAny(t *testing.T) {
	repo, clockMock := makeRepo()
	ctx := context.Background()
	now := clockMock.Now()

	repo.AddBetween( // nolint:errcheck
		ctx,
		probes.New(addr.MustNewFromString("1.1.1.1", 10480), 10480, probes.GoalDetails),
		now.Add(time.Millisecond*75),
		probes.NC,
	)
	repo.AddBetween( // nolint:errcheck
		ctx,
		probes.New(addr.MustNewFromString("2.2.2.2", 10480), 10480, probes.GoalDetails),
		now.Add(time.Millisecond*50),
		probes.NC,
	)
	repo.Add(ctx, probes.New(addr.MustNewFromString("3.3.3.3", 10480), 10480, probes.GoalDetails)) // nolint:errcheck
	repo.Add(ctx, probes.New(addr.MustNewFromString("4.4.4.4", 10480), 10480, probes.GoalDetails)) // nolint:errcheck
	repo.AddBetween(                                                                               // nolint:errcheck
		ctx,
		probes.New(addr.MustNewFromString("5.5.5.5", 10480), 10480, probes.GoalDetails),
		now.Add(time.Millisecond*100),
		probes.NC,
	)

	popped := make([]string, 0)
	for i := 0; i < 5; i++ {
		tgt, err := repo.PopAny(ctx)
		require.NoError(t, err)
		popped = append(popped, tgt.GetDottedIP())
	}

	assert.Equal(t, []string{"1.1.1.1", "2.2.2.2", "3.3.3.3", "4.4.4.4", "5.5.5.5"}, popped)

	// queue is empty now
	_, err := repo.Pop(ctx)
	assert.ErrorIs(t, err, probes.ErrQueueIsEmpty)
}

func TestProbesMemoryRepo_PopMany(t *testing.T) {
	tests := []struct {
		name      string
		count     int
		popped    []string
		remaining []string
		expired1  int
		expired2  int
		expired3  int
	}{
		{
			"pop nothing",
			0,
			[]string{},
			[]string{"1.1.1.1", "2.2.2.2", "3.3.3.3", "4.4.4.4", "5.5.5.5"},
			0,
			1,
			2,
		},
		{
			"pop just 1 target",
			1,
			[]string{"3.3.3.3"},
			[]string{"4.4.4.4", "5.5.5.5", "1.1.1.1", "2.2.2.2"},
			1,
			2,
			0,
		},
		{
			"pop exactly as there are targets in queue",
			6,
			[]string{"3.3.3.3", "4.4.4.4", "6.6.6.6"},
			[]string{"1.1.1.1", "2.2.2.2", "5.5.5.5"},
			2,
			0,
			0,
		},
		{
			"pop exactly as there are available targets in queue",
			3,
			[]string{"3.3.3.3", "4.4.4.4", "6.6.6.6"},
			[]string{"1.1.1.1", "2.2.2.2", "5.5.5.5"},
			1,
			1,
			0,
		},
		{
			"pop more targets than in queue",
			10,
			[]string{"3.3.3.3", "4.4.4.4", "6.6.6.6"},
			[]string{"1.1.1.1", "2.2.2.2", "5.5.5.5"},
			2,
			0,
			0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, clockMock := makeRepo()
			ctx := context.Background()
			now := clockMock.Now()

			repo.AddBetween( // nolint:errcheck
				ctx,
				probes.New(addr.MustNewFromString("7.7.7.7", 10480), 10480, probes.GoalDetails),
				probes.NC,
				now.Add(-time.Millisecond*1),
			)
			repo.AddBetween( // nolint:errcheck
				ctx,
				probes.New(addr.MustNewFromString("1.1.1.1", 10480), 10480, probes.GoalDetails),
				now.Add(time.Millisecond*75),
				probes.NC,
			)
			repo.AddBetween( // nolint:errcheck
				ctx,
				probes.New(addr.MustNewFromString("2.2.2.2", 10480), 10480, probes.GoalDetails),
				now.Add(time.Millisecond*50),
				probes.NC,
			)
			repo.Add(ctx, probes.New( // nolint:errcheck
				addr.MustNewFromString("3.3.3.3", 10480),
				10480,
				probes.GoalDetails,
			))
			repo.AddBetween( // nolint:errcheck
				ctx,
				probes.New(addr.MustNewFromString("4.4.4.4", 10480), 10480, probes.GoalDetails),
				probes.NC,
				now.Add(time.Millisecond*150),
			)
			repo.AddBetween( // nolint:errcheck
				ctx,
				probes.New(addr.MustNewFromString("5.5.5.5", 10480), 10480, probes.GoalDetails),
				now.Add(time.Millisecond*100),
				probes.NC,
			)
			repo.AddBetween( // nolint:errcheck
				ctx,
				probes.New(addr.MustNewFromString("6.6.6.6", 10480), 10480, probes.GoalDetails),
				probes.NC,
				now.Add(time.Millisecond*10),
			)
			repo.AddBetween( // nolint:errcheck
				ctx,
				probes.New(addr.MustNewFromString("8.8.8.8", 10480), 10480, probes.GoalDetails),
				probes.NC,
				now.Add(-time.Millisecond*10),
			)

			popped, expired, err := repo.PopMany(ctx, tt.count)
			assert.NoError(t, err)
			assert.Equal(t, tt.popped, getTargetsIPs(popped))
			assert.Equal(t, tt.expired1, expired)

			clockMock.Add(time.Millisecond * 100)
			remaining, expired, err := repo.PopMany(ctx, 5)
			assert.NoError(t, err)
			assert.Equal(t, tt.remaining, getTargetsIPs(remaining))
			assert.Equal(t, tt.expired2, expired)

			// queue is empty now
			maybeMore, expired, _ := repo.PopMany(ctx, 5)
			assert.Equal(t, 0, len(maybeMore))
			assert.Equal(t, tt.expired3, expired)

			count, err := repo.Count(ctx)
			require.NoError(t, err)
			assert.Equal(t, 0, count)
		})
	}
}

func TestProbesMemoryRepo_PopEmpty(t *testing.T) {
	repo, _ := makeRepo()
	_, err := repo.Pop(context.Background())
	assert.ErrorIs(t, err, probes.ErrQueueIsEmpty)
}

func TestProbesMemoryRepo_PopManyEmpty(t *testing.T) {
	repo, _ := makeRepo()
	popped, expired, err := repo.PopMany(context.Background(), 5)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(popped))
	assert.Equal(t, 0, expired)
}

func TestProbesMemoryRepo_Count(t *testing.T) {
	repo, _ := makeRepo()
	ctx := context.Background()

	assertCount := func(expected int) {
		cnt, err := repo.Count(ctx)
		assert.NoError(t, err)
		assert.Equal(t, expected, cnt)
	}

	assertCount(0)

	t1 := probes.New(addr.MustNewFromString("1.1.1.1", 10480), 10480, probes.GoalDetails)
	_ = repo.Add(ctx, t1)
	assertCount(1)

	t2 := probes.New(addr.MustNewFromString("2.2.2.2", 10480), 10480, probes.GoalDetails)
	_ = repo.Add(ctx, t2)
	assertCount(2)

	_, _ = repo.Pop(ctx)
	assertCount(1)

	_, _ = repo.Pop(ctx)
	assertCount(0)

	_, _ = repo.Pop(ctx)
	assertCount(0)

	t3 := probes.New(addr.MustNewFromString("3.3.3.3", 10480), 10480, probes.GoalDetails)
	_ = repo.Add(ctx, t3)
	assertCount(1)
}

func getTargetsIPs(targets []probes.Target) []string {
	ips := make([]string, 0, len(targets))
	for _, tgt := range targets {
		ips = append(ips, tgt.GetDottedIP())
	}
	return ips
}
