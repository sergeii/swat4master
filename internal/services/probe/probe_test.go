package probe_test

import (
	"context"
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	repos "github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/persistence/memory"
	"github.com/sergeii/swat4master/internal/services/monitoring"
	sp "github.com/sergeii/swat4master/internal/services/probe"
)

func makeApp(tb fxtest.TB, extra ...fx.Option) {
	fxopts := []fx.Option{
		fx.Provide(func() clock.Clock { return clock.NewMock() }),
		fx.Provide(func() *zerolog.Logger {
			logger := zerolog.Nop()
			return &logger
		}),
		fx.Provide(func(c clock.Clock) (repos.ServerRepository, repos.InstanceRepository, repos.ProbeRepository) {
			mem := memory.New(c)
			return mem.Servers, mem.Instances, mem.Probes
		}),
		fx.Provide(
			monitoring.NewMetricService,
			sp.NewService,
		),
		fx.NopLogger,
	}
	fxopts = append(fxopts, extra...)
	app := fxtest.New(tb, fxopts...)
	app.RequireStart().RequireStop()
}

func TestProbeService_PopMany(t *testing.T) {
	ctx := context.TODO()

	var repo repos.ProbeRepository
	var service *sp.Service
	makeApp(t, fx.Populate(&repo, &service))

	// empty
	empty, err := service.PopMany(ctx, 5)
	assert.NoError(t, err)
	assert.Len(t, empty, 0)

	for _, ipaddr := range []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"} {
		repo.Add(ctx, probe.New(addr.MustNewFromString(ipaddr, 10480), 10480, probe.GoalDetails)) // nolint: errcheck
	}

	probes, _ := service.PopMany(ctx, 2)
	assert.Len(t, probes, 2)
	assert.Equal(t, "1.1.1.1", probes[0].Addr.GetDottedIP())
	assert.Equal(t, "2.2.2.2", probes[1].Addr.GetDottedIP())

	probes, _ = service.PopMany(ctx, 2)
	assert.Len(t, probes, 1)
	assert.Equal(t, "3.3.3.3", probes[0].Addr.GetDottedIP())

	// exhausted
	probes, _ = service.PopMany(ctx, 2)
	assert.Len(t, probes, 0)
}
