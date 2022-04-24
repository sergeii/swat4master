package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type MetricService struct {
	ReporterRequests  *prometheus.CounterVec
	ReporterErrors    *prometheus.CounterVec
	ReporterReceived  prometheus.Counter
	ReporterSent      prometheus.Counter
	ReporterDurations *prometheus.HistogramVec
	BrowserRequests   prometheus.Counter
	BrowserErrors     prometheus.Counter
	BrowserReceived   prometheus.Counter
	BrowserSent       prometheus.Counter
	BrowserDurations  prometheus.Histogram
}

func NewMetricService() *MetricService {
	ms := &MetricService{
		ReporterRequests: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "reporter_requests_total",
			Help: "The total number of successful reporting requests",
		}, []string{"type"}),
		ReporterErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "reporter_errors_total",
			Help: "The total number of failed reporting requests that",
		}, []string{"type"}),
		ReporterReceived: promauto.NewCounter(prometheus.CounterOpts{
			Name: "reporter_received_bytes_total",
			Help: "The total amount of bytes received by reporter",
		}),
		ReporterSent: promauto.NewCounter(prometheus.CounterOpts{
			Name: "reporter_sent_bytes_total",
			Help: "The total amount of bytes sent by reporter",
		}),
		ReporterDurations: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name: "reporter_duration_seconds",
			Help: "Duration of reporting requests",
		}, []string{"type"}),
		BrowserRequests: promauto.NewCounter(prometheus.CounterOpts{
			Name: "browser_requests_total",
			Help: "The total number server browsing requests",
		}),
		BrowserErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "browser_errors_total",
			Help: "The total number of failed server browsing requests",
		}),
		BrowserReceived: promauto.NewCounter(prometheus.CounterOpts{
			Name: "browser_received_bytes_total",
			Help: "The total amount of bytes received by browser",
		}),
		BrowserSent: promauto.NewCounter(prometheus.CounterOpts{
			Name: "browser_sent_bytes_total",
			Help: "The total amount of bytes sent by browser",
		}),
		BrowserDurations: promauto.NewHistogram(prometheus.HistogramOpts{
			Name: "browser_duration_seconds",
			Help: "Duration of server browsing requests",
		}),
	}
	return ms
}
