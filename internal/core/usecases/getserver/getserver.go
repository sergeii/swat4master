package getserver

import (
	"context"
	"errors"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
)

var (
	ErrServerNotFound       = errors.New("server not found")
	ErrServerHasNoDetails   = errors.New("server has no details")
	ErrUnableToObtainServer = errors.New("unable to obtain server from repository")
)

type UseCase struct {
	serverRepo repositories.ServerRepository
}

func New(
	serverRepo repositories.ServerRepository,
) UseCase {
	return UseCase{
		serverRepo: serverRepo,
	}
}

func (uc *UseCase) Execute(ctx context.Context, address addr.Addr) (server.Server, error) {
	svr, err := uc.serverRepo.Get(ctx, address)
	if err != nil {
		switch {
		case errors.Is(err, repositories.ErrServerNotFound):
			return server.Blank, ErrServerNotFound
		default:
			return server.Blank, ErrUnableToObtainServer
		}
	}

	if !svr.HasDiscoveryStatus(ds.Details) {
		return server.Blank, ErrServerHasNoDetails
	}

	return svr, nil
}
