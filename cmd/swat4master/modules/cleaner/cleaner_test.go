package cleaner_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/cleaner"
	"github.com/sergeii/swat4master/internal/core/servers"
)

func TestCleaner_Run(t *testing.T) {
	var repo servers.Repository

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	app := fx.New(
		application.Module,
		fx.Provide(func() config.Config {
			return config.Config{
				CleanRetention: time.Millisecond * 50,
				CleanInterval:  time.Millisecond * 100,
			}
		}),
		cleaner.Module,
		fx.NopLogger,
		fx.Invoke(func(*cleaner.Cleaner) {}),
		fx.Populate(&repo),
	)
	app.Start(context.TODO()) // nolint: errcheck
	defer func() {
		app.Stop(context.TODO()) // nolint: errcheck
	}()

	gs1, _ := servers.New(net.ParseIP("1.1.1.1"), 10480, 10481)
	gs1.Refresh()
	repo.Add(ctx, gs1, nil) // nolint: errcheck
	gs2, _ := servers.New(net.ParseIP("2.2.2.2"), 10480, 10481)
	gs2.Refresh()
	repo.Add(ctx, gs2, servers.OnConflictIgnore) // nolint: errcheck

	cnt, _ := repo.Count(ctx)
	assert.Equal(t, 2, cnt)

	<-time.After(time.Millisecond * 75)

	gs3, _ := servers.New(net.ParseIP("3.3.3.3"), 10480, 10481)
	gs3.Refresh()
	repo.Add(ctx, gs3, servers.OnConflictIgnore) // nolint: errcheck

	<-time.After(time.Millisecond * 30)

	cnt, err := repo.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, cnt)
}
