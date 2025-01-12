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
	"github.com/sergeii/swat4master/cmd/swat4master/modules/cleaner"
	"github.com/sergeii/swat4master/internal/core/entities/instance"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/metrics"
	"github.com/sergeii/swat4master/internal/testutils/factories/instancefactory"
	"github.com/sergeii/swat4master/internal/testutils/factories/serverfactory"
	"github.com/sergeii/swat4master/tests/testapp"
)

const (
	DEADBEEF = "\xde\xad\xbe\xef"
	FEEDFOOD = "\xfe\xed\xf0\x0d"
	CAFEBABE = "\xca\xfe\xba\xbe"
	BAADCODE = "\xba\xad\xc0\xde"
)

type b []byte

func makeAppWithCleaner(extra ...fx.Option) (*fx.App, func()) {
	fxopts := []fx.Option{
		fx.Provide(testapp.ProvidePersistence),
		application.Module,
		fx.Provide(func() config.Config {
			return config.Config{
				CleanRetention: time.Millisecond * 200,
				CleanInterval:  time.Millisecond * 10,
			}
		}),
		cleaner.Module,
		fx.NopLogger,
		fx.Invoke(func(*cleaner.Cleaner) {}),
	}
	fxopts = append(fxopts, extra...)
	app := fx.New(fxopts...)
	return app, func() {
		app.Stop(context.TODO()) // nolint: errcheck
	}
}

func TestCleaner_OK(t *testing.T) {
	var serverRepo repositories.ServerRepository
	var instanceRepo repositories.InstanceRepository
	var collector *metrics.Collector

	ctx := context.TODO()
	app, cancel := makeAppWithCleaner(
		fx.Populate(&serverRepo, &instanceRepo, &collector),
	)
	defer cancel()
	app.Start(ctx) // nolint: errcheck

	ins1 := instancefactory.Create(
		ctx,
		instanceRepo,
		instancefactory.WithStringID(DEADBEEF),
		instancefactory.WithServerAddress("1.1.1.1", 10480),
	)
	instancefactory.Create(
		ctx,
		instanceRepo,
		instancefactory.WithStringID(FEEDFOOD),
		instancefactory.WithServerAddress("3.3.3.3", 10480),
	)
	instancefactory.Create(
		ctx,
		instanceRepo,
		instancefactory.WithStringID(CAFEBABE),
		instancefactory.WithServerAddress("4.4.4.4", 10480),
	)

	gs1 := serverfactory.Create(
		ctx,
		serverRepo,
		serverfactory.WithAddress("1.1.1.1", 10480),
		serverfactory.WithQueryPort(10481),
	)
	serverfactory.Create(
		ctx,
		serverRepo,
		serverfactory.WithAddress("2.2.2.2", 10480),
		serverfactory.WithQueryPort(10481),
	)
	serverfactory.Create(
		ctx,
		serverRepo,
		serverfactory.WithAddress("3.3.3.3", 10480),
		serverfactory.WithQueryPort(10481),
	)

	// wait for cleaner to run some cycles
	<-time.After(time.Millisecond * 100)

	// refresh server 1 to prevent it from being cleaned
	gs1.Refresh(time.Now())
	serverRepo.Update(ctx, gs1, repositories.ServerOnConflictIgnore) // nolint: errcheck

	// add a new server with an instance, it should not be cleaned right away
	gs5 := serverfactory.Create(
		ctx,
		serverRepo,
		serverfactory.WithAddress("5.5.5.5", 10480),
		serverfactory.WithQueryPort(10481),
	)
	instancefactory.Create(
		ctx,
		instanceRepo,
		instancefactory.WithStringID(BAADCODE),
		instancefactory.WithServerAddress("5.5.5.5", 10480),
	)

	// refresh the first instance to prevent it from being cleaned
	instanceRepo.Add(ctx, ins1) // nolint: errcheck

	// wait for cleaner to clean servers 2 and 3
	<-time.After(time.Millisecond * 150)

	// check that the refreshed server and the new one are still there
	svrCount, err := serverRepo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, svrCount)

	_, err = serverRepo.Get(ctx, gs1.Addr)
	require.NoError(t, err)
	_, err = serverRepo.Get(ctx, gs5.Addr)
	require.NoError(t, err)

	// 2 instances should be cleaned up
	insCount, err := instanceRepo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, insCount)

	_, err = instanceRepo.Get(ctx, instance.MustNewID(b(DEADBEEF)))
	require.NoError(t, err)
	_, err = instanceRepo.Get(ctx, instance.MustNewID(b(BAADCODE)))
	require.NoError(t, err)

	serverRemovalsValue := testutil.ToFloat64(collector.CleanerRemovals.WithLabelValues("servers"))
	assert.Equal(t, 2.0, serverRemovalsValue)
	serverErrorsValue := testutil.ToFloat64(collector.CleanerErrors.WithLabelValues("servers"))
	assert.Equal(t, 0.0, serverErrorsValue)

	instanceRemovalsValue := testutil.ToFloat64(collector.CleanerRemovals.WithLabelValues("instances"))
	assert.Equal(t, 2.0, instanceRemovalsValue)
	instanceErrorsValue := testutil.ToFloat64(collector.CleanerErrors.WithLabelValues("instances"))
	assert.Equal(t, 0.0, instanceErrorsValue)
}

func TestCleaner_NoErrorWhenNothingToClean(t *testing.T) {
	var collector *metrics.Collector

	ctx := context.TODO()
	app, cancel := makeAppWithCleaner(
		fx.Populate(&collector),
	)
	defer cancel()
	app.Start(ctx) // nolint: errcheck

	// wait for cleaner to run some cycles
	<-time.After(time.Millisecond * 100)

	for _, kind := range []string{"servers", "instances"} {
		removalValue := testutil.ToFloat64(collector.CleanerRemovals.WithLabelValues(kind))
		errorValue := testutil.ToFloat64(collector.CleanerErrors.WithLabelValues(kind))
		assert.Equal(t, 0.0, removalValue)
		assert.Equal(t, 0.0, errorValue)
	}
}
