package prober

import (
	"context"

	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/prober/probers"
	"github.com/sergeii/swat4master/internal/prober/probers/detailsprober"
	"github.com/sergeii/swat4master/internal/prober/probers/portprober"
	"github.com/sergeii/swat4master/internal/prober/proberunner"
)

type Prober struct{}

func Run(
	stop chan struct{},
	stopped chan struct{},
	logger *zerolog.Logger,
	wp *proberunner.Runner,
) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wp.Start(ctx)

	<-stop
	logger.Info().Msg("Stopping prober")
	close(stopped)
}

func New(
	lc fx.Lifecycle,
	wp *proberunner.Runner,
	logger *zerolog.Logger,
) *Prober {
	stopped := make(chan struct{})
	stop := make(chan struct{})

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go Run(stop, stopped, logger, wp) // nolint: contextcheck
			return nil
		},
		OnStop: func(context.Context) error {
			close(stop)
			<-stopped
			return nil
		},
	})

	return &Prober{}
}

var Module = fx.Module("prober",
	fx.Provide(
		fx.Private,
		func(cfg config.Config) proberunner.RunnerOpts {
			return proberunner.RunnerOpts{
				PollInterval: cfg.ProbePollSchedule,
				Concurrency:  cfg.ProbeConcurrency,
				ProbeTimeout: cfg.ProbeTimeout,
			}
		},
		func(cfg config.Config) portprober.Opts {
			return portprober.Opts{
				Offsets: cfg.DiscoveryRevivalPorts,
			}
		},
	),
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
