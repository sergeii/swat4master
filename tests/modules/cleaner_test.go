package modules_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
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

	ins1 := instance.MustNew("foo", net.ParseIP("1.1.1.1"), 10480)
	ins2 := instance.MustNew("bar", net.ParseIP("3.3.3.3"), 10480)
	ins4 := instance.MustNew("baz", net.ParseIP("4.4.4.4"), 10480)

	instancefactory.Save(ctx, instanceRepo, ins1)
	instancefactory.Save(ctx, instanceRepo, ins2)
	instancefactory.Save(ctx, instanceRepo, ins4)

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

	ins5 := instance.MustNew("qux", net.ParseIP("5.5.5.5"), 10480)
	instancefactory.Save(ctx, instanceRepo, ins5)

	// wait for cleaner to clean servers 2 and 3
	<-time.After(time.Millisecond * 150)

	// check that the refreshed server and the new one are still there
	svrCount, err := serverRepo.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, svrCount)

	_, err = serverRepo.Get(ctx, gs1.Addr)
	assert.NoError(t, err)
	_, err = serverRepo.Get(ctx, gs5.Addr)
	assert.NoError(t, err)

	// no instance was removed
	insCount, err := instanceRepo.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 4, insCount)

	_, err = instanceRepo.Get(ctx, "foo")
	assert.NoError(t, err)
	_, err = instanceRepo.Get(ctx, "baz")
	assert.NoError(t, err)
	_, err = instanceRepo.Get(ctx, "qux")
	assert.NoError(t, err)

	removalValue := testutil.ToFloat64(collector.CleanerRemovals)
	assert.Equal(t, 2.0, removalValue)
	errorValue := testutil.ToFloat64(collector.CleanerErrors)
	assert.Equal(t, 0.0, errorValue)
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

	removalValue := testutil.ToFloat64(collector.CleanerRemovals)
	errorValue := testutil.ToFloat64(collector.CleanerErrors)
	assert.Equal(t, 0.0, removalValue)
	assert.Equal(t, 0.0, errorValue)
}
