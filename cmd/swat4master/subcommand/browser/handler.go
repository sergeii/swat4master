package browser

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/rs/zerolog/log"

	browserapi "github.com/sergeii/swat4master/internal/api/master/browser"
	"github.com/sergeii/swat4master/internal/api/monitoring"
	"github.com/sergeii/swat4master/pkg/logutils"
)

type RequestHandler struct {
	api     *browserapi.MasterBrowserService
	metrics *monitoring.MetricService
}

func NewRequestHandler(
	mbs *browserapi.MasterBrowserService,
	metrics *monitoring.MetricService,
) *RequestHandler {
	return &RequestHandler{mbs, metrics}
}

func (h *RequestHandler) Handle(ctx context.Context, conn *net.TCPConn) {
	defer conn.Close()
	reqStarted := time.Now()

	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read server browser request from TCP socket")
		return
	}

	req := buf[:n]
	log.Debug().
		Int("len", len(req)).Stringer("src", conn.RemoteAddr()).
		Msg("Received server browser request")
	if e := log.Debug(); e.Enabled() {
		logutils.Hexdump(req) // nolint: errcheck
	}

	h.metrics.BrowserReceived.Add(float64(len(req)))

	remoteAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		panic(fmt.Sprintf("%v is not a *TCPAddr", conn.RemoteAddr()))
	}

	resp, err := h.api.HandleRequest(ctx, remoteAddr, req)
	if err != nil {
		h.metrics.BrowserErrors.Inc()
		log.Warn().
			Err(err).
			Int("len", len(req)).Stringer("src", conn.RemoteAddr()).
			Msg("Failed to handle browser request")
		return
	}
	if resp != nil {
		log.Debug().
			Int("len", len(resp)).Stringer("dst", conn.RemoteAddr()).
			Msg("Sending server browser response")
		if _, err := conn.Write(resp); err != nil {
			log.Warn().
				Err(err).
				Int("len", len(resp)).Stringer("dst", conn.RemoteAddr()).
				Msg("Failed to send server browser response")
		} else {
			h.metrics.BrowserSent.Add(float64(len(resp)))
		}
	}
	h.metrics.BrowserRequests.Inc()
	h.metrics.BrowserDurations.Observe(time.Since(reqStarted).Seconds())
}
