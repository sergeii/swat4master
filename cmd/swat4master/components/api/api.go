package api

import (
	"context"
	"net"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/build"
	"github.com/sergeii/swat4master/cmd/swat4master/commander"
	"github.com/sergeii/swat4master/internal/rest"
	"github.com/sergeii/swat4master/internal/rest/api"
	"github.com/sergeii/swat4master/pkg/http/httpserver"
)

type Config struct {
	HTTPListenAddr      string
	HTTPReadTimeout     time.Duration
	HTTPWriteTimeout    time.Duration
	HTTPShutdownTimeout time.Duration
}

type Component struct{}

func New(
	lc fx.Lifecycle,
	shutdowner fx.Shutdowner,
	router *gin.Engine,
	cfg Config,
	logger *zerolog.Logger,
) (*Component, error) {
	ready := make(chan struct{})

	svr, err := httpserver.New(
		cfg.HTTPListenAddr,
		httpserver.WithShutdownTimeout(cfg.HTTPShutdownTimeout),
		httpserver.WithReadTimeout(cfg.HTTPReadTimeout),
		httpserver.WithWriteTimeout(cfg.HTTPWriteTimeout),
		httpserver.WithHandler(router),
		httpserver.WithReadySignal(func(addr net.Addr) {
			logger.Info().Stringer("addr", addr).Msg("API server is ready to accept connections")
			close(ready)
		}),
	)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to set up API server")
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				if serveErr := svr.ListenAndServe(); serveErr != nil {
					logger.Warn().Err(serveErr).Msg("API server exited prematurely")
					if shutErr := shutdowner.Shutdown(); shutErr != nil {
						logger.Error().Err(shutErr).Msg("Failed to handle premature API server shutdown")
					}
				}
			}()
			<-ready
			return nil
		},
		OnStop: func(stopCtx context.Context) error {
			if stopErr := svr.Stop(stopCtx); stopErr != nil {
				logger.Error().Err(stopErr).Msg("Failed to stop API server gracefully")
				return stopErr
			}
			logger.Info().Msg("API server stopped")
			return nil
		},
	})

	return &Component{}, nil
}

type command struct {
	HTTPListenAddress   string        `default:":3000" help:"Sets the address where the API server listens for incoming http requests"`         // nolint:lll
	HTTPReadTimeout     time.Duration `default:"5s"    help:"Sets the maximum duration to write a response before timing out"`                  // nolint:lll
	HTTPWriteTimeout    time.Duration `default:"5s"    help:"Sets the maximum duration to write a response after reading the request body"`     // nolint:lll
	HTTPShutdownTimeout time.Duration `default:"10s"   help:"Defines how long the server waits to gracefully close connections before exiting"` // nolint:lll
}

func (c *command) Run(_ *commander.Globals, builder *application.Builder) error {
	app := builder.
		Add(
			fx.Supply(
				Config{
					HTTPListenAddr:      c.HTTPListenAddress,
					HTTPReadTimeout:     c.HTTPReadTimeout,
					HTTPWriteTimeout:    c.HTTPWriteTimeout,
					HTTPShutdownTimeout: c.HTTPShutdownTimeout,
				},
			),
			Module,
			fx.Invoke(func(logger *zerolog.Logger, _ *Component) {
				logger.Info().
					Str("version", build.Version).
					Str("commit", build.Commit).
					Str("built", build.Time).
					Str("address", c.HTTPListenAddress).
					Msg("Starting API server")
			}),
		).
		WithExporter().
		Build()
	app.Run()
	return nil
}

type CLI struct {
	API command `cmd:"" help:"Start API server"`
}

var Module = fx.Module("api",
	fx.Provide(fx.Private, api.New),
	fx.Provide(rest.NewRouter),
	fx.Provide(New),
)
