package metrics

import (
	"context"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Collector struct {
	mutex     sync.Mutex
	registry  *prometheus.Registry
	observers []Observer

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

func New() *Collector {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	registry.MustRegister(collectors.NewGoCollector())

	c := &Collector{
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
			Help: "The total number of discovery probes put in discovery queue",
		}),
		DiscoveryQueueConsumed: promauto.With(registry).NewCounter(prometheus.CounterOpts{
			Name: "discovery_queue_consumed_total",
			Help: "The total number of discovery probes consumed from discovery queue",
		}),
		DiscoveryQueueExpired: promauto.With(registry).NewCounter(prometheus.CounterOpts{
			Name: "discovery_queue_expired_total",
			Help: "The total number of expired probes in discovery queue",
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
	return c
}

func (c *Collector) GetRegistry() *prometheus.Registry {
	return c.registry
}

func (c *Collector) AddObserver(observer Observer) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.observers = append(c.observers, observer)
}

func (c *Collector) Observe(ctx context.Context) {
	for _, observer := range c.observers {
		go observer.Observe(ctx, c)
	}
}
