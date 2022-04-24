package reporter

import (
	"context"

	"github.com/sergeii/swat4master/cmd/swat4master/config"

	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/cmd/swat4master/running"
	"github.com/sergeii/swat4master/internal/application"
	"github.com/sergeii/swat4master/internal/services/master/reporting"
	udp "github.com/sergeii/swat4master/pkg/udp/server"
)

func Run(ctx context.Context, runner *running.Runner, app *application.App, cfg config.Config) {
	defer runner.Quit(ctx.Done())
	mrs := reporting.NewService(app.Servers, app.Instances, app.FindingService, app.MetricService)
	svr, err := udp.New(
		cfg.ReporterListenAddr,
		udp.WithBufferSize(cfg.ReporterBufferSize),
		udp.WithHandler(NewRequestHandler(mrs, app.MetricService).Handle),
		udp.WithReadySignal(runner.Ready),
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to setup UDP server for reporter service")
		return
	}
	if err := svr.Listen(ctx); err != nil {
		log.Error().Err(err).Msg("UDP reporter server exited prematurely")
		return
	}
}
