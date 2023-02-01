package finding

import (
	"context"
	"time"

	"github.com/sergeii/swat4master/internal/core/probes"
	"github.com/sergeii/swat4master/internal/entity/addr"
	"github.com/sergeii/swat4master/internal/services/probe"
)

type Service struct {
	queue *probe.Service
}

type Option func(s *Service)

func NewService(queue *probe.Service) *Service {
	service := &Service{
		queue: queue,
	}
	return service
}

func (s *Service) DiscoverDetails(
	ctx context.Context,
	addr addr.Addr,
	queryPort int,
	deadline time.Time,
) error {
	target := probes.New(addr, queryPort, probes.GoalDetails)
	if err := s.queue.AddBefore(ctx, target, deadline); err != nil {
		return err
	}
	return nil
}

func (s *Service) DiscoverPort(
	ctx context.Context,
	addr addr.Addr,
	countdown time.Time,
	deadline time.Time,
) error {
	target := probes.New(addr, addr.Port, probes.GoalPort)
	if err := s.queue.AddBetween(ctx, target, countdown, deadline); err != nil {
		return err
	}
	return nil
}
