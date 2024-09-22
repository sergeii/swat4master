package reviveservers

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/filterset"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/metrics"
	"github.com/sergeii/swat4master/pkg/random"
)

type UseCaseOptions struct {
	MaxProbeRetries int
}

type UseCase struct {
	serverRepo repositories.ServerRepository
	probeRepo  repositories.ProbeRepository
	opts       UseCaseOptions
	metrics    *metrics.Collector
	logger     *zerolog.Logger
}

func New(
	serverRepo repositories.ServerRepository,
	probeRepo repositories.ProbeRepository,
	opts UseCaseOptions,
	metrics *metrics.Collector,
	logger *zerolog.Logger,
) UseCase {
	return UseCase{
		serverRepo: serverRepo,
		probeRepo:  probeRepo,
		opts:       opts,
		metrics:    metrics,
		logger:     logger,
	}
}

type Request struct {
	MinScope     time.Time
	MaxScope     time.Time
	MinCountdown time.Time
	MaxCountdown time.Time
	Deadline     time.Time
}

func NewRequest(
	minScope time.Time,
	maxScope time.Time,
	minCountdown time.Time,
	maxCountdown time.Time,
	deadline time.Time,
) Request {
	return Request{
		MinScope:     minScope,
		MaxScope:     maxScope,
		MinCountdown: minCountdown,
		MaxCountdown: maxCountdown,
		Deadline:     deadline,
	}
}

type Response struct {
	Count int
}

var NoResponse = Response{}

func (uc UseCase) Execute(ctx context.Context, req Request) (Response, error) {
	fs := filterset.New().ActiveAfter(req.MinScope).ActiveBefore(req.MaxScope).NoStatus(ds.Port | ds.PortRetry)

	serversWithoutPort, err := uc.serverRepo.Filter(ctx, fs)
	if err != nil {
		uc.logger.Error().Err(err).Msg("Unable to obtain servers for port discovery")
		return NoResponse, err
	}

	probeCount := 0
	for _, svr := range serversWithoutPort {
		countdown := selectCountdown(req.MinCountdown, req.MaxCountdown)
		if err := uc.addProbe(ctx, svr.Addr, countdown, req.Deadline); err != nil {
			uc.logger.Warn().
				Err(err).
				Stringer("server", svr).Time("countdown", countdown).Time("deadline", req.Deadline).
				Msg("Failed to add server to port discovery queue")
			continue
		}
		uc.logger.Debug().
			Time("countdown", countdown).Time("deadline", req.Deadline).Stringer("server", svr).
			Msg("Added server to port discovery queue")
		probeCount++
	}

	if probeCount != 0 {
		uc.metrics.DiscoveryQueueProduced.Add(float64(probeCount))
	}

	return Response{probeCount}, nil
}

func (uc UseCase) addProbe(
	ctx context.Context,
	svrAddr addr.Addr,
	countdown time.Time,
	deadline time.Time,
) error {
	prb := probe.New(svrAddr, svrAddr.Port, probe.GoalPort, uc.opts.MaxProbeRetries)
	return uc.probeRepo.AddBetween(ctx, prb, countdown, deadline)
}

func selectCountdown(minVal, maxVal time.Time) time.Time {
	if !maxVal.After(minVal) {
		return minVal
	}
	spread := maxVal.Sub(minVal)
	countdown := random.RandInt(0, int(spread))
	return minVal.Add(time.Duration(countdown))
}
