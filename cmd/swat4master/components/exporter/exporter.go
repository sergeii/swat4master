package exporter

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/internal/metrics"
	"github.com/sergeii/swat4master/pkg/http/httpserver"
)

type Config struct {
	HTTPListenAddress   string
	HTTPReadTimeout     time.Duration
	HTTPWriteTimeout    time.Duration
	HTTPShutdownTimeout time.Duration
}

type Component struct{}

func New(
	lc fx.Lifecycle,
	shutdowner fx.Shutdowner,
	cfg Config,
	logger *zerolog.Logger,
	collector *metrics.Collector,
) (*Component, error) {
	registry := collector.GetRegistry()
	svr, err := httpserver.New(
		cfg.HTTPListenAddress,
		httpserver.WithShutdownTimeout(cfg.HTTPShutdownTimeout),
		httpserver.WithReadTimeout(cfg.HTTPReadTimeout),
		httpserver.WithWriteTimeout(cfg.HTTPWriteTimeout),
		httpserver.WithHandler(promhttp.InstrumentMetricHandler(
			registry,
			promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
		)),
	)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to set up exporter server")
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			if err := svr.Listen(); err != nil {
				logger.Error().Err(err).Msg("Failed to start exporter server")
				return err
			}
			logger.Info().Stringer("addr", svr.ListenAddr()).Msg("Exporter server is ready to accept connections")
			go func() {
				if serveErr := svr.Serve(); serveErr != nil {
					logger.Warn().Err(serveErr).Msg("Exporter server exited prematurely")
					if shutErr := shutdowner.Shutdown(); shutErr != nil {
						logger.Error().Err(shutErr).Msg("Failed to handle premature exporter server shutdown")
					}
				}
			}()
			return nil
		},
		OnStop: func(stopCtx context.Context) error {
			if stopErr := svr.Stop(stopCtx); stopErr != nil {
				logger.Error().Err(stopErr).Msg("Failed to stop exporter server gracefully")
				return stopErr
			}
			logger.Info().Msg("Exporter server stopped")
			return nil
		},
	})

	return &Component{}, nil
}

var Module = fx.Module("exporter",
	fx.Provide(New),
)
