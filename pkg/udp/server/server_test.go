package server_test

import (
	"context"
	"net"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	udp "github.com/sergeii/swat4master/pkg/udp/server"
)

func TestServerListen(t *testing.T) {
	ready := make(chan struct{})

	server, err := udp.New(
		"localhost:0", // 0 - listen an any available port
		udp.HandleFunc(func(_ context.Context, conn *net.UDPConn, addr *net.UDPAddr, req []byte) {
			resp := req
			slices.Reverse(resp)
			conn.WriteToUDP(resp, addr) // nolint: errcheck
		}),
		udp.WithReadySignal(func() {
			close(ready)
		}),
	)
	defer server.Stop() // nolint: errcheck
	require.NoError(t, err)

	go func() {
		server.Listen() // nolint: errcheck
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
