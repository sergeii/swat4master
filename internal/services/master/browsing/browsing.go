package browsing

import (
	"context"
	"encoding/binary"
	"net"
	"time"

	"github.com/rs/zerolog"

	"github.com/sergeii/swat4master/internal/core/servers"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
	"github.com/sergeii/swat4master/internal/services/server"
	"github.com/sergeii/swat4master/pkg/gamespy/browsing"
	"github.com/sergeii/swat4master/pkg/gamespy/browsing/query"
	"github.com/sergeii/swat4master/pkg/gamespy/crypt"
	"github.com/sergeii/swat4master/pkg/gamespy/serverquery/params"
	"github.com/sergeii/swat4master/pkg/logutils"
)

const GameEncKey = "tG3j8c"

type ServiceOpts struct {
	Liveness time.Duration
}

type Service struct {
	serverService *server.Service
	logger        *zerolog.Logger
	opts          ServiceOpts
	gameKey       [6]byte
}

func NewService(
	ss *server.Service,
	logger *zerolog.Logger,
	opts ServiceOpts,
) *Service {
	mbs := &Service{
		serverService: ss,
		logger:        logger,
		opts:          opts,
	}
	copy(mbs.gameKey[:], GameEncKey)
	return mbs
}

func (s *Service) HandleRequest(ctx context.Context, addr *net.TCPAddr, payload []byte) ([]byte, error) {
	var q query.Query

	req, err := browsing.NewRequest(payload)
	if err != nil {
		return nil, err
	}

	// unless any browser query filters are skipped, filter out the available that dont match those filters
	if req.Filters != "" {
		q, err = query.NewFromString(req.Filters)
		if err != nil {
			s.logger.Warn().
				Err(err).
				Stringer("src", addr).Str("filters", req.Filters).
				Msg("Unable to apply filters")
		}
	}

	available, err := s.serverService.FilterRecent(ctx, s.opts.Liveness, q, ds.Master)
	if err != nil {
		return nil, err
	}

	resp := s.packServers(available, addr, req.Fields)
	s.logger.Debug().
		Int("count", len(available)).Stringer("src", addr).Str("filters", req.Filters).
		Msg("Packed available")
	if e := s.logger.Debug(); e.Enabled() {
		logutils.Hexdump(resp) // nolint: errcheck
	}

	return crypt.Encrypt(s.gameKey, req.Challenge, resp), nil
}

func (s *Service) packServers(servers []servers.Server, addr *net.TCPAddr, fields []string) []byte {
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
		svrDetails := svr.GetInfo()
		svrParams, err := params.Marshal(&svrDetails)
		if err != nil {
			s.logger.Warn().
				Err(err).Stringer("addr", svr.GetAddr()).
				Msg("Unable to obtain params for server")
			continue
		}
		// first 7 bytes is the server address
		serverAddr := make([]byte, 7)
		serverAddr[0] = 0x51
		copy(serverAddr[1:5], svr.GetIP())
		binary.BigEndian.PutUint16(serverAddr[5:7], uint16(svr.GetQueryPort()))
		payload = append(payload, serverAddr...)
		// insert field values' in the same order as in the field declaration
		for _, field := range fields {
			payload = append(payload, 0xff)
			val, exists := svrParams[field]
			if !exists {
				s.logger.Warn().
					Str("field", field).Stringer("server", svr.GetAddr()).Stringer("src", addr).
					Msg("Requested field is missing")
			} else {
				payload = append(payload, []byte(val)...)
			}
			payload = append(payload, 0x00)
		}
	}
	return append(payload, 0x00, 0xff, 0xff, 0xff, 0xff)
}
