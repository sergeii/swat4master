package reporter

import (
	"context"
	"fmt"
	"net"
	"time"

	reporterapi "github.com/sergeii/swat4master/internal/services/master/reporting"
	"github.com/sergeii/swat4master/internal/services/monitoring"

	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/pkg/logutils"
)

type RequestHandler struct {
	api     *reporterapi.MasterReporterService
	metrics *monitoring.MetricService
}

func NewRequestHandler(
	mrs *reporterapi.MasterReporterService,
	metrics *monitoring.MetricService,
) *RequestHandler {
	return &RequestHandler{mrs, metrics}
}

func (h *RequestHandler) Handle(ctx context.Context, conn *net.UDPConn, addr *net.UDPAddr, req []byte) {
	reqStarted := time.Now()

	log.Debug().
		Str("type", fmt.Sprintf("0x%02x", req[0])).Stringer("src", addr).Int("len", len(req)).
		Msg("Received request")
	if e := log.Debug(); e.Enabled() {
		logutils.Hexdump(req) // nolint: errcheck
	}

	h.metrics.ReporterReceived.Add(float64(len(req)))

	resp, reqType, err := h.api.DispatchRequest(ctx, req, addr)
	if err != nil {
		h.metrics.ReporterErrors.WithLabelValues(reqType.String()).Inc()
		log.Error().
			Err(err).
			Stringer("src", addr).Stringer("type", reqType).Int("len", len(req)).
			Msg("Failed to dispatch request")
		return
	}
	// responses are optional for some request types, such as keepalive requests
	if resp != nil {
		log.Debug().
			Stringer("dst", addr).Int("len", len(resp)).
			Msg("Sending response")
		if e := log.Debug(); e.Enabled() {
			logutils.Hexdump(resp) // nolint: errcheck
		}
		if _, err := conn.WriteToUDP(resp, addr); err != nil {
			log.Error().
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
		Observe(time.Since(reqStarted).Seconds())
}
