package reporter

import (
	"context"

	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/internal/reporter"
	"github.com/sergeii/swat4master/internal/reporter/handlers/available"
	"github.com/sergeii/swat4master/internal/reporter/handlers/challenge"
	"github.com/sergeii/swat4master/internal/reporter/handlers/heartbeat"
	"github.com/sergeii/swat4master/internal/reporter/handlers/keepalive"
	udp "github.com/sergeii/swat4master/pkg/udp/server"
)

type Reporter struct{}

func NewReporter(
	lc fx.Lifecycle,
	shutdowner fx.Shutdowner,
	dispatcher *reporter.Dispatcher,
	cfg config.Config,
	logger *zerolog.Logger,
) (*Reporter, error) {
	ready := make(chan struct{})

	svr, err := udp.New(
		cfg.ReporterListenAddr,
		dispatcher,
		udp.WithBufferSize(cfg.ReporterBufferSize),
		udp.WithReadySignal(func() {
			close(ready)
		}),
	)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to setup reporter UDP server")
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				logger.Info().Str("listen", cfg.ReporterListenAddr).Msg("Starting reporter")
				if err := svr.Listen(); err != nil {
					logger.Error().Err(err).Msg("Reporter UDP server exited prematurely")
					if shutErr := shutdowner.Shutdown(); shutErr != nil {
						logger.Error().Err(shutErr).Msg("Failed to invoke shutdown")
					}
				}
			}()
			<-ready
			return nil
		},
		OnStop: func(context.Context) error {
			logger.Info().Msg("Stopping reporter")
			if err := svr.Stop(); err != nil {
				return err
			}
			logger.Info().Msg("Reporter UDP server stopped successfully")
			return nil
		},
	})

	return &Reporter{}, nil
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
	fx.Provide(NewReporter),
)
