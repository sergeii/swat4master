package main

import (
	"github.com/alecthomas/kong"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/commander"
	"github.com/sergeii/swat4master/cmd/swat4master/components/api"
	"github.com/sergeii/swat4master/cmd/swat4master/components/browser"
	"github.com/sergeii/swat4master/cmd/swat4master/components/cleaner"
	"github.com/sergeii/swat4master/cmd/swat4master/components/exporter"
	"github.com/sergeii/swat4master/cmd/swat4master/components/observer"
	"github.com/sergeii/swat4master/cmd/swat4master/components/prober"
	"github.com/sergeii/swat4master/cmd/swat4master/components/refresher"
	"github.com/sergeii/swat4master/cmd/swat4master/components/reporter"
	"github.com/sergeii/swat4master/cmd/swat4master/components/reviver"
	"github.com/sergeii/swat4master/cmd/swat4master/logging"
	"github.com/sergeii/swat4master/cmd/swat4master/persistence"
	"github.com/sergeii/swat4master/internal/settings"
)

func main() {
	cli := commander.CLI{}
	cli.Run.Plugins = kong.Plugins{
		&api.CLI{},
		&browser.CLI{},
		&cleaner.CLI{},
		&observer.CLI{},
		&prober.CLI{},
		&refresher.CLI{},
		&reporter.CLI{},
		&reviver.CLI{},
	}
	ctx := kong.Parse(
		&cli,
		kong.Name("swat4master"),
		kong.Description("SWAT4 Master Server"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Summary:   true,
			Tree:      true,
			FlagsLast: true,
		}),
	)

	builder := application.NewBuilder(
		fx.Supply(persistence.Config{
			RedisURL: cli.Globals.RedisURL,
		}),
		fx.Provide(persistence.Provide),
		application.Module,
		fx.Supply(logging.Config{
			LogLevel:  cli.Globals.LogLevel,
			LogOutput: cli.Globals.LogOutput,
		}),
		fx.Supply(settings.Settings{
			ServerLiveness:          cli.Globals.BrowsingServerLiveness,
			DiscoveryRevivalRetries: cli.Globals.DiscoveryRevivalRetries,
			DiscoveryRefreshRetries: cli.Globals.DiscoveryRefreshRetries,
		}),
		fx.Provide(logging.Provide),
		fx.WithLogger(logging.FxLogger),
		fx.Supply(exporter.Config{
			HTTPListenAddress:   cli.Globals.ExporterHTTPListenAddress,
			HTTPReadTimeout:     cli.Globals.ExporterHTTPReadTimeout,
			HTTPWriteTimeout:    cli.Globals.ExporterHTTPWriteTimeout,
			HTTPShutdownTimeout: cli.Globals.ExporterHTTPShutdownTimeout,
		}),
		exporter.Module,
	)

	if err := ctx.Run(&cli.Globals, builder); err != nil {
		ctx.FatalIfErrorf(err)
	}
}
