package prober

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/commander"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/prober/probers"
	"github.com/sergeii/swat4master/internal/prober/probers/detailsprober"
	"github.com/sergeii/swat4master/internal/prober/probers/portprober"
	"github.com/sergeii/swat4master/internal/prober/proberunner"
)

type Config struct {
	PollInterval time.Duration
	Concurrency  int
	ProbeTimeout time.Duration
	PortOffsets  []int
}

type Component struct{}

func run(
	stop chan struct{},
	stopped chan struct{},
	logger *zerolog.Logger,
	runner *proberunner.Runner,
	cfg Config,
) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Info().
		Dur("interval", cfg.PollInterval).
		Int("concurrency", cfg.Concurrency).
		Dur("timeout", cfg.ProbeTimeout).
		Msg("Starting prober")

	runner.Start(ctx)

	<-stop
	close(stopped)
}

func New(
	lc fx.Lifecycle,
	runner *proberunner.Runner,
	logger *zerolog.Logger,
	cfg Config,
) *Component {
	stopped := make(chan struct{})
	stop := make(chan struct{})

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go run(stop, stopped, logger, runner, cfg) // nolint: contextcheck
			return nil
		},
		OnStop: func(context.Context) error {
			close(stop)
			<-stopped
			logger.Info().Msg("Prober stopped")
			return nil
		},
	})

	return &Component{}
}

type command struct{}

func (c *command) Run(globals *commander.Globals, builder *application.Builder) error {
	app := builder.
		Add(
			fx.Supply(Config{
				PollInterval: globals.ProbePollSchedule,
				Concurrency:  globals.ProbeConcurrency,
				ProbeTimeout: globals.ProbeTimeout,
				PortOffsets:  globals.DiscoveryRevivalPorts,
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
	Prober command `cmd:"" help:"Start prober"`
}

func provideRunnerOpts(cfg Config) proberunner.RunnerOpts {
	return proberunner.RunnerOpts{
		PollInterval: cfg.PollInterval,
		Concurrency:  cfg.Concurrency,
		ProbeTimeout: cfg.ProbeTimeout,
	}
}

func providePortProberOpts(cfg Config) portprober.Opts {
	return portprober.Opts{
		Offsets: cfg.PortOffsets,
	}
}

var Module = fx.Module("prober",
	fx.Provide(fx.Private, provideRunnerOpts),
	fx.Provide(fx.Private, providePortProberOpts),
	fx.Provide(
		portprober.New,
		detailsprober.New,
	),
	fx.Provide(
		fx.Private,
		proberunner.New,
	),
	fx.Provide(
		func(portProber portprober.PortProber, detailsProber detailsprober.DetailsProber) probers.ForGoal {
			return probers.ForGoal{
				probe.GoalPort:    portProber,
				probe.GoalDetails: detailsProber,
			}
		},
	),
	fx.Provide(New),
)
