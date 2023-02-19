package exporter

import (
	"context"
	"net"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/internal/services/monitoring"
	http "github.com/sergeii/swat4master/pkg/http/server"
)

type Exporter struct{}

func NewExporter(
	lc fx.Lifecycle,
	shutdowner fx.Shutdowner,
	cfg config.Config,
	logger *zerolog.Logger,
	metrics *monitoring.MetricService,
) (*Exporter, error) {
	ready := make(chan struct{})

	registry := metrics.GetRegistry()
	svr, err := http.New(
		cfg.ExporterListenAddr,
		http.WithShutdownTimeout(cfg.HTTPShutdownTimeout),
		http.WithReadTimeout(cfg.HTTPReadTimeout),
		http.WithWriteTimeout(cfg.HTTPWriteTimeout),
		http.WithHandler(promhttp.InstrumentMetricHandler(
			registry,
			promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
		)),
		http.WithReadySignal(func(addr net.Addr) {
			logger.Info().Stringer("addr", addr).Msg("Exporter HTTP server connection ready")
			close(ready)
		}),
	)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to setup exporter http server")
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				if err = svr.ListenAndServe(); err != nil {
					logger.Error().Err(err).Msg("Exporter http server exited prematurely")
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
					Msg("Failed to stop exporter http server gracefully")
				return err
			}
			logger.Info().Msg("Exporter http server stopped successfully")
			return nil
		},
	})

	return &Exporter{}, nil
}

var Module = fx.Module("exporter",
	fx.Provide(NewExporter),
)
