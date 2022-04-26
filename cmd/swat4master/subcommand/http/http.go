package http

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/cmd/swat4master/subcommand"
	"github.com/sergeii/swat4master/cmd/swat4master/subcommand/http/router"
	"github.com/sergeii/swat4master/internal/application"
	http "github.com/sergeii/swat4master/pkg/http/server"
)

func Run(ctx context.Context, gCtx *subcommand.GroupContext, app *application.App) {
	defer gCtx.Quit()
	svr, err := http.New(
		gCtx.Cfg.HTTPListenAddr,
		http.WithShutdownTimeout(gCtx.Cfg.HTTPShutdownTimeout),
		http.WithReadTimeout(gCtx.Cfg.HTTPReadTimeout),
		http.WithWriteTimeout(gCtx.Cfg.HTTPWriteTimeout),
		http.WithHandler(router.New()),
		http.WithReadySignal(gCtx.Ready),
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
