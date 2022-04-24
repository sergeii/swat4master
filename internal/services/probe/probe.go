package probe

import (
	"context"
	"time"

	"github.com/sergeii/swat4master/internal/core/probes"
	"github.com/sergeii/swat4master/internal/services/monitoring"
)

type Service struct {
	queue   probes.Repository
	metrics *monitoring.MetricService
}

func NewService(repo probes.Repository, ms *monitoring.MetricService) *Service {
	return &Service{
		queue:   repo,
		metrics: ms,
	}
}

func (s *Service) AddAfter(ctx context.Context, target probes.Target, after time.Time) error {
	err := s.queue.AddBetween(ctx, target, after, probes.NC)
	if err != nil {
		return err
	}
	s.metrics.DiscoveryQueueProduced.Inc()
	return nil
}

func (s *Service) AddBefore(ctx context.Context, target probes.Target, before time.Time) error {
	err := s.queue.AddBetween(ctx, target, probes.NC, before)
	if err != nil {
		return err
	}
	s.metrics.DiscoveryQueueProduced.Inc()
	return nil
}

func (s *Service) AddBetween(ctx context.Context, target probes.Target, after time.Time, before time.Time) error {
	err := s.queue.AddBetween(ctx, target, after, before)
	if err != nil {
		return err
	}
	s.metrics.DiscoveryQueueProduced.Inc()
	return nil
}

func (s *Service) PopMany(ctx context.Context, count int) ([]probes.Target, error) {
	targets, expired, err := s.queue.PopMany(ctx, count)
	if err != nil {
		return nil, err
	}
	s.metrics.DiscoveryQueueConsumed.Add(float64(len(targets)))
	// measure the number of expired targets
	if expired > 0 {
		s.metrics.DiscoveryQueueExpired.Add(float64(expired))
	}
	return targets, err
}
