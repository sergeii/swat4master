package config

import (
	"errors"
	"flag"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Version bool

	LogLevel  string
	LogOutput string

	RedisURL string

	ReporterListenAddr string
	ReporterBufferSize int

	BrowserListenAddr     string
	BrowserClientTimeout  time.Duration
	BrowserServerLiveness time.Duration

	HTTPListenAddr      string
	HTTPReadTimeout     time.Duration
	HTTPWriteTimeout    time.Duration
	HTTPShutdownTimeout time.Duration

	MetricObserverInterval time.Duration

	ExporterListenAddr string

	DiscoveryRefreshInterval time.Duration
	DiscoveryRefreshRetries  int

	DiscoveryRevivalInterval  time.Duration
	DiscoveryRevivalScope     time.Duration
	DiscoveryRevivalCountdown time.Duration
	DiscoveryRevivalPorts     []int
	DiscoveryRevivalRetries   int

	ProbePollSchedule time.Duration
	ProbeTimeout      time.Duration
	ProbeConcurrency  int

	CleanRetention time.Duration
	CleanInterval  time.Duration
}

func Provide() Config {
	cfg := Config{
		// set default value for port suggestions
		DiscoveryRevivalPorts: []int{1, 2, 3, 4},
	}
	flag.BoolVar(&cfg.Version, "v", false, "Show the version")
	flag.BoolVar(&cfg.Version, "version", false, "Show the version")
	flag.StringVar(
		&cfg.LogLevel, "log.level", "info",
		"Only log messages with the given severity or above.\n"+
			"For example: debug, info, warn, error and other levels supported by zerolog",
	)
	flag.StringVar(
		&cfg.LogOutput, "log.output", "console",
		"Output format of log messages. Available options: console, stdout, json",
	)
	flag.StringVar(
		&cfg.RedisURL, "redis.url", "redis://localhost:6379",
		"URL to connect to the Redis server",
	)
	flag.StringVar(
		&cfg.ReporterListenAddr, "reporter.address", ":27900",
		"Address to listen on for the reporter service",
	)
	flag.IntVar(
		&cfg.ReporterBufferSize, "reporter.buffer", 2048,
		"UDP buffer size used by reporter for reading incoming connections",
	)
	flag.StringVar(
		&cfg.BrowserListenAddr, "browser.address", ":28910",
		"Address to listen on for the browser service",
	)
	flag.DurationVar(
		&cfg.BrowserClientTimeout, "browser.timeout", time.Second,
		"Maximum duration before timing out an accepted connection by the browser service",
	)
	flag.DurationVar(
		&cfg.BrowserServerLiveness, "browser.liveness", time.Second*180,
		"Total amount of time since the most recent heartbeat it takes to declare a game server offline",
	)
	flag.StringVar(
		&cfg.HTTPListenAddr, "http.address", ":3000",
		"Address to listen on for the API server",
	)
	flag.DurationVar(
		&cfg.HTTPShutdownTimeout, "http.shutdown-timeout", time.Second*10,
		"The amount of time the server will wait gracefully closing connections before exiting",
	)
	flag.DurationVar(
		&cfg.HTTPReadTimeout, "http.read-timeout", time.Second*5,
		"Limits the time it takes from accepting a new connection till reading of the request body",
	)
	flag.DurationVar(
		&cfg.HTTPWriteTimeout, "http.write-timeout", time.Second*5,
		"Limits the time it takes from reading the body of a request till the end of the response",
	)
	flag.DurationVar(
		&cfg.MetricObserverInterval, "observer.interval", time.Second,
		"Defines the interval between periodic metric observer runs",
	)
	flag.StringVar(
		&cfg.ExporterListenAddr, "exporter.address", ":9000",
		"Prometheus exporter address to listen on",
	)
	flag.DurationVar(
		&cfg.DiscoveryRefreshInterval, "discovery.interval", time.Second*5,
		"Defines how often servers' details are refreshed",
	)
	flag.IntVar(
		&cfg.DiscoveryRefreshRetries, "discovery.retries", 4,
		"Determines how many times a failed server refresh is retried",
	)
	flag.DurationVar(
		&cfg.DiscoveryRevivalInterval, "revival.interval", time.Minute*10,
		"Defines how often unlisted servers are checked for a chance of occasional revival",
	)
	flag.DurationVar(
		&cfg.DiscoveryRevivalScope, "revival.scope", time.Hour,
		"Limits the time period beyond which the unlisted servers are not revived",
	)
	flag.DurationVar(
		&cfg.DiscoveryRevivalCountdown, "revival.countdown", time.Minute*5,
		"Limits the upper bound for random countdown at which revival probes are launched",
	)
	flag.Func(
		"revival.ports",
		"List of port offsets to search for a good query port (+1 +2 and so forth)",
		func(value string) error {
			portNumbers := strings.Fields(value)
			ports := make([]int, 0, len(portNumbers))
			for _, portNumber := range portNumbers {
				port, err := strconv.Atoi(portNumber)
				if err != nil {
					return errors.New("must contain a list of integer values")
				}
				ports = append(ports, port)
			}
			if len(ports) == 0 {
				return errors.New("must contain a non-empty list of port offsets")
			}
			cfg.DiscoveryRevivalPorts = ports
			return nil
		},
	)
	flag.IntVar(
		&cfg.DiscoveryRevivalRetries, "revival.retries", 2,
		"Determines how many times a failed revival probe is retried",
	)
	flag.DurationVar(
		&cfg.ProbePollSchedule, "probe.schedule", time.Millisecond*250,
		"Defines how often the discovery queue is checked for new probes",
	)
	flag.DurationVar(
		&cfg.ProbeTimeout, "probe.timeout", time.Second,
		"Limits the maximum time a discovery probe will be waited for a complete response",
	)
	flag.IntVar(
		&cfg.ProbeConcurrency, "probe.concurrency", 25,
		"Limits how many discovery probes can be ran simultaneously",
	)
	flag.DurationVar(
		&cfg.CleanRetention, "clean.retention", time.Hour,
		"Defines how long a game server should stay in the memory storage after going offline",
	)
	flag.DurationVar(
		&cfg.CleanInterval, "clean.interval", time.Minute*10,
		"Defines how often should the memory storage should be cleaned",
	)
	flag.Parse()
	return cfg
}
