package memory

import (
	"github.com/benbjohnson/clock"

	"github.com/sergeii/swat4master/internal/data/memory/instances"
	"github.com/sergeii/swat4master/internal/data/memory/probes"
	"github.com/sergeii/swat4master/internal/data/memory/servers"
	"github.com/sergeii/swat4master/internal/persistence"
)

func New(c clock.Clock) persistence.Repositories {
	return persistence.Repositories{
		Servers:   servers.New(c),
		Instances: instances.New(),
		Probes:    probes.New(c),
	}
}
