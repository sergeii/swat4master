package server

import (
	"context"
	"fmt"
	"net"

	"github.com/rs/zerolog/log"
)

const (
	defaultBufferSize = 1024
)

type Option func(*Server) error
type HandlerFunc func(context.Context, *net.UDPConn, *net.UDPAddr, []byte)

type Server struct {
	addr          *net.UDPAddr
	conn          *net.UDPConn
	bufferSize    int
	handler       HandlerFunc
	readyCallback func()
}

func WithBufferSize(sz int) Option {
	return func(s *Server) error {
		s.bufferSize = sz
		return nil
	}
}

func WithHandler(hf HandlerFunc) Option {
	return func(s *Server) error {
		s.handler = hf
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
	udpAddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return nil, err
	}
	server := &Server{
		addr: udpAddr,
		// set defaults
		bufferSize: defaultBufferSize,
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

	log.Info().Stringer("addr", s.addr).Msg("Starting UDP server")
	conn, err := net.ListenUDP("udp", s.addr)
	if err != nil {
		return err
	}
	s.conn = conn

	defer s.conn.Close()
	log.Info().Stringer("addr", s.conn.LocalAddr()).Msg("UDP server launched")

	if s.readyCallback != nil {
		s.readyCallback()
	}

	go func() {
		buffer := make([]byte, s.bufferSize)
		for {
			n, raddr, err := s.conn.ReadFromUDP(buffer)
			if err != nil {
				log.Error().Err(err).Stringer("addr", s.addr).Msg("Failed to read on UDP socket")
				fatal <- err
				return
			}
			if n > 0 && s.handler != nil {
				payload := make([]byte, n)
				copy(payload, buffer[:n])
				go s.handler(ctx, s.conn, raddr, payload)
			}
		}
	}()

	select {
	case err := <-fatal:
		log.Warn().Err(err).Stringer("addr", s.addr).Msg("UDP server failed unexpectedly")
	case <-ctx.Done():
	}
	return s.Stop()
}

func (s *Server) LocalAddr() net.Addr {
	return s.conn.LocalAddr()
}

func (s *Server) Stop() error {
	log.Info().Stringer("addr", s.addr).Msg("Stopping UDP server")
	if err := s.conn.Close(); err != nil {
		return fmt.Errorf("failed to stop UDP server %s due to: %w", s.addr, err)
	}
	log.Info().Stringer("addr", s.addr).Msg("UDP server stopped successfully")
	return nil
}
