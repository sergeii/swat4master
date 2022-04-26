package server

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	defaultConnTimeout = time.Second
)

type Option func(*Server) error
type HandlerFunc func(context.Context, *net.TCPConn)

type Server struct {
	addr          *net.TCPAddr
	listener      *net.TCPListener
	handler       HandlerFunc
	readyCallback func()
	connTimeout   time.Duration
}

func WithHandler(hf HandlerFunc) Option {
	return func(s *Server) error {
		s.handler = hf
		return nil
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(s *Server) error {
		s.connTimeout = timeout
		return nil
	}
}

func WithReadySignal(cb func()) Option {
	return func(s *Server) error {
		s.readyCallback = cb
		return nil
	}
}

func New(addr string, opts ...Option) (*Server, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}
	server := &Server{
		addr: tcpAddr,
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

func (s *Server) Listen(ctx context.Context) error {
	fatal := make(chan error, 1)

	log.Info().Stringer("addr", s.addr).Msg("Starting TCP server")
	listener, err := net.ListenTCP("tcp", s.addr)
	if err != nil {
		return err
	}
	s.listener = listener
	defer listener.Close()
	log.Info().Stringer("addr", s.listener.Addr()).Msg("TCP server launched")

	// signal to any possible watchers that we are ready to listen
	if s.readyCallback != nil {
		s.readyCallback()
	}

	go func() {
		for {
			conn, err := listener.AcceptTCP()
			if err != nil {
				log.Error().Err(err).Stringer("addr", s.addr).Msg("Failed to listen on TCP socket")
				fatal <- err
				return
			}
			if s.handler != nil {
				if err := conn.SetDeadline(time.Now().Add(s.connTimeout)); err != nil {
					log.Error().Err(err).Stringer("addr", s.addr).Msg("Failed to set deadline on TCP socket")
					continue
				}
				go s.handler(ctx, conn)
			} else {
				conn.Close()
			}
		}
	}()

	select {
	case err := <-fatal:
		log.Warn().Err(err).Stringer("addr", s.addr).Msg("TCP server failed unexpectedly")
	case <-ctx.Done():
	}
	return s.Stop()
}

func (s *Server) LocalAddr() net.Addr {
	return s.listener.Addr()
}

func (s *Server) Stop() error {
	log.Info().Stringer("addr", s.addr).Msg("Stopping TCP server")
	if err := s.listener.Close(); err != nil {
		return fmt.Errorf("failed to stop TCP server %s due to: %w", s.addr, err)
	}
	log.Info().Stringer("addr", s.addr).Msg("TCP server stopped successfully")
	return nil
}
