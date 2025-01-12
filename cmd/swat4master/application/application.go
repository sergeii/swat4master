package application

import (
	"github.com/jonboulle/clockwork"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/container"
	"github.com/sergeii/swat4master/cmd/swat4master/logging"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/metrics"
	"github.com/sergeii/swat4master/internal/persistence/memory/servers"
	"github.com/sergeii/swat4master/internal/persistence/redis/instances"
	"github.com/sergeii/swat4master/internal/persistence/redis/probes"
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

var Module = fx.Module("application",
	fx.Provide(logging.Provide),
	fx.Invoke(logging.NoGlobal),
	fx.Provide(clockwork.NewRealClock),
	fx.Provide(validation.New),
	fx.Provide(servers.New, instances.New, probes.New),
	fx.Provide(provideRepositories),
	fx.Provide(metrics.New),
	container.Module,
)
