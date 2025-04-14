package serverobserver

import (
	"context"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"

	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/filterset"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/metrics"
)

type Opts struct {
	ServerLiveness time.Duration
}

type ServerObserver struct {
	opts       Opts
	serverRepo repositories.ServerRepository
	clock      clockwork.Clock
	logger     *zerolog.Logger
}

func New(
	collector *metrics.Collector,
	serverRepo repositories.ServerRepository,
	clock clockwork.Clock,
	logger *zerolog.Logger,
	opts Opts,
) ServerObserver {
	observer := ServerObserver{
		serverRepo: serverRepo,
		clock:      clock,
		logger:     logger,
		opts:       opts,
	}
	collector.AddObserver(&observer)
	return observer
}

func (o ServerObserver) Observe(ctx context.Context, m *metrics.Collector) {
	o.observeServerRepoSize(ctx, m)
	o.observeDiscoveredServers(ctx, m)
	o.observeActiveServers(ctx, m)
}

func (o ServerObserver) observeServerRepoSize(ctx context.Context, m *metrics.Collector) {
	count, err := o.serverRepo.Count(ctx)
	if err != nil {
		o.logger.Error().Err(err).Msg("Unable to observe server count")
		return
	}
	m.ServerRepositorySize.Set(float64(count))
	o.logger.Debug().Int("count", count).Msg("Observed server count")
}

func (o ServerObserver) observeDiscoveredServers(ctx context.Context, m *metrics.Collector) {
	countByStatus, err := o.serverRepo.CountByStatus(ctx)
	if err != nil {
		o.logger.Error().Err(err).Msg("Unable to observe discovered server count")
		return
	}
	for status, serverCount := range countByStatus {
		if status == ds.NoStatus || serverCount == 0 {
			continue
		}
		m.GameDiscoveredServers.WithLabelValues(status.BitString()).Set(float64(serverCount))
		o.logger.Debug().Int("count", serverCount).Str("status", status.String()).Msg("Observed server count by status")
	}
}

func (o ServerObserver) observeActiveServers(ctx context.Context, m *metrics.Collector) {
	players := make(map[string]int)
	allServers := make(map[string]int)
	playedServers := make(map[string]int)

	activeSince := o.clock.Now().Add(-o.opts.ServerLiveness)
	fs := filterset.NewServerFilterSet().ActiveAfter(activeSince).WithStatus(ds.Info)
	activeServers, err := o.serverRepo.Filter(ctx, fs)
	if err != nil {
		o.logger.Error().Err(err).Msg("Unable to observe active server count")
		return
	}

	for _, s := range activeServers {
		allServers[s.Info.GameType]++
		if s.Info.NumPlayers > 0 {
			players[s.Info.GameType] += s.Info.NumPlayers
			playedServers[s.Info.GameType]++
		}
	}

	for gametype, playerCount := range players {
		m.GamePlayers.WithLabelValues(gametype).Set(float64(playerCount))
	}
	for gametype, serverCount := range allServers {
		m.GameActiveServers.WithLabelValues(gametype).Set(float64(serverCount))
	}
	for gametype, serverCount := range playedServers {
		m.GamePlayedServers.WithLabelValues(gametype).Set(float64(serverCount))
	}

	o.logger.Debug().Int("players", len(players)).Int("servers", len(activeServers)).Msg("Observed active server count")
}
