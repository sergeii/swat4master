package modules_test

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/reviver"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/metrics"
	tu "github.com/sergeii/swat4master/internal/testutils"
	"github.com/sergeii/swat4master/internal/testutils/factories/serverfactory"
	"github.com/sergeii/swat4master/tests/testapp"
)

func makeAppWithReviver(extra ...fx.Option) (*fx.App, func()) {
	fxopts := []fx.Option{
		fx.Provide(testapp.ProvidePersistence),
		application.Module,
		fx.Provide(func() config.Config {
			return config.Config{
				DiscoveryRevivalInterval:  time.Millisecond * 100,
				DiscoveryRevivalScope:     time.Millisecond * 1000,
				DiscoveryRevivalCountdown: time.Millisecond,
				DiscoveryRevivalPorts:     []int{0},
			}
		}),
		reviver.Module,
		fx.NopLogger,
		fx.Invoke(func(*reviver.Reviver) {}),
	}
	fxopts = append(fxopts, extra...)
	app := fx.New(fxopts...)
	return app, func() {
		app.Stop(context.TODO()) // nolint: errcheck
	}
}

type reviverProbeCount struct {
	count   int
	expired int
	probes  []string
}

func countReviverProbes(
	ctx context.Context,
	repo repositories.ProbeRepository,
) (reviverProbeCount, error) {
	count, err := repo.Count(ctx)
	if err != nil {
		return reviverProbeCount{}, err
	}

	probes, expired, err := repo.PopMany(ctx, count)
	if err != nil {
		return reviverProbeCount{}, err
	}

	portProbes := make([]string, 0, count)
	for _, prb := range probes {
		if prb.Goal == probe.GoalPort {
			portProbes = append(portProbes, prb.Addr.String())
		}
	}
	return reviverProbeCount{
		count:   count,
		expired: expired,
		probes:  portProbes,
	}, nil
}

func TestReviver_OK(t *testing.T) {
	var serverRepo repositories.ServerRepository
	var probeRepo repositories.ProbeRepository
	var collector *metrics.Collector

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	now := time.Now()

	gs1 := serverfactory.Build(
		serverfactory.WithAddress("1.1.1.1", 10480),
		serverfactory.WithQueryPort(10481),
		serverfactory.WithDiscoveryStatus(ds.Master),
		serverfactory.WithRefreshedAt(now),
	)
	gs2 := serverfactory.Build(
		serverfactory.WithAddress("2.2.2.2", 10480),
		serverfactory.WithQueryPort(10481),
		serverfactory.WithDiscoveryStatus(ds.Port),
		serverfactory.WithRefreshedAt(now),
	)
	gs3 := serverfactory.Build(
		serverfactory.WithAddress("3.3.3.3", 10480),
		serverfactory.WithQueryPort(10481),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Details|ds.Port),
		serverfactory.WithRefreshedAt(now),
	)
	gs4 := serverfactory.Build(
		serverfactory.WithAddress("4.4.4.4", 10480),
		serverfactory.WithQueryPort(10481),
		serverfactory.WithDiscoveryStatus(ds.DetailsRetry),
		serverfactory.WithRefreshedAt(now),
	)
	gs5 := serverfactory.Build(
		serverfactory.WithAddress("5.5.5.5", 10480),
		serverfactory.WithQueryPort(10481),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Info|ds.Details),
		serverfactory.WithRefreshedAt(now),
	)
	gs6 := serverfactory.Build(
		serverfactory.WithAddress("6.6.6.6", 10480),
		serverfactory.WithQueryPort(10481),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.PortRetry),
		serverfactory.WithRefreshedAt(now),
	)
	// No refresh time
	gs7 := serverfactory.Build(
		serverfactory.WithAddress("7.7.7.7", 10480),
		serverfactory.WithQueryPort(10481),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Info|ds.Details),
	)
	// Server refresh time is far in the past
	gs8 := serverfactory.Build(
		serverfactory.WithAddress("8.8.8.8", 10480),
		serverfactory.WithQueryPort(10481),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
		serverfactory.WithRefreshedAt(now.Add(-time.Second)),
	)

	app, cancel := makeAppWithReviver(
		fx.Populate(&serverRepo, &probeRepo, &collector),
	)
	defer cancel()
	app.Start(ctx) // nolint: errcheck

	for _, svr := range []*server.Server{&gs1, &gs2, &gs3, &gs4, &gs5, &gs6, &gs7, &gs8} {
		*svr = tu.Must(serverRepo.Add(ctx, *svr, repositories.ServerOnConflictIgnore))
	}

	// let refresher run a cycle
	<-time.After(time.Millisecond * 150)

	// port probes are added
	result, err := countReviverProbes(ctx, probeRepo)
	require.NoError(t, err)
	assert.Equal(t, 3, result.count)
	assert.Equal(t, 0, result.expired)
	// because probes are inserted by the use case with a random readiness time,
	// we can't predict the order of the probes
	assert.ElementsMatch(t, []string{"5.5.5.5:10480", "4.4.4.4:10480", "1.1.1.1:10480"}, result.probes)

	// make gs3 revivable
	gs3.ClearDiscoveryStatus(ds.Port)
	gs3 = tu.Must(serverRepo.Update(ctx, gs3, repositories.ServerOnConflictIgnore))

	// let reviver run another cycle
	<-time.After(time.Millisecond * 100)
	result, err = countReviverProbes(ctx, probeRepo)
	require.NoError(t, err)
	assert.Equal(t, 4, result.count)
	assert.Equal(t, 0, result.expired)
	assert.ElementsMatch(t, []string{"3.3.3.3:10480", "5.5.5.5:10480", "4.4.4.4:10480", "1.1.1.1:10480"}, result.probes)

	// run a couple of cycles, expect some probes to expire
	<-time.After(time.Millisecond * 200)
	result, err = countReviverProbes(ctx, probeRepo)
	require.NoError(t, err)
	assert.Equal(t, 8, result.count)
	assert.Equal(t, 4, result.expired)
	assert.ElementsMatch(t, []string{"3.3.3.3:10480", "5.5.5.5:10480", "4.4.4.4:10480", "1.1.1.1:10480"}, result.probes)

	// make the remaining servers non-revivable
	gs1.UpdateDiscoveryStatus(ds.Port)
	gs4.UpdateDiscoveryStatus(ds.PortRetry)
	gs5.UpdateDiscoveryStatus(ds.Port)
	gs1 = tu.Must(serverRepo.Update(ctx, gs1, repositories.ServerOnConflictIgnore))
	gs4 = tu.Must(serverRepo.Update(ctx, gs4, repositories.ServerOnConflictIgnore))
	gs5 = tu.Must(serverRepo.Update(ctx, gs5, repositories.ServerOnConflictIgnore))

	// run another cycle, expect only gs3 to be revived
	<-time.After(time.Millisecond * 100)
	result, err = countReviverProbes(ctx, probeRepo)
	require.NoError(t, err)
	assert.Equal(t, 1, result.count)
	assert.Equal(t, 0, result.expired)
	assert.Equal(t, []string{"3.3.3.3:10480"}, result.probes)

	// the remaining server goes out of revival scope
	<-time.After(time.Millisecond * 500)
	result, err = countReviverProbes(ctx, probeRepo)
	require.NoError(t, err)
	assert.Equal(t, 4, result.count)
	assert.Equal(t, 4, result.expired)
	assert.Equal(t, []string{}, result.probes)

	producedMetricValue := testutil.ToFloat64(collector.DiscoveryQueueProduced)
	assert.Equal(t, 20.0, producedMetricValue)
}
