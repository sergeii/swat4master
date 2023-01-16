package servers

import (
	"context"
	"errors"
	"time"

	"github.com/sergeii/swat4master/internal/entity/addr"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
)

var ErrServerNotFound = errors.New("the requested server was not found")
var ErrVersionConflict = errors.New("version conflict")

type Unlocker interface {
	Unlock()
}

type Repository interface {
	Get(ctx context.Context, addr addr.Addr) (Server, error)
	GetForUpdate(ctx context.Context, addr addr.Addr) (Server, Unlocker, error)
	AddOrUpdate(ctx context.Context, server Server) (Server, error)
	Update(ctx context.Context, unlocker Unlocker, server Server) (Server, error)
	Filter(ctx context.Context, fs FilterSet) ([]Server, error)
	Count(ctx context.Context) (int, error)
	CountByStatus(ctx context.Context) (map[ds.DiscoveryStatus]int, error)
	Remove(ctx context.Context, server Server) error
	CleanNext(ctx context.Context, before time.Time) (Server, bool)
}
