package server_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/pkg/slice"
	udp "github.com/sergeii/swat4master/pkg/udp/server"
)

func TestServerListen(t *testing.T) {
	ready := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server, err := udp.New(
		"localhost:0", // 0 - listen an any available port
		udp.WithHandler(func(ctx context.Context, conn *net.UDPConn, addr *net.UDPAddr, req []byte) {
			resp := slice.Reverse(req)
			conn.WriteToUDP(resp, addr) // nolint: errcheck
		}),
		udp.WithReadySignal(func() {
			ready <- struct{}{}
		}),
	)
	defer server.Stop() // nolint: errcheck
	require.NoError(t, err)

	go func() {
		server.Listen(ctx) // nolint: errcheck
	}()
	// wait for the server to start
	<-ready

	conn, err := net.Dial("udp", server.LocalAddr().String())
	require.NoError(t, err)
	conn.Write([]byte("hello world")) // nolint: errcheck
	// read back the reversed string
	buf := make([]byte, 16)
	n, _ := conn.Read(buf)
	assert.Equal(t, 11, n)
	assert.Equal(t, "dlrow olleh", string(buf[:n]))
}
