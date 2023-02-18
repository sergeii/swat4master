package memory

import (
	instances "github.com/sergeii/swat4master/internal/core/instances/memory"
	probes "github.com/sergeii/swat4master/internal/core/probes/memory"
	servers "github.com/sergeii/swat4master/internal/core/servers/memory"
	"github.com/sergeii/swat4master/internal/persistence"
)

func New() persistence.Repositories {
	return persistence.Repositories{
		Servers:   servers.New(),
		Instances: instances.New(),
		Probes:    probes.New(),
	}
}
