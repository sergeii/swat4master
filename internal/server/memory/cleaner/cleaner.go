package cleaner

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

type Cleanable interface {
	Clean(duration time.Duration)
}

type Worker struct {
	repo      Cleanable
	interval  time.Duration
	retention time.Duration
}

func New(repo Cleanable, interval time.Duration, retention time.Duration) *Worker {
	return &Worker{repo, interval, retention}
}

func (w *Worker) Run(ctx context.Context) {
	go func() {
		log.Info().
			Dur("interval", w.interval).Dur("retention", w.retention).
			Msg("Starting background memory cleaner")
		ticker := time.NewTicker(w.interval)
		for {
			select {
			case <-ctx.Done():
				log.Info().Msg("Stopping memory cleaner")
				ticker.Stop()
				return
			case <-ticker.C:
				log.Info().Dur("retention", w.retention).Msg("Launching memory sweep")
				w.repo.Clean(w.retention)
			}
		}
	}()
}
