package tcpserver_test

import (
	"context"
	"io"
	"log"
	"net"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/pkg/tcp/tcpserver"
)

func TestServerListen(t *testing.T) {
	server, err := tcpserver.New(
		"localhost:0", // 0 - listen an any available port
		tcpserver.HandleFunc(func(_ context.Context, conn *net.TCPConn) {
			defer conn.Close()
			buf := make([]byte, 1024)
			n, _ := conn.Read(buf)
			resp := buf[:n]
			slices.Reverse(resp)
			conn.Write(resp) //nolint: errcheck
		}),
	)
	defer server.Stop() //nolint: errcheck
	require.NoError(t, err)

	require.NoError(t, server.Listen())
	go func() {
		server.Serve() //nolint: errcheck
	}()

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
	server, err := tcpserver.New(
		"localhost:0", // 0 - listen an any available port
		tcpserver.HandleFunc(func(_ context.Context, conn *net.TCPConn) {
			defer conn.Close()
			buf := make([]byte, 1024)
			n, _ := conn.Read(buf)
			// sleep for more than the timeout duration
			time.Sleep(time.Millisecond * 20)
			conn.Write(buf[:n]) //nolint: errcheck
		}),
		tcpserver.WithTimeout(time.Millisecond*10),
	)
	defer server.Stop() //nolint: errcheck
	require.NoError(t, err)

	require.NoError(t, server.Listen())
	go func() {
		server.Serve() //nolint: errcheck
	}()

	conn, _ := net.Dial("tcp", server.LocalAddr().String())
	n, _ := conn.Write([]byte("I'm a teapot"))
	require.Equal(t, 12, n)
	buf := make([]byte, 16)
	n, err = conn.Read(buf)
	assert.Equal(t, 0, n)
	assert.ErrorIs(t, err, io.EOF)
}

func TestServerStopWaitsForHandlers(t *testing.T) {
	started := make(chan struct{})
	var finished atomic.Bool

	server, err := tcpserver.New(
		"localhost:0", // 0 - listen an any available port
		tcpserver.HandleFunc(func(ctx context.Context, conn *net.TCPConn) {
			defer conn.Close()
			close(started)
			<-ctx.Done()
			// keep working for a while after being cancelled
			time.Sleep(time.Millisecond * 50)
			finished.Store(true)
		}),
	)
	require.NoError(t, err)

	require.NoError(t, server.Listen())
	go func() {
		server.Serve() //nolint: errcheck
	}()

	conn, err := net.Dial("tcp", server.LocalAddr().String())
	require.NoError(t, err)
	defer conn.Close()
	// wait for the handler to pick up the connection
	<-started

	require.NoError(t, server.Stop())
	assert.True(t, finished.Load())
}

func TestServerServeReturnsNilOnStop(t *testing.T) {
	// stopping races with the acceptor, so repeat it to make the race likely to show up
	for range 100 {
		server, err := tcpserver.New(
			"localhost:0", // 0 - listen an any available port
			tcpserver.HandleFunc(func(context.Context, *net.TCPConn) {}),
		)
		require.NoError(t, err)

		require.NoError(t, server.Listen())
		serveErr := make(chan error, 1)
		go func() {
			serveErr <- server.Serve()
		}()

		require.NoError(t, server.Stop())
		require.NoError(t, <-serveErr)
	}
}

func TestServerRecoversFromHandlerPanic(t *testing.T) {
	// the recovered panic is logged, keep it out of the test output
	log.SetOutput(io.Discard)
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
	})

	server, err := tcpserver.New(
		"localhost:0", // 0 - listen an any available port
		tcpserver.HandleFunc(func(_ context.Context, conn *net.TCPConn) {
			defer conn.Close()
			buf := make([]byte, 1024)
			n, _ := conn.Read(buf)
			if string(buf[:n]) == "panic" {
				panic("handler panic")
			}
			conn.Write(buf[:n]) //nolint: errcheck
		}),
	)
	require.NoError(t, err)
	defer server.Stop() //nolint: errcheck

	require.NoError(t, server.Listen())
	go func() {
		server.Serve() //nolint: errcheck
	}()

	panicConn, err := net.Dial("tcp", server.LocalAddr().String())
	require.NoError(t, err)
	defer panicConn.Close()
	_, err = panicConn.Write([]byte("panic"))
	require.NoError(t, err)
	// the panicking handler closes the connection without a response
	_, err = panicConn.Read(make([]byte, 16))
	require.ErrorIs(t, err, io.EOF)

	// the server is expected to keep serving after the panic
	conn, err := net.Dial("tcp", server.LocalAddr().String())
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.Write([]byte("hello"))
	require.NoError(t, err)
	buf := make([]byte, 16)
	n, err := conn.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(buf[:n]))
}

func TestServerQueuesConnectionsOverHandlerLimit(t *testing.T) {
	started := make(chan struct{}, 10)
	unblock := make(chan struct{})
	release := sync.OnceFunc(func() {
		close(unblock)
	})

	server, err := tcpserver.New(
		"localhost:0", // 0 - listen an any available port
		tcpserver.HandleFunc(func(_ context.Context, conn *net.TCPConn) {
			defer conn.Close()
			started <- struct{}{}
			<-unblock
		}),
		tcpserver.WithMaxHandlers(1),
	)
	require.NoError(t, err)
	require.NoError(t, server.Listen())
	go func() {
		server.Serve() //nolint: errcheck
	}()
	defer server.Stop() //nolint: errcheck
	// unblock the handlers before stopping, even if the test fails early
	defer release()

	// the first connection takes the only handler slot
	first, err := net.Dial("tcp", server.LocalAddr().String())
	require.NoError(t, err)
	defer first.Close()
	<-started

	// the second one connects, but waits to be accepted while the slot is busy
	second, err := net.Dial("tcp", server.LocalAddr().String())
	require.NoError(t, err)
	defer second.Close()
	select {
	case <-started:
		require.Fail(t, "expected the connection to wait for a free slot")
	case <-time.After(time.Millisecond * 100):
	}

	// it is handled once the first handler returns
	release()
	select {
	case <-started:
	case <-time.After(time.Second):
		require.Fail(t, "expected the connection to be handled once the slot is free")
	}
}

func TestNewPanicsOnNilHandler(t *testing.T) {
	assert.PanicsWithValue(t, "tcpserver: nil handler", func() {
		tcpserver.New("localhost:0", nil) //nolint: errcheck
	})
}
