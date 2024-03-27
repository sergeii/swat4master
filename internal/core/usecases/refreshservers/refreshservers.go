package refreshservers

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/filterset"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/repositories"
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

type Response struct {
	Count int
}

var NoResponse = Response{}

func (uc UseCase) Execute(ctx context.Context, deadline time.Time) (Response, error) {
	fs := filterset.New().WithStatus(ds.Port).NoStatus(ds.DetailsRetry)
	serversWithDetails, err := uc.serverRepo.Filter(ctx, fs)
	if err != nil {
		uc.logger.Error().Err(err).Msg("Unable to obtain servers for refresh")
		return NoResponse, err
	}

	cnt := 0
	for _, svr := range serversWithDetails {
		if err := uc.addProbe(ctx, svr.Addr, svr.QueryPort, deadline); err != nil {
			uc.logger.Warn().
				Err(err).Stringer("server", svr).
				Msg("Failed to add server to details discovery queue")
			continue
		}
		cnt++
	}

	return Response{cnt}, nil
}

func (uc UseCase) addProbe(
	ctx context.Context,
	svrAddr addr.Addr,
	queryPort int,
	deadline time.Time,
) error {
	prb := probe.New(svrAddr, queryPort, probe.GoalDetails)
	return uc.probeRepo.AddBetween(ctx, prb, repositories.NC, deadline)
}
