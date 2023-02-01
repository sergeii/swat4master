package collector_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/running"
	"github.com/sergeii/swat4master/cmd/swat4master/running/collector"
	"github.com/sergeii/swat4master/internal/core/servers"
)

func TestCollector_Run(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	cfg := config.Config{
		CollectorInterval: time.Millisecond * 10,
	}
	app := application.Configure()
	runner := running.NewRunner(app, cfg)
	runner.Add(collector.Run, ctx)

	gs, _ := servers.New(net.ParseIP("1.1.1.1"), 10480, 10481)
	app.Servers.Add(ctx, gs, servers.OnConflictIgnore) // nolint: errcheck

	valueBeforeTick := testutil.ToFloat64(app.MetricService.ServerRepositorySize)
	assert.Equal(t, 0.0, valueBeforeTick)

	time.Sleep(time.Millisecond * 20)

	valueAfterTick := testutil.ToFloat64(app.MetricService.ServerRepositorySize)
	assert.Equal(t, 1.0, valueAfterTick)

	cancel()
	runner.WaitQuit()
}
