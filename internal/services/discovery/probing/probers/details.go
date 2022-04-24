package probers

import (
	"context"
	"net/netip"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/entity/details"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
	"github.com/sergeii/swat4master/internal/services/monitoring"
	"github.com/sergeii/swat4master/pkg/gamespy/serverquery/gs1"
)

type DetailsProber struct {
	metrics *monitoring.MetricService
}

func NewDetailsProber(metrics *monitoring.MetricService) *DetailsProber {
	return &DetailsProber{metrics}
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
) (servers.Server, error) {
	addr := svr.GetAddr()
	qAddr := netip.AddrPortFrom(netip.AddrFrom4(addr.GetIP4()), uint16(queryPort))

	queryStarted := time.Now()

	resp, err := gs1.Query(ctx, qAddr, timeout)
	if err != nil {
		log.Info().
			Err(err).
			Dur("timeout", timeout).Stringer("addr", addr).Int("port", queryPort).
			Msg("Failed to probe details")
		return servers.Blank, err
	}

	queryDur := time.Since(queryStarted).Seconds()
	s.metrics.DiscoveryQueryDurations.Observe(queryDur)
	log.Debug().
		Stringer("addr", addr).Int("port", queryPort).
		Float64("duration", queryDur).Stringer("version", resp.Version).
		Msg("Successfully queried server")

	svrDetails, err := details.NewDetailsFromParams(resp.Fields, resp.Players, resp.Objectives)
	if err != nil {
		log.Error().
			Err(err).Stringer("addr", addr).Int("port", queryPort).
			Msg("Failed to parse query response")
		return servers.Blank, err
	}

	svr.UpdateInfo(svrDetails.Info)
	svr.UpdateDetails(svrDetails)
	svr.UpdateDiscoveryStatus(ds.Info | ds.Details)
	svr.ClearDiscoveryStatus(ds.NoDetails | ds.DetailsRetry)

	return svr, nil
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
