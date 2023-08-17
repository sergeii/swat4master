package memory

import (
	"github.com/benbjohnson/clock"

	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/persistence/memory/instances"
	"github.com/sergeii/swat4master/internal/persistence/memory/probes"
	"github.com/sergeii/swat4master/internal/persistence/memory/servers"
)

type Repositories struct {
	Servers   repositories.ServerRepository
	Instances repositories.InstanceRepository
	Probes    repositories.ProbeRepository
}

func New(c clock.Clock) Repositories {
	return Repositories{
		Servers:   servers.New(c),
		Instances: instances.New(),
		Probes:    probes.New(c),
	}
}
