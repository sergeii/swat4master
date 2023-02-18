package browser

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/sergeii/swat4master/pkg/logutils"
)

func (h *Handler) Handle(ctx context.Context, conn *net.TCPConn) {
	defer conn.Close()
	reqStarted := time.Now()

	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		h.logger.Warn().Err(err).Msg("Failed to read server browser request from TCP socket")
		return
	}

	req := buf[:n]
	h.logger.Debug().
		Int("len", len(req)).Stringer("src", conn.RemoteAddr()).
		Msg("Received server browser request")
	if e := h.logger.Debug(); e.Enabled() {
		logutils.Hexdump(req) // nolint: errcheck
	}

	h.metrics.BrowserReceived.Add(float64(len(req)))

	remoteAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		panic(fmt.Sprintf("%v is not a *TCPAddr", conn.RemoteAddr()))
	}

	resp, err := h.service.HandleRequest(ctx, remoteAddr, req)
	if err != nil {
		h.metrics.BrowserErrors.Inc()
		h.logger.Warn().
			Err(err).
			Int("len", len(req)).Stringer("src", conn.RemoteAddr()).
			Msg("Failed to handle browser request")
		return
	}
	if resp != nil {
		h.logger.Debug().
			Int("len", len(resp)).Stringer("dst", conn.RemoteAddr()).
			Msg("Sending server browser response")
		if _, err := conn.Write(resp); err != nil {
			h.logger.Warn().
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
