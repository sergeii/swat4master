package probing

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	ps "github.com/sergeii/swat4master/internal/services/probe"
)

var (
	ErrProbeRetried = errors.New("probe is retried")
	ErrOutOfRetries = errors.New("retry limit reached")
)

type ServiceOpts struct {
	ProbeTimeout time.Duration
}

type Service struct {
	servers repositories.ServerRepository
	queue   *ps.Service
	opts    ServiceOpts
	probers map[probe.Goal]Prober
	logger  *zerolog.Logger
	mutex   sync.Mutex
}

func NewService(
	servers repositories.ServerRepository,
	queue *ps.Service,
	logger *zerolog.Logger,
	opts ServiceOpts,
) *Service {
	service := &Service{
		servers: servers,
		queue:   queue,
		probers: make(map[probe.Goal]Prober),
		logger:  logger,
		opts:    opts,
	}
	return service
}

func (s *Service) Register(pg probe.Goal, prober Prober) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if another, exists := s.probers[pg]; exists {
		return fmt.Errorf("prober '%v' has already been registered for goal '%s'", another, pg)
	}
	s.probers[pg] = prober
	return nil
}

func (s *Service) Probe(ctx context.Context, prb probe.Probe) error {
	prober, err := s.selectProber(prb.Goal)
	if err != nil {
		return err
	}

	svr, err := s.servers.Get(ctx, prb.Addr)
	if err != nil {
		s.logger.Error().
			Err(err).
			Stringer("addr", prb.Addr).Stringer("goal", prb.Goal).Int("port", prb.Port).
			Msg("Failed to obtain server for probing")
		return err
	}

	result, probeErr := prober.Probe(ctx, svr, prb.Port, s.opts.ProbeTimeout)

	if probeErr != nil {
		s.logger.Warn().
			Err(probeErr).
			Stringer("addr", prb.Addr).Stringer("goal", prb.Goal).Int("port", prb.Port).
			Msg("Probe failed")
		return s.retry(ctx, prober, prb, svr)
	}

	svr = prober.HandleSuccess(result, svr)

	if _, updateErr := s.servers.Update(ctx, svr, func(s *server.Server) bool {
		*s = prober.HandleSuccess(result, *s)
		return true
	}); updateErr != nil {
		s.logger.Error().
			Err(updateErr).
			Stringer("addr", prb.Addr).Int("port", prb.Port).Stringer("goal", prb.Goal).
			Msg("Unable to update probed server")
		return updateErr
	}

	s.logger.Debug().
		Stringer("server", svr).Int("port", prb.Port).
		Stringer("goal", prb.Goal).Int("retries", prb.Retries).
		Msg("Successfully probed server")

	return nil
}

func (s *Service) selectProber(goal probe.Goal) (Prober, error) {
	if prober, ok := s.probers[goal]; ok {
		return prober, nil
	}
	return nil, fmt.Errorf("no associated prober for goal '%s'", goal)
}

func (s *Service) retry(
	ctx context.Context,
	prober Prober,
	prb probe.Probe,
	svr server.Server,
) error {
	retries, ok := prb.IncRetries()
	if !ok {
		s.logger.Info().
			Stringer("server", svr).
			Stringer("goal", prb.Goal).Int("retries", retries).Int("max", prb.MaxRetries).
			Msg("Max retries reached")
		if failErr := s.fail(ctx, prober, prb, svr); failErr != nil {
			return failErr
		}
		return ErrOutOfRetries
	}

	retryDelay := time.Second * time.Duration(math.Exp(float64(retries)))
	retryAfter := time.Now().Add(retryDelay)
	if err := s.queue.AddAfter(ctx, prb, retryAfter); err != nil {
		s.logger.Error().
			Err(err).
			Stringer("addr", prb.Addr).Int("port", prb.Port).
			Stringer("goal", prb.Goal).Int("retries", retries).Dur("delay", retryDelay).
			Msg("Failed to add retry for failed probe")
		return err
	}

	svr = prober.HandleRetry(svr)

	if _, updateErr := s.servers.Update(ctx, svr, func(s *server.Server) bool {
		*s = prober.HandleFailure(*s)
		return true
	}); updateErr != nil {
		s.logger.Error().
			Err(updateErr).
			Stringer("server", svr).Int("port", prb.Port).Stringer("goal", prb.Goal).
			Msg("Unable to update retried server")
		return updateErr
	}

	s.logger.Info().
		Stringer("addr", prb.Addr).Int("port", prb.Port).
		Stringer("goal", prb.Goal).Int("retries", retries).Dur("delay", retryDelay).
		Msg("Added retry for failed probe")

	return ErrProbeRetried
}

func (s *Service) fail(
	ctx context.Context,
	prober Prober,
	prb probe.Probe,
	svr server.Server,
) error {
	svr = prober.HandleFailure(svr)

	if _, updateErr := s.servers.Update(ctx, svr, func(s *server.Server) bool {
		*s = prober.HandleFailure(*s)
		return true
	}); updateErr != nil {
		s.logger.Error().
			Err(updateErr).
			Stringer("server", svr).Int("port", prb.Port).Stringer("goal", prb.Goal).
			Msg("Unable to update failed server")
		return updateErr
	}

	return nil
}
