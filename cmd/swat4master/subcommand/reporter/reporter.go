package reporter

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/cmd/swat4master/subcommand"
	reporterapi "github.com/sergeii/swat4master/internal/api/master/reporter"
	"github.com/sergeii/swat4master/internal/application"
	udp "github.com/sergeii/swat4master/pkg/udp/server"
)

func Run(ctx context.Context, gCtx *subcommand.GroupContext, app *application.App) {
	defer gCtx.Quit()
	mrs, err := reporterapi.NewService(reporterapi.WithServerRepository(app.Servers))
	if err != nil {
		log.Error().Err(err).Msg("Failed to init reporter service")
		return
	}
	svr, err := udp.New(
		gCtx.Cfg.ReporterListenAddr,
		udp.WithBufferSize(gCtx.Cfg.ReporterBufferSize),
		udp.WithHandler(NewRequestHandler(mrs, app.MetricService).Handle),
		udp.WithReadySignal(gCtx.Ready),
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
