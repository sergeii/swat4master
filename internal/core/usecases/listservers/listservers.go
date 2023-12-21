package listservers

import (
	"context"
	"errors"
	"time"

	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/pkg/gamespy/browsing/query"
)

var ErrUnableToObtainServers = errors.New("unable to obtain servers from repository")

type UseCase struct {
	serverRepo repositories.ServerRepository
}

type Request struct {
	query           query.Query
	recentness      time.Duration
	discoveryStatus ds.DiscoveryStatus
}

func New(
	serverRepo repositories.ServerRepository,
) UseCase {
	return UseCase{
		serverRepo: serverRepo,
	}
}

func NewRequest(
	query query.Query,
	recentness time.Duration,
	discoveryStatus ds.DiscoveryStatus,
) Request {
	return Request{
		query:           query,
		recentness:      recentness,
		discoveryStatus: discoveryStatus,
	}
}

func (uc *UseCase) Execute(ctx context.Context, req Request) ([]server.Server, error) {
	fs := repositories.NewServerFilterSet().
		ActiveAfter(time.Now().Add(-req.recentness)).
		WithStatus(req.discoveryStatus)

	recent, err := uc.serverRepo.Filter(ctx, fs)
	if err != nil {
		return nil, ErrUnableToObtainServers
	}

	filtered := make([]server.Server, 0, len(recent))
	for _, svr := range recent {
		info := svr.Info
		if req.query.Match(&info) {
			filtered = append(filtered, svr)
		}
	}

	return filtered, nil
}
