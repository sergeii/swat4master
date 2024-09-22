package reportserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/entities/details"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/instance"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/metrics"
)

var ErrInvalidRequestPayload = errors.New("invalid request payload")

type UseCaseOptions struct {
	MaxProbeRetries int
}

type UseCase struct {
	serverRepo   repositories.ServerRepository
	instanceRepo repositories.InstanceRepository
	probeRepo    repositories.ProbeRepository
	opts         UseCaseOptions
	metrics      *metrics.Collector
	validate     *validator.Validate
	clock        clockwork.Clock
	logger       *zerolog.Logger
}

func New(
	serverRepo repositories.ServerRepository,
	instanceRepo repositories.InstanceRepository,
	probeRepo repositories.ProbeRepository,
	opts UseCaseOptions,
	validate *validator.Validate,
	metrics *metrics.Collector,
	clock clockwork.Clock,
	logger *zerolog.Logger,
) UseCase {
	return UseCase{
		serverRepo:   serverRepo,
		instanceRepo: instanceRepo,
		probeRepo:    probeRepo,
		opts:         opts,
		metrics:      metrics,
		validate:     validate,
		clock:        clock,
		logger:       logger,
	}
}

type Request struct {
	svrAddr    addr.Addr
	queryPort  int
	instanceID string
	fields     map[string]string
}

func NewRequest(
	svrAddr addr.Addr,
	queryPort int,
	instanceID string,
	fields map[string]string,
) Request {
	return Request{
		svrAddr:    svrAddr,
		queryPort:  queryPort,
		instanceID: instanceID,
		fields:     fields,
	}
}

func (uc UseCase) Execute(ctx context.Context, req Request) error {
	svr, err := uc.obtainServerByAddr(ctx, req.svrAddr, req.queryPort)
	if err != nil {
		return err
	}

	inst, err := instance.New(req.instanceID, svr.Addr.GetIP(), svr.Addr.Port)
	if err != nil {
		uc.logger.Error().
			Err(err).
			Stringer("addr", req.svrAddr).Str("instance", fmt.Sprintf("% x", req.instanceID)).
			Msg("Failed to create an instance")
		return err
	}

	info, err := details.NewInfoFromParams(req.fields)
	if err != nil {
		uc.logger.Error().
			Err(err).
			Stringer("addr", req.svrAddr).Str("instance", fmt.Sprintf("% x", req.instanceID)).
			Msg("Failed to parse reported fields")
		return ErrInvalidRequestPayload
	}
	if validateErr := info.Validate(uc.validate); validateErr != nil {
		uc.logger.Error().
			Err(validateErr).
			Stringer("addr", req.svrAddr).Str("instance", fmt.Sprintf("% x", req.instanceID)).
			Msg("Failed to validate reported fields")
		return ErrInvalidRequestPayload
	}

	now := uc.clock.Now()

	svr.UpdateInfo(info, now)
	svr.UpdateDiscoveryStatus(ds.Master | ds.Info)

	if svr, err = uc.serverRepo.AddOrUpdate(ctx, svr, func(conflict *server.Server) {
		// in case of conflict, just do all the same
		conflict.UpdateInfo(info, now)
		conflict.UpdateDiscoveryStatus(ds.Master | ds.Info)
	}); err != nil {
		uc.logger.Error().
			Err(err).
			Stringer("addr", req.svrAddr).Str("instance", fmt.Sprintf("% x", req.instanceID)).
			Msg("Failed to add server to repository")
		return err
	}

	if err := uc.instanceRepo.Add(ctx, inst); err != nil {
		uc.logger.Error().
			Err(err).
			Stringer("svr", svr).Str("instance", fmt.Sprintf("% x", req.instanceID)).
			Msg("Failed to add instance to repository")
		return err
	}

	// attempt to discover query port for newly reported servers
	if err := uc.maybeDiscoverPort(ctx, svr); err != nil {
		// it's not critical if we fail here, so don't return an error but log it
		uc.logger.Error().
			Err(err).
			Stringer("addr", req.svrAddr).Str("instance", fmt.Sprintf("% x", req.instanceID)).
			Stringer("server", svr).
			Msg("Failed to add server for port discovery")
	}

	uc.logger.Info().
		Stringer("addr", req.svrAddr).
		Str("instance", fmt.Sprintf("% x", req.instanceID)).
		Stringer("server", svr).
		Msg("Successfully reported server")

	return nil
}

func (uc UseCase) maybeDiscoverPort(ctx context.Context, pending server.Server) error {
	var err error
	// the server has either already go its port discovered
	// or it is currently in the queue
	if !pending.HasNoDiscoveryStatus(ds.Port | ds.PortRetry) {
		return nil
	}

	prb := probe.New(pending.Addr, pending.Addr.Port, probe.GoalPort, uc.opts.MaxProbeRetries)
	if err = uc.probeRepo.Add(ctx, prb); err != nil {
		return err
	}
	uc.metrics.DiscoveryQueueProduced.Inc()

	pending.UpdateDiscoveryStatus(ds.PortRetry)

	if _, err = uc.serverRepo.Update(ctx, pending, func(conflict *server.Server) bool {
		// while we were updating this server,
		// it's got the port, or it was put in the queue.
		// In such a case, resolve the conflict by not doing anything
		if conflict.HasDiscoveryStatus(ds.Port | ds.PortRetry) {
			return false
		}
		conflict.UpdateDiscoveryStatus(ds.PortRetry)
		return true
	}); err != nil {
		return err
	}

	return nil
}

func (uc UseCase) obtainServerByAddr(
	ctx context.Context,
	svrAddr addr.Addr,
	queryPort int,
) (server.Server, error) {
	svr, err := uc.serverRepo.Get(ctx, svrAddr)
	if err != nil {
		switch {
		case errors.Is(err, repositories.ErrServerNotFound):
			// create new server
			if svr, err = server.NewFromAddr(svrAddr, queryPort); err != nil {
				return server.Blank, err
			}
			return svr, nil
		default:
			return server.Blank, err
		}
	}
	return svr, nil
}
