package browser

import (
	"context"
	"net"

	"github.com/benbjohnson/clock"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/internal/services/master/browsing"
	"github.com/sergeii/swat4master/internal/services/monitoring"
	tcp "github.com/sergeii/swat4master/pkg/tcp/server"
)

type Browser struct{}

type Handler struct {
	service *browsing.Service
	metrics *monitoring.MetricService
	clock   clock.Clock
	logger  *zerolog.Logger
}

func newHandler(
	service *browsing.Service,
	metrics *monitoring.MetricService,
	clock clock.Clock,
	logger *zerolog.Logger,
) *Handler {
	return &Handler{service, metrics, clock, logger}
}

func NewBrowser(
	lc fx.Lifecycle,
	shutdowner fx.Shutdowner,
	handler *Handler,
	cfg config.Config,
	logger *zerolog.Logger,
) (*Browser, error) {
	ready := make(chan struct{})

	svr, err := tcp.New(
		cfg.BrowserListenAddr,
		handler,
		tcp.WithTimeout(cfg.BrowserClientTimeout),
		tcp.WithReadySignal(func(addr net.Addr) {
			logger.Info().Stringer("addr", addr).Msg("Browser tcp connection ready")
			close(ready)
		}),
	)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to setup browser server")
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				logger.Info().
					Str("listen", cfg.BrowserListenAddr).Dur("timeout", cfg.BrowserClientTimeout).
					Msg("Starting browser")
				if err := svr.Listen(); err != nil {
					logger.Error().Err(err).Msg("Browser server exited prematurely")
					if shutErr := shutdowner.Shutdown(); shutErr != nil {
						logger.Error().Err(shutErr).Msg("Failed to invoke shutdown")
					}
				}
			}()
			<-ready
			return nil
		},
		OnStop: func(stopCtx context.Context) error {
			logger.Info().Msg("Stopping browser")
			if err := svr.Stop(); err != nil {
				logger.Error().
					Err(err).
					Msg("Failed to stop browser server")
				return err
			}
			logger.Info().Msg("Browser server stopped successfully")
			return nil
		},
	})

	return &Browser{}, nil
}

var Module = fx.Module("reporter",
	fx.Provide(
		fx.Private,
		func(cfg config.Config) browsing.ServiceOpts {
			return browsing.ServiceOpts{
				Liveness: cfg.BrowserServerLiveness,
			}
		},
	),
	fx.Provide(
		fx.Private,
		browsing.NewService,
	),
	fx.Provide(
		fx.Private,
		newHandler,
	),
	fx.Provide(NewBrowser),
)
