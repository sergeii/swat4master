package cleaner

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/running"
	"github.com/sergeii/swat4master/internal/application"
	"github.com/sergeii/swat4master/internal/core/instances"
	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/services/monitoring"
)

func Run(ctx context.Context, runner *running.Runner, app *application.App, cfg config.Config) {
	defer runner.Quit(ctx.Done())

	ticker := time.NewTicker(cfg.CleanInterval)
	defer ticker.Stop()

	log.Info().
		Dur("interval", cfg.CleanInterval).Dur("retention", cfg.CleanRetention).
		Msg("Starting cleaner")

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Stopping cleaner")
			return
		case <-ticker.C:
			until := time.Now().Add(-cfg.CleanRetention)
			if err := clean(ctx, app.Servers, app.Instances, app.MetricService, until); err != nil {
				log.Error().
					Err(err).
					Msg("Failed to clean outdated servers")
			}
		}
	}
}

func clean(
	ctx context.Context,
	servers servers.Repository,
	instances instances.Repository,
	metrics *monitoring.MetricService,
	until time.Time,
) error {
	var before, after, removed, errors int
	var err error

	if before, err = servers.Count(ctx); err != nil {
		return err
	}

	log.Info().
		Stringer("until", until).Int("servers", before).
		Msg("Starting to clean outdated servers")

	for {
		svr, ok := servers.CleanNext(ctx, until)
		if !ok {
			log.Info().Stringer("until", until).Msg("No more outdated servers")
			break
		}
		if err = instances.RemoveByAddr(ctx, svr.GetAddr()); err != nil {
			log.Error().
				Err(err).
				Stringer("until", until).Stringer("addr", svr.GetAddr()).
				Msg("Failed to remove instance for removed server")
			errors++
			continue
		}
		if err = servers.Remove(ctx, svr); err != nil {
			log.Error().
				Err(err).
				Stringer("until", until).Stringer("addr", svr.GetAddr()).
				Msg("Failed to remove outdated server")
			errors++
			continue
		}
		removed++
	}

	metrics.CleanerRemovals.Add(float64(removed))
	metrics.CleanerErrors.Add(float64(errors))

	if after, err = servers.Count(ctx); err != nil {
		return err
	}

	log.Info().
		Stringer("until", until).
		Int("removed", removed).Int("errors", errors).
		Int("before", before).Int("after", after).
		Msg("Finished cleaning outdated servers")

	return nil
}
