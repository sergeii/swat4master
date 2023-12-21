package persistence

import (
	"github.com/sergeii/swat4master/internal/core/repositories"
)

type Repositories struct {
	Servers   repositories.ServerRepository
	Instances repositories.InstanceRepository
	Probes    repositories.ProbeRepository
}
