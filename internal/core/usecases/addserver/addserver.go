package addserver

import (
	"context"
	"errors"

	"github.com/rs/zerolog"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
)

var (
	ErrInvalidAddress            = errors.New("invalid address")
	ErrUnableToCreateServer      = errors.New("unable to create server")
	ErrUnableToDiscoverServer    = errors.New("unable to discover server")
	ErrServerDiscoveryInProgress = errors.New("server discovery is in progress")
	ErrServerHasNoQueryablePort  = errors.New("server has no queryable port")
)

type UseCase struct {
	serverRepo repositories.ServerRepository
	probeRepo  repositories.ProbeRepository
	logger     *zerolog.Logger
}

func New(
	serverRepo repositories.ServerRepository,
	probeRepo repositories.ProbeRepository,
	logger *zerolog.Logger,
) UseCase {
	return UseCase{
		serverRepo: serverRepo,
		probeRepo:  probeRepo,
		logger:     logger,
	}
}

func (uc UseCase) Execute(ctx context.Context, address addr.Addr) (server.Server, error) {
	if err := uc.validateAddress(address); err != nil {
		return server.Blank, err
	}

	svr, err := uc.getOrCreateServer(ctx, address)
	if err != nil {
		return server.Blank, err
	}

	if discErr := uc.maybeDiscoverServer(ctx, svr); discErr != nil {
		return server.Blank, discErr
	}

	return svr, nil
}

func (uc UseCase) validateAddress(address addr.Addr) error {
	ipv4 := address.GetIP()
	if !ipv4.IsGlobalUnicast() || ipv4.IsPrivate() {
		return ErrInvalidAddress
	}
	return nil
}

func (uc UseCase) getOrCreateServer(ctx context.Context, address addr.Addr) (server.Server, error) {
	svr, err := uc.serverRepo.Get(ctx, address)
	if err != nil {
		switch {
		case errors.Is(err, repositories.ErrServerNotFound):
			newSvr, createErr := uc.createServerFromAddress(ctx, address)
			if createErr != nil {
				uc.logger.Error().
					Err(err).Stringer("addr", address).
					Msg("Unable to create new server")
				return server.Blank, ErrUnableToCreateServer
			}
			return newSvr, nil
		default:
			uc.logger.Warn().
				Err(err).Stringer("addr", address).
				Msg("Unable to obtain server due to error")
			return server.Blank, ErrUnableToCreateServer
		}
	}

	return svr, nil
}

func (uc UseCase) createServerFromAddress(
	ctx context.Context,
	address addr.Addr,
) (server.Server, error) {
	svr, err := server.NewFromAddr(address, address.Port+1)
	if err != nil {
		return server.Blank, err
	}

	if svr, err = uc.serverRepo.Add(ctx, svr, func(upd *server.Server) bool {
		// a server with exactly same address was created in the process,
		// we cannot proceed further
		return false
	}); err != nil {
		return server.Blank, err
	}

	return svr, nil
}

func (uc UseCase) maybeDiscoverServer(
	ctx context.Context,
	svr server.Server,
) error {
	switch {
	case svr.HasDiscoveryStatus(ds.Details):
		uc.logger.Debug().
			Stringer("server", svr).Stringer("status", svr.DiscoveryStatus).
			Msg("Server already has details")
		return nil
	// server discovery is still pending
	case svr.HasAnyDiscoveryStatus(ds.PortRetry | ds.DetailsRetry):
		uc.logger.Debug().
			Stringer("server", svr).Stringer("status", svr.DiscoveryStatus).
			Msg("Server discovery is in progress")
		return ErrServerDiscoveryInProgress
	case svr.HasDiscoveryStatus(ds.NoPort):
		uc.logger.Debug().
			Stringer("server", svr).Stringer("status", svr.DiscoveryStatus).
			Msg("No port has been discovered for server")
		return ErrServerHasNoQueryablePort
	// other status - send the server for port discovery
	default:
		if discErr := uc.discoverServer(ctx, svr); discErr != nil {
			uc.logger.Warn().
				Err(discErr).Stringer("server", svr).
				Msg("Unable to submit discovery for server")
			return ErrUnableToDiscoverServer
		}
		uc.logger.Info().Stringer("server", svr).Msg("Added server for discovery")
		return ErrServerDiscoveryInProgress
	}
}

func (uc UseCase) discoverServer(
	ctx context.Context,
	svr server.Server,
) error {
	svr.UpdateDiscoveryStatus(ds.PortRetry)

	if _, err := uc.serverRepo.Update(ctx, svr, func(updated *server.Server) bool {
		// some of the statuses that we don't want to run discovery for,
		// have appeared in the process, so we abort here
		if updated.HasDiscoveryStatus(ds.Details | ds.PortRetry | ds.DetailsRetry) {
			return false
		}
		// prevent further submissions until the retry status is cleared
		updated.UpdateDiscoveryStatus(ds.PortRetry)
		return true
	}); err != nil {
		return err
	}

	prb := probe.New(svr.Addr, svr.Addr.Port, probe.GoalPort)
	if err := uc.probeRepo.AddBetween(ctx, prb, repositories.NC, repositories.NC); err != nil {
		uc.logger.Warn().
			Err(err).Stringer("server", svr).
			Msg("Unable to add server to port discovery queue")
		return ErrUnableToDiscoverServer
	}

	return nil
}
