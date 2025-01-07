package proberunner

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/core/usecases/probeserver"
	"github.com/sergeii/swat4master/internal/metrics"
	"github.com/sergeii/swat4master/internal/prober/probers"
)

var ErrUnsupportedGoal = errors.New("no associated prober for goal")

type RunnerOpts struct {
	PollInterval time.Duration
	Concurrency  int
	ProbeTimeout time.Duration
}

type Runner struct {
	opts      RunnerOpts
	uc        probeserver.UseCase
	probeRepo repositories.ProbeRepository
	probers   probers.ForGoal
	metrics   *metrics.Collector
	logger    *zerolog.Logger
	queue     chan probe.Probe
	busy      int64
}

func New(
	opts RunnerOpts,
	probeRepo repositories.ProbeRepository,
	uc probeserver.UseCase,
	probers probers.ForGoal,
	metrics *metrics.Collector,
	logger *zerolog.Logger,
) *Runner {
	return &Runner{
		opts:      opts,
		probeRepo: probeRepo,
		uc:        uc,
		probers:   probers,
		metrics:   metrics,
		logger:    logger,
		queue:     make(chan probe.Probe, opts.Concurrency),
	}
}

func (r *Runner) Start(ctx context.Context) {
	r.logger.Info().
		Dur("interval", r.opts.PollInterval).
		Dur("timeout", r.opts.ProbeTimeout).
		Int("concurrency", r.opts.Concurrency).
		Msg("Starting prober")

	for range r.opts.Concurrency {
		go r.worker(ctx)
	}
	r.metrics.DiscoveryWorkersAvailable.Add(float64(r.opts.Concurrency))

	go r.scheduler(ctx)
}

func (r *Runner) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			r.logger.Debug().Msg("Stopping worker")
			return
		case prb := <-r.queue:
			r.probe(ctx, prb)
		}
	}
}

func (r *Runner) scheduler(ctx context.Context) {
	ticker := time.NewTicker(r.opts.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Debug().Msg("Stopping scheduler")
			return
		case <-ticker.C:
			r.schedule(ctx)
		}
	}
}

func (r *Runner) probe(ctx context.Context, prb probe.Probe) {
	atomic.AddInt64(&r.busy, 1)
	r.metrics.DiscoveryWorkersBusy.Inc()
	r.metrics.DiscoveryWorkersAvailable.Dec()
	defer func() {
		atomic.AddInt64(&r.busy, -1)
		r.metrics.DiscoveryWorkersBusy.Dec()
		r.metrics.DiscoveryWorkersAvailable.Inc()
	}()

	goalLabel := prb.Goal.String()
	before := time.Now()

	r.logger.Debug().
		Stringer("addr", prb.Addr).Stringer("goal", prb.Goal).
		Int64("busyness", atomic.LoadInt64(&r.busy)).Int("retries", prb.Retries).
		Msg("About to start probing")

	prober, err := r.selectProber(prb.Goal)
	if err != nil {
		r.logger.Error().Err(err).Stringer("goal", prb.Goal).Msg("Unable to select prober")
		return
	}
	ucReq := probeserver.NewRequest(prb, prober, r.opts.ProbeTimeout)

	if err := r.uc.Execute(ctx, ucReq); err != nil {
		if errors.Is(err, probeserver.ErrProbeRetried) { // nolint: gocritic
			r.metrics.DiscoveryProbeRetries.WithLabelValues(goalLabel).Inc()
			r.logger.Debug().
				Stringer("addr", prb.Addr).Stringer("goal", prb.Goal).
				Msg("Probe is retried")
		} else if errors.Is(err, probeserver.ErrOutOfRetries) {
			r.metrics.DiscoveryProbeFailures.WithLabelValues(goalLabel).Inc()
			r.logger.Debug().
				Stringer("addr", prb.Addr).Stringer("goal", prb.Goal).
				Msg("Probe failed after retries")
		} else {
			r.metrics.DiscoveryProbeErrors.WithLabelValues(goalLabel).Inc()
			r.logger.Error().
				Err(err).
				Stringer("addr", prb.Addr).Stringer("goal", prb.Goal).
				Msg("Probe failed due to error")
		}
	} else {
		r.metrics.DiscoveryProbeSuccess.WithLabelValues(goalLabel).Inc()
	}

	r.logger.Debug().
		Stringer("addr", prb.Addr).Stringer("goal", prb.Goal).
		Int64("busyness", atomic.LoadInt64(&r.busy)).
		Dur("elapsed", time.Since(before)).
		Msg("Finished probing")

	r.metrics.DiscoveryProbes.WithLabelValues(goalLabel).Inc()
	r.metrics.DiscoveryProbeDurations.WithLabelValues(goalLabel).Observe(time.Since(before).Seconds())
}

func (r *Runner) selectProber(goal probe.Goal) (probers.Prober, error) {
	if selected, ok := r.probers[goal]; ok {
		return selected, nil
	}
	return nil, fmt.Errorf("%w: no associated prober for goal '%s'", ErrUnsupportedGoal, goal)
}

func (r *Runner) schedule(ctx context.Context) {
	availability := r.Available()
	if availability <= 0 {
		r.logger.Info().Int("availability", availability).Msg("Workers are busy")
		return
	}

	probes, expired, err := r.probeRepo.PopMany(ctx, availability)
	if err != nil {
		r.metrics.DiscoveryQueueErrors.Inc()
		r.logger.Warn().
			Err(err).
			Int("availability", availability).
			Msg("Unable to fetch new probes")
		return
	}
	r.metrics.DiscoveryQueueConsumed.Add(float64(len(probes)))
	// measure the number of expired probes
	if expired > 0 {
		r.metrics.DiscoveryQueueExpired.Add(float64(expired))
	}

	if len(probes) == 0 {
		return
	}

	r.logger.Debug().Int("availability", availability).Int("probes", len(probes)).Msg("Obtained probes")

	for _, prb := range probes {
		r.queue <- prb
	}

	r.logger.Debug().Int("availability", availability).Int("probes", len(probes)).Msg("Sent probes to queue")
}

func (r *Runner) Busy() int {
	return int(atomic.LoadInt64(&r.busy))
}

func (r *Runner) Available() int {
	return r.opts.Concurrency - int(atomic.LoadInt64(&r.busy))
}
