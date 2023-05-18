package cleaner

import (
	"context"

	"github.com/benbjohnson/clock"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/internal/services/cleaning"
)

type Cleaner struct{}

func Run(
	stop chan struct{},
	stopped chan struct{},
	clock clock.Clock,
	logger *zerolog.Logger,
	service *cleaning.Service,
	cfg config.Config,
) {
	ticker := clock.Ticker(cfg.CleanInterval)
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
		case <-ticker.C:
			if err := service.Clean(ctx, clock.Now().Add(-cfg.CleanRetention)); err != nil {
				logger.Error().
					Err(err).
					Msg("Failed to clean outdated servers")
			}
		}
	}
}

func NewCleaner(
	lc fx.Lifecycle,
	cfg config.Config,
	clock clock.Clock,
	service *cleaning.Service,
	logger *zerolog.Logger,
) *Cleaner {
	stopped := make(chan struct{})
	stop := make(chan struct{})

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go Run(stop, stopped, clock, logger, service, cfg) // nolint: contextcheck
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

var Module = fx.Module("cleaner",
	fx.Provide(
		fx.Private,
		cleaning.NewService,
	),
	fx.Provide(NewCleaner),
)
