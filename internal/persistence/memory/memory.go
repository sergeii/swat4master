package memory

import (
	"github.com/jonboulle/clockwork"

	"github.com/sergeii/swat4master/internal/persistence"
	"github.com/sergeii/swat4master/internal/persistence/memory/instances"
	"github.com/sergeii/swat4master/internal/persistence/memory/probes"
	"github.com/sergeii/swat4master/internal/persistence/memory/servers"
)

func New(clock clockwork.Clock) persistence.Repositories {
	return persistence.Repositories{
		Servers:   servers.New(clock),
		Instances: instances.New(),
		Probes:    probes.New(clock),
	}
}
