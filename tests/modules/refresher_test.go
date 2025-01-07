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
	"github.com/sergeii/swat4master/cmd/swat4master/modules/refresher"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/metrics"
	"github.com/sergeii/swat4master/internal/testutils/factories/serverfactory"
	"github.com/sergeii/swat4master/tests/testapp"
)

func makeAppWithRefresher(extra ...fx.Option) (*fx.App, func()) {
	fxopts := []fx.Option{
		fx.Provide(testapp.ProvidePersistence),
		application.Module,
		fx.Provide(func() config.Config {
			return config.Config{
				DiscoveryRefreshInterval: time.Millisecond * 100,
			}
		}),
		refresher.Module,
		fx.NopLogger,
		fx.Invoke(func(*refresher.Refresher) {}),
	}
	fxopts = append(fxopts, extra...)
	app := fx.New(fxopts...)
	return app, func() {
		app.Stop(context.TODO()) // nolint: errcheck
	}
}

type refresherProbeCount struct {
	count   int
	expired int
	probes  []string
}

func countRefresherProbes(
	ctx context.Context,
	repo repositories.ProbeRepository,
) (refresherProbeCount, error) {
	count, err := repo.Count(ctx)
	if err != nil {
		return refresherProbeCount{}, err
	}

	probes, expired, err := repo.PopMany(ctx, count)
	if err != nil {
		return refresherProbeCount{}, err
	}

	detailsProbes := make([]string, 0, count)
	for _, prb := range probes {
		if prb.Goal == probe.GoalDetails {
			detailsProbes = append(detailsProbes, prb.Addr.String())
		}
	}
	return refresherProbeCount{
		count:   count,
		expired: expired,
		probes:  detailsProbes,
	}, nil
}

func TestRefresher_OK(t *testing.T) {
	var serverRepo repositories.ServerRepository
	var probeRepo repositories.ProbeRepository
	var collector *metrics.Collector

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	gs1 := serverfactory.Build(
		serverfactory.WithAddress("1.1.1.1", 10480),
		serverfactory.WithQueryPort(10481),
		serverfactory.WithDiscoveryStatus(ds.Master),
	)
	gs2 := serverfactory.Build(
		serverfactory.WithAddress("2.2.2.2", 10480),
		serverfactory.WithQueryPort(10481),
		serverfactory.WithDiscoveryStatus(ds.Port),
	)
	gs3 := serverfactory.Build(
		serverfactory.WithAddress("3.3.3.3", 10480),
		serverfactory.WithQueryPort(10481),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Details|ds.Port),
	)
	gs4 := serverfactory.Build(
		serverfactory.WithAddress("5.5.5.5", 10480),
		serverfactory.WithQueryPort(10481),
		serverfactory.WithDiscoveryStatus(ds.DetailsRetry),
	)
	gs5 := serverfactory.Build(
		serverfactory.WithAddress("6.6.6.6", 10480),
		serverfactory.WithQueryPort(10481),
		serverfactory.WithDiscoveryStatus(ds.Port|ds.Details|ds.DetailsRetry),
	)
	gs6 := serverfactory.Build(
		serverfactory.WithAddress("7.7.7.7", 10480),
		serverfactory.WithQueryPort(10481),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Info|ds.Details),
	)
	gs7 := serverfactory.Build(
		serverfactory.WithAddress("9.9.9.9", 10480),
		serverfactory.WithQueryPort(10481),
		serverfactory.WithDiscoveryStatus(ds.Port|ds.PortRetry),
	)

	app, cancel := makeAppWithRefresher(
		fx.Populate(&serverRepo, &probeRepo, &collector),
	)
	defer cancel()
	app.Start(ctx) // nolint: errcheck

	for _, gs := range []server.Server{gs1, gs2, gs3, gs4, gs5, gs6, gs7} {
		serverfactory.Save(ctx, serverRepo, gs)
	}

	// let refresher run a cycle
	<-time.After(time.Millisecond * 150)

	// details probes are added
	result, err := countRefresherProbes(ctx, probeRepo)
	require.NoError(t, err)
	assert.Equal(t, 3, result.count)
	assert.Equal(t, 0, result.expired)
	assert.Equal(t, []string{"9.9.9.9:10480", "3.3.3.3:10480", "2.2.2.2:10480"}, result.probes)

	// clear the server's refreshable status, so that it doesn't get picked up again
	gs3.ClearDiscoveryStatus(ds.Details | ds.Port)
	gs3, _ = serverRepo.Update(ctx, gs3, repositories.ServerOnConflictIgnore)

	// clear the retry status, so it's actually picked up
	gs5.ClearDiscoveryStatus(ds.DetailsRetry)
	gs5, _ = serverRepo.Update(ctx, gs5, repositories.ServerOnConflictIgnore)

	// let refresher run another cycle
	<-time.After(time.Millisecond * 100)

	result, err = countRefresherProbes(ctx, probeRepo)
	require.NoError(t, err)
	assert.Equal(t, 3, result.count)
	assert.Equal(t, 0, result.expired)
	assert.Equal(t, []string{"6.6.6.6:10480", "9.9.9.9:10480", "2.2.2.2:10480"}, result.probes)

	// run a couple of cycles, expect some probes to expire
	<-time.After(time.Millisecond * 200)

	result, err = countRefresherProbes(ctx, probeRepo)
	require.NoError(t, err)
	assert.Equal(t, 6, result.count)
	assert.Equal(t, 3, result.expired)
	assert.Equal(t, []string{"6.6.6.6:10480", "9.9.9.9:10480", "2.2.2.2:10480"}, result.probes)

	// make the remaining servers non-refreshable
	gs2.ClearDiscoveryStatus(ds.Port)
	gs5.ClearDiscoveryStatus(ds.Port)
	gs7.UpdateDiscoveryStatus(ds.DetailsRetry)

	gs2, _ = serverRepo.Update(ctx, gs2, repositories.ServerOnConflictIgnore)
	gs5, _ = serverRepo.Update(ctx, gs5, repositories.ServerOnConflictIgnore)
	gs7, _ = serverRepo.Update(ctx, gs7, repositories.ServerOnConflictIgnore)

	// run another cycle, expect no probes
	<-time.After(time.Millisecond * 100)

	result, err = countRefresherProbes(ctx, probeRepo)
	require.NoError(t, err)
	assert.Equal(t, 0, result.count)
	assert.Equal(t, 0, result.expired)
	assert.Equal(t, []string{}, result.probes)

	producedMetricValue := testutil.ToFloat64(collector.DiscoveryQueueProduced)
	assert.Equal(t, 12.0, producedMetricValue)
}
