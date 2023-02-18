package cleaning

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/sergeii/swat4master/internal/core/instances"
	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/services/monitoring"
)

type Service struct {
	servers   servers.Repository
	instances instances.Repository
	metrics   *monitoring.MetricService
	logger    *zerolog.Logger
}

func NewService(
	servers servers.Repository,
	instances instances.Repository,
	metrics *monitoring.MetricService,
	logger *zerolog.Logger,
) *Service {
	return &Service{
		servers:   servers,
		instances: instances,
		metrics:   metrics,
		logger:    logger,
	}
}

func (s *Service) Clean(ctx context.Context, until time.Time) error {
	var before, after, removed, errors int
	var err error

	if before, err = s.servers.Count(ctx); err != nil {
		return err
	}
	s.logger.Info().
		Stringer("until", until).Int("servers", before).
		Msg("Starting to clean outdated servers")

	fs := servers.NewFilterSet().UpdatedBefore(until)
	outdatedServers, err := s.servers.Filter(ctx, fs)
	if err != nil {
		s.logger.Error().Err(err).Msg("Unable to obtain servers for cleanup")
		return err
	}

	for _, svr := range outdatedServers {
		if err = s.instances.RemoveByAddr(ctx, svr.GetAddr()); err != nil {
			s.logger.Error().
				Err(err).
				Stringer("until", until).Stringer("addr", svr.GetAddr()).
				Msg("Failed to remove instance for removed server")
			errors++
			continue
		}
		if err = s.servers.Remove(ctx, svr, func(conflict *servers.Server) bool {
			refreshedAt := conflict.GetRefreshedAt()
			if refreshedAt.After(until) {
				s.logger.Info().
					Stringer("server", conflict).Stringer("refreshed", refreshedAt).
					Msg("Removed server is more recent")
				return false
			}
			return true
		}); err != nil {
			s.logger.Error().
				Err(err).
				Stringer("until", until).Stringer("addr", svr.GetAddr()).
				Msg("Failed to remove outdated server")
			errors++
			continue
		}
		removed++
	}

	s.metrics.CleanerRemovals.Add(float64(removed))
	s.metrics.CleanerErrors.Add(float64(errors))

	if after, err = s.servers.Count(ctx); err != nil {
		return err
	}

	s.logger.Info().
		Stringer("until", until).
		Int("removed", removed).Int("errors", errors).
		Int("before", before).Int("after", after).
		Msg("Finished cleaning outdated servers")

	return nil
}
