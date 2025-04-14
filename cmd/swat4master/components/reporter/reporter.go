package reporter

import (
	"context"

	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/commander"
	"github.com/sergeii/swat4master/internal/reporter"
	"github.com/sergeii/swat4master/internal/reporter/handlers/available"
	"github.com/sergeii/swat4master/internal/reporter/handlers/challenge"
	"github.com/sergeii/swat4master/internal/reporter/handlers/heartbeat"
	"github.com/sergeii/swat4master/internal/reporter/handlers/keepalive"
	"github.com/sergeii/swat4master/pkg/udp/udpserver"
)

type Config struct {
	ListenAddr string
	BufferSize int
}

type Component struct{}

func New(
	lc fx.Lifecycle,
	shutdowner fx.Shutdowner,
	dispatcher *reporter.Dispatcher,
	cfg Config,
	logger *zerolog.Logger,
) (*Component, error) {
	ready := make(chan struct{})

	svr, err := udpserver.New(
		cfg.ListenAddr,
		dispatcher,
		udpserver.WithBufferSize(cfg.BufferSize),
		udpserver.WithReadySignal(func() {
			close(ready)
		}),
	)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to set up reporter server")
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() { // nolint: contextcheck
				logger.Info().Str("listen", cfg.ListenAddr).Msg("Starting reporter server")
				if serveErr := svr.Listen(); serveErr != nil {
					logger.Warn().Err(serveErr).Msg("Reporter server exited prematurely")
					if shutErr := shutdowner.Shutdown(); shutErr != nil {
						logger.Error().Err(shutErr).Msg("Failed to handle premature shutdown")
					}
				}
			}()
			<-ready
			return nil
		},
		OnStop: func(context.Context) error {
			if stopErr := svr.Stop(); stopErr != nil {
				logger.Error().Err(stopErr).Msg("Failed to stop reporter server")
				return stopErr
			}
			logger.Info().Msg("Reporter server stopped")
			return nil
		},
	})

	return &Component{}, nil
}

type command struct {
	ReporterListenAddr string `default:":27900" help:"Sets the listen address for the reporter UDP server"`
	ReporterBufferSize int    `default:"2048"   help:"Sets the UDP buffer size for incoming packets"`
}

func (c *command) Run(_ *commander.Globals, builder *application.Builder) error {
	app := builder.
		Add(
			fx.Supply(Config{
				ListenAddr: c.ReporterListenAddr,
				BufferSize: c.ReporterBufferSize,
			}),
			Module,
			fx.Invoke(func(_ *Component) {}),
		).
		WithExporter().
		Build()
	app.Run()
	return nil
}

type CLI struct {
	Reporter command `cmd:"" help:"Start reporter server"`
}

var Module = fx.Module("reporter",
	fx.Provide(
		fx.Private,
		reporter.NewDispatcher,
	),
	fx.Invoke(
		available.New,
		challenge.New,
		heartbeat.New,
		keepalive.New,
	),
	fx.Provide(New),
)
