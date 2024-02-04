package probe

import (
	"context"
	"time"

	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/services/monitoring"
)

type Service struct {
	queue   repositories.ProbeRepository
	metrics *monitoring.MetricService
}

func NewService(repo repositories.ProbeRepository, metrics *monitoring.MetricService) *Service {
	return &Service{
		queue:   repo,
		metrics: metrics,
	}
}

func (s *Service) AddAfter(ctx context.Context, prb probe.Probe, after time.Time) error {
	err := s.queue.AddBetween(ctx, prb, after, repositories.NC)
	if err != nil {
		return err
	}
	s.metrics.DiscoveryQueueProduced.Inc()
	return nil
}

func (s *Service) AddBefore(ctx context.Context, prb probe.Probe, before time.Time) error {
	err := s.queue.AddBetween(ctx, prb, repositories.NC, before)
	if err != nil {
		return err
	}
	s.metrics.DiscoveryQueueProduced.Inc()
	return nil
}

func (s *Service) AddBetween(ctx context.Context, prb probe.Probe, after time.Time, before time.Time) error {
	err := s.queue.AddBetween(ctx, prb, after, before)
	if err != nil {
		return err
	}
	s.metrics.DiscoveryQueueProduced.Inc()
	return nil
}

func (s *Service) PopMany(ctx context.Context, count int) ([]probe.Probe, error) {
	probes, expired, err := s.queue.PopMany(ctx, count)
	if err != nil {
		return nil, err
	}
	s.metrics.DiscoveryQueueConsumed.Add(float64(len(probes)))
	// measure the number of expired probes
	if expired > 0 {
		s.metrics.DiscoveryQueueExpired.Add(float64(expired))
	}
	return probes, err
}
