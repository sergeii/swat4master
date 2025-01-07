package portprober

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"sync"
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
	ErrPortDiscoveryFailed = errors.New("port discovery probes failed")
	ErrParseFailed         = errors.New("failed to parse discovered port response")
	ErrValidationFailed    = errors.New("failed to validate discovered port response")
)

type Result struct {
	Details details.Details
	Port    int
}

var NoResult Result

type response struct {
	Response gs1.Response
	Port     int
}

type Opts struct {
	Offsets []int
}

type PortProber struct {
	metrics  *metrics.Collector
	validate *validator.Validate
	clock    clockwork.Clock
	logger   *zerolog.Logger
	opts     Opts
}

func New(
	opts Opts,
	validate *validator.Validate,
	clock clockwork.Clock,
	metrics *metrics.Collector,
	logger *zerolog.Logger,
) PortProber {
	return PortProber{
		metrics:  metrics,
		validate: validate,
		clock:    clock,
		logger:   logger,
		opts:     opts,
	}
}

// Probe attempts to discover a query port for a given server address.
// To discover the query port, several ports are tried: public port +1, +2 and so forth.
// In case when multiple query ports are available, the preferred port would be selected
// according to this order: gs1 mod, admin mod, vanilla response.
func (p PortProber) Probe(
	ctx context.Context,
	svrAddr addr.Addr,
	_ int,
	timeout time.Duration,
) (any, error) {
	results := make(chan response)
	done := make(chan struct{})
	wg := &sync.WaitGroup{}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ip := netip.AddrFrom4(svrAddr.IP)
	for _, pIdx := range p.opts.Offsets {
		wg.Add(1)
		go p.probePort(ctx, wg, results, ip, svrAddr.Port, svrAddr.Port+pIdx, timeout)
	}

	go func() {
		wg.Wait()
		close(done)
	}()

	best, ok := p.collectResponses(results, done, timeout)
	if !ok {
		return NoResult, ErrPortDiscoveryFailed
	}

	p.logger.Debug().
		Stringer("addr", svrAddr).Stringer("version", best.Response.Version).Int("port", best.Port).
		Msg("Selected preferred response")

	det, err := details.NewDetailsFromParams(best.Response.Fields, best.Response.Players, best.Response.Objectives)
	if err != nil {
		p.logger.Error().
			Err(err).
			Stringer("addr", svrAddr).Stringer("version", best.Response.Version).Int("port", best.Port).
			Msg("Unable to parse response")
		return NoResult, fmt.Errorf("%w: %w", ErrParseFailed, err)
	}
	if validateErr := det.Validate(p.validate); validateErr != nil {
		p.logger.Error().
			Err(validateErr).
			Stringer("addr", svrAddr).Stringer("version", best.Response.Version).Int("Port", best.Port).
			Msg("Failed to validate query response")
		return details.Blank, fmt.Errorf("%w: %w", ErrValidationFailed, validateErr)
	}

	result := Result{
		Details: det,
		Port:    best.Port,
	}
	return result, nil
}

func (p PortProber) probePort(
	ctx context.Context,
	wg *sync.WaitGroup,
	responses chan response,
	ip netip.Addr,
	gamePort int,
	queryPort int,
	timeout time.Duration,
) {
	defer wg.Done()
	queryStarted := time.Now()

	resp, err := gs1.Query(ctx, netip.AddrPortFrom(ip, uint16(queryPort)), timeout) // nolint:gosec
	if err != nil {
		p.logger.Debug().
			Err(err).
			Dur("timeout", timeout).Stringer("ip", ip).Int("Port", queryPort).
			Msg("Unable to probe port")
		return
	}

	hostPort, err := strconv.Atoi(resp.Fields["hostport"])
	switch {
	case err != nil:
		p.logger.Error().
			Err(err).
			Stringer("ip", ip).Int("port", queryPort).
			Msg("Unable to parse server hostport")
		return
	case hostPort != gamePort:
		p.logger.Warn().
			Stringer("ip", ip).Int("port", queryPort).
			Int("hostport", hostPort).Int("gameport", gamePort).
			Msg("Server ports dont match")
		return
	}

	queryDur := time.Since(queryStarted).Seconds()
	p.metrics.DiscoveryQueryDurations.Observe(queryDur)
	p.logger.Debug().
		Stringer("ip", ip).Int("port", queryPort).
		Stringer("version", resp.Version).Float64("duration", queryDur).
		Msg("Successfully probed port")

	responses <- response{resp, queryPort}
}

func (p PortProber) collectResponses(
	ch chan response,
	done chan struct{},
	timeout time.Duration,
) (response, bool) {
	var best response
	// this timeout should never trigger
	// because we expect query goroutines to stop within configured probe timeout
	// but in case of unexpected goroutine hangup, add this emergency timeout
	exitTimeout := time.After(timeout * 2)
	ok := false
	for {
		select {
		case <-done:
			return best, ok
		case result := <-ch:
			best = p.compareResponses(best, result)
			ok = true
		case <-exitTimeout:
			return response{}, false
		}
	}
}

func (p PortProber) compareResponses(this, that response) response {
	if this.Response.Version > that.Response.Version {
		return this
	}
	return that
}

func (p PortProber) HandleSuccess(res any, svr server.Server) server.Server {
	result, ok := res.(Result)
	if !ok {
		panic(fmt.Errorf("unexpected result type %T, %v", result, result))
	}
	svr.QueryPort = result.Port
	svr.UpdateDetails(result.Details, p.clock.Now())
	svr.UpdateDiscoveryStatus(ds.Info | ds.Details | ds.Port)
	svr.ClearDiscoveryStatus(ds.NoDetails | ds.DetailsRetry | ds.PortRetry | ds.NoPort)
	return svr
}

func (p PortProber) HandleRetry(svr server.Server) server.Server {
	svr.UpdateDiscoveryStatus(ds.PortRetry)
	return svr
}

func (p PortProber) HandleFailure(svr server.Server) server.Server {
	svr.ClearDiscoveryStatus(ds.PortRetry)
	svr.UpdateDiscoveryStatus(ds.NoPort)
	return svr
}
