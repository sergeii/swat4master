package config

import (
	"flag"
	"time"
)

type Config struct {
	Version bool

	Debug                 bool
	ReporterListenAddr    string
	ReporterBufferSize    int
	BrowserListenAddr     string
	BrowserClientTimeout  time.Duration
	BrowserServerLiveness time.Duration
	MemoryRetention       time.Duration
	MemoryCleanInterval   time.Duration
}

func Init() *Config {
	cfg := Config{}
	flag.BoolVar(&cfg.Version, "v", false, "Prints the version")
	flag.BoolVar(&cfg.Version, "version", false, "Prints the version")
	flag.BoolVar(&cfg.Debug, "debug", false, "Enable debug logging")
	flag.StringVar(
		&cfg.ReporterListenAddr, "reporter.address", ":27900",
		"Reporter server listen address in the form of [host]:port",
	)
	flag.IntVar(
		&cfg.ReporterBufferSize, "reporter.buffer", 2048,
		"UDP read buffer size used by reporter server",
	)
	flag.StringVar(
		&cfg.BrowserListenAddr, "browser.address", ":28910",
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
