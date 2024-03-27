package cleanservers

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/sergeii/swat4master/internal/core/entities/filterset"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
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

type Response struct {
	Count  int
	Errors int
}

var NoResponse = Response{}

func (uc UseCase) Execute(ctx context.Context, until time.Time) (Response, error) {
	var before, after, removed, errors int
	var err error

	if before, err = uc.serverRepo.Count(ctx); err != nil {
		return NoResponse, err
	}

	uc.logger.Info().
		Stringer("until", until).Int("servers", before).
		Msg("Starting to clean outdated servers")

	fs := filterset.New().UpdatedBefore(until)
	outdatedServers, err := uc.serverRepo.Filter(ctx, fs)
	if err != nil {
		uc.logger.Error().Err(err).Msg("Unable to obtain servers for cleanup")
		return NoResponse, err
	}

	for _, svr := range outdatedServers {
		if err = uc.instanceRepo.RemoveByAddr(ctx, svr.Addr); err != nil {
			uc.logger.Error().
				Err(err).
				Stringer("until", until).Stringer("addr", svr.Addr).
				Msg("Failed to remove instance for removed server")
			errors++
			continue
		}
		if err = uc.serverRepo.Remove(ctx, svr, func(conflict *server.Server) bool {
			if conflict.RefreshedAt.After(until) {
				uc.logger.Info().
					Stringer("server", conflict).Stringer("refreshed", conflict.RefreshedAt).
					Msg("Removed server is more recent")
				return false
			}
			return true
		}); err != nil {
			uc.logger.Error().
				Err(err).
				Stringer("until", until).Stringer("addr", svr.Addr).
				Msg("Failed to remove outdated server")
			errors++
			continue
		}
		removed++
	}

	if after, err = uc.serverRepo.Count(ctx); err != nil {
		return NoResponse, err
	}

	uc.logger.Info().
		Stringer("until", until).
		Int("removed", removed).Int("errors", errors).
		Int("before", before).Int("after", after).
		Msg("Finished cleaning outdated servers")

	return Response{Count: removed, Errors: errors}, nil
}
