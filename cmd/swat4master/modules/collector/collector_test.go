package collector_test

import (
	"context"
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/collector"
	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/services/monitoring"
)

func TestCollector_Run(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	var repo servers.Repository
	var metrics *monitoring.MetricService

	clockMock := clock.NewMock()

	app := fx.New(
		application.Module,
		fx.Provide(func() config.Config {
			return config.Config{
				CollectorInterval: time.Millisecond * 10,
			}
		}),
		fx.Decorate(func() clock.Clock { return clockMock }),
		collector.Module,
		fx.NopLogger,
		fx.Invoke(func(_ *collector.Collector, m *monitoring.MetricService) {}),
		fx.Populate(&metrics, &repo),
	)
	app.Start(context.TODO()) // nolint: errcheck
	defer func() {
		app.Stop(context.TODO()) // nolint: errcheck
	}()
	runtime.Gosched()

	gs, _ := servers.New(net.ParseIP("1.1.1.1"), 10480, 10481)
	repo.Add(ctx, gs, servers.OnConflictIgnore) // nolint: errcheck

	valueBeforeTick := testutil.ToFloat64(metrics.ServerRepositorySize)
	assert.Equal(t, 0.0, valueBeforeTick)

	clockMock.Add(time.Millisecond * 20)

	valueAfterTick := testutil.ToFloat64(metrics.ServerRepositorySize)
	assert.Equal(t, 1.0, valueAfterTick)
}
