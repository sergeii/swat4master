package collector

import (
	"context"

	"github.com/benbjohnson/clock"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/internal/services/monitoring"
)

type Collector struct{}

func Run(
	stop chan struct{},
	stopped chan struct{},
	clock clock.Clock,
	logger *zerolog.Logger,
	metrics *monitoring.MetricService,
	cfg config.Config,
) {
	ticker := clock.Ticker(cfg.CollectorInterval)
	defer ticker.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Info().
		Dur("interval", cfg.CollectorInterval).
		Msg("Starting collector")

	for {
		select {
		case <-stop:
			logger.Info().Msg("Stopping collector")
			close(stopped)
			return
		case <-ticker.C:
			metrics.Observe(ctx, monitoring.ObserverConfig{
				ServerLiveness: cfg.BrowserServerLiveness,
			})
		}
	}
}

func NewCollector(
	lc fx.Lifecycle,
	cfg config.Config,
	metrics *monitoring.MetricService,
	clock clock.Clock,
	logger *zerolog.Logger,
) *Collector {
	stopped := make(chan struct{})
	stop := make(chan struct{})

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go Run(stop, stopped, clock, logger, metrics, cfg) // nolint: contextcheck
			return nil
		},
		OnStop: func(context.Context) error {
			close(stop)
			<-stopped
			return nil
		},
	})

	return &Collector{}
}

var Module = fx.Module("collector",
	fx.Provide(NewCollector),
)
