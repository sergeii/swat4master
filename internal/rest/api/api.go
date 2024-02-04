package api

import (
	"github.com/rs/zerolog"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/container"
)

type API struct {
	cfg       config.Config
	container container.Container
	logger    *zerolog.Logger
}

type Error struct {
	Error string
}

func New(
	cfg config.Config,
	logger *zerolog.Logger,
	container container.Container,
) *API {
	return &API{
		container: container,
		cfg:       cfg,
		logger:    logger,
	}
}
