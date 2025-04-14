package gs1

import (
	"bytes"
	"context"
	"net"

	"github.com/sergeii/swat4master/pkg/udp/udpserver"
)

func ServerFactory(
	handler func(ctx context.Context, conn *net.UDPConn, addr *net.UDPAddr, req []byte),
) (*udpserver.Server, func()) {
	ready := make(chan struct{})
	server, _ := udpserver.New(
		"localhost:0", // 0 - listen an any available port
		udpserver.HandleFunc(handler),
		udpserver.WithReadySignal(func() {
			ready <- struct{}{}
		}),
	)
	go func() {
		server.Listen() // nolint: errcheck
	}()
	<-ready
	return server, func() {
		server.Stop() // nolint: errcheck
	}
}

func PrepareGS1Server(responses chan []byte) (*udpserver.Server, func()) {
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
