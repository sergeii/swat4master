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
	addr     *net.TCPAddr
	listener *net.TCPListener
	server   *http.Server
	cfg      *serverConfig
}

type Option func(*HTTPServer)

func WithShutdownTimeout(timeout time.Duration) Option {
	return func(c *HTTPServer) {
		c.cfg.shutdownTimeout = timeout
	}
}

func WithReadTimeout(timeout time.Duration) Option {
	return func(c *HTTPServer) {
		c.cfg.readTimeout = timeout
	}
}

func WithWriteTimeout(timeout time.Duration) Option {
	return func(c *HTTPServer) {
		c.cfg.writeTimeout = timeout
	}
}

func WithHandler(handler http.Handler) Option {
	return func(c *HTTPServer) {
		c.cfg.handler = handler
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
	svr := &http.Server{Addr: addr} //nolint: gosec
	server := &HTTPServer{
		addr:   tcpAddr,
		cfg:    cfg,
		server: svr,
	}
	for _, opt := range opts {
		opt(server)
	}
	svr.WriteTimeout = cfg.writeTimeout
	svr.ReadTimeout = cfg.readTimeout
	svr.ReadHeaderTimeout = cfg.readTimeout
	svr.Handler = cfg.handler
	return server, nil
}

func (s *HTTPServer) Listen() error {
	listener, err := net.ListenTCP("tcp", s.addr)
	if err != nil {
		return err
	}
	s.listener = listener
	return nil
}

// Serve serves HTTP requests until Stop is called and then returns nil, or until serving fails.
func (s *HTTPServer) Serve() error {
	if err := s.server.Serve(s.listener); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *HTTPServer) ListenAddr() net.Addr {
	return s.listener.Addr()
}

func (s *HTTPServer) Stop(ctx context.Context) error {
	stopCtx, cancel := context.WithTimeout(ctx, s.cfg.shutdownTimeout)
	defer cancel()
	if err := s.server.Shutdown(stopCtx); err != nil {
		return fmt.Errorf("http server: shutdown %s: %w", s.addr, err)
	}
	return nil
}
