package browser

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"

	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/usecases/listservers"
	"github.com/sergeii/swat4master/internal/metrics"
	"github.com/sergeii/swat4master/pkg/gamespy/browsing"
	"github.com/sergeii/swat4master/pkg/gamespy/browsing/query"
	"github.com/sergeii/swat4master/pkg/gamespy/crypt"
	"github.com/sergeii/swat4master/pkg/gamespy/serverquery/params"
)

const GameEncKey = "tG3j8c"

type HandlerOpts struct {
	Liveness time.Duration
}

type Handler struct {
	metrics *metrics.Collector
	logger  *zerolog.Logger
	clock   clockwork.Clock
	uc      listservers.UseCase
	opts    HandlerOpts
	gameKey [6]byte
}

func NewHandler(
	metrics *metrics.Collector,
	logger *zerolog.Logger,
	clock clockwork.Clock,
	uc listservers.UseCase,
	opts HandlerOpts,
) Handler {
	handler := Handler{
		metrics: metrics,
		logger:  logger,
		clock:   clock,
		uc:      uc,
		opts:    opts,
	}
	copy(handler.gameKey[:], GameEncKey)
	return handler
}

func (h Handler) Handle(ctx context.Context, conn *net.TCPConn) {
	defer conn.Close()
	reqStarted := h.clock.Now()

	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		h.logger.Warn().Err(err).Msg("Failed to read server browser request from TCP socket")
		return
	}

	payload := buf[:n]
	h.logger.Debug().
		Int("len", len(payload)).Stringer("src", conn.RemoteAddr()).
		Msg("Received server browser request")

	remoteAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		panic(fmt.Sprintf("%v is not a *TCPAddr", conn.RemoteAddr()))
	}

	h.metrics.BrowserReceived.Add(float64(len(payload)))

	resp, err := h.process(ctx, remoteAddr, payload)
	if err != nil {
		h.metrics.BrowserErrors.Inc()
		h.logger.Warn().
			Err(err).
			Int("len", len(payload)).Stringer("src", conn.RemoteAddr()).
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

func (h Handler) process(
	ctx context.Context,
	remoteAddr *net.TCPAddr,
	payload []byte,
) ([]byte, error) {
	var q query.Query

	req, err := browsing.NewRequest(payload)
	if err != nil {
		return nil, err
	}

	// unless any browser query filters are skipped, filter out the available that don't match those filters
	if req.Filters != "" {
		q, err = query.NewFromString(req.Filters)
		if err != nil {
			h.logger.Warn().
				Err(err).
				Stringer("src", remoteAddr).Str("filters", req.Filters).
				Msg("Unable to apply filters")
		}
	}

	ucRequest := listservers.NewRequest(q, h.opts.Liveness, ds.Master)

	servers, err := h.uc.Execute(ctx, ucRequest)
	if err != nil {
		return nil, err
	}

	resp := h.packServers(servers, remoteAddr, req.Fields)
	h.logger.Debug().
		Int("count", len(servers)).Stringer("src", remoteAddr).Str("filters", req.Filters).
		Msg("Packed available")

	return crypt.Encrypt(h.gameKey, req.Challenge, resp), nil
}

func (h Handler) packServers(servers []server.Server, addr *net.TCPAddr, fields []string) []byte {
	payload := make([]byte, 6, 26)
	// the first 6 bytes are the client's IP and port
	copy(payload[:4], addr.IP.To4())
	binary.BigEndian.PutUint16(payload[4:6], uint16(addr.Port)) // nolint:gosec
	// make sure the fields slice is not bigger than 255 elements,
	// so its length can be encoded in a single byte
	if len(fields) > 255 {
		fields = fields[:255]
	}
	payload = append(payload, uint8(len(fields)), 0x00) // nolint:gosec
	// declare the fields
	for _, field := range fields {
		payload = append(payload, []byte(field)...)
		payload = append(payload, 0x00, 0x00)
	}
	for _, svr := range servers {
		svrInfo := svr.Info
		svrParams, err := params.Marshal(&svrInfo)
		if err != nil {
			h.logger.Warn().
				Err(err).Stringer("addr", svr.Addr).
				Msg("Unable to obtain params for server")
			continue
		}
		// first 7 bytes is the server address
		serverAddr := make([]byte, 7)
		serverAddr[0] = 0x51
		copy(serverAddr[1:5], svr.Addr.GetIP())
		binary.BigEndian.PutUint16(serverAddr[5:7], uint16(svr.QueryPort)) // nolint:gosec
		payload = append(payload, serverAddr...)
		// insert field values' in the same order as in the field declaration
		for _, field := range fields {
			payload = append(payload, 0xff)
			val, exists := svrParams[field]
			if !exists {
				h.logger.Warn().
					Str("field", field).Stringer("server", svr.Addr).Stringer("src", addr).
					Msg("Requested field is missing")
			} else {
				payload = append(payload, []byte(val)...)
			}
			payload = append(payload, 0x00)
		}
	}
	return append(payload, 0x00, 0xff, 0xff, 0xff, 0xff)
}
