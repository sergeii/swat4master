package monitoring

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/internal/core/instances"
	"github.com/sergeii/swat4master/internal/core/probes"
	"github.com/sergeii/swat4master/internal/core/servers"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
)

type MetricService struct {
	registry *prometheus.Registry

	ReporterRequests  *prometheus.CounterVec
	ReporterErrors    *prometheus.CounterVec
	ReporterReceived  prometheus.Counter
	ReporterSent      prometheus.Counter
	ReporterRemovals  prometheus.Counter
	ReporterDurations *prometheus.HistogramVec
	BrowserRequests   prometheus.Counter
	BrowserErrors     prometheus.Counter
	BrowserReceived   prometheus.Counter
	BrowserSent       prometheus.Counter
	BrowserDurations  prometheus.Histogram

	CleanerRemovals prometheus.Counter
	CleanerErrors   prometheus.Counter

	DiscoveryWorkersBusy      prometheus.Gauge
	DiscoveryWorkersAvailable prometheus.Gauge
	DiscoveryQueueProduced    prometheus.Counter
	DiscoveryQueueConsumed    prometheus.Counter
	DiscoveryQueueExpired     prometheus.Counter
	DiscoveryProbes           *prometheus.CounterVec
	DiscoveryProbeSuccess     *prometheus.CounterVec
	DiscoveryProbeRetries     *prometheus.CounterVec
	DiscoveryProbeFailures    *prometheus.CounterVec
	DiscoveryProbeErrors      *prometheus.CounterVec
	DiscoveryProbeDurations   *prometheus.HistogramVec
	DiscoveryQueryDurations   prometheus.Histogram

	ServerRepositorySize   prometheus.Gauge
	InstanceRepositorySize prometheus.Gauge
	ProbeRepositorySize    prometheus.Gauge

	GameDiscoveredServers *prometheus.GaugeVec
	GameActiveServers     *prometheus.GaugeVec
	GamePlayedServers     *prometheus.GaugeVec
	GamePlayers           *prometheus.GaugeVec
}

func NewMetricService() *MetricService {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	registry.MustRegister(collectors.NewGoCollector())

	ms := &MetricService{
		registry: registry,

		ReporterRequests: promauto.With(registry).NewCounterVec(prometheus.CounterOpts{
			Name: "reporter_requests_total",
			Help: "The total number of successful reporting requests",
		}, []string{"type"}),
		ReporterErrors: promauto.With(registry).NewCounterVec(prometheus.CounterOpts{
			Name: "reporter_errors_total",
			Help: "The total number of failed reporting requests that",
		}, []string{"type"}),
		ReporterReceived: promauto.With(registry).NewCounter(prometheus.CounterOpts{
			Name: "reporter_received_bytes_total",
			Help: "The total amount of bytes received by reporter",
		}),
		ReporterSent: promauto.With(registry).NewCounter(prometheus.CounterOpts{
			Name: "reporter_sent_bytes_total",
			Help: "The total amount of bytes sent by reporter",
		}),
		ReporterRemovals: promauto.With(registry).NewCounter(prometheus.CounterOpts{
			Name: "reporter_removals_total",
			Help: "The total number of removals requests accepted by reporter",
		}),
		ReporterDurations: promauto.With(registry).NewHistogramVec(prometheus.HistogramOpts{
			Name: "reporter_duration_seconds",
			Help: "Duration of reporting requests",
		}, []string{"type"}),
		BrowserRequests: promauto.With(registry).NewCounter(prometheus.CounterOpts{
			Name: "browser_requests_total",
			Help: "The total number server browsing requests",
		}),
		BrowserErrors: promauto.With(registry).NewCounter(prometheus.CounterOpts{
			Name: "browser_errors_total",
			Help: "The total number of failed server browsing requests",
		}),
		BrowserReceived: promauto.With(registry).NewCounter(prometheus.CounterOpts{
			Name: "browser_received_bytes_total",
			Help: "The total amount of bytes received by browser",
		}),
		BrowserSent: promauto.With(registry).NewCounter(prometheus.CounterOpts{
			Name: "browser_sent_bytes_total",
			Help: "The total amount of bytes sent by browser",
		}),
		BrowserDurations: promauto.With(registry).NewHistogram(prometheus.HistogramOpts{
			Name: "browser_duration_seconds",
			Help: "Duration of server browsing requests",
		}),
		CleanerRemovals: promauto.With(registry).NewCounter(prometheus.CounterOpts{
			Name: "cleaner_removals_total",
			Help: "The total number of inactive servers removed",
		}),
		CleanerErrors: promauto.With(registry).NewCounter(prometheus.CounterOpts{
			Name: "cleaner_errors_total",
			Help: "The total number of errors occurred during cleaner runs",
		}),
		DiscoveryWorkersBusy: promauto.With(registry).NewGauge(prometheus.GaugeOpts{
			Name: "discovery_busy_workers",
			Help: "The total number of busy discovery workers",
		}),
		DiscoveryWorkersAvailable: promauto.With(registry).NewGauge(prometheus.GaugeOpts{
			Name: "discovery_available_workers",
			Help: "The total number of available discovery workers",
		}),
		DiscoveryQueueProduced: promauto.With(registry).NewCounter(prometheus.CounterOpts{
			Name: "discovery_queue_produced_total",
			Help: "The total number of discovery targets put in discovery queue",
		}),
		DiscoveryQueueConsumed: promauto.With(registry).NewCounter(prometheus.CounterOpts{
			Name: "discovery_queue_consumed_total",
			Help: "The total number of discovery targets consumed from discovery queue",
		}),
		DiscoveryQueueExpired: promauto.With(registry).NewCounter(prometheus.CounterOpts{
			Name: "discovery_queue_expired_total",
			Help: "The total number of expired targets in discovery queue",
		}),
		DiscoveryProbeDurations: promauto.With(registry).NewHistogramVec(prometheus.HistogramOpts{
			Name: "discovery_probe_duration_seconds",
			Help: "Duration of discovery probes",
		}, []string{"goal"}),
		DiscoveryQueryDurations: promauto.With(registry).NewHistogram(prometheus.HistogramOpts{
			Name: "discovery_query_duration_seconds",
			Help: "Duration of probe gs1 queries",
		}),
		DiscoveryProbes: promauto.With(registry).NewCounterVec(prometheus.CounterOpts{
			Name: "discovery_probes_total",
			Help: "The total number of performed discovery probes",
		}, []string{"goal"}),
		DiscoveryProbeSuccess: promauto.With(registry).NewCounterVec(prometheus.CounterOpts{
			Name: "discovery_probe_success_total",
			Help: "The total number of successful discovery probes",
		}, []string{"goal"}),
		DiscoveryProbeRetries: promauto.With(registry).NewCounterVec(prometheus.CounterOpts{
			Name: "discovery_probe_retries_total",
			Help: "The total number of retried discovery probes",
		}, []string{"goal"}),
		DiscoveryProbeFailures: promauto.With(registry).NewCounterVec(prometheus.CounterOpts{
			Name: "discovery_probe_failures_total",
			Help: "The total number of unsuccessful discovery probes",
		}, []string{"goal"}),
		DiscoveryProbeErrors: promauto.With(registry).NewCounterVec(prometheus.CounterOpts{
			Name: "discovery_probe_errors_total",
			Help: "The total number of unexpected errors occurred during a discovery probe",
		}, []string{"goal"}),
		ServerRepositorySize: promauto.With(registry).NewGauge(prometheus.GaugeOpts{
			Name: "repo_servers_size",
			Help: "The number of servers stored in the repository",
		}),
		InstanceRepositorySize: promauto.With(registry).NewGauge(prometheus.GaugeOpts{
			Name: "repo_instances_size",
			Help: "The number of server instances stored in the repository",
		}),
		ProbeRepositorySize: promauto.With(registry).NewGauge(prometheus.GaugeOpts{
			Name: "repo_probes_size",
			Help: "The number of queue probes stored in the repository",
		}),
		GameDiscoveredServers: promauto.With(registry).NewGaugeVec(prometheus.GaugeOpts{
			Name: "game_discovered_servers",
			Help: "The number of discovered game servers",
		}, []string{"status"}),
		GameActiveServers: promauto.With(registry).NewGaugeVec(prometheus.GaugeOpts{
			Name: "game_active_servers",
			Help: "The number of active game servers",
		}, []string{"gametype"}),
		GamePlayedServers: promauto.With(registry).NewGaugeVec(prometheus.GaugeOpts{
			Name: "game_played_servers",
			Help: "The number of active game servers with at least 1 player",
		}, []string{"gametype"}),
		GamePlayers: promauto.With(registry).NewGaugeVec(prometheus.GaugeOpts{
			Name: "game_players",
			Help: "The number of players currently playing",
		}, []string{"gametype"}),
	}
	return ms
}

func (ms *MetricService) GetRegistry() *prometheus.Registry {
	return ms.registry
}

func (ms *MetricService) Observe(
	ctx context.Context,
	cfg config.Config,
	serversRepo servers.Repository,
	instancesRepo instances.Repository,
	probesRepo probes.Repository,
) {
	go ms.observeServerRepoSize(ctx, serversRepo)
	go ms.observeInstanceRepoSize(ctx, instancesRepo)
	go ms.observeProbeRepoSize(ctx, probesRepo)
	go ms.observeActiveServers(ctx, serversRepo, cfg)
	go ms.observeDiscoveredServers(ctx, serversRepo)
}

func (ms *MetricService) observeServerRepoSize(ctx context.Context, svrRepo servers.Repository) {
	count, err := svrRepo.Count(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Unable to observe server count")
		return
	}
	ms.ServerRepositorySize.Set(float64(count))
}

func (ms *MetricService) observeInstanceRepoSize(ctx context.Context, insRepo instances.Repository) {
	count, err := insRepo.Count(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Unable to observe instance count")
		return
	}
	ms.InstanceRepositorySize.Set(float64(count))
}

func (ms *MetricService) observeProbeRepoSize(ctx context.Context, probRepo probes.Repository) {
	count, err := probRepo.Count(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Unable to observe probe count")
		return
	}
	ms.ProbeRepositorySize.Set(float64(count))
}

func (ms *MetricService) observeActiveServers(
	ctx context.Context,
	svrRepo servers.Repository,
	cfg config.Config,
) {
	players := make(map[string]int)
	allServers := make(map[string]int)
	playedServers := make(map[string]int)

	activeSince := time.Now().Add(-cfg.BrowserServerLiveness)
	fs := servers.NewFilterSet().After(activeSince).WithStatus(ds.Info)
	activeServers, err := svrRepo.Filter(ctx, fs)
	if err != nil {
		log.Error().Err(err).Msg("Unable to observe active server count")
		return
	}

	for _, server := range activeServers {
		info := server.GetInfo()
		allServers[info.GameType]++
		if info.NumPlayers > 0 {
			players[info.GameType] += info.NumPlayers
			playedServers[info.GameType]++
		}
	}

	for gametype, playerCount := range players {
		ms.GamePlayers.WithLabelValues(gametype).Set(float64(playerCount))
	}
	for gametype, serverCount := range allServers {
		ms.GameActiveServers.WithLabelValues(gametype).Set(float64(serverCount))
	}
	for gametype, serverCount := range playedServers {
		ms.GamePlayedServers.WithLabelValues(gametype).Set(float64(serverCount))
	}
}

func (ms *MetricService) observeDiscoveredServers(
	ctx context.Context,
	svrRepo servers.Repository,
) {
	countByStatus, err := svrRepo.CountByStatus(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Unable to observe discovered server count")
		return
	}
	for status, serverCount := range countByStatus {
		switch status { // nolint: exhaustive
		case ds.NoStatus:
			continue
		default:
			ms.GameDiscoveredServers.WithLabelValues(status.String()).Set(float64(serverCount))
		}
	}
}
