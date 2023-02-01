package cleaner_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/running"
	"github.com/sergeii/swat4master/cmd/swat4master/running/cleaner"
	"github.com/sergeii/swat4master/internal/core/servers"
)

func TestCleaner_Run(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	cfg := config.Config{
		CleanRetention: time.Millisecond * 50,
		CleanInterval:  time.Millisecond * 100,
	}
	app := application.Configure()
	runner := running.NewRunner(app, cfg)
	runner.Add(cleaner.Run, ctx)

	gs1, _ := servers.New(net.ParseIP("1.1.1.1"), 10480, 10481)
	app.Servers.Add(ctx, gs1, nil) // nolint: errcheck
	gs2, _ := servers.New(net.ParseIP("2.2.2.2"), 10480, 10481)
	app.Servers.Add(ctx, gs2, servers.OnConflictIgnore) // nolint: errcheck

	cnt, _ := app.Servers.Count(ctx)
	assert.Equal(t, 2, cnt)

	<-time.After(time.Millisecond * 75)

	gs3, _ := servers.New(net.ParseIP("3.3.3.3"), 10480, 10481)
	app.Servers.Add(ctx, gs3, servers.OnConflictIgnore) // nolint: errcheck

	<-time.After(time.Millisecond * 30)

	cnt, err := app.Servers.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, cnt)

	cancel()
	runner.WaitQuit()
}
