package application

import (
	"github.com/sergeii/swat4master/internal/core/instances"
	"github.com/sergeii/swat4master/internal/core/probes"
	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/services/discovery/finding"
	"github.com/sergeii/swat4master/internal/services/monitoring"
	"github.com/sergeii/swat4master/internal/services/probe"
	"github.com/sergeii/swat4master/internal/services/server"
)

type App struct {
	Servers   servers.Repository
	Instances instances.Repository
	Probes    probes.Repository

	ServerService  *server.Service
	ProbeService   *probe.Service
	FindingService *finding.Service
	MetricService  *monitoring.MetricService
}

func NewApp(
	serversRepo servers.Repository,
	instancesRepo instances.Repository,
	probesRepo probes.Repository,
	serverService *server.Service,
	probeService *probe.Service,
	findingService *finding.Service,
	metrics *monitoring.MetricService,
) *App {
	return &App{
		Servers:        serversRepo,
		Instances:      instancesRepo,
		Probes:         probesRepo,
		ServerService:  serverService,
		ProbeService:   probeService,
		FindingService: findingService,
		MetricService:  metrics,
	}
}
