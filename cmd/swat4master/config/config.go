package config

import (
	"flag"
	"time"
)

type Config struct {
	Version bool

	LogLevel  string
	LogOutput string

	ReporterListenAddr string
	ReporterBufferSize int

	BrowserListenAddr     string
	BrowserClientTimeout  time.Duration
	BrowserServerLiveness time.Duration

	HTTPListenAddr      string
	HTTPReadTimeout     time.Duration
	HTTPWriteTimeout    time.Duration
	HTTPShutdownTimeout time.Duration

	MemoryRetention     time.Duration
	MemoryCleanInterval time.Duration
}

func Init() *Config {
	cfg := Config{}
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
		&cfg.BrowserServerLiveness, "browser.liveness", time.Second*60,
		"Total amount of time since the most recent heartbeat it takes to declare a game server offline",
	)
	flag.StringVar(
		&cfg.HTTPListenAddr, "http.address", ":3000",
		"Address to listen on for API and telemetry",
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
		&cfg.MemoryRetention, "memory.retention", time.Hour,
		"Defines how long a game server should stay in the memory storage after going offline",
	)
	flag.DurationVar(
		&cfg.MemoryCleanInterval, "memory.clean", time.Minute,
		"Defines how often should the memory storage should be cleaned",
	)
	flag.Parse()
	return &cfg
}
