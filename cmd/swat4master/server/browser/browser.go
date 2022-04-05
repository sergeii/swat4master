package browser

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	api "github.com/sergeii/swat4master/internal/api/master/browser"
	"github.com/sergeii/swat4master/internal/server"
	"github.com/sergeii/swat4master/pkg/logging"
	tcp "github.com/sergeii/swat4master/pkg/tcp/server"
)

func newHandler(mbs *api.MasterBrowserService) tcp.HandlerFunc {
	return func(ctx context.Context, conn *net.TCPConn) {
		defer conn.Close()
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
			logging.Hexdump(req) // nolint: errcheck
		}

		remoteAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
		if !ok {
			panic(fmt.Sprintf("%v is not a *TCPAddr", conn.RemoteAddr()))
		}

		resp, err := mbs.HandleRequest(ctx, remoteAddr, req)
		if err != nil {
			log.Warn().
				Err(err).
				Int("len", len(req)).Stringer("src", conn.RemoteAddr()).
				Msg("Failed to handle browser request")
			return
		} else if resp != nil {
			log.Debug().
				Int("len", len(resp)).Stringer("dst", conn.RemoteAddr()).
				Msg("Sending server browser response")
			if _, err := conn.Write(resp); err != nil {
				log.Warn().
					Err(err).
					Int("len", len(resp)).Stringer("dst", conn.RemoteAddr()).
					Msg("Failed to send server browser response")
			}
		}
	}
}

func Run(ctx context.Context, wg *sync.WaitGroup, cfg *config.Config, fail chan struct{}, repo server.Repository) {
	defer wg.Done()
	defer func() {
		fail <- struct{}{}
	}()
	mbs, err := api.NewService(
		api.WithServerRepository(repo),
		api.WithLivenessDuration(cfg.BrowserServerLiveness),
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to init browser service")
		return
	}
	svr, err := tcp.New(
		cfg.BrowserListenAddr,
		tcp.WithHandler(newHandler(mbs)),
		tcp.WithTimeout(cfg.BrowserClientTimeout),
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to setup TCP server for browser service")
		return
	}
	if err := svr.Listen(ctx); err != nil {
		log.Error().Err(err).Msg("TCP browser server exited prematurely")
		return
	}
}
