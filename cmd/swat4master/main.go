package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/build"
	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/logging"
	"github.com/sergeii/swat4master/cmd/swat4master/running"
	"github.com/sergeii/swat4master/cmd/swat4master/running/api"
	"github.com/sergeii/swat4master/cmd/swat4master/running/browser"
	"github.com/sergeii/swat4master/cmd/swat4master/running/cleaner"
	"github.com/sergeii/swat4master/cmd/swat4master/running/collector"
	"github.com/sergeii/swat4master/cmd/swat4master/running/exporter"
	"github.com/sergeii/swat4master/cmd/swat4master/running/finder"
	"github.com/sergeii/swat4master/cmd/swat4master/running/prober"
	"github.com/sergeii/swat4master/cmd/swat4master/running/reporter"
	"github.com/sergeii/swat4master/internal/validation"
	"github.com/sergeii/swat4master/pkg/random"
)

func main() {
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	version := fmt.Sprintf("Version: %s (%s) built at %s", build.Version, build.Commit, build.Time)
	cfg := config.Init()

	if cfg.Version {
		fmt.Println(version) // nolint: forbidigo
		os.Exit(0)
	}

	logger, err := logging.ConfigureLogging(cfg)
	if err != nil {
		panic(err)
	}
	log.Logger = logger

	if err := validation.Register(); err != nil {
		log.Panic().Err(err).Msg("Unable to start without validate")
	}

	// must have properly seeded rng
	if err := random.Seed(); err != nil {
		log.Panic().Err(err).Msg("Unable to start without rand")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := application.Configure()
	runner := running.NewRunner(app, cfg)

	runner.Add(reporter.Run, ctx)
	runner.Add(browser.Run, ctx)
	runner.Add(api.Run, ctx)
	runner.Add(finder.Run, ctx)
	runner.Add(prober.Run, ctx)
	runner.Add(exporter.Run, ctx)
	runner.Add(cleaner.Run, ctx)
	runner.Add(collector.Run, ctx)

	go func() {
		select {
		case <-runner.Exit:
			log.Error().Msg("Exiting due to a service failure")
			cancel()
		case <-shutdown:
			log.Info().Msg("Exiting due to shutdown signal")
			cancel()
		}
	}()

	log.Info().
		Str("version", build.Version).
		Str("commit", build.Commit).
		Str("built", build.Time).
		Msg("Welcome to SWAT4 master server!")

	runner.WaitQuit()
}
