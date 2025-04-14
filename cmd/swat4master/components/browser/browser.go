package browser

import (
	"context"
	"net"
	"time"

	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/commander"
	"github.com/sergeii/swat4master/internal/browser"
	"github.com/sergeii/swat4master/internal/settings"
	"github.com/sergeii/swat4master/pkg/tcp/tcpserver"
)

type Config struct {
	ListenAddr    string
	ClientTimeout time.Duration
}

type Component struct{}

func New(
	lc fx.Lifecycle,
	shutdowner fx.Shutdowner,
	handler browser.Handler,
	cfg Config,
	logger *zerolog.Logger,
) (*Component, error) {
	ready := make(chan struct{})

	svr, err := tcpserver.New(
		cfg.ListenAddr,
		handler,
		tcpserver.WithTimeout(cfg.ClientTimeout),
		tcpserver.WithReadySignal(func(addr net.Addr) {
			logger.Info().Stringer("addr", addr).Msg("Browser server is ready to accept connections")
			close(ready)
		}),
	)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to set up browser server")
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() { // nolint: contextcheck
				logger.Info().Str("addr", cfg.ListenAddr).Msg("Starting browser server")
				if serveErr := svr.Listen(); serveErr != nil {
					logger.Warn().Err(serveErr).Msg("Browser server exited prematurely")
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
				logger.Error().Err(stopErr).Msg("Failed to stop browser server")
				return stopErr
			}
			logger.Info().Msg("Browser server stopped")
			return nil
		},
	})

	return &Component{}, nil
}

type command struct {
	BrowserListenAddr    string        `default:":28910" help:"Sets the listen address for the browser TCP server"`
	BrowserClientTimeout time.Duration `default:"1s"     help:"Sets the maximum duration before an accepted connection times out"` // nolint:lll
}

func (c *command) Run(_ *commander.Globals, builder *application.Builder) error {
	app := builder.
		Add(
			fx.Supply(Config{
				ListenAddr:    c.BrowserListenAddr,
				ClientTimeout: c.BrowserClientTimeout,
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
	Browser command `cmd:"" help:"Start browser server"`
}

var Module = fx.Module("browser",
	fx.Provide(
		fx.Private,
		func(settings settings.Settings) browser.HandlerOpts {
			return browser.HandlerOpts{
				Liveness: settings.ServerLiveness,
			}
		},
	),
	fx.Provide(fx.Private, browser.NewHandler),
	fx.Provide(New),
)
