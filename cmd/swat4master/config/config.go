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
	flag.BoolVar(&cfg.Version, "v", false, "Prints the version")
	flag.BoolVar(&cfg.Version, "version", false, "Prints the version")
	flag.StringVar(&cfg.LogLevel, "log.level", "info", "Set logging level")
	flag.StringVar(
		&cfg.LogOutput, "log.output", "console",
		"Set output format for logs. Available options: console, stdout, json",
	)
	flag.StringVar(
		&cfg.ReporterListenAddr, "reporter.address", "localhost:27900",
		"Reporter server listen address in the form of [host]:port",
	)
	flag.IntVar(
		&cfg.ReporterBufferSize, "reporter.buffer", 2048,
		"UDP read buffer size used by reporter server",
	)
	flag.StringVar(
		&cfg.BrowserListenAddr, "browser.address", "localhost:28910",
		"Browser server listen address in the form of [host]:port",
	)
	flag.DurationVar(
		&cfg.BrowserClientTimeout, "browser.timeout", time.Second,
		"Client timeout value for connections accepted by browser server",
	)
	flag.DurationVar(
		&cfg.BrowserServerLiveness, "browser.liveness", time.Second*60,
		"The amount of time since the most recent communication a server is considered alive",
	)
	flag.StringVar(
		&cfg.HTTPListenAddr, "http.address", "localhost:3000",
		"",
	)
	flag.DurationVar(
		&cfg.HTTPShutdownTimeout, "http.shutdown-timeout", time.Second*10,
		"",
	)
	flag.DurationVar(
		&cfg.HTTPReadTimeout, "http.read-timeout", time.Second*5,
		"",
	)
	flag.DurationVar(
		&cfg.HTTPWriteTimeout, "http.write-timeout", time.Second*5,
		"",
	)
	flag.DurationVar(
		&cfg.MemoryRetention, "memory.retention", time.Hour,
		"Define how long a server should stay in the memory storage after going offline",
	)
	flag.DurationVar(
		&cfg.MemoryCleanInterval, "memory.clean", time.Minute,
		"Define how often should the memory storage should be cleaned",
	)
	flag.Parse()
	return &cfg
}
