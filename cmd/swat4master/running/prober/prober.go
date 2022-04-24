package prober

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/running"
	"github.com/sergeii/swat4master/internal/application"
	"github.com/sergeii/swat4master/internal/core/probes"
	"github.com/sergeii/swat4master/internal/services/discovery/probing"
	"github.com/sergeii/swat4master/internal/services/probe"
)

func Run(ctx context.Context, runner *running.Runner, app *application.App, cfg config.Config) {
	defer runner.Quit(ctx.Done())

	ticker := time.NewTicker(cfg.ProbePollSchedule)
	defer ticker.Stop()

	prober := probing.NewService(
		app.Servers,
		app.ProbeService,
		app.MetricService,
		probing.WithProbeTimeout(cfg.ProbeTimeout),
		probing.WithMaxRetries(cfg.ProbeRetries),
		probing.WithPortSuggestions(cfg.DiscoveryRevivalPorts),
	)

	wg := NewWorkerGroup(cfg.ProbeConcurrency, prober, app.MetricService)
	pool := wg.Start(ctx)

	log.Info().
		Dur("interval", cfg.ProbePollSchedule).
		Dur("timeout", cfg.ProbeTimeout).
		Int("concurrency", cfg.ProbeConcurrency).
		Msg("Starting discovery prober")

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Stopping discovery prober")
			return
		case <-ticker.C:
			feed(ctx, wg, pool, app.ProbeService)
		}
	}
}

func feed(
	ctx context.Context,
	wg *WorkerGroup,
	pool chan probes.Target,
	queue *probe.Service,
) {
	availability := wg.Available()
	if availability <= 0 {
		log.Info().Int("availability", availability).Msg("Probe workers are busy")
		return
	}

	targets, err := queue.PopMany(ctx, availability)
	if err != nil {
		log.Warn().
			Err(err).Int("availability", availability).
			Msg("Got error obtaining new discovery targets")
		return
	}

	if len(targets) == 0 {
		return
	}

	log.Debug().
		Int("availability", availability).Int("targets", len(targets)).
		Msg("Obtained discovery targets")

	for _, target := range targets {
		pool <- target
	}

	log.Debug().
		Int("availability", availability).Int("targets", len(targets)).
		Msg("Sent discovery targets to work pool")
}
