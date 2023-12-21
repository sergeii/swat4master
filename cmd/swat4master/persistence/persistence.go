package persistence

import (
	"github.com/jonboulle/clockwork"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/persistence/memory"
)

type Repositories struct {
	fx.Out

	Servers   repositories.ServerRepository
	Instances repositories.InstanceRepository
	Probes    repositories.ProbeRepository
}

func Provide(clock clockwork.Clock) Repositories {
	repos := memory.New(clock)

	return Repositories{
		Servers:   repos.Servers,
		Instances: repos.Instances,
		Probes:    repos.Probes,
	}
}
