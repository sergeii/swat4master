package cleaning_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/sergeii/swat4master/internal/core/instances"
	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/entity/addr"
	"github.com/sergeii/swat4master/internal/persistence/memory"
	"github.com/sergeii/swat4master/internal/services/cleaning"
	"github.com/sergeii/swat4master/internal/services/monitoring"
)

func makeApp(tb fxtest.TB, extra ...fx.Option) {
	fxopts := []fx.Option{
		fx.Provide(memory.New),
		fx.Provide(func() *zerolog.Logger {
			logger := zerolog.Nop()
			return &logger
		}),
		fx.Provide(
			monitoring.NewMetricService,
			cleaning.NewService,
		),
		fx.NopLogger,
	}
	fxopts = append(fxopts, extra...)
	app := fxtest.New(tb, fxopts...)
	app.RequireStart().RequireStop()
}

func TestCleaningService_Clean(t *testing.T) {
	var service *cleaning.Service
	var serversRepo servers.Repository
	var instancesRepo instances.Repository

	ctx := context.TODO()
	makeApp(t, fx.Populate(&service, &serversRepo, &instancesRepo))

	beforeAll := time.Now()
	instance1 := instances.MustNew("foo", net.ParseIP("1.1.1.1"), 10480)
	instancesRepo.Add(ctx, instance1) // nolint: errcheck
	server1 := servers.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	serversRepo.Add(ctx, server1, servers.OnConflictIgnore) // nolint: errcheck

	before2 := time.Now()
	server2 := servers.MustNew(net.ParseIP("2.2.2.2"), 10480, 10481)
	serversRepo.Add(ctx, server2, servers.OnConflictIgnore) // nolint: errcheck

	instance3 := instances.MustNew("bar", net.ParseIP("3.3.3.3"), 10480)
	instancesRepo.Add(ctx, instance3) // nolint: errcheck
	server3 := servers.MustNew(net.ParseIP("3.3.3.3"), 10480, 10481)
	serversRepo.Add(ctx, server3, servers.OnConflictIgnore) // nolint: errcheck

	instance4 := instances.MustNew("baz", net.ParseIP("4.4.4.4"), 10480)
	instancesRepo.Add(ctx, instance4) // nolint: errcheck
	server4 := servers.MustNew(net.ParseIP("4.4.4.4"), 10480, 10481)
	serversRepo.Add(ctx, server4, servers.OnConflictIgnore) // nolint: errcheck
	afterAll := time.Now()

	svrCount, _ := serversRepo.Count(ctx)
	insCount, _ := instancesRepo.Count(ctx)
	assert.Equal(t, 4, svrCount)
	assert.Equal(t, 3, insCount)

	err := service.Clean(context.TODO(), beforeAll)
	assert.NoError(t, err)
	// no changes
	svrCount, _ = serversRepo.Count(ctx)
	assert.Equal(t, 4, svrCount)
	insCount, _ = instancesRepo.Count(ctx)
	assert.Equal(t, 3, insCount)

	err = service.Clean(context.TODO(), before2)
	assert.NoError(t, err)
	svrCount, _ = serversRepo.Count(ctx)
	assert.Equal(t, 3, svrCount)
	insCount, _ = instancesRepo.Count(ctx)
	assert.Equal(t, 2, insCount)
	_, getSvrErr := serversRepo.Get(ctx, addr.MustNewFromString("1.1.1.1", 10480))
	assert.ErrorIs(t, getSvrErr, servers.ErrServerNotFound)
	_, getInsErr := instancesRepo.GetByID(ctx, "foo")
	assert.ErrorIs(t, getInsErr, instances.ErrInstanceNotFound)

	serversRepo.Update(ctx, server3, servers.OnConflictIgnore) // nolint: errcheck
	err = service.Clean(context.TODO(), afterAll)
	assert.NoError(t, err)
	svrCount, _ = serversRepo.Count(ctx)
	assert.Equal(t, 1, svrCount)
	insCount, _ = instancesRepo.Count(ctx)
	assert.Equal(t, 1, insCount)
	_, getSvrErr = serversRepo.Get(ctx, addr.MustNewFromString("3.3.3.3", 10480))
	assert.NoError(t, getSvrErr)
	_, getInsErr = instancesRepo.GetByID(ctx, "bar")
	assert.NoError(t, getInsErr)

	err = service.Clean(context.TODO(), time.Now())
	assert.NoError(t, err)
	svrCount, _ = serversRepo.Count(ctx)
	assert.Equal(t, 0, svrCount)
	insCount, _ = instancesRepo.Count(ctx)
	assert.Equal(t, 0, insCount)
}

func TestCleaningService_Clean_EmptyNoError(t *testing.T) {
	var service *cleaning.Service
	makeApp(t, fx.Populate(&service))
	err := service.Clean(context.TODO(), time.Now())
	assert.NoError(t, err)
}