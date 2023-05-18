package probers

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
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

var (
	ErrGlobalProbeTimeout = errors.New("global probe timeout reached")
	ErrPortProbesFailed   = errors.New("all port probes failed")
)

type Result struct {
	details details.Details
	port    int
}

var NoResult Result

type response struct {
	Response gs1.Response
	Port     int
}

type PortProberOpts struct {
	Offsets []int
}

type PortProber struct {
	metrics  *monitoring.MetricService
	validate *validator.Validate
	clock    clock.Clock
	logger   *zerolog.Logger
	opts     PortProberOpts
}

func NewPortProber(
	service *probing.Service,
	metrics *monitoring.MetricService,
	validate *validator.Validate,
	clock clock.Clock,
	logger *zerolog.Logger,
	opts PortProberOpts,
) (*PortProber, error) {
	pp := &PortProber{
		metrics:  metrics,
		validate: validate,
		clock:    clock,
		logger:   logger,
		opts:     opts,
	}
	if err := service.Register(probes.GoalPort, pp); err != nil {
		return nil, err
	}
	return pp, nil
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
	for _, pIdx := range s.opts.Offsets {
		wg.Add(1)
		go s.probePort(ctx, wg, results, ip, svrAddr.Port, svrAddr.Port+pIdx, timeout)
	}

	go func() {
		wg.Wait()
		close(done)
	}()

	best, ok, err := s.collectResponses(results, done, timeout)
	if err != nil {
		s.logger.Error().
			Err(err).Stringer("server", svr).
			Msg("Failed to collect port probe results")
		return NoResult, err
	} else if !ok {
		return NoResult, ErrPortProbesFailed
	}

	s.logger.Debug().
		Stringer("server", svr).Stringer("version", best.Response.Version).Int("Port", best.Port).
		Msg("Selected preferred response")

	det, err := details.NewDetailsFromParams(best.Response.Fields, best.Response.Players, best.Response.Objectives)
	if err != nil {
		s.logger.Error().
			Err(err).
			Stringer("server", svr).Stringer("version", best.Response.Version).Int("Port", best.Port).
			Msg("Unable to parse response")
		return NoResult, err
	}
	if validateErr := det.Validate(s.validate); validateErr != nil {
		s.logger.Error().
			Err(validateErr).
			Stringer("server", svr).Stringer("version", best.Response.Version).Int("Port", best.Port).
			Msg("Failed to validate query response")
		return details.Blank, validateErr
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
	gamePort int,
	queryPort int,
	timeout time.Duration,
) {
	defer wg.Done()
	queryStarted := s.clock.Now()

	resp, err := gs1.Query(ctx, netip.AddrPortFrom(ip, uint16(queryPort)), timeout)
	if err != nil {
		s.logger.Debug().
			Err(err).
			Dur("timeout", timeout).Stringer("ip", ip).Int("Port", queryPort).
			Msg("Unable to probe port")
		return
	}

	hostPort, err := strconv.Atoi(resp.Fields["hostport"])
	switch {
	case err != nil:
		s.logger.Error().
			Err(err).
			Stringer("ip", ip).Int("port", queryPort).
			Msg("Unable to parse server hostport")
		return
	case hostPort != gamePort:
		s.logger.Warn().
			Stringer("ip", ip).Int("port", queryPort).
			Int("hostport", hostPort).Int("gameport", gamePort).
			Msg("Server ports dont match")
		return
	}

	queryDur := s.clock.Since(queryStarted).Seconds()
	s.metrics.DiscoveryQueryDurations.Observe(queryDur)
	s.logger.Debug().
		Stringer("ip", ip).Int("port", queryPort).
		Stringer("version", resp.Version).Float64("duration", queryDur).
		Msg("Successfully probed port")

	responses <- response{resp, queryPort}
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
	exitTimeout := s.clock.After(timeout * 2)
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
	svr.UpdateDetails(result.details, s.clock.Now())
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
