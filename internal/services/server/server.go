package server

import (
	"context"
	"time"

	"github.com/sergeii/swat4master/internal/core/servers"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
	"github.com/sergeii/swat4master/pkg/gamespy/browsing/query"
)

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
