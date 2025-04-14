package reviver

import (
	"context"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/commander"
	"github.com/sergeii/swat4master/internal/core/usecases/reviveservers"
)

type Config struct {
	RevivalInterval  time.Duration
	RevivalCountdown time.Duration
	RevivalScope     time.Duration
}

type Component struct{}

func run(
	stop chan struct{},
	stopped chan struct{},
	clock clockwork.Clock,
	logger *zerolog.Logger,
	uc reviveservers.UseCase,
	cfg Config,
) {
	reviver := clock.NewTicker(cfg.RevivalInterval)
	reviverCh := reviver.Chan()
	defer reviver.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Info().
		Dur("interval", cfg.RevivalInterval).
		Dur("countdown", cfg.RevivalCountdown).
		Dur("scope", cfg.RevivalScope).
		Msg("Starting reviver")

	for {
		select {
		case <-stop:
			close(stopped)
			return
		case <-reviverCh:
			revive(ctx, clock, logger, uc, cfg)
		}
	}
}

func New(
	lc fx.Lifecycle,
	cfg Config,
	clock clockwork.Clock,
	uc reviveservers.UseCase,
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
			logger.Info().Msg("Reviver stopped")
			return nil
		},
	})

	return &Component{}
}

func revive(
	ctx context.Context,
	clock clockwork.Clock,
	logger *zerolog.Logger,
	uc reviveservers.UseCase,
	cfg Config,
) {
	now := clock.Now()

	// make sure the probes don't run beyond the next cycle of discovery
	deadline := now.Add(cfg.RevivalInterval)

	ucRequest := reviveservers.NewRequest(
		now.Add(-cfg.RevivalScope),    // min scope
		now.Add(-cfg.RevivalInterval), // max scope
		now,                           // min countdown
		now.Add(cfg.RevivalCountdown), // max countdown
		deadline,
	)
	result, err := uc.Execute(ctx, ucRequest)
	if err != nil {
		logger.Warn().Err(err).Msg("Unable to revive outdated servers")
		return
	}

	if result.Count > 0 {
		logger.Info().Int("count", result.Count).Msg("Added servers to revival queue")
	} else {
		logger.Debug().Msg("Added no servers to revival queue")
	}
}

type command struct{}

func (c *command) Run(globals *commander.Globals, builder *application.Builder) error {
	app := builder.
		Add(
			fx.Supply(Config{
				RevivalInterval:  globals.DiscoveryRevivalInterval,
				RevivalCountdown: globals.DiscoveryRevivalCountdown,
				RevivalScope:     globals.DiscoveryRevivalScope,
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
	Reviver command `cmd:"" help:"Start reviver"`
}

var Module = fx.Module("reviver",
	fx.Provide(New),
)
