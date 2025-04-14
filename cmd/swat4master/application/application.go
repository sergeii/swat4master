package application

import (
	"github.com/jonboulle/clockwork"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/components/exporter"
	"github.com/sergeii/swat4master/cmd/swat4master/container"
	"github.com/sergeii/swat4master/cmd/swat4master/logging"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/metrics"
	"github.com/sergeii/swat4master/internal/persistence/redis/redislock"
	"github.com/sergeii/swat4master/internal/persistence/redis/repositories/instances"
	"github.com/sergeii/swat4master/internal/persistence/redis/repositories/probes"
	"github.com/sergeii/swat4master/internal/persistence/redis/repositories/servers"
	"github.com/sergeii/swat4master/internal/validation"
)

type Repositories struct {
	fx.Out

	Servers   repositories.ServerRepository
	Instances repositories.InstanceRepository
	Probes    repositories.ProbeRepository
}

func provideRepositories(
	serverRepo *servers.Repository,
	instanceRepo *instances.Repository,
	probeRepo *probes.Repository,
) Repositories {
	return Repositories{
		Servers:   serverRepo,
		Instances: instanceRepo,
		Probes:    probeRepo,
	}
}

type Builder struct {
	opts []fx.Option
}

func NewBuilder(opts ...fx.Option) *Builder {
	return &Builder{
		opts: opts,
	}
}

func (b *Builder) Add(opts ...fx.Option) *Builder {
	b.opts = append(b.opts, opts...)
	return b
}

func (b *Builder) WithExporter() *Builder {
	return b.Add(
		fx.Invoke(func(*exporter.Component) {}),
	)
}

func (b *Builder) Build() *fx.App {
	return fx.New(b.opts...)
}

var Module = fx.Module("application",
	fx.Invoke(logging.NoGlobal),
	fx.Provide(clockwork.NewRealClock),
	fx.Provide(validation.New),
	fx.Provide(redislock.NewManager),
	fx.Provide(servers.New, instances.New, probes.New),
	fx.Provide(provideRepositories),
	fx.Provide(metrics.New),
	container.Module,
)
