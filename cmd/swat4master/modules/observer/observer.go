package observer

import (
	"context"

	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/internal/metrics"
	"github.com/sergeii/swat4master/internal/metrics/observers/instanceobserver"
	"github.com/sergeii/swat4master/internal/metrics/observers/probeobserver"
	"github.com/sergeii/swat4master/internal/metrics/observers/serverobserver"
)

type Observer struct{}

func Run(
	stop chan struct{},
	stopped chan struct{},
	clock clockwork.Clock,
	logger *zerolog.Logger,
	collector *metrics.Collector,
	cfg config.Config,
) {
	ticker := clock.NewTicker(cfg.MetricObserverInterval)
	tickerCh := ticker.Chan()
	defer ticker.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Info().
		Dur("interval", cfg.MetricObserverInterval).
		Msg("Starting observer")

	for {
		select {
		case <-stop:
			logger.Info().Msg("Stopping observer")
			close(stopped)
			return
		case <-tickerCh:
			collector.Observe(ctx)
		}
	}
}

func NewObserver(
	lc fx.Lifecycle,
	cfg config.Config,
	clock clockwork.Clock,
	collector *metrics.Collector,
	logger *zerolog.Logger,
) *Observer {
	stopped := make(chan struct{})
	stop := make(chan struct{})

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go Run(stop, stopped, clock, logger, collector, cfg) // nolint: contextcheck
			return nil
		},
		OnStop: func(context.Context) error {
			close(stop)
			<-stopped
			return nil
		},
	})

	return &Observer{}
}

type Opts struct {
	fx.Out

	ServerObserverOpts serverobserver.Opts
}

func provideObserverConfigs(cfg config.Config) Opts {
	return Opts{
		ServerObserverOpts: serverobserver.Opts{
			ServerLiveness: cfg.BrowserServerLiveness,
		},
	}
}

var Module = fx.Module("observer",
	fx.Provide(provideObserverConfigs),
	fx.Invoke(
		serverobserver.New,
		instanceobserver.New,
		probeobserver.New,
	),
	fx.Provide(NewObserver),
)
