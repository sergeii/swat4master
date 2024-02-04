package refresher

import (
	"context"

	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/internal/services/discovery/finding"
)

type Refresher struct{}

func Run(
	stop chan struct{},
	stopped chan struct{},
	clock clockwork.Clock,
	logger *zerolog.Logger,
	service *finding.Service,
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
			refresh(ctx, clock, logger, service, cfg)
		}
	}
}

func NewRefresher(
	lc fx.Lifecycle,
	cfg config.Config,
	clock clockwork.Clock,
	service *finding.Service,
	logger *zerolog.Logger,
) *Refresher {
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

	return &Refresher{}
}

func refresh(
	ctx context.Context,
	clock clockwork.Clock,
	logger *zerolog.Logger,
	service *finding.Service,
	cfg config.Config,
) {
	// make sure the probes don't run beyond the next cycle of discovery
	deadline := clock.Now().Add(cfg.DiscoveryRefreshInterval)
	cnt, err := service.RefreshDetails(ctx, deadline)
	if err != nil {
		logger.Warn().Err(err).Msg("Unable to refresh details for servers")
		return
	}
	if cnt > 0 {
		logger.Info().Int("count", cnt).Msg("Added servers to details discovery queue")
	} else {
		logger.Debug().Msg("Added no servers to details discovery queue")
	}
}

var Module = fx.Module("refresher",
	fx.Provide(NewRefresher),
)
