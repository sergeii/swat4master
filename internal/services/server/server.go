package server

import (
	"context"
	"errors"
	"time"

	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/entity/addr"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
	"github.com/sergeii/swat4master/pkg/gamespy/browsing/query"
)

var ErrCreateAborted = errors.New("server creation aborted on caller decision")
var ErrUpdateAborted = errors.New("server update aborted on caller decision")

type Service struct {
	servers servers.Repository
}

func NewService(repo servers.Repository) *Service {
	return &Service{
		servers: repo,
	}
}

func (s *Service) FilterRecent(
	ctx context.Context,
	recentness time.Duration,
	q query.Query,
	withStatus ds.DiscoveryStatus,
) ([]servers.Server, error) {
	fs := servers.NewFilterSet().After(time.Now().Add(-recentness)).WithStatus(withStatus)

	recent, err := s.servers.Filter(ctx, fs)
	if err != nil {
		return nil, err
	}

	filtered := make([]servers.Server, 0, len(recent))
	for _, svr := range recent {
		details := svr.GetInfo()
		if q.Match(&details) {
			filtered = append(filtered, svr)
		}
	}

	return filtered, nil
}

func (s *Service) Get(ctx context.Context, address addr.Addr) (servers.Server, error) {
	return s.servers.Get(ctx, address)
}

func (s *Service) Create(
	ctx context.Context,
	svr servers.Server,
	onConflict func(*servers.Server) bool,
) (servers.Server, error) {
	return s.servers.Add(ctx, svr, onConflict)
}

func (s *Service) Update(
	ctx context.Context,
	svr servers.Server,
	onConflict func(*servers.Server) bool,
) (servers.Server, error) {
	return s.servers.Update(ctx, svr, onConflict)
}

func (s *Service) CreateOrUpdate(
	ctx context.Context,
	svr servers.Server,
	onConflictDo func(*servers.Server),
) (servers.Server, error) {
	repoOnConflict := func(s *servers.Server) bool {
		onConflictDo(s)
		// we never want to fail an update
		return true
	}
	if _, err := s.servers.Get(ctx, svr.GetAddr()); err != nil {
		switch {
		case errors.Is(err, servers.ErrServerNotFound):
			return s.servers.Add(ctx, svr, repoOnConflict)
		default:
			return servers.Blank, err
		}
	}
	return s.servers.Update(ctx, svr, repoOnConflict)
}
