package gs1

import (
	"bytes"
	"context"
	"net"
	"net/netip"

	"github.com/sergeii/swat4master/pkg/udp/udpserver"
)

func ServerFactory(
	handler func(ctx context.Context, conn *net.UDPConn, addr netip.AddrPort, req []byte),
) (*udpserver.Server, func()) {
	server, _ := udpserver.New(
		"localhost:0", // 0 - listen an any available port
		udpserver.HandleFunc(handler),
	)
	if err := server.Listen(); err != nil {
		panic(err)
	}
	go func() {
		server.Serve() //nolint: errcheck
	}()
	return server, func() {
		server.Stop() //nolint: errcheck
	}
}

func PrepareGS1Server(responses chan []byte) (*udpserver.Server, func()) {
	return ServerFactory(
		func(ctx context.Context, conn *net.UDPConn, addr netip.AddrPort, req []byte) {
			if !bytes.Equal([]byte("\\status\\"), req[:8]) {
				return
			}
			for {
				select {
				case resp := <-responses:
					conn.WriteToUDPAddrPort(resp, addr) //nolint: errcheck
				case <-ctx.Done():
					return
				}
			}
		},
	)
}
