package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/cmd/swat4master/build"
	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/logging"
	"github.com/sergeii/swat4master/cmd/swat4master/subcommand"
	"github.com/sergeii/swat4master/cmd/swat4master/subcommand/browser"
	"github.com/sergeii/swat4master/cmd/swat4master/subcommand/http"
	"github.com/sergeii/swat4master/cmd/swat4master/subcommand/reporter"
	"github.com/sergeii/swat4master/internal/api/monitoring"
	"github.com/sergeii/swat4master/internal/application"
	"github.com/sergeii/swat4master/internal/server/memory"
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

	// must have properly seeded rng
	if err := random.Seed(); err != nil {
		log.Panic().Err(err).Msg("Unable to start without rand")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gCtx := subcommand.NewGroupContext(cfg, 3)
	go func() {
		select {
		case <-gCtx.Exit:
			log.Error().Msg("Exiting due to a service failure")
			cancel()
		case <-shutdown:
			log.Info().Msg("Exiting due to shutdown signal")
			cancel()
		}
	}()

	metrics := monitoring.NewMetricService()
	app := application.NewApp(
		application.WithMetricService(metrics),
		application.WithServerRepository(
			memory.New(memory.WithCleaner(ctx, cfg.MemoryCleanInterval, cfg.MemoryRetention)),
		),
	)
	go reporter.Run(ctx, gCtx, app)
	go browser.Run(ctx, gCtx, app)
	go http.Run(ctx, gCtx, app)

	log.Info().
		Str("version", build.Version).
		Str("commit", build.Commit).
		Str("built", build.Time).
		Msg("Welcome to SWAT4 master server!")

	gCtx.WaitQuit()
}
