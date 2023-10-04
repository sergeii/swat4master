package finding

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/repositories"
	ps "github.com/sergeii/swat4master/internal/services/probe"
	"github.com/sergeii/swat4master/pkg/random"
)

type Service struct {
	servers repositories.ServerRepository
	queue   *ps.Service
	logger  *zerolog.Logger
}

func NewService(
	servers repositories.ServerRepository,
	queue *ps.Service,
	logger *zerolog.Logger,
) *Service {
	service := &Service{
		servers: servers,
		queue:   queue,
		logger:  logger,
	}
	return service
}

func (s *Service) RefreshDetails(
	ctx context.Context,
	deadline time.Time,
) (int, error) {
	fs := repositories.NewServerFilterSet().WithStatus(ds.Port).NoStatus(ds.DetailsRetry)
	serversWithDetails, err := s.servers.Filter(ctx, fs)
	if err != nil {
		s.logger.Error().Err(err).Msg("Unable to obtain servers for details discovery")
		return -1, err
	}

	cnt := 0
	for _, svr := range serversWithDetails {
		if err := s.DiscoverDetails(ctx, svr.Addr, svr.QueryPort, deadline); err != nil {
			s.logger.Warn().
				Err(err).Stringer("server", svr).
				Msg("Failed to add server to details discovery queue")
			continue
		}
		cnt++
	}

	return cnt, nil
}

func (s *Service) ReviveServers(
	ctx context.Context,
	minScope time.Time,
	maxScope time.Time,
	minCountdown time.Time,
	maxCountdown time.Time,
	deadline time.Time,
) (int, error) {
	fs := repositories.NewServerFilterSet().ActiveAfter(minScope).ActiveBefore(maxScope).NoStatus(ds.Port | ds.PortRetry)
	serversWithoutPort, err := s.servers.Filter(ctx, fs)
	if err != nil {
		s.logger.Error().Err(err).Msg("Unable to obtain servers for port discovery")
		return -1, err
	}

	cnt := 0
	for _, svr := range serversWithoutPort {
		countdown := selectCountdown(minCountdown, maxCountdown)
		if err := s.DiscoverPort(ctx, svr.Addr, countdown, deadline); err != nil {
			s.logger.Warn().
				Err(err).
				Stringer("server", svr).Time("countdown", countdown).Time("deadline", deadline).
				Msg("Failed to add server to port discovery queue")
			continue
		}
		s.logger.Debug().
			Time("countdown", countdown).Time("deadline", deadline).Stringer("server", svr).
			Msg("Added server to port discovery queue")
		cnt++
	}

	return cnt, nil
}

func (s *Service) DiscoverDetails(
	ctx context.Context,
	addr addr.Addr,
	queryPort int,
	deadline time.Time,
) error {
	prb := probe.New(addr, queryPort, probe.GoalDetails)
	return s.queue.AddBefore(ctx, prb, deadline)
}

func (s *Service) DiscoverPort(
	ctx context.Context,
	addr addr.Addr,
	countdown time.Time,
	deadline time.Time,
) error {
	prb := probe.New(addr, addr.Port, probe.GoalPort)
	return s.queue.AddBetween(ctx, prb, countdown, deadline)
}

func selectCountdown(min, max time.Time) time.Time {
	if !max.After(min) {
		return min
	}
	spread := max.Sub(min)
	countdown := random.RandInt(0, int(spread))
	return min.Add(time.Duration(countdown))
}
