package application

import (
	"github.com/sergeii/swat4master/internal/application"
	instances "github.com/sergeii/swat4master/internal/core/instances/memory"
	probes "github.com/sergeii/swat4master/internal/core/probes/memory"
	servers "github.com/sergeii/swat4master/internal/core/servers/memory"
	"github.com/sergeii/swat4master/internal/services/discovery/finding"
	"github.com/sergeii/swat4master/internal/services/monitoring"
	"github.com/sergeii/swat4master/internal/services/probe"
	"github.com/sergeii/swat4master/internal/services/server"
)

func Configure() *application.App {
	svrRepo := servers.New()
	instanceRepo := instances.New()
	probeRepo := probes.New()

	metrics := monitoring.NewMetricService()
	probeService := probe.NewService(probeRepo, metrics)
	findingService := finding.NewService(probeService)
	serverService := server.NewService(svrRepo)

	return application.NewApp(
		svrRepo,
		instanceRepo,
		probeRepo,
		serverService,
		probeService,
		findingService,
		metrics,
	)
}
