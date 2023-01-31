package probing

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/internal/core/probes"
	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/services/discovery/probing/probers"
	"github.com/sergeii/swat4master/internal/services/monitoring"
	"github.com/sergeii/swat4master/internal/services/probe"
)

var ErrUnknownGoalType = errors.New("unknown goal type")
var ErrProbeRetried = errors.New("probe is retried")
var ErrOutOfRetries = errors.New("retry limit reached")

type Probers struct {
	Details *probers.DetailsProber
	Port    *probers.PortProber
}

type Service struct {
	servers servers.Repository

	queue *probe.Service

	maxRetries       int
	probeTimeout     time.Duration
	probePortOffsets []int

	probers Probers
}

type Option func(s *Service)

func NewService(
	servers servers.Repository,
	queue *probe.Service,
	metrics *monitoring.MetricService,
	opts ...Option,
) *Service {
	service := &Service{
		servers: servers,
		queue:   queue,
	}
	for _, opt := range opts {
		opt(service)
	}
	service.probers = Probers{
		Details: probers.NewDetailsProber(metrics),
		Port:    probers.NewPortProber(metrics, probers.WithPortOffsets(service.probePortOffsets)),
	}
	return service
}

func WithMaxRetries(retries int) Option {
	return func(s *Service) {
		s.maxRetries = retries
	}
}

func WithPortSuggestions(offsets []int) Option {
	return func(s *Service) {
		s.probePortOffsets = offsets
	}
}

func WithProbeTimeout(timeout time.Duration) Option {
	return func(s *Service) {
		s.probeTimeout = timeout
	}
}

func (s *Service) Probe(ctx context.Context, target probes.Target) error {
	goal := target.GetGoal()
	addr := target.GetAddr()
	queryPort := target.GetPort()

	prober, err := s.chooseProber(goal)
	if err != nil {
		return err
	}

	svr, err := s.servers.Get(ctx, addr)
	if err != nil {
		log.Error().
			Err(err).
			Stringer("addr", addr).Stringer("goal", goal).Int("port", queryPort).
			Msg("Failed to obtain server for probing")
		return err
	}

	result, probeErr := prober.Probe(ctx, svr, queryPort, s.probeTimeout)

	if probeErr != nil {
		log.Warn().
			Err(probeErr).
			Stringer("addr", addr).Stringer("goal", goal).Int("port", queryPort).
			Msg("Probe failed")
		return s.retry(ctx, prober, target, svr)
	}

	svr = prober.HandleSuccess(result, svr)

	if _, updateErr := s.servers.Update(ctx, svr, func(s *servers.Server) bool {
		*s = prober.HandleSuccess(result, *s)
		return true
	}); updateErr != nil {
		log.Error().
			Err(updateErr).
			Stringer("addr", addr).Int("port", queryPort).Stringer("goal", goal).
			Msg("Unable to update probed server")
		return updateErr
	}

	log.Debug().
		Stringer("server", svr).Int("port", queryPort).
		Stringer("goal", goal).Int("retries", target.GetRetries()).
		Msg("Successfully probed server")

	return nil
}

func (s *Service) chooseProber(goal probes.Goal) (probers.Prober, error) {
	switch goal {
	case probes.GoalDetails:
		return s.probers.Details, nil
	case probes.GoalPort:
		return s.probers.Port, nil
	default:
		log.Error().Stringer("goal", goal).Msg("Unknown goal type")
		return nil, ErrUnknownGoalType
	}
}

func (s *Service) retry(
	ctx context.Context,
	prober probers.Prober,
	tgt probes.Target,
	svr servers.Server,
) error {
	goal := tgt.GetGoal()
	addr := tgt.GetAddr()

	retries, ok := tgt.IncRetries(s.maxRetries)
	if !ok {
		log.Info().
			Stringer("server", svr).
			Stringer("goal", goal).Int("retries", retries).Int("max", s.maxRetries).
			Msg("Max retries reached")
		if failErr := s.fail(ctx, prober, tgt, svr); failErr != nil {
			return failErr
		}
		return ErrOutOfRetries
	}

	retryDelay := time.Second * time.Duration(math.Exp(float64(retries)))
	retryAfter := time.Now().Add(retryDelay)
	if err := s.queue.AddAfter(ctx, tgt, retryAfter); err != nil {
		log.Error().
			Err(err).
			Stringer("addr", addr).Int("port", tgt.GetPort()).
			Stringer("goal", goal).Int("retries", retries).Dur("delay", retryDelay).
			Msg("Failed to add retry for failed probe")
		return err
	}

	svr = prober.HandleRetry(svr)

	if _, updateErr := s.servers.Update(ctx, svr, func(s *servers.Server) bool {
		*s = prober.HandleFailure(*s)
		return true
	}); updateErr != nil {
		log.Error().
			Err(updateErr).
			Stringer("server", svr).Int("port", tgt.GetPort()).Stringer("goal", tgt.GetGoal()).
			Msg("Unable to update retried server")
		return updateErr
	}

	log.Info().
		Stringer("addr", addr).Int("port", tgt.GetPort()).
		Stringer("goal", goal).Int("retries", retries).Dur("delay", retryDelay).
		Msg("Added retry for failed probe")

	return ErrProbeRetried
}

func (s *Service) fail(
	ctx context.Context,
	prober probers.Prober,
	tgt probes.Target,
	svr servers.Server,
) error {
	svr = prober.HandleFailure(svr)

	if _, updateErr := s.servers.Update(ctx, svr, func(s *servers.Server) bool {
		*s = prober.HandleFailure(*s)
		return true
	}); updateErr != nil {
		log.Error().
			Err(updateErr).
			Stringer("server", svr).Int("port", tgt.GetPort()).Stringer("goal", tgt.GetGoal()).
			Msg("Unable to update failed server")
		return updateErr
	}

	return nil
}
