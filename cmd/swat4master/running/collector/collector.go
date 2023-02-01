package collector

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/running"
	"github.com/sergeii/swat4master/internal/application"
)

func Run(ctx context.Context, runner *running.Runner, app *application.App, cfg config.Config) {
	defer runner.Quit(ctx.Done())

	ticker := time.NewTicker(cfg.CollectorInterval)
	defer ticker.Stop()

	log.Info().Dur("interval", cfg.CollectorInterval).Msg("Starting collector")

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Stopping collector")
			return
		case <-ticker.C:
			app.MetricService.Observe(ctx, cfg, app.Servers, app.Instances, app.Probes)
		}
	}
}
