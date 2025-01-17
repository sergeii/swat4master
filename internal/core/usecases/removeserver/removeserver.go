package removeserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/entities/instance"
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
	instanceID []byte
	svrAddr    addr.Addr
}

func NewRequest(instanceID []byte, svrAddr addr.Addr) Request {
	return Request{
		instanceID: instanceID,
		svrAddr:    svrAddr,
	}
}

func (uc UseCase) Execute(ctx context.Context, req Request) error {
	uc.logger.Info().
		Stringer("addr", req.svrAddr).Str("instance", fmt.Sprintf("% x", req.instanceID)).
		Msg("Removing server on request")

	svr, err := uc.getServer(ctx, req.svrAddr)
	if err != nil {
		return err
	}

	inst, err := uc.getInstance(ctx, req.instanceID, svr.Addr)
	if err != nil {
		return err
	}

	if err = uc.serverRepo.Remove(ctx, svr, func(_ *server.Server) bool {
		return true
	}); err != nil {
		return err
	}
	if err = uc.instanceRepo.Remove(ctx, inst.ID); err != nil {
		return err
	}

	uc.logger.Info().
		Stringer("addr", req.svrAddr).Str("instance", fmt.Sprintf("% x", req.instanceID)).
		Msg("Removed server on request")

	return nil
}

func (uc UseCase) getServer(ctx context.Context, svrAddr addr.Addr) (server.Server, error) {
	svr, err := uc.serverRepo.Get(ctx, svrAddr)
	if err != nil {
		if errors.Is(err, repositories.ErrServerNotFound) {
			uc.logger.Info().Stringer("addr", svrAddr).Msg("Removed server not found")
			return server.Blank, ErrServerNotFound
		}
		return server.Blank, err
	}
	return svr, nil
}

func (uc UseCase) getInstance(
	ctx context.Context,
	instanceID []byte,
	svrAddr addr.Addr,
) (instance.Instance, error) {
	instID, err := instance.NewID(instanceID)
	if err != nil {
		return instance.Blank, err
	}

	inst, err := uc.instanceRepo.Get(ctx, instID)
	if err != nil {
		// this could be a race condition - ignore
		if errors.Is(err, repositories.ErrInstanceNotFound) {
			uc.logger.Info().
				Str("instance", fmt.Sprintf("% x", instanceID)).
				Msg("Instance for removed server not found")
			return instance.Blank, ErrInstanceNotFound
		}
		return instance.Blank, err
	}

	// make sure to verify the "owner" of the provided instance id
	if inst.Addr.GetDottedIP() != svrAddr.GetDottedIP() {
		return instance.Blank, ErrInstanceAddrMismatch
	}

	return inst, nil
}
