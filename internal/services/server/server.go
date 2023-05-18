package server

import (
	"context"
	"errors"
	"time"

	"github.com/benbjohnson/clock"

	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/entity/addr"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
	"github.com/sergeii/swat4master/pkg/gamespy/browsing/query"
)

type Service struct {
	servers servers.Repository
	clock   clock.Clock
}

func NewService(
	repo servers.Repository,
	clock clock.Clock,
) *Service {
	return &Service{
		servers: repo,
		clock:   clock,
	}
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

func (s *Service) Get(ctx context.Context, address addr.Addr) (servers.Server, error) {
	return s.servers.Get(ctx, address)
}

func (s *Service) FilterRecent(
	ctx context.Context,
	recentness time.Duration,
	q query.Query,
	withStatus ds.DiscoveryStatus,
) ([]servers.Server, error) {
	fs := servers.NewFilterSet().ActiveAfter(s.clock.Now().Add(-recentness)).WithStatus(withStatus)

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
