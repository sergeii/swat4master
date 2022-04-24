package prober

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/internal/core/probes"
	"github.com/sergeii/swat4master/internal/services/discovery/probing"
	"github.com/sergeii/swat4master/internal/services/monitoring"
)

type WorkerGroup struct {
	concurrency int
	busy        int64
	prober      *probing.Service
	metrics     *monitoring.MetricService
}

func NewWorkerGroup(
	concurrency int,
	prober *probing.Service,
	metrics *monitoring.MetricService,
) *WorkerGroup {
	return &WorkerGroup{
		concurrency: concurrency,
		prober:      prober,
		metrics:     metrics,
	}
}

func (wp *WorkerGroup) Start(ctx context.Context) chan probes.Target {
	wp.metrics.DiscoveryWorkersAvailable.Add(float64(wp.concurrency))
	queue := make(chan probes.Target, wp.concurrency)
	for i := 0; i < wp.concurrency; i++ {
		go wp.work(ctx, i, queue)
	}
	return queue
}

func (wp *WorkerGroup) work(
	ctx context.Context,
	worker int,
	queue chan probes.Target,
) {
	for {
		select {
		case <-ctx.Done():
			log.Debug().Int("worker", worker).Msg("Stopping worker")
			return
		case target := <-queue:
			wp.probe(ctx, worker, target)
		}
	}
}

func (wp *WorkerGroup) probe(ctx context.Context, worker int, target probes.Target) {
	atomic.AddInt64(&wp.busy, 1)
	wp.metrics.DiscoveryWorkersBusy.Inc()
	wp.metrics.DiscoveryWorkersAvailable.Dec()
	defer func() {
		atomic.AddInt64(&wp.busy, -1)
		wp.metrics.DiscoveryWorkersBusy.Dec()
		wp.metrics.DiscoveryWorkersAvailable.Inc()
	}()
	goal := target.GetGoal()
	goalLabel := goal.String()

	log.Debug().
		Stringer("addr", target.GetAddr()).Stringer("goal", goal).
		Int("worker", worker).Int("busyness", int(wp.busy)).Int("retries", target.GetRetries()).
		Msg("About to start probing")

	before := time.Now()

	if err := wp.prober.Probe(ctx, target); err != nil {
		if errors.Is(err, probing.ErrProbeRetried) { // nolint: gocritic
			wp.metrics.DiscoveryProbeRetries.WithLabelValues(goalLabel).Inc()
			log.Debug().
				Stringer("addr", target.GetAddr()).Stringer("goal", goal).
				Int("worker", worker).
				Msg("Probe is retried")
		} else if errors.Is(err, probing.ErrOutOfRetries) {
			wp.metrics.DiscoveryProbeFailures.WithLabelValues(goalLabel).Inc()
			log.Debug().
				Stringer("addr", target.GetAddr()).Stringer("goal", goal).
				Int("worker", worker).
				Msg("Probe failed after retries")
		} else {
			wp.metrics.DiscoveryProbeErrors.WithLabelValues(goalLabel).Inc()
			log.Error().
				Err(err).
				Stringer("addr", target.GetAddr()).Stringer("goal", goal).
				Int("worker", worker).
				Msg("Probe failed due to error")
		}
	} else {
		wp.metrics.DiscoveryProbeSuccess.WithLabelValues(goalLabel).Inc()
	}

	log.Debug().
		Stringer("addr", target.GetAddr()).Stringer("goal", goal).
		Int("worker", worker).Int("busyness", int(wp.busy)).
		Dur("elapsed", time.Since(before)).
		Msg("Finished probing")

	wp.metrics.DiscoveryProbes.WithLabelValues(goalLabel).Inc()
	wp.metrics.DiscoveryProbeDurations.WithLabelValues(goalLabel).Observe(time.Since(before).Seconds())
}

func (wp *WorkerGroup) Busy() int {
	return int(wp.busy)
}

func (wp *WorkerGroup) Available() int {
	return wp.concurrency - int(wp.busy)
}
