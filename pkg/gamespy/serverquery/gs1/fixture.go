package gs1

import (
	"bytes"
	"context"
	"net"

	udp "github.com/sergeii/swat4master/pkg/udp/server"
)

func ServerFactory(
	handler func(ctx context.Context, conn *net.UDPConn, addr *net.UDPAddr, req []byte),
) (*udp.Server, func()) {
	ready := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	server, _ := udp.New(
		"localhost:0", // 0 - listen an any available port
		udp.WithHandler(handler),
		udp.WithReadySignal(func() {
			ready <- struct{}{}
		}),
	)
	go func() {
		server.Listen(ctx) // nolint: errcheck
	}()
	<-ready
	return server, func() {
		defer server.Stop() // nolint: errcheck
		defer cancel()
	}
}

func PrepareGS1Server(responses chan []byte) (*udp.Server, func()) {
	return ServerFactory(
		func(ctx context.Context, conn *net.UDPConn, addr *net.UDPAddr, req []byte) {
			if !bytes.Equal([]byte("\\status\\"), req[:8]) {
				return
			}
			for {
				select {
				case resp := <-responses:
					conn.WriteToUDP(resp, addr) // nolint: errcheck
				case <-ctx.Done():
					return
				}
			}
		},
	)
}
