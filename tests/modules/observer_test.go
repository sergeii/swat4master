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
	"github.com/sergeii/swat4master/cmd/swat4master/modules/observer"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/metrics"
)

func TestObserver_Run(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	var serverRepo repositories.ServerRepository
	var collector *metrics.Collector

	app := fx.New(
		application.Module,
		fx.Provide(func() config.Config {
			return config.Config{
				MetricObserverInterval: time.Millisecond * 10,
			}
		}),
		observer.Module,
		fx.NopLogger,
		fx.Invoke(func(_ *observer.Observer, _ *metrics.Collector) {}),
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