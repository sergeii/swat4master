package repositories

import (
	"context"
	"errors"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/filterset"
	"github.com/sergeii/swat4master/internal/core/entities/server"
)

var (
	ErrServerNotFound = errors.New("the requested server was not found")
	ErrServerExists   = errors.New("server already exists")
)

func ServerOnConflictIgnore(_ *server.Server) bool {
	return false
}

type ServerRepository interface {
	Get(ctx context.Context, addr addr.Addr) (server.Server, error)
	Add(ctx context.Context, server server.Server, onConflict func(*server.Server) bool) (server.Server, error)
	Update(ctx context.Context, server server.Server, onConflict func(*server.Server) bool) (server.Server, error)
	Remove(ctx context.Context, server server.Server, onConflict func(*server.Server) bool) error
	Filter(ctx context.Context, fs filterset.ServerFilterSet) ([]server.Server, error)
	Count(ctx context.Context) (int, error)
	CountByStatus(ctx context.Context) (map[ds.DiscoveryStatus]int, error)
}
