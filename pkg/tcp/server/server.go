package server

import (
	"context"
	"fmt"
	"net"
	"time"
)

const (
	defaultConnTimeout = time.Second
)

type Option func(*Server) error

type Server struct {
	addr          *net.TCPAddr
	listener      *net.TCPListener
	handler       Handler
	closer        chan struct{}
	readyCallback func(net.Addr)
	connTimeout   time.Duration
}

func WithTimeout(timeout time.Duration) Option {
	return func(s *Server) error {
		s.connTimeout = timeout
		return nil
	}
}

func WithReadySignal(cb func(net.Addr)) Option {
	return func(s *Server) error {
		s.readyCallback = cb
		return nil
	}
}

func New(addr string, handler Handler, opts ...Option) (*Server, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}
	server := &Server{
		addr:    tcpAddr,
		handler: handler,
		closer:  make(chan struct{}),
		// set defaults
		connTimeout: defaultConnTimeout,
	}
	for _, opt := range opts {
		if optErr := opt(server); optErr != nil {
			return nil, optErr
		}
	}
	return server, nil
}

func (s *Server) Listen() error {
	fatal := make(chan error, 1)

	listener, err := net.ListenTCP("tcp", s.addr)
	if err != nil {
		return err
	}
	s.listener = listener
	defer listener.Close()

	// signal to any possible watchers that we are ready to listen
	if s.readyCallback != nil {
		s.readyCallback(s.listener.Addr())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-s.closer
		cancel()
	}()

	go func() {
		for {
			conn, err := listener.AcceptTCP()
			if err != nil {
				fatal <- err
				return
			}
			if err := conn.SetDeadline(time.Now().Add(s.connTimeout)); err != nil {
				continue
			}
			go s.handler.Handle(ctx, conn)
		}
	}()

	select {
	case err := <-fatal:
		return err
	case <-s.closer:
		return nil
	}
}

func (s *Server) LocalAddr() net.Addr {
	return s.listener.Addr()
}

func (s *Server) Stop() error {
	close(s.closer)
	if err := s.listener.Close(); err != nil {
		return fmt.Errorf("failed to stop TCP server %s due to: %w", s.addr, err)
	}
	return nil
}
