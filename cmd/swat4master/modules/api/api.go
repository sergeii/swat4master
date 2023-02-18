package api

import (
	"context"
	"net"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/internal/rest/api"
	http "github.com/sergeii/swat4master/pkg/http/server"
)

type API struct{}

func NewAPI(
	lc fx.Lifecycle,
	shutdowner fx.Shutdowner,
	router *gin.Engine,
	cfg config.Config,
	logger *zerolog.Logger,
) (*API, error) {
	ready := make(chan struct{})

	svr, err := http.New(
		cfg.HTTPListenAddr,
		http.WithShutdownTimeout(cfg.HTTPShutdownTimeout),
		http.WithReadTimeout(cfg.HTTPReadTimeout),
		http.WithWriteTimeout(cfg.HTTPWriteTimeout),
		http.WithHandler(router),
		http.WithReadySignal(func(addr net.Addr) {
			logger.Info().Stringer("addr", addr).Msg("API http server connection ready")
			close(ready)
		}),
	)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to setup API http server")
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				if err = svr.ListenAndServe(); err != nil {
					logger.Error().Err(err).Msg("API http server exited prematurely")
					if shutErr := shutdowner.Shutdown(); shutErr != nil {
						logger.Error().Err(shutErr).Msg("Failed to invoke shutdown")
					}
				}
			}()
			<-ready
			return nil
		},
		OnStop: func(stopCtx context.Context) error {
			if err := svr.Stop(stopCtx); err != nil {
				logger.Error().
					Err(err).
					Msg("Failed to stop API http server gracefully")
				return err
			}
			logger.Info().Msg("API http server stopped successfully")
			return nil
		},
	})

	return &API{}, nil
}

var Module = fx.Module("api",
	fx.Provide(
		fx.Private,
		api.New,
	),
	fx.Provide(
		NewRouter,
	),
	fx.Provide(
		NewAPI,
	),
)
