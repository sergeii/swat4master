package finder

import (
	"context"

	"github.com/benbjohnson/clock"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/internal/services/discovery/finding"
)

type Finder struct{}

func Run(
	stop chan struct{},
	stopped chan struct{},
	clock clock.Clock,
	logger *zerolog.Logger,
	service *finding.Service,
	cfg config.Config,
) {
	refresher := clock.Ticker(cfg.DiscoveryRefreshInterval)
	defer refresher.Stop()

	reviver := clock.Ticker(cfg.DiscoveryRevivalInterval)
	defer reviver.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Info().
		Dur("refresh", cfg.DiscoveryRefreshInterval).Dur("revival", cfg.DiscoveryRevivalInterval).
		Msg("Starting finder")

	for {
		select {
		case <-stop:
			logger.Info().Msg("Stopping finder")
			close(stopped)
			return
		case <-refresher.C:
			refresh(ctx, clock, logger, service, cfg)
		case <-reviver.C:
			revive(ctx, clock, logger, service, cfg)
		}
	}
}

func NewFinder(
	lc fx.Lifecycle,
	cfg config.Config,
	clock clock.Clock,
	service *finding.Service,
	logger *zerolog.Logger,
) *Finder {
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

	return &Finder{}
}

func refresh(
	ctx context.Context,
	clock clock.Clock,
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

func revive(
	ctx context.Context,
	clock clock.Clock,
	logger *zerolog.Logger,
	service *finding.Service,
	cfg config.Config,
) {
	now := clock.Now()

	// make sure the probes don't run beyond the next cycle of discovery
	deadline := now.Add(cfg.DiscoveryRevivalInterval)

	cnt, err := service.ReviveServers(
		ctx,
		now.Add(-cfg.DiscoveryRevivalScope),    // min scope
		now.Add(-cfg.DiscoveryRevivalInterval), // max scope
		now,                                    // min countdown
		now.Add(cfg.DiscoveryRevivalCountdown), // max countdown
		deadline,
	)
	if err != nil {
		logger.Warn().Err(err).Msg("Unable to refresh revive outdated servers")
		return
	}
	if cnt > 0 {
		logger.Info().Int("count", cnt).Msg("Added servers to port discovery queue")
	} else {
		logger.Debug().Msg("Added no servers to port discovery queue")
	}
}

var Module = fx.Module("finder",
	fx.Provide(NewFinder),
)
