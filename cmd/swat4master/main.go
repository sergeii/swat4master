package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/server/browser"
	"github.com/sergeii/swat4master/cmd/swat4master/server/reporter"
	"github.com/sergeii/swat4master/internal/server/memory"
	"github.com/sergeii/swat4master/pkg/logging"
	"github.com/sergeii/swat4master/pkg/random"
)

var (
	BuildVersion = "development"
	BuildCommit  = "uncommitted"
	BuildTime    = "unknown"
)

func main() {
	fail := make(chan struct{}, 2)
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	version := fmt.Sprintf("Version: %s (%s) built at %s", BuildVersion, BuildCommit, BuildTime)
	cfg := config.Init()

	if cfg.Version {
		fmt.Println(version) // nolint: forbidigo
		os.Exit(0)
	}

	configureLogging(cfg)

	// must have properly seeded rng
	if err := random.Seed(); err != nil {
		log.Panic().Err(err).Msg("Unable to start without rand")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-fail:
			log.Error().Msg("Exiting due to failure")
			cancel()
		case <-shutdown:
			log.Warn().Msg("Exiting due to shutdown signal")
			cancel()
		}
	}()

	repo := memory.New(memory.WithCleaner(ctx, cfg.MemoryCleanInterval, cfg.MemoryRetention))
	wg := &sync.WaitGroup{}
	wg.Add(2)
	go reporter.Run(ctx, wg, cfg, fail, repo)
	go browser.Run(ctx, wg, cfg, fail, repo)

	log.Info().
		Str("version", BuildVersion).
		Str("commit", BuildCommit).
		Str("built", BuildTime).
		Msg("Welcome to SWAT4 master server!")

	wg.Wait()
}

func configureLogging(cfg *config.Config) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMicro
	zerolog.DurationFieldUnit = time.Second
	zerolog.CallerMarshalFunc = logging.ShortCallerFormatter

	log.Logger = log.
		With().Caller().Logger().
		Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	if cfg.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}
