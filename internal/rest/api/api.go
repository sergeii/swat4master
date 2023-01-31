package api

import (
	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/internal/application"
)

type API struct {
	app *application.App
	cfg config.Config
}

type Error struct {
	Error string
}

func New(app *application.App, cfg config.Config) *API {
	return &API{
		app: app,
		cfg: cfg,
	}
}
