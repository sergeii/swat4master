package cleaner

import (
	"context"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/internal/core/usecases/cleanservers"
	"github.com/sergeii/swat4master/internal/metrics"
)

type Cleaner struct{}

func Run(
	stop chan struct{},
	stopped chan struct{},
	clock clockwork.Clock,
	logger *zerolog.Logger,
	metrics *metrics.Collector,
	uc cleanservers.UseCase,
	cfg config.Config,
) {
	ticker := clock.NewTicker(cfg.CleanInterval)
	tickerCh := ticker.Chan()
	defer ticker.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Info().
		Dur("interval", cfg.CleanInterval).Dur("retention", cfg.CleanRetention).
		Msg("Starting cleaner")

	for {
		select {
		case <-stop:
			logger.Info().Msg("Stopping cleaner")
			close(stopped)
			return
		case <-tickerCh:
			clean(ctx, clock, logger, metrics, uc, cfg.CleanRetention)
		}
	}
}

func NewCleaner(
	lc fx.Lifecycle,
	cfg config.Config,
	clock clockwork.Clock,
	metrics *metrics.Collector,
	uc cleanservers.UseCase,
	logger *zerolog.Logger,
) *Cleaner {
	stopped := make(chan struct{})
	stop := make(chan struct{})

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go Run(stop, stopped, clock, logger, metrics, uc, cfg) // nolint: contextcheck
			return nil
		},
		OnStop: func(context.Context) error {
			close(stop)
			<-stopped
			return nil
		},
	})

	return &Cleaner{}
}

func clean(
	ctx context.Context,
	clock clockwork.Clock,
	logger *zerolog.Logger,
	metrics *metrics.Collector,
	uc cleanservers.UseCase,
	retention time.Duration,
) {
	resp, err := uc.Execute(ctx, clock.Now().Add(-retention))
	if err != nil {
		logger.Error().
			Err(err).
			Msg("Failed to clean outdated servers")
	}
	metrics.CleanerRemovals.Add(float64(resp.Count))
	metrics.CleanerErrors.Add(float64(resp.Errors))
}

var Module = fx.Module("cleaner",
	fx.Provide(NewCleaner),
)
