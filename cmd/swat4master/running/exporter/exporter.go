package exporter

import (
	"context"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/running"
	"github.com/sergeii/swat4master/internal/application"
	http "github.com/sergeii/swat4master/pkg/http/server"
)

func Run(ctx context.Context, runner *running.Runner, app *application.App, cfg config.Config) {
	defer runner.Quit(ctx.Done())
	registry := app.MetricService.GetRegistry()
	svr, err := http.New(
		cfg.ExporterListenAddr,
		http.WithShutdownTimeout(cfg.HTTPShutdownTimeout),
		http.WithReadTimeout(cfg.HTTPReadTimeout),
		http.WithWriteTimeout(cfg.HTTPWriteTimeout),
		http.WithHandler(promhttp.InstrumentMetricHandler(
			registry,
			promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
		)),
		http.WithReadySignal(runner.Ready),
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to setup exporter server")
		return
	}
	if err = svr.ListenAndServe(ctx); err != nil {
		log.Error().Err(err).Msg("Exporter server exited prematurely")
		return
	}
}
