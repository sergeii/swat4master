package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	defaultShutdownTimeout = time.Second * 60
	defaultReadTimeout     = time.Second * 60
	defaultWriteTimeout    = time.Second * 60
)

var ErrServerShutdownFailed = fmt.Errorf("server shutdown failed")

type serverConfig struct {
	readTimeout     time.Duration
	writeTimeout    time.Duration
	shutdownTimeout time.Duration
	handler         http.Handler
}

type HTTPServer struct {
	addr          *net.TCPAddr
	listener      *net.TCPListener
	svr           *http.Server
	cfg           *serverConfig
	readyCallback func()
}

type Option func(*HTTPServer) error

func WithShutdownTimeout(timeout time.Duration) Option {
	return func(c *HTTPServer) error {
		c.cfg.shutdownTimeout = timeout
		return nil
	}
}

func WithReadTimeout(timeout time.Duration) Option {
	return func(c *HTTPServer) error {
		c.cfg.readTimeout = timeout
		return nil
	}
}

func WithWriteTimeout(timeout time.Duration) Option {
	return func(c *HTTPServer) error {
		c.cfg.writeTimeout = timeout
		return nil
	}
}

func WithHandler(handler http.Handler) Option {
	return func(c *HTTPServer) error {
		c.cfg.handler = handler
		return nil
	}
}

func WithReadySignal(cb func()) Option {
	return func(s *HTTPServer) error {
		s.readyCallback = cb
		return nil
	}
}

func New(addr string, opts ...Option) (*HTTPServer, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}
	cfg := &serverConfig{
		// set defaults
		writeTimeout:    defaultWriteTimeout,
		readTimeout:     defaultReadTimeout,
		shutdownTimeout: defaultShutdownTimeout,
	}
	svr := &http.Server{Addr: addr}
	server := &HTTPServer{
		addr: tcpAddr,
		cfg:  cfg,
		svr:  svr,
	}
	for _, opt := range opts {
		if optErr := opt(server); optErr != nil {
			return nil, optErr
		}
	}
	svr.WriteTimeout = cfg.writeTimeout
	svr.ReadTimeout = cfg.readTimeout
	svr.Handler = cfg.handler
	return server, nil
}

func (s *HTTPServer) ListenAndServe(ctx context.Context) error {
	fatal := make(chan error, 1)

	log.Info().Stringer("addr", s.addr).Msg("Preparing HTTP server")
	listener, err := net.ListenTCP("tcp", s.addr)
	if err != nil {
		return err
	}
	s.listener = listener
	defer listener.Close()
	log.Info().Stringer("addr", s.listener.Addr()).Msg("HTTP server connection ready")

	// signal to any possible watchers that we are ready to listen
	if s.readyCallback != nil {
		s.readyCallback()
	}

	go func() {
		log.Info().Stringer("addr", s.addr).Msg("Starting HTTP server")
		if err := s.svr.Serve(s.listener); err != nil {
			fatal <- err
		}
	}()
	log.Info().Stringer("addr", s.addr).Msg("HTTP server launched")

	select {
	case err := <-fatal:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Error().Err(err).Stringer("addr", s.addr).Msg("HTTP server failed unexpectedly")
		} else {
			log.Warn().Stringer("addr", s.addr).Msg("HTTP server closed")
		}
	case <-ctx.Done():
	}
	return s.Stop()
}

func (s *HTTPServer) ListenAddr() net.Addr {
	return s.listener.Addr()
}

func (s *HTTPServer) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.shutdownTimeout)
	defer cancel()
	log.Info().
		Stringer("addr", s.addr).Dur("timeout", s.cfg.shutdownTimeout).
		Msg("Stopping HTTP server gracefully")
	if err := s.svr.Shutdown(ctx); err != nil {
		return ErrServerShutdownFailed
	}
	log.Info().Stringer("addr", s.addr).Msg("HTTP server stopped successfully")
	return nil
}
