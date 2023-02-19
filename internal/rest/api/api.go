package api

import (
	"github.com/rs/zerolog"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/internal/services/discovery/finding"
	"github.com/sergeii/swat4master/internal/services/server"
)

type App struct {
	ServerService  *server.Service
	FindingService *finding.Service
}

type API struct {
	cfg    config.Config
	app    *App
	logger *zerolog.Logger
}

type Error struct {
	Error string
}

func New(
	cfg config.Config,
	serverService *server.Service,
	findingService *finding.Service,
	logger *zerolog.Logger,
) *API {
	return &API{
		app: &App{
			ServerService:  serverService,
			FindingService: findingService,
		},
		cfg:    cfg,
		logger: logger,
	}
}
