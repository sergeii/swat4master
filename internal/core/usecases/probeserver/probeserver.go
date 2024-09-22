package probeserver

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"

	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/metrics"
	"github.com/sergeii/swat4master/internal/prober/probers"
)

var (
	ErrProbeRetried = errors.New("probe is retried")
	ErrOutOfRetries = errors.New("retry limit reached")
)

type Request struct {
	Probe        probe.Probe
	Prober       probers.Prober
	ProbeTimeout time.Duration
}

func NewRequest(prb probe.Probe, prober probers.Prober, probeTimeout time.Duration) Request {
	return Request{
		Probe:        prb,
		Prober:       prober,
		ProbeTimeout: probeTimeout,
	}
}

type UseCase struct {
	serverRepo repositories.ServerRepository
	probeRepo  repositories.ProbeRepository
	metrics    *metrics.Collector
	clock      clockwork.Clock
	logger     *zerolog.Logger
}

func New(
	serverRepo repositories.ServerRepository,
	probeRepo repositories.ProbeRepository,
	metrics *metrics.Collector,
	clock clockwork.Clock,
	logger *zerolog.Logger,
) UseCase {
	return UseCase{
		serverRepo: serverRepo,
		probeRepo:  probeRepo,
		metrics:    metrics,
		clock:      clock,
		logger:     logger,
	}
}

func (uc UseCase) Execute(ctx context.Context, req Request) error {
	svr, err := uc.serverRepo.Get(ctx, req.Probe.Addr)
	if err != nil {
		uc.logger.Error().
			Err(err).
			Stringer("addr", req.Probe.Addr).Stringer("goal", req.Probe.Goal).Int("port", req.Probe.Port).
			Msg("Failed to obtain server for probing")
		return err
	}

	result, probeErr := req.Prober.Probe(ctx, svr.Addr, req.Probe.Port, req.ProbeTimeout)

	if probeErr != nil {
		uc.logger.Warn().
			Err(probeErr).
			Stringer("addr", req.Probe.Addr).Stringer("goal", req.Probe.Goal).Int("port", req.Probe.Port).
			Msg("Probe failed")
		return uc.retry(ctx, req.Prober, req.Probe, svr)
	}

	svr = req.Prober.HandleSuccess(result, svr)

	if _, updateErr := uc.serverRepo.Update(ctx, svr, func(s *server.Server) bool {
		*s = req.Prober.HandleSuccess(result, *s)
		return true
	}); updateErr != nil {
		uc.logger.Error().
			Err(updateErr).
			Stringer("addr", req.Probe.Addr).Int("port", req.Probe.Port).Stringer("goal", req.Probe.Goal).
			Msg("Unable to update probed server")
		return updateErr
	}

	uc.logger.Debug().
		Stringer("server", svr).Int("port", req.Probe.Port).
		Stringer("goal", req.Probe.Goal).Int("retries", req.Probe.Retries).
		Msg("Successfully probed server")

	return nil
}

func (uc UseCase) addToQueue(ctx context.Context, prb probe.Probe, after time.Time) error {
	err := uc.probeRepo.AddBetween(ctx, prb, after, repositories.NC)
	if err != nil {
		return err
	}
	uc.metrics.DiscoveryQueueProduced.Inc()
	return nil
}

func (uc UseCase) retry(
	ctx context.Context,
	prober probers.Prober,
	prb probe.Probe,
	svr server.Server,
) error {
	retries, ok := prb.IncRetries()
	if !ok {
		uc.logger.Info().
			Stringer("server", svr).
			Stringer("goal", prb.Goal).Int("retries", retries).Int("max", prb.MaxRetries).
			Msg("Max retries reached")
		if failErr := uc.fail(ctx, prober, prb, svr); failErr != nil {
			return failErr
		}
		return ErrOutOfRetries
	}

	retryDelay := time.Second * time.Duration(math.Exp(float64(retries)))
	retryAfter := uc.clock.Now().Add(retryDelay)
	if err := uc.addToQueue(ctx, prb, retryAfter); err != nil {
		uc.logger.Error().
			Err(err).
			Stringer("addr", prb.Addr).Int("port", prb.Port).
			Stringer("goal", prb.Goal).Int("retries", retries).Dur("delay", retryDelay).
			Msg("Failed to add retry for failed probe")
		return err
	}

	svr = prober.HandleRetry(svr)

	if _, updateErr := uc.serverRepo.Update(ctx, svr, func(s *server.Server) bool {
		*s = prober.HandleFailure(*s)
		return true
	}); updateErr != nil {
		uc.logger.Error().
			Err(updateErr).
			Stringer("server", svr).Int("port", prb.Port).Stringer("goal", prb.Goal).
			Msg("Unable to update retried server")
		return updateErr
	}

	uc.logger.Info().
		Stringer("addr", prb.Addr).Int("port", prb.Port).
		Stringer("goal", prb.Goal).Int("retries", retries).Dur("delay", retryDelay).
		Msg("Added retry for failed probe")

	return ErrProbeRetried
}

func (uc UseCase) fail(
	ctx context.Context,
	prober probers.Prober,
	prb probe.Probe,
	svr server.Server,
) error {
	svr = prober.HandleFailure(svr)

	if _, updateErr := uc.serverRepo.Update(ctx, svr, func(s *server.Server) bool {
		*s = prober.HandleFailure(*s)
		return true
	}); updateErr != nil {
		uc.logger.Error().
			Err(updateErr).
			Stringer("server", svr).Int("port", prb.Port).Stringer("goal", prb.Goal).
			Msg("Unable to update failed server")
		return updateErr
	}

	return nil
}
