package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

const (
	defaultShutdownTimeout = time.Second * 60
	defaultReadTimeout     = time.Second * 60
	defaultWriteTimeout    = time.Second * 60
)

type serverConfig struct {
	readTimeout     time.Duration
	writeTimeout    time.Duration
	shutdownTimeout time.Duration
	handler         http.Handler
}

type HTTPServer struct {
	addr          *net.TCPAddr
	listener      *net.TCPListener
	server        *http.Server
	cfg           *serverConfig
	closer        chan struct{}
	readyCallback func(net.Addr)
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

func WithReadySignal(cb func(net.Addr)) Option {
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
	svr := &http.Server{Addr: addr} // nolint: gosec
	server := &HTTPServer{
		addr:   tcpAddr,
		cfg:    cfg,
		server: svr,
		closer: make(chan struct{}),
	}
	for _, opt := range opts {
		if optErr := opt(server); optErr != nil {
			return nil, optErr
		}
	}
	svr.WriteTimeout = cfg.writeTimeout
	svr.ReadTimeout = cfg.readTimeout
	svr.ReadHeaderTimeout = cfg.readTimeout
	svr.Handler = cfg.handler
	return server, nil
}

func (s *HTTPServer) ListenAndServe() error {
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

	go func() {
		if err := s.server.Serve(s.listener); err != nil {
			fatal <- err
		}
	}()

	select {
	case err := <-fatal:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-s.closer:
		return nil
	}
}

func (s *HTTPServer) ListenAddr() net.Addr {
	return s.listener.Addr()
}

func (s *HTTPServer) Stop(ctx context.Context) error {
	close(s.closer)
	stopCtx, cancel := context.WithTimeout(ctx, s.cfg.shutdownTimeout)
	defer cancel()
	if err := s.server.Shutdown(stopCtx); err != nil {
		return fmt.Errorf("http server: shutdown %s: %w", s.addr, err)
	}
	return nil
}
