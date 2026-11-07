package udpserver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/netip"
	"runtime/debug"

	"golang.org/x/sync/errgroup"
)

const (
	defaultBufferSize = 1024
)

type Option func(*Server)

type Server struct {
	addr       *net.UDPAddr
	conn       *net.UDPConn
	bufferSize int
	handler    Handler
	closer     chan struct{}  // closed by Stop to stop reading and cancel the handlers
	done       chan struct{}  // closed once Serve has returned
	handlers   errgroup.Group // runs the handlers, at most the configured limit at once
	onDrop     func()
}

func WithBufferSize(sz int) Option {
	return func(s *Server) {
		s.bufferSize = sz
	}
}

// WithMaxHandlers limits the number of handlers running at once, 0 means no limit.
// Datagrams that arrive while all handlers are busy are dropped.
func WithMaxHandlers(n int) Option {
	return func(s *Server) {
		if n > 0 {
			s.handlers.SetLimit(n)
		}
	}
}

func WithDropCallback(callback func()) Option {
	return func(s *Server) {
		s.onDrop = callback
	}
}

func New(addr string, handler Handler, opts ...Option) (*Server, error) {
	if handler == nil {
		panic("udpserver: nil handler")
	}
	udpAddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return nil, err
	}
	server := &Server{
		addr:    udpAddr,
		handler: handler,
		closer:  make(chan struct{}),
		done:    make(chan struct{}),
		// set defaults
		bufferSize: defaultBufferSize,
		onDrop:     func() {},
	}
	for _, opt := range opts {
		opt(server)
	}
	return server, nil
}

func (s *Server) Listen() error {
	conn, err := net.ListenUDP("udp", s.addr)
	if err != nil {
		return err
	}
	s.conn = conn
	return nil
}

// Serve blocks until Stop is called and then returns nil, or until reading fails.
func (s *Server) Serve() error {
	defer close(s.done)

	ctx, cancel := s.stopContext()
	defer cancel()

	buffer := make([]byte, s.bufferSize)
	for {
		n, raddr, err := s.conn.ReadFromUDPAddrPort(buffer)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil // closed by Stop
			}
			return err
		}
		// on a dual-stack socket, IPv4 clients arrive as IPv4-mapped IPv6 addresses
		raddr = netip.AddrPortFrom(raddr.Addr().Unmap(), raddr.Port())
		s.dispatch(ctx, raddr, buffer[:n])
	}
}

func (s *Server) dispatch(ctx context.Context, raddr netip.AddrPort, data []byte) {
	// skip empty datagrams, so that handlers can rely on a non-empty payload
	if len(data) == 0 {
		return
	}
	payload := bytes.Clone(data)
	started := s.handlers.TryGo(func() error {
		defer recoverHandler()
		s.handler.Handle(ctx, s.conn, raddr, payload)
		return nil
	})
	if !started {
		s.onDrop()
	}
}

// stopContext returns a context that is canceled once Stop is called.
func (s *Server) stopContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-s.closer:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

func recoverHandler() {
	if r := recover(); r != nil {
		log.Printf("udpserver: panic in handler: %v\n%s", r, debug.Stack())
	}
}

func (s *Server) LocalAddr() *net.UDPAddr {
	udpAddr, ok := s.conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		panic("address must be of type *UDPAddr")
	}
	return udpAddr
}

func (s *Server) LocalAddrPort() netip.AddrPort {
	return s.LocalAddr().AddrPort()
}

// Stop cancels the running handlers and waits for them to return.
// It must only be called once Serve has been started.
func (s *Server) Stop() error {
	close(s.closer)
	closeErr := s.conn.Close()
	// no more handlers can be started once Serve has returned
	<-s.done
	s.handlers.Wait() //nolint: errcheck // handlers never return errors
	if closeErr != nil {
		return fmt.Errorf("udp server: close connection %s: %w", s.addr, closeErr)
	}
	return nil
}
