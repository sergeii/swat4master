package modules_test

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
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
)

func TestCleaner_Run(t *testing.T) {
	var repo repositories.ServerRepository

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	app := fx.New(
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
		fx.Populate(&repo),
	)
	app.Start(context.TODO()) // nolint: errcheck
	defer func() {
		app.Stop(context.TODO()) // nolint: errcheck
	}()

	gs1 := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	gs2 := server.MustNew(net.ParseIP("2.2.2.2"), 10480, 10481)
	gs3 := server.MustNew(net.ParseIP("3.3.3.3"), 10480, 10481)

	repo.Add(ctx, gs1, nil)                                 // nolint: errcheck
	repo.Add(ctx, gs2, repositories.ServerOnConflictIgnore) // nolint: errcheck
	repo.Add(ctx, gs3, repositories.ServerOnConflictIgnore) // nolint: errcheck

	// wait for cleaner to run some cycles
	<-time.After(time.Millisecond * 100)

	// refresh server 1 to prevent it from being cleaned
	gs1.Refresh(time.Now())
	repo.Update(ctx, gs1, repositories.ServerOnConflictIgnore) // nolint: errcheck

	// wait for cleaner to clean servers 2 and 3
	<-time.After(time.Millisecond * 150)

	cnt, err := repo.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, cnt)
}
