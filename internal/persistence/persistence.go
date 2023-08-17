package persistence

import (
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/internal/core/repositories"
)

type Repositories struct {
	fx.Out

	Servers   repositories.ServerRepository
	Instances repositories.InstanceRepository
	Probes    repositories.ProbeRepository
}
