package probing

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
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
	MaxRetries   int
	ProbeTimeout time.Duration
}

type Service struct {
	servers repositories.ServerRepository
	queue   *ps.Service
	opts    ServiceOpts
	probers map[probe.Goal]Prober
	clock   clock.Clock
	logger  *zerolog.Logger
	mutex   sync.Mutex
}

func NewService(
	servers repositories.ServerRepository,
	queue *ps.Service,
	clock clock.Clock,
	logger *zerolog.Logger,
	opts ServiceOpts,
) *Service {
	service := &Service{
		servers: servers,
		queue:   queue,
		probers: make(map[probe.Goal]Prober),
		clock:   clock,
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
	goal := prb.GetGoal()
	addr := prb.GetAddr()
	queryPort := prb.GetPort()

	prober, err := s.selectProber(goal)
	if err != nil {
		return err
	}

	svr, err := s.servers.Get(ctx, addr)
	if err != nil {
		s.logger.Error().
			Err(err).
			Stringer("addr", addr).Stringer("goal", goal).Int("port", queryPort).
			Msg("Failed to obtain server for probing")
		return err
	}

	result, probeErr := prober.Probe(ctx, svr, queryPort, s.opts.ProbeTimeout)

	if probeErr != nil {
		s.logger.Warn().
			Err(probeErr).
			Stringer("addr", addr).Stringer("goal", goal).Int("port", queryPort).
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
			Stringer("addr", addr).Int("port", queryPort).Stringer("goal", goal).
			Msg("Unable to update probed server")
		return updateErr
	}

	s.logger.Debug().
		Stringer("server", svr).Int("port", queryPort).
		Stringer("goal", goal).Int("retries", prb.GetRetries()).
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
	tgt probe.Probe,
	svr server.Server,
) error {
	goal := tgt.GetGoal()
	addr := tgt.GetAddr()

	retries, ok := tgt.IncRetries(s.opts.MaxRetries)
	if !ok {
		s.logger.Info().
			Stringer("server", svr).
			Stringer("goal", goal).Int("retries", retries).Int("max", s.opts.MaxRetries).
			Msg("Max retries reached")
		if failErr := s.fail(ctx, prober, tgt, svr); failErr != nil {
			return failErr
		}
		return ErrOutOfRetries
	}

	retryDelay := time.Second * time.Duration(math.Exp(float64(retries)))
	retryAfter := s.clock.Now().Add(retryDelay)
	if err := s.queue.AddAfter(ctx, tgt, retryAfter); err != nil {
		s.logger.Error().
			Err(err).
			Stringer("addr", addr).Int("port", tgt.GetPort()).
			Stringer("goal", goal).Int("retries", retries).Dur("delay", retryDelay).
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
			Stringer("server", svr).Int("port", tgt.GetPort()).Stringer("goal", tgt.GetGoal()).
			Msg("Unable to update retried server")
		return updateErr
	}

	s.logger.Info().
		Stringer("addr", addr).Int("port", tgt.GetPort()).
		Stringer("goal", goal).Int("retries", retries).Dur("delay", retryDelay).
		Msg("Added retry for failed probe")

	return ErrProbeRetried
}

func (s *Service) fail(
	ctx context.Context,
	prober Prober,
	tgt probe.Probe,
	svr server.Server,
) error {
	svr = prober.HandleFailure(svr)

	if _, updateErr := s.servers.Update(ctx, svr, func(s *server.Server) bool {
		*s = prober.HandleFailure(*s)
		return true
	}); updateErr != nil {
		s.logger.Error().
			Err(updateErr).
			Stringer("server", svr).Int("port", tgt.GetPort()).Stringer("goal", tgt.GetGoal()).
			Msg("Unable to update failed server")
		return updateErr
	}

	return nil
}
