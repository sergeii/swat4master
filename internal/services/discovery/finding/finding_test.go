package finding_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	repos "github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/persistence/memory"
	"github.com/sergeii/swat4master/internal/services/discovery/finding"
	"github.com/sergeii/swat4master/internal/services/monitoring"
	sp "github.com/sergeii/swat4master/internal/services/probe"
)

func makeApp(tb fxtest.TB, extra ...fx.Option) {
	fxopts := []fx.Option{
		fx.Provide(func(c clockwork.Clock) (repos.ServerRepository, repos.InstanceRepository, repos.ProbeRepository) {
			mem := memory.New(c)
			return mem.Servers, mem.Instances, mem.Probes
		}),
		fx.Provide(func() *zerolog.Logger {
			logger := zerolog.Nop()
			return &logger
		}),
		fx.Provide(
			monitoring.NewMetricService,
			sp.NewService,
			finding.NewService,
		),
		fx.NopLogger,
	}
	fxopts = append(fxopts, extra...)
	app := fxtest.New(tb, fxopts...)
	app.RequireStart().RequireStop()
}

func provideClock(c clockwork.Clock) fx.Option {
	return fx.Provide(
		func() clockwork.Clock {
			return c
		},
	)
}

func TestFindingService_DiscoverDetails(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()

	var queue repos.ProbeRepository
	var finder *finding.Service
	makeApp(t, fx.Populate(&finder, &queue), provideClock(c))

	deadline := c.Now().Add(time.Millisecond * 10)
	for _, ipaddr := range []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"} {
		err := finder.DiscoverDetails(ctx, addr.MustNewFromDotted(ipaddr, 10480), 10481, deadline)
		assert.NoError(t, err)
	}

	p1, _ := queue.Pop(ctx)
	assert.Equal(t, "1.1.1.1", p1.Addr.GetDottedIP())
	assert.Equal(t, probe.GoalDetails, p1.Goal)
	assert.Equal(t, 10481, p1.Port)

	c.Advance(time.Millisecond * 15)

	_, err := queue.Pop(ctx)
	assert.ErrorIs(t, err, repos.ErrProbeQueueIsEmpty)
}

func TestFindingService_DiscoverPort(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()

	var queue repos.ProbeRepository
	var finder *finding.Service
	makeApp(t, fx.Populate(&finder, &queue), provideClock(c))

	now := c.Now()
	countdown := now.Add(time.Millisecond * 5)
	deadline := now.Add(time.Millisecond * 15)

	for _, ipaddr := range []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"} {
		err := finder.DiscoverPort(ctx, addr.MustNewFromDotted(ipaddr, 10480), countdown, deadline)
		assert.NoError(t, err)
	}

	_, err := queue.Pop(ctx)
	assert.ErrorIs(t, err, repos.ErrProbeIsNotReady)

	c.Advance(time.Millisecond * 5)

	p1, _ := queue.Pop(ctx)
	assert.Equal(t, "1.1.1.1", p1.Addr.GetDottedIP())
	assert.Equal(t, probe.GoalPort, p1.Goal)
	assert.Equal(t, 10480, p1.Port)

	c.Advance(time.Millisecond * 5)

	p2, _ := queue.Pop(ctx)
	assert.Equal(t, "2.2.2.2", p2.Addr.GetDottedIP())
	assert.Equal(t, probe.GoalPort, p2.Goal)
	assert.Equal(t, 10480, p2.Port)

	c.Advance(time.Millisecond * 10)

	_, err = queue.Pop(ctx)
	assert.ErrorIs(t, err, repos.ErrProbeQueueIsEmpty)
}

func TestFindingService_RefreshDetails(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()

	var serversRepo repos.ServerRepository
	var probesRepo repos.ProbeRepository
	var service *finding.Service
	makeApp(t, fx.Populate(&serversRepo, &probesRepo, &service), provideClock(c))

	gs1 := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	gs1.Refresh(c.Now())
	gs1.UpdateDiscoveryStatus(ds.Master)

	gs2 := server.MustNew(net.ParseIP("2.2.2.2"), 10480, 10481)
	gs2.Refresh(c.Now())
	gs2.UpdateDiscoveryStatus(ds.Port)

	gs3 := server.MustNew(net.ParseIP("3.3.3.3"), 10480, 10481)
	gs3.Refresh(c.Now())
	gs3.UpdateDiscoveryStatus(ds.Master | ds.Details | ds.Port)

	gs4 := server.MustNew(net.ParseIP("4.4.4.4"), 10480, 10481)
	gs4.Refresh(c.Now())
	gs4.UpdateDiscoveryStatus(ds.NoDetails)

	gs5 := server.MustNew(net.ParseIP("5.5.5.5"), 10480, 10481)
	gs5.Refresh(c.Now())
	gs5.UpdateDiscoveryStatus(ds.DetailsRetry)

	gs6 := server.MustNew(net.ParseIP("6.6.6.6"), 10480, 10481)
	gs6.Refresh(c.Now())
	gs6.UpdateDiscoveryStatus(ds.Port | ds.Details | ds.DetailsRetry)

	gs7 := server.MustNew(net.ParseIP("7.7.7.7"), 10480, 10481)
	gs7.Refresh(c.Now())
	gs7.UpdateDiscoveryStatus(ds.Master | ds.Info | ds.Details | ds.Port)

	gs1, _ = serversRepo.Add(ctx, gs1, repos.ServerOnConflictIgnore)
	gs2, _ = serversRepo.Add(ctx, gs2, repos.ServerOnConflictIgnore)
	gs3, _ = serversRepo.Add(ctx, gs3, repos.ServerOnConflictIgnore)
	gs4, _ = serversRepo.Add(ctx, gs4, repos.ServerOnConflictIgnore)
	gs5, _ = serversRepo.Add(ctx, gs5, repos.ServerOnConflictIgnore)
	gs6, _ = serversRepo.Add(ctx, gs6, repos.ServerOnConflictIgnore)
	gs7, _ = serversRepo.Add(ctx, gs7, repos.ServerOnConflictIgnore)

	deadline := c.Now().Add(time.Second * 60)

	refreshedCount, err := service.RefreshDetails(ctx, deadline)
	require.NoError(t, err)
	assert.Equal(t, 3, refreshedCount)

	probeCnt, _ := probesRepo.Count(ctx)
	assert.Equal(t, 3, probeCnt)

	refreshedServers := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		prb, err := probesRepo.PopAny(ctx)
		require.NoError(t, err)
		require.Equal(t, probe.GoalDetails, prb.Goal)
		refreshedServers = append(refreshedServers, prb.Addr.GetDottedIP())
	}
	assert.Equal(t, []string{"7.7.7.7", "3.3.3.3", "2.2.2.2"}, refreshedServers)
}

func TestFindingService_ReviveServers(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()

	var serversRepo repos.ServerRepository
	var probesRepo repos.ProbeRepository
	var service *finding.Service
	makeApp(t, fx.Populate(&serversRepo, &probesRepo, &service), provideClock(c))

	c.Advance(time.Millisecond)

	gs1 := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	gs1.Refresh(c.Now())
	gs1.UpdateDiscoveryStatus(ds.Master)

	c.Advance(time.Millisecond)

	gs2 := server.MustNew(net.ParseIP("2.2.2.2"), 10480, 10481)
	gs2.Refresh(c.Now())
	gs2.UpdateDiscoveryStatus(ds.Port)

	before3rd := c.Now()

	c.Advance(time.Millisecond)

	gs3 := server.MustNew(net.ParseIP("3.3.3.3"), 10480, 10481)
	gs3.Refresh(c.Now())
	gs3.UpdateDiscoveryStatus(ds.Master | ds.Details | ds.Port)

	c.Advance(time.Millisecond)

	gs4 := server.MustNew(net.ParseIP("4.4.4.4"), 10480, 10481)
	gs4.Refresh(c.Now())
	gs4.UpdateDiscoveryStatus(ds.NoDetails)

	c.Advance(time.Millisecond)

	gs5 := server.MustNew(net.ParseIP("5.5.5.5"), 10480, 10481)
	gs5.Refresh(c.Now())
	gs5.UpdateDiscoveryStatus(ds.DetailsRetry)

	c.Advance(time.Millisecond)
	gs6 := server.MustNew(net.ParseIP("6.6.6.6"), 10480, 10481)
	gs6.Refresh(c.Now())
	gs6.UpdateDiscoveryStatus(ds.Port | ds.Details | ds.DetailsRetry)

	c.Advance(time.Millisecond)

	gs7 := server.MustNew(net.ParseIP("7.7.7.7"), 10480, 10481)
	gs7.Refresh(c.Now())
	gs7.UpdateDiscoveryStatus(ds.Master | ds.Info | ds.Details)

	c.Advance(time.Millisecond)

	gs8 := server.MustNew(net.ParseIP("8.8.8.8"), 10480, 10481)
	gs8.Refresh(c.Now())
	gs8.UpdateDiscoveryStatus(ds.Master | ds.PortRetry)

	beforeLast := c.Now()

	c.Advance(time.Millisecond)

	gs9 := server.MustNew(net.ParseIP("9.9.9.9"), 10480, 10481)
	gs9.Refresh(c.Now())
	gs9.UpdateDiscoveryStatus(ds.Info)

	gs1, _ = serversRepo.Add(ctx, gs1, repos.ServerOnConflictIgnore)
	gs2, _ = serversRepo.Add(ctx, gs2, repos.ServerOnConflictIgnore)
	gs3, _ = serversRepo.Add(ctx, gs3, repos.ServerOnConflictIgnore)
	gs4, _ = serversRepo.Add(ctx, gs4, repos.ServerOnConflictIgnore)
	gs5, _ = serversRepo.Add(ctx, gs5, repos.ServerOnConflictIgnore)
	gs6, _ = serversRepo.Add(ctx, gs6, repos.ServerOnConflictIgnore)
	gs7, _ = serversRepo.Add(ctx, gs7, repos.ServerOnConflictIgnore)
	gs8, _ = serversRepo.Add(ctx, gs8, repos.ServerOnConflictIgnore)
	gs9, _ = serversRepo.Add(ctx, gs9, repos.ServerOnConflictIgnore)

	now := c.Now()
	minCountdown := now
	maxCountdown := now
	deadline := now.Add(time.Second * 60)

	revivedCnt, err := service.ReviveServers(ctx, before3rd, beforeLast, minCountdown, maxCountdown, deadline)
	require.NoError(t, err)
	assert.Equal(t, 3, revivedCnt)

	probeCnt, _ := probesRepo.Count(ctx)
	assert.Equal(t, 3, probeCnt)

	revivedServers := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		prb, err := probesRepo.PopAny(ctx)
		require.NoError(t, err)
		require.Equal(t, probe.GoalPort, prb.Goal)
		revivedServers = append(revivedServers, prb.Addr.GetDottedIP())
	}
	assert.Equal(t, []string{"7.7.7.7", "5.5.5.5", "4.4.4.4"}, revivedServers)
}
