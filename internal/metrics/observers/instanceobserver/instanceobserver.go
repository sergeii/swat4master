package instanceobserver

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/metrics"
)

type InstanceObserver struct {
	instanceRepo repositories.InstanceRepository
	logger       *zerolog.Logger
}

func New(
	collector *metrics.Collector,
	instanceRepo repositories.InstanceRepository,
	logger *zerolog.Logger,
) InstanceObserver {
	observer := InstanceObserver{
		instanceRepo: instanceRepo,
		logger:       logger,
	}
	collector.AddObserver(&observer)
	return observer
}

func (o InstanceObserver) Observe(ctx context.Context, m *metrics.Collector) {
	o.observeInstanceRepoSize(ctx, m)
}

func (o InstanceObserver) observeInstanceRepoSize(ctx context.Context, m *metrics.Collector) {
	count, err := o.instanceRepo.Count(ctx)
	if err != nil {
		o.logger.Error().Err(err).Msg("Unable to observe instance count")
		return
	}
	m.InstanceRepositorySize.Set(float64(count))
	o.logger.Debug().Int("count", count).Msg("Observed instance count")
}
