package cleaner

import (
	"context"

	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/internal/cleanup"
	"github.com/sergeii/swat4master/internal/cleanup/cleaners/instancecleaner"
	"github.com/sergeii/swat4master/internal/cleanup/cleaners/servercleaner"
)

type Cleaner struct{}

func Run(
	stop chan struct{},
	stopped chan struct{},
	clock clockwork.Clock,
	logger *zerolog.Logger,
	manager *cleanup.Manager,
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
			manager.Clean(ctx)
		}
	}
}

func NewCleaner(
	lc fx.Lifecycle,
	cfg config.Config,
	clock clockwork.Clock,
	manager *cleanup.Manager,
	logger *zerolog.Logger,
) *Cleaner {
	stopped := make(chan struct{})
	stop := make(chan struct{})

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go Run(stop, stopped, clock, logger, manager, cfg) // nolint: contextcheck
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

type Opts struct {
	fx.Out

	ServerCleanerOpts   servercleaner.Opts
	InstanceCleanerOpts instancecleaner.Opts
}

func provideCleanerConfigs(cfg config.Config) Opts {
	return Opts{
		ServerCleanerOpts: servercleaner.Opts{
			Retention: cfg.CleanRetention,
		},
		InstanceCleanerOpts: instancecleaner.Opts{
			Retention: cfg.CleanRetention,
		},
	}
}

var Module = fx.Module("cleaner",
	fx.Provide(cleanup.NewManager),
	fx.Provide(provideCleanerConfigs),
	fx.Invoke(
		servercleaner.New,
		instancecleaner.New,
	),
	fx.Provide(NewCleaner),
)
