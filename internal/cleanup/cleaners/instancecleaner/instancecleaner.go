package instancecleaner

import (
	"context"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"

	"github.com/sergeii/swat4master/internal/cleanup"
	"github.com/sergeii/swat4master/internal/core/entities/filterset"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/metrics"
)

type Opts struct {
	Retention time.Duration
}

type InstanceCleaner struct {
	opts         Opts
	instanceRepo repositories.InstanceRepository
	clock        clockwork.Clock
	metrics      *metrics.Collector
	logger       *zerolog.Logger
}

func New(
	manager *cleanup.Manager,
	opts Opts,
	instanceRepo repositories.InstanceRepository,
	clock clockwork.Clock,
	metrics *metrics.Collector,
	logger *zerolog.Logger,
) InstanceCleaner {
	cleaner := InstanceCleaner{
		opts:         opts,
		instanceRepo: instanceRepo,
		clock:        clock,
		metrics:      metrics,
		logger:       logger,
	}
	manager.AddCleaner(&cleaner)
	return cleaner
}

func (c InstanceCleaner) Clean(ctx context.Context) {
	// Calculate the cutoff time for cleaning instances.
	cleanUntil := c.clock.Now().Add(-c.opts.Retention)
	fs := filterset.NewInstanceFilterSet().UpdatedBefore(cleanUntil)

	c.logger.Info().Stringer("until", cleanUntil).Msg("Starting to clean instances")

	count, err := c.instanceRepo.Clear(ctx, fs)
	if err != nil {
		c.metrics.CleanerErrors.WithLabelValues("instances").Inc()
		c.logger.Error().Err(err).Stringer("until", cleanUntil).Msg("Failed to clean instances")
		return
	}

	c.metrics.CleanerRemovals.WithLabelValues("instances").Add(float64(count))
	c.logger.Info().Stringer("until", cleanUntil).Int("removed", count).Msg("Finished cleaning instances")
}
