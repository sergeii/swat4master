package probeobserver

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/metrics"
)

type ProbeObserver struct {
	probeRepo repositories.ProbeRepository
	logger    *zerolog.Logger
}

func New(
	collector *metrics.Collector,
	probeRepo repositories.ProbeRepository,
	logger *zerolog.Logger,
) ProbeObserver {
	observer := ProbeObserver{
		probeRepo: probeRepo,
		logger:    logger,
	}
	collector.AddObserver(&observer)
	return observer
}

func (o ProbeObserver) Observe(ctx context.Context, m *metrics.Collector) {
	o.observeProbeRepoSize(ctx, m)
}

func (o ProbeObserver) observeProbeRepoSize(ctx context.Context, m *metrics.Collector) {
	count, err := o.probeRepo.Count(ctx)
	if err != nil {
		o.logger.Error().Err(err).Msg("Unable to observe probe count")
		return
	}
	m.ProbeRepositorySize.Set(float64(count))
}
