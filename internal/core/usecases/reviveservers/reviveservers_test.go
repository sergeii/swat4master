package reviveservers_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/filterset"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/core/usecases/reviveservers"
	"github.com/sergeii/swat4master/internal/metrics"
	"github.com/sergeii/swat4master/internal/testutils/factories/serverfactory"
)

type MockServerRepository struct {
	mock.Mock
	repositories.ServerRepository
}

func (m *MockServerRepository) Filter(ctx context.Context, fs filterset.ServerFilterSet) ([]server.Server, error) {
	args := m.Called(ctx, fs)
	return args.Get(0).([]server.Server), args.Error(1) // nolint: forcetypeassert
}

type MockProbeRepository struct {
	mock.Mock
	repositories.ProbeRepository
}

func (m *MockProbeRepository) AddBetween(ctx context.Context, prb probe.Probe, countdown, deadline time.Time) error {
	args := m.Called(ctx, prb, countdown, deadline)
	return args.Error(0)
}

func TestReviveServersUseCase_Success(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()
	collector := metrics.New()

	now := time.Now()
	minScope := now.Add(-time.Hour)
	maxScope := now.Add(-time.Minute * 10)
	minCountdown := now
	maxCountdown := now.Add(time.Minute * 5)
	deadline := now.Add(time.Minute * 10)

	svr1 := serverfactory.BuildRandom()
	svr2 := serverfactory.BuildRandom()
	serversToRevive := []server.Server{svr1, svr2}

	serverRepo := new(MockServerRepository)
	serverRepo.On("Filter", ctx, mock.Anything).Return(serversToRevive, nil).Once()

	probeRepo := new(MockProbeRepository)
	probeRepo.On("AddBetween", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil).Twice()

	ucOpts := reviveservers.UseCaseOptions{
		MaxProbeRetries: 3,
	}
	uc := reviveservers.New(serverRepo, probeRepo, ucOpts, collector, &logger)

	req := reviveservers.NewRequest(minScope, maxScope, minCountdown, maxCountdown, deadline)
	resp, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.Equal(t, 2, resp.Count)

	serverRepo.AssertExpectations(t)
	probeRepo.AssertExpectations(t)
	serverRepo.AssertCalled(
		t,
		"Filter",
		ctx,
		mock.MatchedBy(func(fs filterset.ServerFilterSet) bool {
			noStatus, hasNoStatus := fs.GetNoStatus()
			activeAfter, hasActiveAfter := fs.GetActiveAfter()
			activeBefore, hasActiveBefore := fs.GetActiveBefore()
			wantNoStatus := hasNoStatus && (noStatus == ds.Port|ds.PortRetry)
			wantActiveAfter := hasActiveAfter && activeAfter.Equal(req.MinScope)
			wantActiveBefore := hasActiveBefore && activeBefore.Equal(req.MaxScope)
			return wantNoStatus && wantActiveAfter && wantActiveBefore
		}),
	)
	for _, svr := range serversToRevive {
		probeRepo.AssertCalled(
			t,
			"AddBetween",
			ctx,
			probe.New(svr.Addr, svr.Addr.Port, probe.GoalPort, 3),
			mock.MatchedBy(func(countdown time.Time) bool {
				gteMinCountdown := countdown.Equal(req.MinCountdown) || countdown.After(req.MinCountdown)
				lteMaxCountdown := countdown.Equal(req.MaxCountdown) || countdown.Before(req.MaxCountdown)
				return gteMinCountdown && lteMaxCountdown
			}),
			deadline,
		)
	}

	probesProducedMetricValue := testutil.ToFloat64(collector.DiscoveryQueueProduced)
	assert.Equal(t, float64(2), probesProducedMetricValue)
}

func TestReviveServersUseCase_FilterError(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()
	collector := metrics.New()

	now := time.Now()
	filterErr := errors.New("filter error")

	serverRepo := new(MockServerRepository)
	serverRepo.On("Filter", ctx, mock.Anything).Return([]server.Server{}, filterErr).Once()

	probeRepo := new(MockProbeRepository)

	ucOpts := reviveservers.UseCaseOptions{
		MaxProbeRetries: 3,
	}
	uc := reviveservers.New(serverRepo, probeRepo, ucOpts, collector, &logger)

	req := reviveservers.NewRequest(
		now.Add(-time.Hour),
		now.Add(-time.Minute*10),
		now,
		now.Add(time.Minute*5),
		now.Add(time.Minute*10),
	)
	resp, err := uc.Execute(ctx, req)

	assert.ErrorIs(t, err, filterErr)
	assert.Equal(t, 0, resp.Count)

	serverRepo.AssertExpectations(t)
	probeRepo.AssertExpectations(t)
	probeRepo.AssertNotCalled(t, "AddBetween", mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	probesProducedMetricValue := testutil.ToFloat64(collector.DiscoveryQueueProduced)
	assert.Equal(t, float64(0), probesProducedMetricValue)
}

func TestReviveServersUseCase_AddProbeError(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()
	collector := metrics.New()

	now := time.Now()
	addProbeErr := errors.New("probe error")

	svr1 := serverfactory.BuildRandom()
	svr2 := serverfactory.BuildRandom()
	serversToRevive := []server.Server{svr1, svr2}

	serverRepo := new(MockServerRepository)
	serverRepo.On("Filter", ctx, mock.Anything).Return(serversToRevive, nil).Once()

	probeRepo := new(MockProbeRepository)
	probeRepo.On("AddBetween", ctx, mock.Anything, mock.Anything, mock.Anything).Return(addProbeErr).Twice()

	ucOpts := reviveservers.UseCaseOptions{
		MaxProbeRetries: 3,
	}
	uc := reviveservers.New(serverRepo, probeRepo, ucOpts, collector, &logger)

	req := reviveservers.NewRequest(
		now.Add(-time.Hour),
		now.Add(-time.Minute*10),
		now,
		now.Add(time.Minute*5),
		now.Add(time.Minute*10),
	)
	resp, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Count)

	serverRepo.AssertExpectations(t)
	probeRepo.AssertExpectations(t)

	probesProducedMetricValue := testutil.ToFloat64(collector.DiscoveryQueueProduced)
	assert.Equal(t, float64(0), probesProducedMetricValue)
}
