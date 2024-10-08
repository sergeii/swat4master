package probes_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/persistence/memory/probes"
)

func TestProbesMemoryRepo_Add(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	repo := probes.New(c)

	err := repo.Add(ctx, probe.New(addr.MustNewFromDotted("1.1.1.1", 10480), 10480, probe.GoalDetails, 0))
	assert.NoError(t, err)
	cnt, _ := repo.Count(ctx)
	assert.Equal(t, 1, cnt)

	err = repo.Add(ctx, probe.New(addr.MustNewFromDotted("2.2.2.2", 10480), 10480, probe.GoalDetails, 0))
	assert.NoError(t, err)
	cnt, _ = repo.Count(ctx)
	assert.Equal(t, 2, cnt)

	p1, err := repo.Pop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "1.1.1.1", p1.Addr.GetDottedIP())

	p2, err := repo.Pop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "2.2.2.2", p2.Addr.GetDottedIP())

	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, repositories.ErrProbeQueueIsEmpty)
}

func TestProbesMemoryRepo_AddBetween(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	repo := probes.New(c)
	now := c.Now()

	err := repo.AddBetween(
		ctx,
		probe.New(addr.MustNewFromDotted("1.1.1.1", 10480), 10480, probe.GoalDetails, 0),
		now.Add(time.Millisecond*10),
		now.Add(time.Millisecond*50),
	)
	assert.NoError(t, err)

	cnt, _ := repo.Count(ctx)
	assert.Equal(t, 1, cnt)
	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, repositories.ErrProbeIsNotReady)

	err = repo.AddBetween(
		ctx,
		probe.New(addr.MustNewFromDotted("2.2.2.2", 10480), 10480, probe.GoalDetails, 0),
		now.Add(time.Millisecond*10),
		now.Add(time.Millisecond*15),
	)
	assert.NoError(t, err)

	err = repo.AddBetween(
		ctx,
		probe.New(addr.MustNewFromDotted("3.3.3.3", 10480), 10480, probe.GoalDetails, 0),
		now.Add(time.Millisecond*25),
		now.Add(time.Millisecond*50),
	)
	assert.NoError(t, err)

	cnt, _ = repo.Count(ctx)
	assert.Equal(t, 3, cnt)
	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, repositories.ErrProbeIsNotReady)

	c.Advance(time.Millisecond * 5)

	// not ready yet
	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, repositories.ErrProbeIsNotReady)

	c.Advance(time.Millisecond * 15)

	// 1st item is ready
	p1, err := repo.Pop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "1.1.1.1", p1.Addr.GetDottedIP())

	// 2nd item has expired
	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, repositories.ErrProbeIsNotReady)

	// 3rd item is not ready
	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, repositories.ErrProbeIsNotReady)

	c.Advance(time.Millisecond * 5)

	// 3rd item is now ready
	p3, err := repo.Pop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "3.3.3.3", p3.Addr.GetDottedIP())

	// queue is empty now
	cnt, _ = repo.Count(ctx)
	assert.Equal(t, 0, cnt)
	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, repositories.ErrProbeQueueIsEmpty)
}

func TestProbesMemoryRepo_AddBetween_After(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	repo := probes.New(c)
	now := c.Now()

	err := repo.AddBetween(
		ctx,
		probe.New(addr.MustNewFromDotted("1.1.1.1", 10480), 10480, probe.GoalDetails, 0),
		now.Add(time.Millisecond*50),
		repositories.NC,
	)
	assert.NoError(t, err)

	cnt, _ := repo.Count(ctx)
	assert.Equal(t, 1, cnt)
	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, repositories.ErrProbeIsNotReady)

	err = repo.AddBetween(
		ctx,
		probe.New(addr.MustNewFromDotted("2.2.2.2", 10480), 10480, probe.GoalDetails, 0),
		now.Add(time.Millisecond*100),
		repositories.NC,
	)
	assert.NoError(t, err)
	cnt, _ = repo.Count(ctx)
	assert.Equal(t, 2, cnt)

	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, repositories.ErrProbeIsNotReady)

	c.Advance(time.Millisecond * 5)
	// not ready yet
	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, repositories.ErrProbeIsNotReady)

	// 1st item is ready
	c.Advance(time.Millisecond * 50)
	p1, err := repo.Pop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "1.1.1.1", p1.Addr.GetDottedIP())

	// 2nd item still not ready
	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, repositories.ErrProbeIsNotReady)

	// 2nd item is now ready
	c.Advance(time.Millisecond * 50)
	p1, err = repo.Pop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "2.2.2.2", p1.Addr.GetDottedIP())

	// queue is empty now
	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, repositories.ErrProbeQueueIsEmpty)
}

func TestProbesMemoryRepo_AddBetween_AddBefore(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	repo := probes.New(c)
	now := c.Now()

	err := repo.AddBetween(
		ctx,
		probe.New(addr.MustNewFromDotted("1.1.1.1", 10480), 10480, probe.GoalDetails, 0),
		repositories.NC,
		now.Add(time.Millisecond*50),
	)
	assert.NoError(t, err)

	err = repo.AddBetween(
		ctx,
		probe.New(addr.MustNewFromDotted("2.2.2.2", 10480), 10480, probe.GoalDetails, 0),
		repositories.NC,
		now.Add(time.Millisecond*50),
	)
	assert.NoError(t, err)

	cntBeforeSleep, _ := repo.Count(ctx)
	assert.Equal(t, 2, cntBeforeSleep)

	c.Advance(time.Millisecond * 10)

	cntAfterSleep, _ := repo.Count(ctx)
	assert.Equal(t, 2, cntAfterSleep)

	p1, err := repo.Pop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "1.1.1.1", p1.Addr.GetDottedIP())

	cntAfterPop, _ := repo.Count(ctx)
	assert.Equal(t, 1, cntAfterPop)

	c.Advance(time.Millisecond * 41)

	cntAfterPopSleep, _ := repo.Count(ctx)
	assert.Equal(t, 1, cntAfterPopSleep)

	// other probe is now expired
	_, err = repo.Pop(ctx)
	assert.ErrorIs(t, err, repositories.ErrProbeQueueIsEmpty)

	cntAfterEmptyPop, _ := repo.Count(ctx)
	assert.Equal(t, 0, cntAfterEmptyPop)
}

func TestProbesMemoryRepo_PopExpired(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	repo := probes.New(c)
	now := c.Now()

	repo.AddBetween( // nolint:errcheck
		ctx,
		probe.New(addr.MustNewFromDotted("1.1.1.1", 10480), 10480, probe.GoalDetails, 0),
		repositories.NC,
		now.Add(-time.Millisecond*50),
	)
	repo.AddBetween( // nolint:errcheck
		ctx,
		probe.New(addr.MustNewFromDotted("2.2.2.2", 10480), 10480, probe.GoalDetails, 0),
		repositories.NC,
		now.Add(-time.Millisecond*10),
	)
	cnt, _ := repo.Count(ctx)
	assert.Equal(t, 2, cnt)

	_, err := repo.Pop(ctx)
	assert.ErrorIs(t, err, repositories.ErrProbeQueueIsEmpty)
	cnt, _ = repo.Count(ctx)
	assert.Equal(t, 0, cnt)
}

func TestProbesMemoryRepo_Pop(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	repo := probes.New(c)
	now := c.Now()

	repo.AddBetween( // nolint:errcheck
		ctx,
		probe.New(addr.MustNewFromDotted("1.1.1.1", 10480), 10480, probe.GoalDetails, 0),
		now.Add(time.Millisecond*75),
		repositories.NC,
	)
	repo.AddBetween( // nolint:errcheck
		ctx,
		probe.New(addr.MustNewFromDotted("2.2.2.2", 10480), 10480, probe.GoalDetails, 0),
		now.Add(time.Millisecond*50),
		repositories.NC,
	)
	repo.Add(ctx, probe.New(addr.MustNewFromDotted("3.3.3.3", 10480), 10480, probe.GoalDetails, 0)) // nolint:errcheck
	repo.Add(ctx, probe.New(addr.MustNewFromDotted("4.4.4.4", 10480), 10480, probe.GoalDetails, 0)) // nolint:errcheck
	repo.AddBetween(                                                                                // nolint:errcheck
		ctx,
		probe.New(addr.MustNewFromDotted("5.5.5.5", 10480), 10480, probe.GoalDetails, 0),
		now.Add(time.Millisecond*100),
		repositories.NC,
	)

	popped := make([]string, 0)
	started := make(chan struct{})
	ready := make(chan struct{})

	advanced := make(chan bool)
	go func() {
		close(started)
		for range advanced {
			prb, err := repo.Pop(ctx)
			if errors.Is(err, repositories.ErrProbeIsNotReady) {
				continue
			} else if errors.Is(err, repositories.ErrProbeQueueIsEmpty) {
				continue
			}
			popped = append(popped, prb.Addr.GetDottedIP())
		}
		close(ready)
	}()

	<-started
	// advance 100ms in steps
	for range 100 {
		c.Advance(time.Millisecond * 1)
		advanced <- true
	}
	close(advanced)
	<-ready

	assert.Equal(t, []string{"3.3.3.3", "4.4.4.4", "2.2.2.2", "1.1.1.1", "5.5.5.5"}, popped)

	// queue is empty now
	_, err := repo.Pop(ctx)
	assert.ErrorIs(t, err, repositories.ErrProbeQueueIsEmpty)
}

func TestProbesMemoryRepo_PopAny(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	repo := probes.New(c)
	now := c.Now()

	repo.AddBetween( // nolint:errcheck
		ctx,
		probe.New(addr.MustNewFromDotted("1.1.1.1", 10480), 10480, probe.GoalDetails, 0),
		now.Add(time.Millisecond*75),
		repositories.NC,
	)
	repo.AddBetween( // nolint:errcheck
		ctx,
		probe.New(addr.MustNewFromDotted("2.2.2.2", 10480), 10480, probe.GoalDetails, 0),
		now.Add(time.Millisecond*50),
		repositories.NC,
	)
	repo.Add(ctx, probe.New(addr.MustNewFromDotted("3.3.3.3", 10480), 10480, probe.GoalDetails, 0)) // nolint:errcheck
	repo.Add(ctx, probe.New(addr.MustNewFromDotted("4.4.4.4", 10480), 10480, probe.GoalDetails, 0)) // nolint:errcheck
	repo.AddBetween(                                                                                // nolint:errcheck
		ctx,
		probe.New(addr.MustNewFromDotted("5.5.5.5", 10480), 10480, probe.GoalDetails, 0),
		now.Add(time.Millisecond*100),
		repositories.NC,
	)

	popped := make([]string, 0)
	for range 5 {
		prb, err := repo.PopAny(ctx)
		require.NoError(t, err)
		popped = append(popped, prb.Addr.GetDottedIP())
	}

	assert.Equal(t, []string{"1.1.1.1", "2.2.2.2", "3.3.3.3", "4.4.4.4", "5.5.5.5"}, popped)

	// queue is empty now
	_, err := repo.Pop(ctx)
	assert.ErrorIs(t, err, repositories.ErrProbeQueueIsEmpty)
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
			"pop just 1 probe",
			1,
			[]string{"3.3.3.3"},
			[]string{"4.4.4.4", "5.5.5.5", "1.1.1.1", "2.2.2.2"},
			1,
			2,
			0,
		},
		{
			"pop exactly as there are probes in queue",
			6,
			[]string{"3.3.3.3", "4.4.4.4", "6.6.6.6"},
			[]string{"1.1.1.1", "2.2.2.2", "5.5.5.5"},
			2,
			0,
			0,
		},
		{
			"pop exactly as there are available probes in queue",
			3,
			[]string{"3.3.3.3", "4.4.4.4", "6.6.6.6"},
			[]string{"1.1.1.1", "2.2.2.2", "5.5.5.5"},
			1,
			1,
			0,
		},
		{
			"pop more probes than in queue",
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
			ctx := context.TODO()

			c := clockwork.NewFakeClock()
			repo := probes.New(c)
			now := c.Now()

			repo.AddBetween( // nolint:errcheck
				ctx,
				probe.New(addr.MustNewFromDotted("7.7.7.7", 10480), 10480, probe.GoalDetails, 0),
				repositories.NC,
				now.Add(-time.Millisecond*1),
			)
			repo.AddBetween( // nolint:errcheck
				ctx,
				probe.New(addr.MustNewFromDotted("1.1.1.1", 10480), 10480, probe.GoalDetails, 0),
				now.Add(time.Millisecond*75),
				repositories.NC,
			)
			repo.AddBetween( // nolint:errcheck
				ctx,
				probe.New(addr.MustNewFromDotted("2.2.2.2", 10480), 10480, probe.GoalDetails, 0),
				now.Add(time.Millisecond*50),
				repositories.NC,
			)
			repo.Add(ctx, probe.New( // nolint:errcheck
				addr.MustNewFromDotted("3.3.3.3", 10480),
				10480,
				probe.GoalDetails,
				0,
			))
			repo.AddBetween( // nolint:errcheck
				ctx,
				probe.New(addr.MustNewFromDotted("4.4.4.4", 10480), 10480, probe.GoalDetails, 0),
				repositories.NC,
				now.Add(time.Millisecond*150),
			)
			repo.AddBetween( // nolint:errcheck
				ctx,
				probe.New(addr.MustNewFromDotted("5.5.5.5", 10480), 10480, probe.GoalDetails, 0),
				now.Add(time.Millisecond*100),
				repositories.NC,
			)
			repo.AddBetween( // nolint:errcheck
				ctx,
				probe.New(addr.MustNewFromDotted("6.6.6.6", 10480), 10480, probe.GoalDetails, 0),
				repositories.NC,
				now.Add(time.Millisecond*10),
			)
			repo.AddBetween( // nolint:errcheck
				ctx,
				probe.New(addr.MustNewFromDotted("8.8.8.8", 10480), 10480, probe.GoalDetails, 0),
				repositories.NC,
				now.Add(-time.Millisecond*10),
			)

			popped, expired, err := repo.PopMany(ctx, tt.count)
			assert.NoError(t, err)
			assert.Equal(t, tt.popped, getProbesIPs(popped))
			assert.Equal(t, tt.expired1, expired)

			c.Advance(time.Millisecond * 100)

			remaining, expired, err := repo.PopMany(ctx, 5)
			assert.NoError(t, err)
			assert.Equal(t, tt.remaining, getProbesIPs(remaining))
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
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	repo := probes.New(c)
	_, err := repo.Pop(ctx)
	assert.ErrorIs(t, err, repositories.ErrProbeQueueIsEmpty)
}

func TestProbesMemoryRepo_PopManyEmpty(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	repo := probes.New(c)

	popped, expired, err := repo.PopMany(ctx, 5)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(popped))
	assert.Equal(t, 0, expired)
}

func TestProbesMemoryRepo_Count(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	repo := probes.New(c)

	assertCount := func(expected int) {
		cnt, err := repo.Count(ctx)
		assert.NoError(t, err)
		assert.Equal(t, expected, cnt)
	}

	assertCount(0)

	t1 := probe.New(addr.MustNewFromDotted("1.1.1.1", 10480), 10480, probe.GoalDetails, 0)
	_ = repo.Add(ctx, t1)
	assertCount(1)

	t2 := probe.New(addr.MustNewFromDotted("2.2.2.2", 10480), 10480, probe.GoalDetails, 0)
	_ = repo.Add(ctx, t2)
	assertCount(2)

	_, _ = repo.Pop(ctx)
	assertCount(1)

	_, _ = repo.Pop(ctx)
	assertCount(0)

	_, _ = repo.Pop(ctx)
	assertCount(0)

	t3 := probe.New(addr.MustNewFromDotted("3.3.3.3", 10480), 10480, probe.GoalDetails, 0)
	_ = repo.Add(ctx, t3)
	assertCount(1)
}

func getProbesIPs(probes []probe.Probe) []string {
	ips := make([]string, 0, len(probes))
	for _, prb := range probes {
		ips = append(ips, prb.Addr.GetDottedIP())
	}
	return ips
}
