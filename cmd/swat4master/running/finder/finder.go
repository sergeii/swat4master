package finder

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/running"
	"github.com/sergeii/swat4master/internal/application"
	"github.com/sergeii/swat4master/internal/core/servers"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
	"github.com/sergeii/swat4master/internal/services/discovery/finding"
	"github.com/sergeii/swat4master/pkg/random"
)

func Run(ctx context.Context, runner *running.Runner, app *application.App, cfg config.Config) {
	defer runner.Quit(ctx.Done())

	refresher := time.NewTicker(cfg.DiscoveryRefreshInterval)
	defer refresher.Stop()

	reviver := time.NewTicker(cfg.DiscoveryRevivalInterval)
	defer reviver.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Stopping finder")
			return
		case <-refresher.C:
			refresh(ctx, app.Servers, app.FindingService, cfg)
		case <-reviver.C:
			revive(ctx, app.Servers, app.FindingService, cfg)
		}
	}
}

func refresh(
	ctx context.Context,
	repo servers.Repository,
	service *finding.Service,
	cfg config.Config,
) {
	fs := servers.NewFilterSet().WithStatus(ds.Port).NoStatus(ds.DetailsRetry)
	serversWithDetails, err := repo.Filter(ctx, fs)
	if err != nil {
		log.Error().Err(err).Msg("Unable to obtain servers for details discovery")
		return
	}

	// make sure the probes don't run beyond the next cycle of discovery
	deadline := time.Now().Add(cfg.DiscoveryRefreshInterval)

	cnt := 0
	for _, svr := range serversWithDetails {
		if err := service.DiscoverDetails(ctx, svr.GetAddr(), svr.GetQueryPort(), deadline); err != nil {
			log.Warn().
				Err(err).Stringer("server", svr).
				Msg("Failed to add server to details discovery queue")
			continue
		}
		cnt++
	}

	if cnt > 0 {
		log.Info().Int("count", cnt).Msg("Added servers to details discovery queue")
	} else {
		log.Debug().Msg("Added no servers to details discovery queue")
	}
}

func revive(
	ctx context.Context,
	repo servers.Repository,
	service *finding.Service,
	cfg config.Config,
) {
	now := time.Now()

	from := now.Add(-cfg.DiscoveryRevivalScope)
	until := now.Add(-cfg.DiscoveryRevivalInterval)
	fs := servers.NewFilterSet().After(from).Before(until).NoStatus(ds.Port | ds.PortRetry)

	serversWithoutPort, err := repo.Filter(ctx, fs)
	if err != nil {
		log.Error().Err(err).Msg("Unable to obtain servers for port discovery")
		return
	}

	// spread the probes within a random time range
	maxCountdown := int(cfg.DiscoveryRevivalInterval/time.Second) / 2
	if maxCountdown <= 0 {
		maxCountdown = 1
	}
	deadline := now.Add(cfg.DiscoveryRevivalInterval)

	cnt := 0
	for _, svr := range serversWithoutPort {
		countdown := random.RandInt(0, maxCountdown)
		notBefore := now.Add(time.Second * time.Duration(countdown))
		if err := service.DiscoverPort(ctx, svr.GetAddr(), notBefore, deadline); err != nil {
			log.Warn().
				Err(err).Stringer("server", svr).
				Msg("Failed to add server to port discovery queue")
			continue
		} else {
			log.Debug().
				Int("countdown", countdown).
				Stringer("server", svr).
				Msg("Added server to port discovery queue")
		}
		cnt++
	}

	if cnt > 0 {
		log.Info().Int("count", cnt).Msg("Added servers to port discovery queue")
	} else {
		log.Debug().Msg("Added no servers to port discovery queue")
	}
}
