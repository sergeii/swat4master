package removeserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
)

var (
	ErrInstanceAddrMismatch = errors.New("instance address mismatch")
	ErrInstanceNotFound     = errors.New("the requested instance was not found")
	ErrServerNotFound       = errors.New("the requested server was not found")
)

type UseCase struct {
	serverRepo   repositories.ServerRepository
	instanceRepo repositories.InstanceRepository
	logger       *zerolog.Logger
}

func New(
	serverRepo repositories.ServerRepository,
	instanceRepo repositories.InstanceRepository,
	logger *zerolog.Logger,
) UseCase {
	return UseCase{
		serverRepo:   serverRepo,
		instanceRepo: instanceRepo,
		logger:       logger,
	}
}

type Request struct {
	instanceID string
	svrAddr    addr.Addr
}

func NewRequest(instanceID string, svrAddr addr.Addr) Request {
	return Request{
		instanceID: instanceID,
		svrAddr:    svrAddr,
	}
}

func (uc UseCase) Execute(
	ctx context.Context,
	req Request,
) error {
	svr, err := uc.serverRepo.Get(ctx, req.svrAddr)
	if err != nil {
		switch {
		case errors.Is(err, repositories.ErrServerNotFound):
			uc.logger.Info().
				Stringer("addr", req.svrAddr).Str("instance", fmt.Sprintf("% x", req.instanceID)).
				Msg("Removed server not found")
			return ErrServerNotFound
		default:
			return err
		}
	}

	inst, err := uc.instanceRepo.GetByID(ctx, req.instanceID)
	if err != nil {
		switch {
		// this could be a race condition - ignore
		case errors.Is(err, repositories.ErrInstanceNotFound):
			uc.logger.Info().
				Stringer("addr", req.svrAddr).
				Stringer("server", svr).
				Str("instance", fmt.Sprintf("% x", req.instanceID)).
				Msg("Instance for removed server not found")
			return ErrInstanceNotFound
		default:
			return err
		}
	}
	// make sure to verify the "owner" of the provided instance id
	if inst.Addr.GetDottedIP() != svr.Addr.GetDottedIP() {
		return ErrInstanceAddrMismatch
	}

	if err = uc.serverRepo.Remove(ctx, svr, func(_ *server.Server) bool {
		return true
	}); err != nil {
		return err
	}
	if err = uc.instanceRepo.RemoveByID(ctx, req.instanceID); err != nil {
		return err
	}

	uc.logger.Info().
		Stringer("addr", req.svrAddr).Str("instance", fmt.Sprintf("% x", req.instanceID)).
		Msg("Successfully removed server on request")

	return nil
}
