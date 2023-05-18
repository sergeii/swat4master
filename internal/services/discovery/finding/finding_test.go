package finding_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/sergeii/swat4master/internal/core/probes"
	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/entity/addr"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
	"github.com/sergeii/swat4master/internal/persistence/memory"
	"github.com/sergeii/swat4master/internal/services/discovery/finding"
	"github.com/sergeii/swat4master/internal/services/monitoring"
	"github.com/sergeii/swat4master/internal/services/probe"
)

func makeApp(tb fxtest.TB, extra ...fx.Option) {
	fxopts := []fx.Option{
		fx.Provide(memory.New),
		fx.Provide(func() *zerolog.Logger {
			logger := zerolog.Nop()
			return &logger
		}),
		fx.Provide(
			monitoring.NewMetricService,
			probe.NewService,
			finding.NewService,
		),
		fx.NopLogger,
	}
	fxopts = append(fxopts, extra...)
	app := fxtest.New(tb, fxopts...)
	app.RequireStart().RequireStop()
}

func provideClock(c clock.Clock) fx.Option {
	return fx.Provide(
		func() clock.Clock {
			return c
		},
	)
}

func TestFindingService_DiscoverDetails(t *testing.T) {
	ctx := context.TODO()
	clockMock := clock.NewMock()

	var queue probes.Repository
	var finder *finding.Service
	makeApp(t, fx.Populate(&finder, &queue), provideClock(clockMock))

	deadline := clockMock.Now().Add(time.Millisecond * 10)

	for _, ipaddr := range []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"} {
		err := finder.DiscoverDetails(ctx, addr.MustNewFromString(ipaddr, 10480), 10481, deadline)
		assert.NoError(t, err)
	}

	t1, _ := queue.Pop(ctx)
	assert.Equal(t, "1.1.1.1", t1.GetDottedIP())
	assert.Equal(t, probes.GoalDetails, t1.GetGoal())
	assert.Equal(t, 10481, t1.GetPort())

	clockMock.Add(time.Millisecond * 15)

	_, err := queue.Pop(ctx)
	assert.ErrorIs(t, err, probes.ErrQueueIsEmpty)
}

func TestFindingService_DiscoverPort(t *testing.T) {
	ctx := context.TODO()
	clockMock := clock.NewMock()

	var queue probes.Repository
	var finder *finding.Service
	makeApp(t, fx.Populate(&finder, &queue), provideClock(clockMock))

	countdown := clockMock.Now().Add(time.Millisecond * 5)
	deadline := clockMock.Now().Add(time.Millisecond * 15)

	for _, ipaddr := range []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"} {
		err := finder.DiscoverPort(ctx, addr.MustNewFromString(ipaddr, 10480), countdown, deadline)
		assert.NoError(t, err)
	}

	_, err := queue.Pop(ctx)
	assert.ErrorIs(t, err, probes.ErrTargetIsNotReady)

	clockMock.Add(time.Millisecond * 5)

	t1, _ := queue.Pop(ctx)
	assert.Equal(t, "1.1.1.1", t1.GetDottedIP())
	assert.Equal(t, probes.GoalPort, t1.GetGoal())
	assert.Equal(t, 10480, t1.GetPort())

	clockMock.Add(time.Millisecond * 5)

	t2, _ := queue.Pop(ctx)
	assert.Equal(t, "2.2.2.2", t2.GetDottedIP())
	assert.Equal(t, probes.GoalPort, t2.GetGoal())
	assert.Equal(t, 10480, t2.GetPort())

	clockMock.Add(time.Millisecond * 10)

	_, err = queue.Pop(ctx)
	assert.ErrorIs(t, err, probes.ErrQueueIsEmpty)
}

func TestFindingService_RefreshDetails(t *testing.T) {
	ctx := context.TODO()
	clockMock := clock.NewMock()

	var serversRepo servers.Repository
	var probesRepo probes.Repository
	var service *finding.Service
	makeApp(t, fx.Populate(&serversRepo, &probesRepo, &service), provideClock(clockMock))

	gs1, _ := servers.New(net.ParseIP("1.1.1.1"), 10480, 10481)
	gs1.Refresh(clockMock.Now())
	gs1.UpdateDiscoveryStatus(ds.Master)

	gs2, _ := servers.New(net.ParseIP("2.2.2.2"), 10480, 10481)
	gs2.Refresh(clockMock.Now())
	gs2.UpdateDiscoveryStatus(ds.Port)

	gs3, _ := servers.New(net.ParseIP("3.3.3.3"), 10480, 10481)
	gs3.Refresh(clockMock.Now())
	gs3.UpdateDiscoveryStatus(ds.Master | ds.Details | ds.Port)

	gs4, _ := servers.New(net.ParseIP("4.4.4.4"), 10480, 10481)
	gs4.Refresh(clockMock.Now())
	gs4.UpdateDiscoveryStatus(ds.NoDetails)

	gs5, _ := servers.New(net.ParseIP("5.5.5.5"), 10480, 10481)
	gs5.Refresh(clockMock.Now())
	gs5.UpdateDiscoveryStatus(ds.DetailsRetry)

	gs6, _ := servers.New(net.ParseIP("6.6.6.6"), 10480, 10481)
	gs6.Refresh(clockMock.Now())
	gs6.UpdateDiscoveryStatus(ds.Port | ds.Details | ds.DetailsRetry)

	gs7, _ := servers.New(net.ParseIP("7.7.7.7"), 10480, 10481)
	gs7.Refresh(clockMock.Now())
	gs7.UpdateDiscoveryStatus(ds.Master | ds.Info | ds.Details | ds.Port)

	gs1, _ = serversRepo.Add(ctx, gs1, servers.OnConflictIgnore)
	gs2, _ = serversRepo.Add(ctx, gs2, servers.OnConflictIgnore)
	gs3, _ = serversRepo.Add(ctx, gs3, servers.OnConflictIgnore)
	gs4, _ = serversRepo.Add(ctx, gs4, servers.OnConflictIgnore)
	gs5, _ = serversRepo.Add(ctx, gs5, servers.OnConflictIgnore)
	gs6, _ = serversRepo.Add(ctx, gs6, servers.OnConflictIgnore)
	gs7, _ = serversRepo.Add(ctx, gs7, servers.OnConflictIgnore)

	deadline := clockMock.Now().Add(time.Second * 60)

	refreshedCount, err := service.RefreshDetails(ctx, deadline)
	require.NoError(t, err)
	assert.Equal(t, 3, refreshedCount)

	probeCnt, _ := probesRepo.Count(ctx)
	assert.Equal(t, 3, probeCnt)

	refreshedServers := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		tgt, err := probesRepo.PopAny(ctx)
		require.NoError(t, err)
		require.Equal(t, probes.GoalDetails, tgt.GetGoal())
		refreshedServers = append(refreshedServers, tgt.GetDottedIP())
	}
	assert.Equal(t, []string{"7.7.7.7", "3.3.3.3", "2.2.2.2"}, refreshedServers)
}

func TestFindingService_ReviveServers(t *testing.T) {
	ctx := context.TODO()
	clockMock := clock.NewMock()

	var serversRepo servers.Repository
	var probesRepo probes.Repository
	var service *finding.Service
	makeApp(t, fx.Populate(&serversRepo, &probesRepo, &service), provideClock(clockMock))

	clockMock.Add(time.Millisecond)
	gs1, _ := servers.New(net.ParseIP("1.1.1.1"), 10480, 10481)
	gs1.Refresh(clockMock.Now())
	gs1.UpdateDiscoveryStatus(ds.Master)

	clockMock.Add(time.Millisecond)
	gs2, _ := servers.New(net.ParseIP("2.2.2.2"), 10480, 10481)
	gs2.Refresh(clockMock.Now())
	gs2.UpdateDiscoveryStatus(ds.Port)

	before3rd := clockMock.Now()

	clockMock.Add(time.Millisecond)
	gs3, _ := servers.New(net.ParseIP("3.3.3.3"), 10480, 10481)
	gs3.Refresh(clockMock.Now())
	gs3.UpdateDiscoveryStatus(ds.Master | ds.Details | ds.Port)

	clockMock.Add(time.Millisecond)
	gs4, _ := servers.New(net.ParseIP("4.4.4.4"), 10480, 10481)
	gs4.Refresh(clockMock.Now())
	gs4.UpdateDiscoveryStatus(ds.NoDetails)

	clockMock.Add(time.Millisecond)
	gs5, _ := servers.New(net.ParseIP("5.5.5.5"), 10480, 10481)
	gs5.Refresh(clockMock.Now())
	gs5.UpdateDiscoveryStatus(ds.DetailsRetry)

	clockMock.Add(time.Millisecond)
	gs6, _ := servers.New(net.ParseIP("6.6.6.6"), 10480, 10481)
	gs6.Refresh(clockMock.Now())
	gs6.UpdateDiscoveryStatus(ds.Port | ds.Details | ds.DetailsRetry)

	clockMock.Add(time.Millisecond)
	gs7, _ := servers.New(net.ParseIP("7.7.7.7"), 10480, 10481)
	gs7.Refresh(clockMock.Now())
	gs7.UpdateDiscoveryStatus(ds.Master | ds.Info | ds.Details)

	clockMock.Add(time.Millisecond)
	gs8, _ := servers.New(net.ParseIP("8.8.8.8"), 10480, 10481)
	gs8.Refresh(clockMock.Now())
	gs8.UpdateDiscoveryStatus(ds.Master | ds.PortRetry)

	beforeLast := clockMock.Now()

	clockMock.Add(time.Millisecond)
	gs9, _ := servers.New(net.ParseIP("9.9.9.9"), 10480, 10481)
	gs9.Refresh(clockMock.Now())
	gs9.UpdateDiscoveryStatus(ds.Info)

	gs1, _ = serversRepo.Add(ctx, gs1, servers.OnConflictIgnore)
	gs2, _ = serversRepo.Add(ctx, gs2, servers.OnConflictIgnore)
	gs3, _ = serversRepo.Add(ctx, gs3, servers.OnConflictIgnore)
	gs4, _ = serversRepo.Add(ctx, gs4, servers.OnConflictIgnore)
	gs5, _ = serversRepo.Add(ctx, gs5, servers.OnConflictIgnore)
	gs6, _ = serversRepo.Add(ctx, gs6, servers.OnConflictIgnore)
	gs7, _ = serversRepo.Add(ctx, gs7, servers.OnConflictIgnore)
	gs8, _ = serversRepo.Add(ctx, gs8, servers.OnConflictIgnore)
	gs9, _ = serversRepo.Add(ctx, gs9, servers.OnConflictIgnore)

	minCountdown := clockMock.Now()
	maxCountdown := clockMock.Now()
	deadline := clockMock.Now().Add(time.Second * 60)

	revivedCnt, err := service.ReviveServers(ctx, before3rd, beforeLast, minCountdown, maxCountdown, deadline)
	require.NoError(t, err)
	assert.Equal(t, 3, revivedCnt)

	probeCnt, _ := probesRepo.Count(ctx)
	assert.Equal(t, 3, probeCnt)

	revivedServers := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		tgt, err := probesRepo.PopAny(ctx)
		require.NoError(t, err)
		require.Equal(t, probes.GoalPort, tgt.GetGoal())
		revivedServers = append(revivedServers, tgt.GetDottedIP())
	}
	assert.Equal(t, []string{"7.7.7.7", "5.5.5.5", "4.4.4.4"}, revivedServers)
}
