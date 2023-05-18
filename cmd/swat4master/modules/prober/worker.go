package prober

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/benbjohnson/clock"
	"github.com/rs/zerolog"

	"github.com/sergeii/swat4master/internal/core/probes"
	"github.com/sergeii/swat4master/internal/services/discovery/probing"
	"github.com/sergeii/swat4master/internal/services/monitoring"
)

type WorkerGroup struct {
	concurrency int
	busy        int64
	prober      *probing.Service
	metrics     *monitoring.MetricService
	clock       clock.Clock
	logger      *zerolog.Logger
}

func NewWorkerGroup(
	concurrency int,
	prober *probing.Service,
	metrics *monitoring.MetricService,
	clock clock.Clock,
	logger *zerolog.Logger,
) *WorkerGroup {
	return &WorkerGroup{
		concurrency: concurrency,
		prober:      prober,
		metrics:     metrics,
		clock:       clock,
		logger:      logger,
	}
}

func (wg *WorkerGroup) Run(ctx context.Context) chan probes.Target {
	wg.metrics.DiscoveryWorkersAvailable.Add(float64(wg.concurrency))
	queue := make(chan probes.Target, wg.concurrency)
	for i := 0; i < wg.concurrency; i++ {
		go wg.work(ctx, queue)
	}
	return queue
}

func (wg *WorkerGroup) work(
	ctx context.Context,
	queue chan probes.Target,
) {
	for {
		select {
		case <-ctx.Done():
			wg.logger.Debug().Msg("Stopping worker")
			return
		case target := <-queue:
			wg.probe(ctx, target)
		}
	}
}

func (wg *WorkerGroup) probe(ctx context.Context, target probes.Target) {
	atomic.AddInt64(&wg.busy, 1)
	wg.metrics.DiscoveryWorkersBusy.Inc()
	wg.metrics.DiscoveryWorkersAvailable.Dec()
	defer func() {
		atomic.AddInt64(&wg.busy, -1)
		wg.metrics.DiscoveryWorkersBusy.Dec()
		wg.metrics.DiscoveryWorkersAvailable.Inc()
	}()
	goal := target.GetGoal()
	goalLabel := goal.String()

	wg.logger.Debug().
		Stringer("addr", target.GetAddr()).Stringer("goal", goal).
		Int64("busyness", atomic.LoadInt64(&wg.busy)).Int("retries", target.GetRetries()).
		Msg("About to start probing")

	before := wg.clock.Now()

	if err := wg.prober.Probe(ctx, target); err != nil {
		if errors.Is(err, probing.ErrProbeRetried) { // nolint: gocritic
			wg.metrics.DiscoveryProbeRetries.WithLabelValues(goalLabel).Inc()
			wg.logger.Debug().
				Stringer("addr", target.GetAddr()).Stringer("goal", goal).
				Msg("Probe is retried")
		} else if errors.Is(err, probing.ErrOutOfRetries) {
			wg.metrics.DiscoveryProbeFailures.WithLabelValues(goalLabel).Inc()
			wg.logger.Debug().
				Stringer("addr", target.GetAddr()).Stringer("goal", goal).
				Msg("Probe failed after retries")
		} else {
			wg.metrics.DiscoveryProbeErrors.WithLabelValues(goalLabel).Inc()
			wg.logger.Error().
				Err(err).
				Stringer("addr", target.GetAddr()).Stringer("goal", goal).
				Msg("Probe failed due to error")
		}
	} else {
		wg.metrics.DiscoveryProbeSuccess.WithLabelValues(goalLabel).Inc()
	}

	wg.logger.Debug().
		Stringer("addr", target.GetAddr()).Stringer("goal", goal).
		Int64("busyness", atomic.LoadInt64(&wg.busy)).
		Dur("elapsed", wg.clock.Since(before)).
		Msg("Finished probing")

	wg.metrics.DiscoveryProbes.WithLabelValues(goalLabel).Inc()
	wg.metrics.DiscoveryProbeDurations.WithLabelValues(goalLabel).Observe(wg.clock.Since(before).Seconds())
}

func (wg *WorkerGroup) Busy() int {
	return int(atomic.LoadInt64(&wg.busy))
}

func (wg *WorkerGroup) Available() int {
	return wg.concurrency - int(atomic.LoadInt64(&wg.busy))
}
