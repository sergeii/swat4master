package reporter

import (
	"context"
	"fmt"
	"net"

	"github.com/benbjohnson/clock"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/internal/services/master/reporting"
	"github.com/sergeii/swat4master/internal/services/monitoring"
	udp "github.com/sergeii/swat4master/pkg/udp/server"
)

type Reporter struct{}

type Handler struct {
	service *reporting.Service
	metrics *monitoring.MetricService
	clock   clock.Clock
	logger  *zerolog.Logger
}

func newHandler(
	mrs *reporting.Service,
	metrics *monitoring.MetricService,
	clock clock.Clock,
	logger *zerolog.Logger,
) *Handler {
	return &Handler{mrs, metrics, clock, logger}
}

func (h *Handler) Handle(
	ctx context.Context,
	conn *net.UDPConn,
	addr *net.UDPAddr,
	req []byte,
) {
	reqStarted := h.clock.Now()

	h.logger.Debug().
		Str("type", fmt.Sprintf("0x%02x", req[0])).Stringer("src", addr).Int("len", len(req)).
		Msg("Received request")

	h.metrics.ReporterReceived.Add(float64(len(req)))

	resp, reqType, err := h.service.DispatchRequest(ctx, req, addr)
	if err != nil {
		h.metrics.ReporterErrors.WithLabelValues(reqType.String()).Inc()
		h.logger.Error().
			Err(err).
			Stringer("src", addr).Stringer("type", reqType).Int("len", len(req)).
			Msg("Failed to dispatch request")
		return
	}
	// responses are optional for some request types, such as keepalive requests
	if resp != nil {
		h.logger.Debug().
			Stringer("dst", addr).Int("len", len(resp)).
			Msg("Sending response")
		if _, err := conn.WriteToUDP(resp, addr); err != nil {
			h.logger.Error().
				Err(err).Stringer("dst", addr).Int("len", len(resp)).
				Msg("Failed to send response")
		} else {
			// only account the size of the response if we were able to actually push it through the socket
			h.metrics.ReporterSent.Add(float64(len(resp)))
		}
	}
	h.metrics.ReporterRequests.WithLabelValues(reqType.String()).Inc()
	h.metrics.ReporterDurations.
		WithLabelValues(reqType.String()).
		Observe(h.clock.Since(reqStarted).Seconds())
}

func NewReporter(
	lc fx.Lifecycle,
	shutdowner fx.Shutdowner,
	handler *Handler,
	cfg config.Config,
	logger *zerolog.Logger,
) (*Reporter, error) {
	ready := make(chan struct{})

	svr, err := udp.New(
		cfg.ReporterListenAddr,
		handler,
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
		newHandler,
	),
	fx.Provide(
		fx.Private,
		reporting.NewService,
	),
	fx.Provide(NewReporter),
)
