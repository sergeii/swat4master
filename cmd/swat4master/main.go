package main

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/sergeii/swat4master/cmd/swat4master/build"
	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/api"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/browser"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/cleaner"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/collector"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/exporter"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/finder"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/prober"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/reporter"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
)

func main() {
	app := fx.New(
		fx.Provide(config.Provide),
		application.Module,
		finder.Module,
		prober.Module,
		browser.Module,
		reporter.Module,
		api.Module,
		cleaner.Module,
		collector.Module,
		exporter.Module,
		fx.WithLogger(
			func(logger *zerolog.Logger, lvl zerolog.Level) fxevent.Logger {
				switch lvl { // nolint: exhaustive
				case zerolog.DebugLevel:
					return &fxevent.ConsoleLogger{
						W: logger,
					}
				default:
					return fxevent.NopLogger
				}
			},
		),
		fx.Invoke(func(cfg config.Config) {
			if !cfg.Version {
				return
			}
			version := fmt.Sprintf("Version: %s (%s) built at %s", build.Version, build.Commit, build.Time)
			fmt.Println(version) // nolint: forbidigo
			os.Exit(0)
		}),
		fx.Invoke(func(
			logger *zerolog.Logger,
			_ *finder.Finder,
			_ *prober.Prober,
			_ *browser.Browser,
			_ *reporter.Reporter,
			_ *api.API,
			_ *cleaner.Cleaner,
			_ *collector.Collector,
			_ *exporter.Exporter,
		) {
			logger.Info().
				Str("version", build.Version).
				Str("commit", build.Commit).
				Str("built", build.Time).
				Msg("Welcome to SWAT4 master server!")
		}),
	)
	app.Run()
}
