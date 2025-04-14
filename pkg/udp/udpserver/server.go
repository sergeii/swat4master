package udpserver

import (
	"context"
	"fmt"
	"net"
	"net/netip"
)

const (
	defaultBufferSize = 1024
)

type Option func(*Server) error

type Server struct {
	addr          *net.UDPAddr
	conn          *net.UDPConn
	bufferSize    int
	handler       Handler
	closer        chan struct{}
	readyCallback func()
}

func WithBufferSize(sz int) Option {
	return func(s *Server) error {
		s.bufferSize = sz
		return nil
	}
}

func WithReadySignal(cb func()) Option {
	return func(s *Server) error {
		s.readyCallback = cb
		return nil
	}
}

func New(addr string, handler Handler, opts ...Option) (*Server, error) {
	udpAddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return nil, err
	}
	server := &Server{
		addr:    udpAddr,
		handler: handler,
		closer:  make(chan struct{}),
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

func (s *Server) Listen() error {
	fatal := make(chan error, 1)

	conn, err := net.ListenUDP("udp", s.addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	s.conn = conn

	if s.readyCallback != nil {
		s.readyCallback()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-s.closer
		cancel()
	}()

	go func() {
		buffer := make([]byte, s.bufferSize)
		for {
			n, raddr, err := s.conn.ReadFromUDP(buffer)
			if err != nil {
				fatal <- err
				return
			}
			if n > 0 && s.handler != nil {
				payload := make([]byte, n)
				copy(payload, buffer[:n])
				go s.handler.Handle(ctx, s.conn, raddr, payload)
			}
		}
	}()

	select {
	case err := <-fatal:
		return err
	case <-ctx.Done():
		return nil
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

func (s *Server) Stop() error {
	close(s.closer)
	if err := s.conn.Close(); err != nil {
		return fmt.Errorf("failed to stop UDP server %s due to: %w", s.addr, err)
	}
	return nil
}
