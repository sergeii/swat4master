package browser

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/cmd/swat4master/subcommand"
	browserapi "github.com/sergeii/swat4master/internal/api/master/browser"
	"github.com/sergeii/swat4master/internal/application"
	tcp "github.com/sergeii/swat4master/pkg/tcp/server"
)

func Run(ctx context.Context, gCtx *subcommand.GroupContext, app *application.App) {
	defer gCtx.Quit()
	mbs, err := browserapi.NewService(
		browserapi.WithServerRepository(app.Servers),
		browserapi.WithLivenessDuration(gCtx.Cfg.BrowserServerLiveness),
		browserapi.WithMetricService(app.MetricService),
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to init browser service")
		return
	}
	svr, err := tcp.New(
		gCtx.Cfg.BrowserListenAddr,
		tcp.WithHandler(NewRequestHandler(mbs, app.MetricService).Handle),
		tcp.WithTimeout(gCtx.Cfg.BrowserClientTimeout),
		tcp.WithReadySignal(gCtx.Ready),
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to setup TCP server for browser service")
		return
	}
	if err = svr.Listen(ctx); err != nil {
		log.Error().Err(err).Msg("TCP browser server exited prematurely")
		return
	}
}
