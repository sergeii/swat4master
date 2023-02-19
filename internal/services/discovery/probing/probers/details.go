package probers

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog"

	"github.com/sergeii/swat4master/internal/core/probes"
	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/entity/details"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
	"github.com/sergeii/swat4master/internal/services/discovery/probing"
	"github.com/sergeii/swat4master/internal/services/monitoring"
	"github.com/sergeii/swat4master/pkg/gamespy/serverquery/gs1"
)

type DetailsProber struct {
	metrics  *monitoring.MetricService
	validate *validator.Validate
	logger   *zerolog.Logger
}

func NewDetailsProber(
	service *probing.Service,
	metrics *monitoring.MetricService,
	validate *validator.Validate,
	logger *zerolog.Logger,
) (*DetailsProber, error) {
	dp := &DetailsProber{
		metrics:  metrics,
		validate: validate,
		logger:   logger,
	}
	if err := service.Register(probes.GoalDetails, dp); err != nil {
		return nil, err
	}
	return dp, nil
}

// Probe probes specified server's GS1 query port
// On success, update the server's extended params
// In case a server with specified identifier does not exit,
// create the server beforehand
func (s *DetailsProber) Probe(
	ctx context.Context,
	svr servers.Server,
	queryPort int,
	timeout time.Duration,
) (any, error) {
	addr := svr.GetAddr()
	qAddr := netip.AddrPortFrom(netip.AddrFrom4(addr.GetIP4()), uint16(queryPort))

	queryStarted := time.Now()

	resp, err := gs1.Query(ctx, qAddr, timeout)
	if err != nil {
		s.logger.Info().
			Err(err).
			Dur("timeout", timeout).Stringer("addr", addr).Int("port", queryPort).
			Msg("Failed to probe details")
		return details.Blank, err
	}

	queryDur := time.Since(queryStarted).Seconds()
	s.metrics.DiscoveryQueryDurations.Observe(queryDur)
	s.logger.Debug().
		Stringer("addr", addr).Int("port", queryPort).
		Float64("duration", queryDur).Stringer("version", resp.Version).
		Msg("Successfully queried server")

	svrDetails, err := details.NewDetailsFromParams(resp.Fields, resp.Players, resp.Objectives)
	if err != nil {
		s.logger.Error().
			Err(err).Stringer("addr", addr).Int("port", queryPort).
			Msg("Failed to parse query response")
		return details.Blank, err
	}
	if validateErr := svrDetails.Validate(s.validate); validateErr != nil {
		s.logger.Error().
			Err(validateErr).Stringer("addr", addr).Int("port", queryPort).
			Msg("Failed to validate query response")
		return details.Blank, validateErr
	}

	return svrDetails, nil
}

func (s *DetailsProber) HandleSuccess(result any, svr servers.Server) servers.Server {
	det, ok := result.(details.Details)
	if !ok {
		panic(fmt.Errorf("unexpected result type %T, %v", result, result))
	}
	svr.UpdateDetails(det)
	svr.UpdateDiscoveryStatus(ds.Info | ds.Details)
	svr.ClearDiscoveryStatus(ds.NoDetails | ds.DetailsRetry)
	return svr
}

func (s *DetailsProber) HandleRetry(svr servers.Server) servers.Server {
	svr.UpdateDiscoveryStatus(ds.DetailsRetry)
	return svr
}

func (s *DetailsProber) HandleFailure(svr servers.Server) servers.Server {
	svr.ClearDiscoveryStatus(ds.Details | ds.Info | ds.DetailsRetry | ds.Port)
	svr.UpdateDiscoveryStatus(ds.NoDetails)
	return svr
}
