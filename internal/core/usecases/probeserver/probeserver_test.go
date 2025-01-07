package probeserver_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/core/usecases/probeserver"
	"github.com/sergeii/swat4master/internal/metrics"
	"github.com/sergeii/swat4master/internal/prober/probers"
	"github.com/sergeii/swat4master/internal/testutils/factories/serverfactory"
)

type MockServerRepository struct {
	mock.Mock
	repositories.ServerRepository
}

func (m *MockServerRepository) Get(ctx context.Context, addr addr.Addr) (server.Server, error) {
	args := m.Called(ctx, addr)
	return args.Get(0).(server.Server), args.Error(1) // nolint: forcetypeassert
}

func (m *MockServerRepository) Update(
	ctx context.Context,
	svr server.Server,
	onConflict func(*server.Server) bool,
) (server.Server, error) {
	args := m.Called(ctx, svr, onConflict)
	return args.Get(0).(server.Server), args.Error(1) // nolint: forcetypeassert
}

type MockProbeRepository struct {
	mock.Mock
	repositories.ProbeRepository
}

func (m *MockProbeRepository) AddBetween(
	ctx context.Context,
	prb probe.Probe,
	before time.Time,
	after time.Time,
) error {
	args := m.Called(ctx, prb, before, after)
	return args.Error(0)
}

type MockProber struct {
	mock.Mock
	probers.Prober
}

type MockProberProbeResult struct {
	Success bool
}

func (p *MockProber) Probe(
	ctx context.Context,
	addr addr.Addr,
	queryPort int,
	timeout time.Duration,
) (any, error) {
	args := p.Called(ctx, addr, queryPort, timeout)
	return args.Get(0).(MockProberProbeResult), args.Error(1) // nolint: forcetypeassert
}

func (p *MockProber) HandleSuccess(result any, svr server.Server) server.Server {
	args := p.Called(result, svr)
	return args.Get(0).(server.Server) // nolint: forcetypeassert
}

func (p *MockProber) HandleRetry(svr server.Server) server.Server {
	args := p.Called(svr)
	return args.Get(0).(server.Server) // nolint: forcetypeassert
}

func (p *MockProber) HandleFailure(svr server.Server) server.Server {
	args := p.Called(svr)
	return args.Get(0).(server.Server) // nolint: forcetypeassert
}

func TestProbeServerUseCase_Success(t *testing.T) {
	ctx := context.TODO()
	clock := clockwork.NewFakeClock()
	logger := zerolog.Nop()
	collector := metrics.New()

	svr := serverfactory.Build(serverfactory.WithAddress("1.1.1.1", 10480))
	prb := probe.New(svr.Addr, svr.QueryPort, probe.GoalDetails, 3)
	probeResult := MockProberProbeResult{Success: true}

	serverRepo := new(MockServerRepository)
	serverRepo.On("Get", ctx, svr.Addr).Return(svr, nil)
	serverRepo.On("Update", ctx, svr, mock.Anything).Return(svr, nil)

	probeRepo := new(MockProbeRepository)

	proberMock := new(MockProber)
	proberMock.On("Probe", ctx, svr.Addr, svr.QueryPort, mock.Anything).Return(probeResult, nil)
	proberMock.On("HandleSuccess", probeResult, svr).Return(svr)

	uc := probeserver.New(serverRepo, probeRepo, collector, clock, &logger)

	ucReq := probeserver.NewRequest(prb, proberMock, time.Millisecond*12345)
	err := uc.Execute(ctx, ucReq)
	require.NoError(t, err)

	probesProducedMetricValue := testutil.ToFloat64(collector.DiscoveryQueueProduced)
	assert.Equal(t, float64(0), probesProducedMetricValue)

	serverRepo.AssertExpectations(t)
	proberMock.AssertExpectations(t)
	probeRepo.AssertExpectations(t)
}

func TestProbeServerUseCase_RetryOnFailure(t *testing.T) {
	tests := []struct {
		name        string
		initRetries int
		wantRetries int
		wantDelay   time.Duration
	}{
		{
			"first retry",
			0,
			1,
			time.Second * 2,
		},
		{
			"third retry",
			2,
			3,
			time.Second * 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			clock := clockwork.NewFakeClock()
			logger := zerolog.Nop()
			collector := metrics.New()

			svr := serverfactory.Build(serverfactory.WithAddress("1.1.1.1", 10480))
			prb := probe.Probe{
				Addr:       svr.Addr,
				Port:       svr.QueryPort,
				Goal:       probe.GoalDetails,
				Retries:    tt.initRetries,
				MaxRetries: 3,
			}
			probeError := errors.New("probing error")

			serverRepo := new(MockServerRepository)
			serverRepo.On("Get", ctx, svr.Addr).Return(svr, nil)
			serverRepo.On("Update", ctx, svr, mock.Anything).Return(svr, nil)

			probeRepo := new(MockProbeRepository)
			probeRepo.On("AddBetween", ctx, mock.Anything, mock.Anything, repositories.NC).Return(nil)

			proberMock := new(MockProber)
			proberMock.On("Probe", ctx, svr.Addr, svr.QueryPort, mock.Anything).Return(MockProberProbeResult{}, probeError)
			proberMock.On("HandleRetry", svr).Return(svr)

			uc := probeserver.New(serverRepo, probeRepo, collector, clock, &logger)

			ucReq := probeserver.NewRequest(prb, proberMock, time.Millisecond*12345)
			err := uc.Execute(ctx, ucReq)
			require.ErrorIs(t, err, probeserver.ErrProbeRetried)

			probesProducedMetricValue := testutil.ToFloat64(collector.DiscoveryQueueProduced)
			assert.Equal(t, float64(1), probesProducedMetricValue)

			proberMock.AssertExpectations(t)
			serverRepo.AssertExpectations(t)
			probeRepo.AssertExpectations(t)
			probeRepo.AssertCalled(
				t,
				"AddBetween",
				ctx,
				probe.Probe{
					Addr:       svr.Addr,
					Port:       svr.QueryPort,
					Goal:       probe.GoalDetails,
					Retries:    tt.wantRetries,
					MaxRetries: 3,
				},
				clock.Now().Add(tt.wantDelay),
				repositories.NC,
			)
		})
	}
}

func TestProbeServerUseCase_FailOnOutOfRetries(t *testing.T) {
	ctx := context.TODO()
	clock := clockwork.NewFakeClock()
	logger := zerolog.Nop()
	collector := metrics.New()

	svr := serverfactory.Build(serverfactory.WithAddress("1.1.1.1", 10480))
	prb := probe.Probe{
		Addr:       svr.Addr,
		Port:       svr.QueryPort,
		Goal:       probe.GoalDetails,
		Retries:    3,
		MaxRetries: 3,
	}
	probeError := errors.New("probing error")

	serverRepo := new(MockServerRepository)
	serverRepo.On("Get", ctx, svr.Addr).Return(svr, nil)
	serverRepo.On("Update", ctx, svr, mock.Anything).Return(svr, nil)

	probeRepo := new(MockProbeRepository)

	proberMock := new(MockProber)
	proberMock.On("Probe", ctx, svr.Addr, svr.QueryPort, mock.Anything).Return(MockProberProbeResult{}, probeError)
	proberMock.On("HandleFailure", svr).Return(svr)

	uc := probeserver.New(serverRepo, probeRepo, collector, clock, &logger)

	ucReq := probeserver.NewRequest(prb, proberMock, time.Millisecond*12345)
	err := uc.Execute(ctx, ucReq)
	require.ErrorIs(t, err, probeserver.ErrOutOfRetries)

	probesProducedMetricValue := testutil.ToFloat64(collector.DiscoveryQueueProduced)
	assert.Equal(t, float64(0), probesProducedMetricValue)

	serverRepo.AssertExpectations(t)
	proberMock.AssertExpectations(t)
	probeRepo.AssertExpectations(t)
}
