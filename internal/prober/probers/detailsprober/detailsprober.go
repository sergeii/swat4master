package detailsprober

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/entities/details"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/metrics"
	"github.com/sergeii/swat4master/pkg/gamespy/serverquery/gs1"
)

var (
	ErrQueryTimeout     = errors.New("query timeout")
	ErrQueryFailed      = errors.New("failed to query server")
	ErrParseFailed      = errors.New("failed to parse server query response")
	ErrValidationFailed = errors.New("failed to validate server query response")
)

type DetailsProber struct {
	metrics  *metrics.Collector
	validate *validator.Validate
	clock    clockwork.Clock
	logger   *zerolog.Logger
}

func New(
	validate *validator.Validate,
	clock clockwork.Clock,
	metrics *metrics.Collector,
	logger *zerolog.Logger,
) DetailsProber {
	return DetailsProber{
		validate: validate,
		clock:    clock,
		metrics:  metrics,
		logger:   logger,
	}
}

// Probe probes specified server's GS1 query port
// On success, update the server's extended params
// In case a server with specified identifier does not exit,
// create the server beforehand
func (p DetailsProber) Probe(
	ctx context.Context,
	svrAddr addr.Addr,
	queryPort int,
	timeout time.Duration,
) (any, error) {
	qAddr := netip.AddrPortFrom(netip.AddrFrom4(svrAddr.IP), uint16(queryPort)) // nolint:gosec

	queryStarted := time.Now()

	resp, err := gs1.Query(ctx, qAddr, timeout)
	if err != nil {
		p.logger.Info().
			Err(err).
			Dur("timeout", timeout).Stringer("addr", svrAddr).Int("port", queryPort).
			Msg("Failed to probe details")
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return details.Blank, fmt.Errorf("%w: %w", ErrQueryTimeout, err)
		}
		return details.Blank, fmt.Errorf("%w: %w", ErrQueryFailed, err)
	}

	queryDur := time.Since(queryStarted).Seconds()
	p.metrics.DiscoveryQueryDurations.Observe(queryDur)
	p.logger.Debug().
		Stringer("addr", svrAddr).Int("port", queryPort).
		Float64("duration", queryDur).Stringer("version", resp.Version).
		Msg("Successfully queried server")

	svrDetails, err := details.NewDetailsFromParams(resp.Fields, resp.Players, resp.Objectives)
	if err != nil {
		p.logger.Error().
			Err(err).Stringer("addr", svrAddr).Int("port", queryPort).
			Msg("Failed to parse query response")
		return details.Blank, fmt.Errorf("%w: %w", ErrParseFailed, err)
	}
	if validateErr := svrDetails.Validate(p.validate); validateErr != nil {
		p.logger.Error().
			Err(validateErr).Stringer("addr", svrAddr).Int("port", queryPort).
			Msg("Failed to validate query response")
		return details.Blank, fmt.Errorf("%w: %w", ErrValidationFailed, validateErr)
	}

	return svrDetails, nil
}

func (p DetailsProber) HandleSuccess(result any, svr server.Server) server.Server {
	det, ok := result.(details.Details)
	if !ok {
		panic(fmt.Errorf("unexpected result type %T, %v", result, result))
	}
	svr.UpdateDetails(det, p.clock.Now())
	svr.UpdateDiscoveryStatus(ds.Info | ds.Details)
	svr.ClearDiscoveryStatus(ds.NoDetails | ds.DetailsRetry)
	return svr
}

func (p DetailsProber) HandleRetry(svr server.Server) server.Server {
	svr.UpdateDiscoveryStatus(ds.DetailsRetry)
	return svr
}

func (p DetailsProber) HandleFailure(svr server.Server) server.Server {
	svr.ClearDiscoveryStatus(ds.Details | ds.Info | ds.DetailsRetry | ds.Port)
	svr.UpdateDiscoveryStatus(ds.NoDetails)
	return svr
}
