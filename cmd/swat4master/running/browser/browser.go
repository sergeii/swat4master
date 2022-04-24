package browser

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/running"
	"github.com/sergeii/swat4master/internal/application"
	"github.com/sergeii/swat4master/internal/services/master/browsing"
	tcp "github.com/sergeii/swat4master/pkg/tcp/server"
)

func Run(ctx context.Context, runner *running.Runner, app *application.App, cfg config.Config) {
	defer runner.Quit(ctx.Done())
	mbs := browsing.NewService(
		app.ServerService,
		browsing.WithLivenessDuration(cfg.BrowserServerLiveness),
	)
	svr, err := tcp.New(
		cfg.BrowserListenAddr,
		tcp.WithHandler(NewRequestHandler(mbs, app.MetricService).Handle),
		tcp.WithTimeout(cfg.BrowserClientTimeout),
		tcp.WithReadySignal(runner.Ready),
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
