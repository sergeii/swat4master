package browser

import (
	"context"
	"encoding/binary"
	"net"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/internal/aggregate"
	"github.com/sergeii/swat4master/internal/server"
	"github.com/sergeii/swat4master/pkg/gamespy/browsing"
	"github.com/sergeii/swat4master/pkg/gamespy/enctypex"
	"github.com/sergeii/swat4master/pkg/gamespy/query"
	"github.com/sergeii/swat4master/pkg/logging"
)

const GameEncKey = "tG3j8c"

type MasterBrowserService struct {
	servers  server.Repository
	liveness time.Duration
	gameKey  [6]byte
}

type Option func(mbs *MasterBrowserService) error

func NewService(cfgs ...Option) (*MasterBrowserService, error) {
	mrs := &MasterBrowserService{}
	copy(mrs.gameKey[:], GameEncKey)
	for _, cfg := range cfgs {
		if err := cfg(mrs); err != nil {
			return nil, err
		}
	}
	return mrs, nil
}

func WithServerRepository(repo server.Repository) Option {
	return func(mbs *MasterBrowserService) error {
		mbs.servers = repo
		return nil
	}
}

func WithLivenessDuration(dur time.Duration) Option {
	return func(mbs *MasterBrowserService) error {
		mbs.liveness = dur
		return nil
	}
}

func (mbs *MasterBrowserService) HandleRequest(ctx context.Context, addr *net.TCPAddr, payload []byte) ([]byte, error) {
	var cntTotal int

	req, err := browsing.NewRequest(payload)
	if err != nil {
		return nil, err
	}

	servers, err := mbs.servers.GetReportedSince(time.Now().Add(-mbs.liveness))
	if err != nil {
		return nil, err
	}
	cntTotal = len(servers)

	// unless any browser query filters are skipped, filter out the servers that dont match those filters
	if req.Filters != "" {
		q, err := query.New(req.Filters)
		if err != nil {
			log.Warn().
				Err(err).
				Stringer("src", addr).Str("filters", req.Filters).
				Msg("Unable to apply filters")
		} else {
			servers = filterServers(servers, q)
		}
	}

	if cntTotal != len(servers) {
		log.Debug().
			Str("filters", req.Filters).
			Int("total", cntTotal).
			Int("count", len(servers)).
			Stringer("src", addr).
			Msg("Successfully applied filters")
	} else if req.Filters != "" {
		log.Debug().
			Str("filters", req.Filters).Int("total", cntTotal).Stringer("src", addr).
			Msg("Applied filters with no change")
	}

	resp := packServers(servers, addr, req.Fields)
	log.Debug().
		Int("count", len(servers)).Stringer("src", addr).Str("filters", req.Filters).
		Msg("Packed servers")
	if e := log.Debug(); e.Enabled() {
		logging.Hexdump(resp) // nolint: errcheck
	}

	return enctypex.Encrypt(mbs.gameKey, req.Challenge, resp), nil
}

func filterServers(unfiltered []*aggregate.GameServer, q *query.Query) []*aggregate.GameServer {
	filtered := make([]*aggregate.GameServer, 0, len(unfiltered))
	for _, svr := range unfiltered {
		if q.Match(svr.GetReportedParams()) {
			filtered = append(filtered, svr)
		}
	}
	return filtered
}

func packServers(servers []*aggregate.GameServer, addr *net.TCPAddr, fields []string) []byte {
	payload := make([]byte, 6, 26)
	// the first 6 bytes are the client's IP and port
	copy(payload[:4], addr.IP.To4())
	binary.BigEndian.PutUint16(payload[4:6], uint16(addr.Port))
	// the number of fields that follow
	payload = append(payload, uint8(len(fields)), 0x00)
	// declare the fields
	for _, field := range fields {
		payload = append(payload, []byte(field)...)
		payload = append(payload, 0x00, 0x00)
	}
	for _, svr := range servers {
		serverAddr := make([]byte, 7)
		serverAddr[0] = 0x51
		copy(serverAddr[1:5], svr.GetIP())
		binary.BigEndian.PutUint16(serverAddr[5:7], uint16(svr.GetQueryPort()))
		payload = append(payload, serverAddr...)
		// insert field values' in the same order as in the field declaration
		svrParams := svr.GetReportedParams()
		for _, field := range fields {
			payload = append(payload, 0xff)
			val, exists := svrParams[field]
			if !exists {
				log.Warn().
					Str("field", field).Str("server", svr.GetAddr()).Stringer("src", addr).
					Msg("Requested field is missing")
			} else {
				payload = append(payload, []byte(val)...)
			}
			payload = append(payload, 0x00)
		}
	}
	return append(payload, 0x00, 0xff, 0xff, 0xff, 0xff)
}
