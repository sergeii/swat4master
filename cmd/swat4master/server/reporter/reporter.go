package reporter

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	api "github.com/sergeii/swat4master/internal/api/master/reporter"
	"github.com/sergeii/swat4master/internal/server"
	"github.com/sergeii/swat4master/pkg/logging"
	udp "github.com/sergeii/swat4master/pkg/udp/server"
)

func newHandler(mrs *api.MasterReporterService) udp.HandlerFunc {
	return func(ctx context.Context, conn *net.UDPConn, addr *net.UDPAddr, req []byte) {
		log.Debug().
			Str("type", fmt.Sprintf("0x%02x", req[0])).Stringer("src", addr).Int("len", len(req)).
			Msg("Received request")
		if e := log.Debug(); e.Enabled() {
			logging.Hexdump(req) // nolint: errcheck
		}
		resp, err := mrs.DispatchRequest(ctx, req, addr)
		if err != nil {
			log.Error().
				Err(err).Stringer("src", addr).Int("len", len(req)).
				Msg("Failed to dispatch request")
			return
		}
		if resp != nil {
			log.Debug().Stringer("dst", addr).Int("len", len(resp)).Msg("Sending response")
			if e := log.Debug(); e.Enabled() {
				logging.Hexdump(resp) // nolint: errcheck
			}
			if _, err := conn.WriteToUDP(resp, addr); err != nil {
				log.Error().
					Err(err).Stringer("dst", addr).Int("len", len(resp)).
					Msg("Failed to send response")
			}
		}
	}
}

func Run(ctx context.Context, wg *sync.WaitGroup, cfg *config.Config, fail chan struct{}, repo server.Repository) {
	defer wg.Done()
	defer func() {
		fail <- struct{}{}
	}()
	mrs, err := api.NewService(api.WithServerRepository(repo))
	if err != nil {
		log.Error().Err(err).Msg("Failed to init reporter service")
		return
	}
	svr, err := udp.New(
		cfg.ReporterListenAddr,
		udp.WithBufferSize(cfg.ReporterBufferSize),
		udp.WithHandler(newHandler(mrs)),
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to setup UDP server for reporter service")
		return
	}
	if err := svr.Listen(ctx); err != nil {
		log.Error().Err(err).Msg("UDP reporter server exited prematurely")
		return
	}
}
