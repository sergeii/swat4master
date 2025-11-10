package components_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/components/observer"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/metrics"
	"github.com/sergeii/swat4master/tests/testapp"
)

func TestObserver_Run(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var serverRepo repositories.ServerRepository
	var collector *metrics.Collector

	app := fx.New(
		fx.Provide(testapp.NoLogging),
		fx.Provide(testapp.ProvideSettings),
		fx.Provide(testapp.ProvidePersistence),
		application.Module,
		fx.Supply(observer.Config{
			ObserveInterval: time.Millisecond * 10,
		}),
		observer.Module,
		fx.NopLogger,
		fx.Invoke(func(_ *observer.Component, _ *metrics.Collector) {}),
		fx.Populate(&collector, &serverRepo),
	)
	app.Start(context.TODO()) // nolint: errcheck
	defer func() {
		app.Stop(context.TODO()) // nolint: errcheck
	}()

	gs := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	serverRepo.Add(ctx, gs, repositories.ServerOnConflictIgnore) // nolint: errcheck

	// wait for the observer to spin up
	<-time.After(time.Millisecond * 100)

	valueAfterTick := testutil.ToFloat64(collector.ServerRepositorySize)
	assert.Equal(t, 1.0, valueAfterTick)
}
