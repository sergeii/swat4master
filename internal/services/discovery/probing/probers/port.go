package probers

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/entity/details"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
	"github.com/sergeii/swat4master/internal/services/monitoring"
	"github.com/sergeii/swat4master/pkg/gamespy/serverquery/gs1"
)

var ErrGlobalProbeTimeout = errors.New("global probe timeout reached")
var ErrPortProbesFailed = errors.New("all port probes failed")

type Result struct {
	details details.Details
	port    int
}

var NoResult Result

type response struct {
	Response gs1.Response
	Port     int
}

type PortProber struct {
	metrics *monitoring.MetricService
	offsets []int
}

type PortProberOpt func(pp *PortProber)

func WithPortOffsets(offsets []int) PortProberOpt {
	return func(pp *PortProber) {
		pp.offsets = offsets
	}
}

func NewPortProber(ms *monitoring.MetricService, opts ...PortProberOpt) *PortProber {
	pp := &PortProber{
		metrics: ms,
	}
	for _, opt := range opts {
		opt(pp)
	}
	return pp
}

// Probe attempts to discover a query port for a given server address.
// To discover the query port, several ports are tried: public port +1, +2 and so forth.
// In case when multiple query ports are available, the preferred port would be selected
// according to this order: gs1 mod, admin mod, vanilla response.
func (s *PortProber) Probe(
	ctx context.Context,
	svr servers.Server,
	_ int,
	timeout time.Duration,
) (any, error) {
	results := make(chan response)
	done := make(chan struct{})
	wg := &sync.WaitGroup{}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	svrAddr := svr.GetAddr()
	ip := netip.AddrFrom4(svrAddr.GetIP4())
	for _, pIdx := range s.offsets {
		wg.Add(1)
		go s.probePort(ctx, wg, results, ip, svrAddr.Port+pIdx, timeout)
	}

	go func() {
		wg.Wait()
		close(done)
	}()

	best, ok, err := s.collectResponses(results, done, timeout)
	if err != nil {
		log.Error().
			Err(err).Stringer("server", svr).
			Msg("Failed to collect port probe results")
		return NoResult, err
	} else if !ok {
		return NoResult, ErrPortProbesFailed
	}

	log.Debug().
		Stringer("server", svr).Stringer("version", best.Response.Version).Int("Port", best.Port).
		Msg("Selected preferred response")

	det, err := details.NewDetailsFromParams(best.Response.Fields, best.Response.Players, best.Response.Objectives)
	if err != nil {
		log.Error().
			Err(err).
			Stringer("server", svr).Stringer("version", best.Response.Version).Int("Port", best.Port).
			Msg("Unable to parse response")
		return NoResult, err
	}

	result := Result{
		details: det,
		port:    best.Port,
	}
	return result, nil
}

func (s *PortProber) probePort(
	ctx context.Context,
	wg *sync.WaitGroup,
	responses chan response,
	ip netip.Addr,
	port int,
	timeout time.Duration,
) {
	defer wg.Done()
	queryStarted := time.Now()

	resp, err := gs1.Query(ctx, netip.AddrPortFrom(ip, uint16(port)), timeout)
	if err != nil {
		log.Debug().
			Err(err).
			Dur("timeout", timeout).Stringer("ip", ip).Int("Port", port).
			Msg("Unable to probe port")
		return
	}

	queryDur := time.Since(queryStarted).Seconds()
	s.metrics.DiscoveryQueryDurations.Observe(queryDur)
	log.Debug().
		Stringer("ip", ip).Int("port", port).
		Stringer("version", resp.Version).Float64("duration", queryDur).
		Msg("Successfully probed port")

	responses <- response{resp, port}
}

func (s *PortProber) collectResponses(
	ch chan response,
	done chan struct{},
	timeout time.Duration,
) (response, bool, error) {
	var best response
	// this timeout should never trigger
	// because we expect query goroutines to stop within configured probe timeout
	// but in case of unexpected goroutine hangup, add this emergency timeout
	exitTimeout := time.After(timeout * 2)
	ok := false
	for {
		select {
		case <-done:
			return best, ok, nil
		case result := <-ch:
			best = s.compareResponses(best, result)
			ok = true
		case <-exitTimeout:
			return response{}, false, ErrGlobalProbeTimeout
		}
	}
}

func (s *PortProber) compareResponses(this, that response) response {
	if this.Response.Version > that.Response.Version {
		return this
	}
	return that
}

func (s *PortProber) HandleSuccess(res any, svr servers.Server) servers.Server {
	result, ok := res.(Result)
	if !ok {
		panic(fmt.Errorf("unexpected result type %T, %v", result, result))
	}
	svr.UpdateQueryPort(result.port)
	svr.UpdateDetails(result.details)
	svr.UpdateDiscoveryStatus(ds.Info | ds.Details | ds.Port)
	svr.ClearDiscoveryStatus(ds.NoDetails | ds.DetailsRetry | ds.PortRetry | ds.NoPort)
	return svr
}

func (s *PortProber) HandleRetry(svr servers.Server) servers.Server {
	svr.UpdateDiscoveryStatus(ds.PortRetry)
	return svr
}

func (s *PortProber) HandleFailure(svr servers.Server) servers.Server {
	svr.ClearDiscoveryStatus(ds.PortRetry)
	svr.UpdateDiscoveryStatus(ds.NoPort)
	return svr
}
