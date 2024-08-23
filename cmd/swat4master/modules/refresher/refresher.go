package refresher

import (
	"context"

	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/internal/core/usecases/refreshservers"
	"github.com/sergeii/swat4master/internal/services/monitoring"
)

type Refresher struct{}

func Run(
	stop chan struct{},
	stopped chan struct{},
	clock clockwork.Clock,
	logger *zerolog.Logger,
	metrics *monitoring.MetricService,
	uc refreshservers.UseCase,
	cfg config.Config,
) {
	refresher := clock.NewTicker(cfg.DiscoveryRefreshInterval)
	defer refresher.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Info().Dur("interval", cfg.DiscoveryRefreshInterval).Msg("Starting refresher")

	for {
		select {
		case <-stop:
			logger.Info().Msg("Stopping refresher")
			close(stopped)
			return
		case <-refresher.Chan():
			refresh(ctx, clock, logger, metrics, uc, cfg)
		}
	}
}

func NewRefresher(
	lc fx.Lifecycle,
	cfg config.Config,
	clock clockwork.Clock,
	metrics *monitoring.MetricService,
	uc refreshservers.UseCase,
	logger *zerolog.Logger,
) *Refresher {
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

	return &Refresher{}
}

func refresh(
	ctx context.Context,
	clock clockwork.Clock,
	logger *zerolog.Logger,
	metrics *monitoring.MetricService,
	uc refreshservers.UseCase,
	cfg config.Config,
) {
	// make sure the probes don't run beyond the next cycle of discovery
	deadline := clock.Now().Add(cfg.DiscoveryRefreshInterval)

	ucRequest := refreshservers.NewRequest(deadline)
	result, err := uc.Execute(ctx, ucRequest)
	if err != nil {
		logger.Warn().Err(err).Msg("Unable to refresh details for servers")
		return
	}

	if result.Count > 0 {
		metrics.DiscoveryQueueProduced.Add(float64(result.Count))
		logger.Info().Int("count", result.Count).Msg("Added servers to details discovery queue")
	} else {
		logger.Debug().Msg("Added no servers to details discovery queue")
	}
}

var Module = fx.Module("refresher",
	fx.Provide(NewRefresher),
)
