package cleaner

import (
	"context"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/commander"
	"github.com/sergeii/swat4master/internal/cleanup"
	"github.com/sergeii/swat4master/internal/cleanup/cleaners/instancecleaner"
	"github.com/sergeii/swat4master/internal/cleanup/cleaners/servercleaner"
)

type Config struct {
	CleanRetention time.Duration
	CleanInterval  time.Duration
}

type Component struct{}

func run(
	stop chan struct{},
	stopped chan struct{},
	clock clockwork.Clock,
	logger *zerolog.Logger,
	manager *cleanup.Manager,
	cfg Config,
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
			close(stopped)
			return
		case <-tickerCh:
			manager.Clean(ctx)
		}
	}
}

func New(
	lc fx.Lifecycle,
	cfg Config,
	clock clockwork.Clock,
	manager *cleanup.Manager,
	logger *zerolog.Logger,
) *Component {
	stopped := make(chan struct{})
	stop := make(chan struct{})

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go run(stop, stopped, clock, logger, manager, cfg) // nolint: contextcheck
			return nil
		},
		OnStop: func(context.Context) error {
			close(stop)
			<-stopped
			logger.Info().Msg("Cleaner stopped")
			return nil
		},
	})

	return &Component{}
}

type Opts struct {
	fx.Out

	ServerCleanerOpts   servercleaner.Opts
	InstanceCleanerOpts instancecleaner.Opts
}

func provideCleanerConfigs(cfg Config) Opts {
	return Opts{
		ServerCleanerOpts: servercleaner.Opts{
			Retention: cfg.CleanRetention,
		},
		InstanceCleanerOpts: instancecleaner.Opts{
			Retention: cfg.CleanRetention,
		},
	}
}

type command struct {
	CleanRetention time.Duration `default:"1h"  help:"Sets how long a game server is kept after going offline"`
	CleanInterval  time.Duration `default:"10m" help:"Sets how often offline servers are cleaned up"`
}

func (c *command) Run(_ *commander.Globals, builder *application.Builder) error {
	app := builder.
		Add(
			fx.Supply(Config{
				CleanRetention: c.CleanRetention,
				CleanInterval:  c.CleanInterval,
			}),
			Module,
			fx.Invoke(func(_ *Component) {}),
		).
		WithExporter().
		Build()
	app.Run()
	return nil
}

type CLI struct {
	Cleaner command `cmd:"" help:"Start cleaner"`
}

var Module = fx.Module("cleaner",
	fx.Provide(cleanup.NewManager),
	fx.Provide(provideCleanerConfigs),
	fx.Invoke(
		servercleaner.New,
		instancecleaner.New,
	),
	fx.Provide(New),
)
