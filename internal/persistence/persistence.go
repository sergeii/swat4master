package persistence

import (
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/internal/core/instances"
	"github.com/sergeii/swat4master/internal/core/probes"
	"github.com/sergeii/swat4master/internal/core/servers"
)

type Repositories struct {
	fx.Out

	Servers   servers.Repository
	Instances instances.Repository
	Probes    probes.Repository
}
