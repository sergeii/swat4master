package server

import (
	"context"
	"errors"
	"time"

	"github.com/jonboulle/clockwork"

	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/filterset"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/pkg/gamespy/browsing/query"
)

type Service struct {
	servers repositories.ServerRepository
	clock   clockwork.Clock
}

func NewService(
	repo repositories.ServerRepository,
	clock clockwork.Clock,
) *Service {
	return &Service{
		servers: repo,
		clock:   clock,
	}
}

func (s *Service) Update(
	ctx context.Context,
	svr server.Server,
	onConflict func(*server.Server) bool,
) (server.Server, error) {
	return s.servers.Update(ctx, svr, onConflict)
}

func (s *Service) CreateOrUpdate(
	ctx context.Context,
	svr server.Server,
	onConflictDo func(*server.Server),
) (server.Server, error) {
	repoOnConflict := func(s *server.Server) bool {
		onConflictDo(s)
		// we never want to fail an update
		return true
	}
	if _, err := s.servers.Get(ctx, svr.Addr); err != nil {
		switch {
		case errors.Is(err, repositories.ErrServerNotFound):
			return s.servers.Add(ctx, svr, repoOnConflict)
		default:
			return server.Blank, err
		}
	}
	return s.servers.Update(ctx, svr, repoOnConflict)
}

func (s *Service) FilterRecent(
	ctx context.Context,
	recentness time.Duration,
	q query.Query,
	withStatus ds.DiscoveryStatus,
) ([]server.Server, error) {
	fs := filterset.New().ActiveAfter(s.clock.Now().Add(-recentness)).WithStatus(withStatus)

	recent, err := s.servers.Filter(ctx, fs)
	if err != nil {
		return nil, err
	}

	filtered := make([]server.Server, 0, len(recent))
	for _, svr := range recent {
		info := svr.Info
		if q.Match(&info) {
			filtered = append(filtered, svr)
		}
	}

	return filtered, nil
}
