package application

import (
	"github.com/sergeii/swat4master/internal/api/monitoring"
	"github.com/sergeii/swat4master/internal/server"
)

type App struct {
	Servers server.Repository

	MetricService *monitoring.MetricService
}

type Option func(a *App)

func NewApp(opts ...Option) *App {
	app := &App{}
	for _, opt := range opts {
		opt(app)
	}
	return app
}

func WithServerRepository(repo server.Repository) Option {
	return func(a *App) {
		a.Servers = repo
	}
}

func WithMetricService(ms *monitoring.MetricService) Option {
	return func(a *App) {
		a.MetricService = ms
	}
}
