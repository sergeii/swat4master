package api

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/running"
	"github.com/sergeii/swat4master/internal/application"
	"github.com/sergeii/swat4master/internal/rest/api"
	http "github.com/sergeii/swat4master/pkg/http/server"
)

func Run(ctx context.Context, runner *running.Runner, app *application.App, cfg config.Config) {
	defer runner.Quit(ctx.Done())
	rest := api.New(app, cfg)
	svr, err := http.New(
		cfg.HTTPListenAddr,
		http.WithShutdownTimeout(cfg.HTTPShutdownTimeout),
		http.WithReadTimeout(cfg.HTTPReadTimeout),
		http.WithWriteTimeout(cfg.HTTPWriteTimeout),
		http.WithHandler(NewRouter(rest)),
		http.WithReadySignal(runner.Ready),
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to setup HTTP server")
		return
	}
	if err = svr.ListenAndServe(ctx); err != nil {
		log.Error().Err(err).Msg("HTTP server exited prematurely")
		return
	}
}
