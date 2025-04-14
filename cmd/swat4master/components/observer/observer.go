package observer

import (
	"context"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/commander"
	"github.com/sergeii/swat4master/internal/metrics"
	"github.com/sergeii/swat4master/internal/metrics/observers/instanceobserver"
	"github.com/sergeii/swat4master/internal/metrics/observers/probeobserver"
	"github.com/sergeii/swat4master/internal/metrics/observers/serverobserver"
	"github.com/sergeii/swat4master/internal/settings"
)

type Config struct {
	ObserveInterval time.Duration
}

type Component struct{}

func run(
	stop chan struct{},
	stopped chan struct{},
	clock clockwork.Clock,
	logger *zerolog.Logger,
	collector *metrics.Collector,
	cfg Config,
) {
	ticker := clock.NewTicker(cfg.ObserveInterval)
	tickerCh := ticker.Chan()
	defer ticker.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Info().Dur("interval", cfg.ObserveInterval).Msg("Starting observer")

	for {
		select {
		case <-stop:
			close(stopped)
			return
		case <-tickerCh:
			collector.Observe(ctx)
		}
	}
}

func New(
	lc fx.Lifecycle,
	cfg Config,
	clock clockwork.Clock,
	collector *metrics.Collector,
	logger *zerolog.Logger,
) *Component {
	stopped := make(chan struct{})
	stop := make(chan struct{})

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go run(stop, stopped, clock, logger, collector, cfg) // nolint: contextcheck
			return nil
		},
		OnStop: func(context.Context) error {
			close(stop)
			<-stopped
			logger.Info().Msg("Observer stopped")
			return nil
		},
	})

	return &Component{}
}

type command struct {
	MetricObserveInterval time.Duration `default:"1s" help:"Sets how often metrics are collected"`
}

func (c *command) Run(_ *commander.Globals, builder *application.Builder) error {
	app := builder.
		Add(
			fx.Supply(Config{
				ObserveInterval: c.MetricObserveInterval,
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
	Observer command `cmd:"" help:"Start observer"`
}

type Opts struct {
	fx.Out

	ServerObserverOpts serverobserver.Opts
}

func provideObserverConfigs(settings settings.Settings) Opts {
	return Opts{
		ServerObserverOpts: serverobserver.Opts{
			ServerLiveness: settings.ServerLiveness,
		},
	}
}

var Module = fx.Module("observer",
	fx.Provide(fx.Private, provideObserverConfigs),
	fx.Invoke(
		serverobserver.New,
		instanceobserver.New,
		probeobserver.New,
	),
	fx.Provide(New),
)
