package udpserver_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/netip"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/pkg/udp/udpserver"
)

func TestServerListen(t *testing.T) {
	server, err := udpserver.New(
		"localhost:0", // 0 - listen an any available port
		udpserver.HandleFunc(func(_ context.Context, conn *net.UDPConn, addr netip.AddrPort, req []byte) {
			resp := req
			slices.Reverse(resp)
			conn.WriteToUDPAddrPort(resp, addr) //nolint: errcheck
		}),
	)
	defer server.Stop() //nolint: errcheck
	require.NoError(t, err)

	require.NoError(t, server.Listen())
	go func() {
		server.Serve() //nolint: errcheck
	}()

	conn, err := net.Dial("udp", server.LocalAddr().String())
	require.NoError(t, err)
	conn.Write([]byte("hello world")) //nolint: errcheck
	// read back the reversed string
	buf := make([]byte, 16)
	n, _ := conn.Read(buf)
	assert.Equal(t, 11, n)
	assert.Equal(t, "dlrow olleh", string(buf[:n]))
}

func TestServerStopWaitsForHandlers(t *testing.T) {
	started := make(chan struct{})
	var finished atomic.Bool

	server, err := udpserver.New(
		"localhost:0", // 0 - listen an any available port
		udpserver.HandleFunc(func(ctx context.Context, _ *net.UDPConn, _ netip.AddrPort, _ []byte) {
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

	conn, err := net.Dial("udp", server.LocalAddr().String())
	require.NoError(t, err)
	conn.Write([]byte("hello world")) //nolint: errcheck
	// wait for the handler to pick up the request
	<-started

	require.NoError(t, server.Stop())
	assert.True(t, finished.Load())
}

func TestServerServeReturnsNilOnStop(t *testing.T) {
	// stopping races with the reader, so repeat it to make the race likely to show up
	for range 100 {
		server, err := udpserver.New(
			"localhost:0", // 0 - listen an any available port
			udpserver.HandleFunc(func(context.Context, *net.UDPConn, netip.AddrPort, []byte) {}),
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

	server, err := udpserver.New(
		"localhost:0", // 0 - listen an any available port
		udpserver.HandleFunc(func(_ context.Context, conn *net.UDPConn, addr netip.AddrPort, req []byte) {
			if string(req) == "panic" {
				panic("handler panic")
			}
			conn.WriteToUDPAddrPort(req, addr) //nolint: errcheck
		}),
	)
	require.NoError(t, err)
	defer server.Stop() //nolint: errcheck

	require.NoError(t, server.Listen())
	go func() {
		server.Serve() //nolint: errcheck
	}()

	conn, err := net.Dial("udp", server.LocalAddr().String())
	require.NoError(t, err)
	conn.SetReadDeadline(time.Now().Add(time.Second)) //nolint: errcheck
	conn.Write([]byte("panic"))                       //nolint: errcheck
	// the server is expected to keep serving after the panic
	conn.Write([]byte("hello")) //nolint: errcheck
	buf := make([]byte, 16)
	n, err := conn.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(buf[:n]))
}

func TestServerDropsDatagramsOverHandlerLimit(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{}, 10)
	unblock := make(chan struct{})
	release := sync.OnceFunc(func() {
		close(unblock)
	})
	dropped := make(chan struct{}, 10)

	server, err := udpserver.New(
		"localhost:0", // 0 - listen an any available port
		udpserver.HandleFunc(func(context.Context, *net.UDPConn, netip.AddrPort, []byte) {
			calls.Add(1)
			started <- struct{}{}
			<-unblock
		}),
		udpserver.WithMaxHandlers(1),
		udpserver.WithDropCallback(func() {
			dropped <- struct{}{}
		}),
	)
	require.NoError(t, err)
	require.NoError(t, server.Listen())
	go func() {
		server.Serve() //nolint: errcheck
	}()
	defer server.Stop() //nolint: errcheck
	// unblock the handlers before stopping, even if the test fails early
	defer release()

	conn, err := net.Dial("udp", server.LocalAddr().String())
	require.NoError(t, err)

	// the first datagram takes the only handler slot
	conn.Write([]byte("first")) //nolint: errcheck
	<-started

	// the second one is dropped while the slot is busy
	conn.Write([]byte("second")) //nolint: errcheck
	select {
	case <-dropped:
	case <-time.After(time.Second):
		require.Fail(t, "expected the datagram to be dropped")
	}
	assert.Equal(t, int32(1), calls.Load())

	// the slot is freed once the handler returns, which happens shortly after it's unblocked
	release()
	deadline := time.After(time.Second)
	for {
		conn.Write([]byte("third")) //nolint: errcheck
		select {
		case <-started:
			return
		case <-time.After(time.Millisecond * 10):
		case <-deadline:
			require.Fail(t, "expected the datagram to be handled once the slot is free")
		}
	}
}

func TestServerPassesIPv4ClientAddresses(t *testing.T) {
	received := make(chan netip.AddrPort, 1)

	server, err := udpserver.New(
		":0", // all interfaces, which makes the socket dual-stack where supported
		udpserver.HandleFunc(func(_ context.Context, _ *net.UDPConn, addr netip.AddrPort, _ []byte) {
			received <- addr
		}),
	)
	require.NoError(t, err)
	require.NoError(t, server.Listen())
	go func() {
		server.Serve() //nolint: errcheck
	}()
	defer server.Stop() //nolint: errcheck

	conn, err := net.Dial("udp4", fmt.Sprintf("127.0.0.1:%d", server.LocalAddr().Port))
	require.NoError(t, err)
	conn.Write([]byte("hello")) //nolint: errcheck

	select {
	case addr := <-received:
		assert.True(t, addr.Addr().Is4(), "expected a plain IPv4 address, got %s", addr)
		assert.Equal(t, "127.0.0.1", addr.Addr().String())
	case <-time.After(time.Second):
		require.Fail(t, "expected the datagram to be handled")
	}
}

func TestNewPanicsOnNilHandler(t *testing.T) {
	assert.PanicsWithValue(t, "udpserver: nil handler", func() {
		udpserver.New("localhost:0", nil) //nolint: errcheck
	})
}
