package api

import (
	"github.com/rs/zerolog"

	"github.com/sergeii/swat4master/cmd/swat4master/container"
	"github.com/sergeii/swat4master/internal/settings"
)

type API struct {
	settings  settings.Settings
	container container.Container
	logger    *zerolog.Logger
}

type Error struct {
	Error string
}

func New(
	settings settings.Settings,
	logger *zerolog.Logger,
	container container.Container,
) *API {
	return &API{
		container: container,
		settings:  settings,
		logger:    logger,
	}
}
