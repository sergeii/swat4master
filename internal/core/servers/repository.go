package servers

import (
	"context"
	"errors"

	"github.com/sergeii/swat4master/internal/entity/addr"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
)

var (
	ErrServerNotFound = errors.New("the requested server was not found")
	ErrServerExists   = errors.New("server already exists")
)

func OnConflictIgnore(_ *Server) bool {
	return false
}

type Repository interface {
	Get(ctx context.Context, addr addr.Addr) (Server, error)
	Add(ctx context.Context, server Server, onConflict func(*Server) bool) (Server, error)
	Update(ctx context.Context, server Server, onConflict func(*Server) bool) (Server, error)
	Remove(ctx context.Context, server Server, onConflict func(*Server) bool) error
	Filter(ctx context.Context, fs FilterSet) ([]Server, error)
	Count(ctx context.Context) (int, error)
	CountByStatus(ctx context.Context) (map[ds.DiscoveryStatus]int, error)
}
