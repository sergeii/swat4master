package servercleaner

import (
	"context"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"

	"github.com/sergeii/swat4master/internal/cleanup"
	"github.com/sergeii/swat4master/internal/core/entities/filterset"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/metrics"
)

type Opts struct {
	Retention time.Duration
}

type ServerCleaner struct {
	opts       Opts
	serverRepo repositories.ServerRepository
	clock      clockwork.Clock
	metrics    *metrics.Collector
	logger     *zerolog.Logger
}

func New(
	manager *cleanup.Manager,
	opts Opts,
	serverRepo repositories.ServerRepository,
	clock clockwork.Clock,
	metrics *metrics.Collector,
	logger *zerolog.Logger,
) ServerCleaner {
	cleaner := ServerCleaner{
		opts:       opts,
		serverRepo: serverRepo,
		clock:      clock,
		metrics:    metrics,
		logger:     logger,
	}
	manager.AddCleaner(&cleaner)
	return cleaner
}

func (c ServerCleaner) Clean(ctx context.Context) {
	cleanUntil := c.clock.Now().Add(-c.opts.Retention)
	fs := filterset.NewServerFilterSet().UpdatedBefore(cleanUntil)

	c.logger.Info().Stringer("until", cleanUntil).Msg("Starting to clean outdated servers")

	outdatedServers, err := c.serverRepo.Filter(ctx, fs)
	if err != nil {
		c.logger.Error().Err(err).Msg("Unable to obtain servers for cleanup")
		return
	}

	removed, errors := c.cleanServers(ctx, outdatedServers, cleanUntil)

	c.metrics.CleanerRemovals.WithLabelValues("servers").Add(float64(removed))
	c.metrics.CleanerErrors.WithLabelValues("servers").Add(float64(errors))
	c.logger.Info().
		Stringer("until", cleanUntil).
		Int("removed", removed).Int("errors", errors).
		Msg("Finished cleaning servers")
}

func (c ServerCleaner) cleanServers(
	ctx context.Context,
	servers []server.Server,
	cleanUntil time.Time,
) (int, int) {
	removed, errors := 0, 0
	for _, svr := range servers {
		err := c.serverRepo.Remove(ctx, svr, func(conflict *server.Server) bool {
			if conflict.RefreshedAt.After(cleanUntil) {
				c.logger.Info().
					Stringer("server", conflict).Stringer("refreshed", conflict.RefreshedAt).
					Msg("Removed server is more recent")
				return false
			}
			return true
		})
		if err != nil {
			c.logger.Error().
				Err(err).
				Stringer("until", cleanUntil).Stringer("addr", svr.Addr).
				Msg("Failed to remove outdated server")
			errors++
			continue
		}
		removed++
	}
	return removed, errors
}
