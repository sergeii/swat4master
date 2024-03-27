package reviver

import (
	"context"

	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/internal/core/usecases/reviveservers"
	"github.com/sergeii/swat4master/internal/services/monitoring"
)

type Reviver struct{}

func Run(
	stop chan struct{},
	stopped chan struct{},
	clock clockwork.Clock,
	logger *zerolog.Logger,
	metrics *monitoring.MetricService,
	uc reviveservers.UseCase,
	cfg config.Config,
) {
	reviver := clock.NewTicker(cfg.DiscoveryRevivalInterval)
	reviverCh := reviver.Chan()
	defer reviver.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Info().
		Dur("interval", cfg.DiscoveryRevivalInterval).
		Dur("countdown", cfg.DiscoveryRevivalCountdown).
		Dur("scope", cfg.DiscoveryRevivalScope).
		Msg("Starting reviver")

	for {
		select {
		case <-stop:
			logger.Info().Msg("Stopping reviver")
			close(stopped)
			return
		case <-reviverCh:
			revive(ctx, clock, logger, metrics, uc, cfg)
		}
	}
}

func NewReviver(
	lc fx.Lifecycle,
	cfg config.Config,
	clock clockwork.Clock,
	metrics *monitoring.MetricService,
	uc reviveservers.UseCase,
	logger *zerolog.Logger,
) *Reviver {
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

	return &Reviver{}
}

func revive(
	ctx context.Context,
	clock clockwork.Clock,
	logger *zerolog.Logger,
	metrics *monitoring.MetricService,
	uc reviveservers.UseCase,
	cfg config.Config,
) {
	now := clock.Now()

	// make sure the probes don't run beyond the next cycle of discovery
	deadline := now.Add(cfg.DiscoveryRevivalInterval)

	ucRequest := reviveservers.NewRequest(
		now.Add(-cfg.DiscoveryRevivalScope),    // min scope
		now.Add(-cfg.DiscoveryRevivalInterval), // max scope
		now,                                    // min countdown
		now.Add(cfg.DiscoveryRevivalCountdown), // max countdown
		deadline,
	)
	result, err := uc.Execute(ctx, ucRequest)
	if err != nil {
		logger.Warn().Err(err).Msg("Unable to revive outdated servers")
		return
	}

	if result.Count > 0 {
		metrics.DiscoveryQueueProduced.Add(float64(result.Count))
		logger.Info().Int("count", result.Count).Msg("Added servers to revival discovery queue")
	} else {
		logger.Debug().Msg("Added no servers to revival discovery queue")
	}
}

var Module = fx.Module("reviver",
	fx.Provide(NewReviver),
)
