package tcpserver_test

import (
	"context"
	"io"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/pkg/tcp/tcpserver"
)

func TestServerListen(t *testing.T) {
	ready := make(chan struct{})

	server, err := tcpserver.New(
		"localhost:0", // 0 - listen an any available port
		tcpserver.HandleFunc(func(_ context.Context, conn *net.TCPConn) {
			defer conn.Close()
			buf := make([]byte, 1024)
			n, _ := conn.Read(buf)
			resp := buf[:n]
			slices.Reverse(resp)
			conn.Write(resp) // nolint: errcheck
		}),
		tcpserver.WithReadySignal(func(net.Addr) {
			ready <- struct{}{}
		}),
	)
	defer server.Stop() // nolint: errcheck
	require.NoError(t, err)

	go func() {
		server.Listen() // nolint: errcheck
	}()
	// wait for the server to start
	<-ready

	conn, err := net.Dial("tcp", server.LocalAddr().String())
	require.NoError(t, err)
	_, err = conn.Write([]byte("I'm a teapot"))
	require.NoError(t, err)

	// read back the reversed string
	buf := make([]byte, 16)
	n, err := conn.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 12, n)
	assert.Equal(t, "topaet a m'I", string(buf[:n]))
}

func TestServerTimeout(t *testing.T) {
	ready := make(chan struct{})

	server, err := tcpserver.New(
		"localhost:0", // 0 - listen an any available port
		tcpserver.HandleFunc(func(_ context.Context, conn *net.TCPConn) {
			defer conn.Close()
			buf := make([]byte, 1024)
			n, _ := conn.Read(buf)
			// sleep for more than the timeout duration
			time.Sleep(time.Millisecond * 20)
			conn.Write(buf[:n]) // nolint: errcheck
		}),
		tcpserver.WithReadySignal(func(net.Addr) {
			ready <- struct{}{}
		}),
		tcpserver.WithTimeout(time.Millisecond*10),
	)
	defer server.Stop() // nolint: errcheck
	require.NoError(t, err)

	go func() {
		server.Listen() // nolint: errcheck
	}()
	// wait for the server to start
	<-ready

	conn, _ := net.Dial("tcp", server.LocalAddr().String())
	n, _ := conn.Write([]byte("I'm a teapot"))
	require.Equal(t, 12, n)
	buf := make([]byte, 16)
	n, err = conn.Read(buf)
	assert.Equal(t, 0, n)
	assert.ErrorIs(t, err, io.EOF)
}
