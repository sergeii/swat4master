package tcpserver

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"runtime/debug"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	defaultConnTimeout = time.Second
)

type Option func(*Server)

type Server struct {
	addr        *net.TCPAddr
	listener    *net.TCPListener
	handler     Handler
	closer      chan struct{}  // closed by Stop to stop accepting and cancel the handlers
	done        chan struct{}  // closed once Serve has returned
	handlers    errgroup.Group // runs the handlers, at most the configured limit at once
	connTimeout time.Duration
}

func WithTimeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.connTimeout = timeout
	}
}

// WithMaxHandlers limits the number of handlers running at once, 0 means no limit.
// While all handlers are busy, new connections wait to be handled.
func WithMaxHandlers(n int) Option {
	return func(s *Server) {
		if n > 0 {
			s.handlers.SetLimit(n)
		}
	}
}

func New(addr string, handler Handler, opts ...Option) (*Server, error) {
	if handler == nil {
		panic("tcpserver: nil handler")
	}
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}
	server := &Server{
		addr:    tcpAddr,
		handler: handler,
		closer:  make(chan struct{}),
		done:    make(chan struct{}),
		// set defaults
		connTimeout: defaultConnTimeout,
	}
	for _, opt := range opts {
		opt(server)
	}
	return server, nil
}

func (s *Server) Listen() error {
	listener, err := net.ListenTCP("tcp", s.addr)
	if err != nil {
		return err
	}
	s.listener = listener
	return nil
}

// Serve blocks until Stop is called and then returns nil, or until accepting fails.
func (s *Server) Serve() error {
	defer close(s.done)

	ctx, cancel := s.stopContext()
	defer cancel()

	for {
		conn, err := s.listener.AcceptTCP()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil // closed by Stop
			}
			return err
		}
		s.dispatch(ctx, conn)
	}
}

// dispatch waits while all handlers are busy.
func (s *Server) dispatch(ctx context.Context, conn *net.TCPConn) {
	s.handlers.Go(func() error {
		defer recoverHandler()
		// the timeout starts once the connection is handled, not while it waits for a handler
		if err := conn.SetDeadline(time.Now().Add(s.connTimeout)); err != nil {
			conn.Close()
			return nil
		}
		s.handler.Handle(ctx, conn)
		return nil
	})
}

// stopContext returns a context that is cancelled once Stop is called.
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
		log.Printf("tcpserver: panic in handler: %v\n%s", r, debug.Stack())
	}
}

func (s *Server) LocalAddr() net.Addr {
	return s.listener.Addr()
}

// Stop cancels the running handlers and waits for them to return.
// It must only be called once Serve has been started.
func (s *Server) Stop() error {
	close(s.closer)
	closeErr := s.listener.Close()
	// no more handlers can be started once Serve has returned
	<-s.done
	s.handlers.Wait() //nolint: errcheck // handlers never return errors
	if closeErr != nil {
		return fmt.Errorf("tcp server: close listener %s: %w", s.addr, closeErr)
	}
	return nil
}
