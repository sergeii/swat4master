package refresher

import (
	"context"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/commander"
	"github.com/sergeii/swat4master/internal/core/usecases/refreshservers"
)

type Config struct {
	RefreshInterval time.Duration
}

type Component struct{}

func run(
	stop chan struct{},
	stopped chan struct{},
	clock clockwork.Clock,
	logger *zerolog.Logger,
	uc refreshservers.UseCase,
	cfg Config,
) {
	refresher := clock.NewTicker(cfg.RefreshInterval)
	defer refresher.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Info().Dur("interval", cfg.RefreshInterval).Msg("Starting refresher")

	for {
		select {
		case <-stop:
			close(stopped)
			return
		case <-refresher.Chan():
			refresh(ctx, clock, logger, uc, cfg)
		}
	}
}

func New(
	lc fx.Lifecycle,
	cfg Config,
	clock clockwork.Clock,
	uc refreshservers.UseCase,
	logger *zerolog.Logger,
) *Component {
	stopped := make(chan struct{})
	stop := make(chan struct{})

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go run(stop, stopped, clock, logger, uc, cfg) // nolint: contextcheck
			return nil
		},
		OnStop: func(context.Context) error {
			close(stop)
			<-stopped
			logger.Info().Msg("Refresher stopped")
			return nil
		},
	})

	return &Component{}
}

func refresh(
	ctx context.Context,
	clock clockwork.Clock,
	logger *zerolog.Logger,
	uc refreshservers.UseCase,
	cfg Config,
) {
	// make sure the probes don't run beyond the next cycle of discovery
	deadline := clock.Now().Add(cfg.RefreshInterval)

	ucRequest := refreshservers.NewRequest(deadline)
	result, err := uc.Execute(ctx, ucRequest)
	if err != nil {
		logger.Warn().Err(err).Msg("Unable to refresh details for servers")
		return
	}

	if result.Count > 0 {
		logger.Info().Int("count", result.Count).Msg("Added servers to refresh queue")
	} else {
		logger.Debug().Msg("Added no servers to refresh queue")
	}
}

type command struct{}

func (c *command) Run(globals *commander.Globals, builder *application.Builder) error {
	app := builder.
		Add(
			fx.Supply(Config{
				RefreshInterval: globals.DiscoveryRefreshInterval,
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
	Refresher command `cmd:"" help:"Start refresher"`
}

var Module = fx.Module("refresher",
	fx.Provide(New),
)
