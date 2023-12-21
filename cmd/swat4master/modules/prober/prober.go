package prober

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/services/discovery/probing"
	"github.com/sergeii/swat4master/internal/services/discovery/probing/probers"
	"github.com/sergeii/swat4master/internal/services/monitoring"
	ps "github.com/sergeii/swat4master/internal/services/probe"
)

type Prober struct{}

func provideWorkerGroup(
	cfg config.Config,
	service *probing.Service,
	metrics *monitoring.MetricService,
	logger *zerolog.Logger,
) *WorkerGroup {
	return NewWorkerGroup(
		cfg.ProbeConcurrency,
		service,
		metrics,
		logger,
	)
}

func Run(
	stop chan struct{},
	stopped chan struct{},
	logger *zerolog.Logger,
	queue *ps.Service,
	wg *WorkerGroup,
	cfg config.Config,
) {
	ticker := time.NewTicker(cfg.ProbePollSchedule)
	defer ticker.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool := wg.Run(ctx)

	logger.Info().
		Dur("interval", cfg.ProbePollSchedule).
		Dur("timeout", cfg.ProbeTimeout).
		Int("concurrency", cfg.ProbeConcurrency).
		Msg("Starting prober")

	for {
		select {
		case <-stop:
			logger.Info().Msg("Stopping prober")
			close(stopped)
			return
		case <-ticker.C:
			feed(ctx, logger, wg, pool, queue)
		}
	}
}

type Params struct {
	fx.In

	// not used, required for dependency
	*probers.PortProber
	*probers.DetailsProber
}

func NewProber(
	lc fx.Lifecycle,
	cfg config.Config,
	queue *ps.Service,
	wg *WorkerGroup,
	logger *zerolog.Logger,
	_ Params,
) *Prober {
	stopped := make(chan struct{})
	stop := make(chan struct{})

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go Run(stop, stopped, logger, queue, wg, cfg) // nolint: contextcheck
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

func feed(
	ctx context.Context,
	logger *zerolog.Logger,
	wg *WorkerGroup,
	pool chan probe.Probe,
	queue *ps.Service,
) {
	availability := wg.Available()
	if availability <= 0 {
		logger.Info().Int("availability", availability).Msg("Workers are busy")
		return
	}

	probes, err := queue.PopMany(ctx, availability)
	if err != nil {
		logger.Warn().
			Err(err).Int("availability", availability).
			Msg("Unable to fetch new probes")
		return
	}

	if len(probes) == 0 {
		return
	}

	logger.Debug().
		Int("availability", availability).Int("probes", len(probes)).
		Msg("Obtained probes")

	for _, prb := range probes {
		pool <- prb
	}

	logger.Debug().
		Int("availability", availability).Int("probes", len(probes)).
		Msg("Sent probes to work pool")
}

var Module = fx.Module("prober",
	fx.Provide(
		fx.Private,
		func(cfg config.Config) probing.ServiceOpts {
			return probing.ServiceOpts{
				MaxRetries:   cfg.ProbeRetries,
				ProbeTimeout: cfg.ProbeTimeout,
			}
		},
		func(cfg config.Config) probers.PortProberOpts {
			return probers.PortProberOpts{
				Offsets: cfg.DiscoveryRevivalPorts,
			}
		},
	),
	fx.Provide(
		fx.Private,
		probing.NewService,
	),
	fx.Provide(
		fx.Private,
		provideWorkerGroup,
	),
	fx.Provide(
		fx.Private,
		probers.NewDetailsProber,
	),
	fx.Provide(
		fx.Private,
		probers.NewPortProber,
	),
	fx.Provide(NewProber),
)
