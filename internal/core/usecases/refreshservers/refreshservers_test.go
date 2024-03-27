package refreshservers_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/filterset"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/core/usecases/refreshservers"
	"github.com/sergeii/swat4master/internal/testutils/factories"
)

type MockServerRepository struct {
	mock.Mock
	repositories.ServerRepository
}

func (m *MockServerRepository) Filter(ctx context.Context, fs filterset.FilterSet) ([]server.Server, error) {
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

func TestRefreshServersUseCase_Success(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()

	now := time.Now()
	deadline := now.Add(time.Minute * 10)

	svr1 := factories.BuildRandomServer()
	svr2 := factories.BuildRandomServer()
	serversToRevive := []server.Server{svr1, svr2}

	serverRepo := new(MockServerRepository)
	serverRepo.On("Filter", ctx, mock.Anything).Return(serversToRevive, nil).Once()

	probeRepo := new(MockProbeRepository)
	probeRepo.On("AddBetween", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil).Twice()

	uc := refreshservers.New(serverRepo, probeRepo, &logger)
	resp, err := uc.Execute(ctx, deadline)

	assert.NoError(t, err)
	assert.Equal(t, 2, resp.Count)

	serverRepo.AssertExpectations(t)
	probeRepo.AssertExpectations(t)

	serverRepo.AssertCalled(
		t,
		"Filter",
		ctx,
		mock.MatchedBy(func(fs filterset.FilterSet) bool {
			noStatus, hasNoStatus := fs.GetNoStatus()
			withStatus, hasWithStatus := fs.GetWithStatus()
			wantNoStatus := hasNoStatus && (noStatus == ds.DetailsRetry)
			wantWithStatus := hasWithStatus && (withStatus == ds.Port)
			return wantWithStatus && wantNoStatus
		}),
	)
	for _, svr := range serversToRevive {
		probeRepo.AssertCalled(
			t,
			"AddBetween",
			ctx,
			probe.New(svr.Addr, svr.QueryPort, probe.GoalDetails),
			repositories.NC,
			deadline,
		)
	}
}

func TestRefreshServersUseCase_FilterError(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()

	filterErr := errors.New("filter error")

	serverRepo := new(MockServerRepository)
	serverRepo.On("Filter", ctx, mock.Anything).Return([]server.Server{}, filterErr).Once()

	probeRepo := new(MockProbeRepository)

	uc := refreshservers.New(serverRepo, probeRepo, &logger)
	resp, err := uc.Execute(ctx, time.Now())

	assert.ErrorIs(t, err, filterErr)
	assert.Equal(t, 0, resp.Count)

	serverRepo.AssertExpectations(t)
	probeRepo.AssertExpectations(t)

	probeRepo.AssertNotCalled(t, "AddBetween", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestRefreshServersUseCase_AddProbeError(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()

	addProbeErr := errors.New("probe error")

	svr1 := factories.BuildRandomServer()
	svr2 := factories.BuildRandomServer()
	serversToRevive := []server.Server{svr1, svr2}

	serverRepo := new(MockServerRepository)
	serverRepo.On("Filter", ctx, mock.Anything).Return(serversToRevive, nil).Once()

	probeRepo := new(MockProbeRepository)
	probeRepo.On("AddBetween", ctx, mock.Anything, mock.Anything, mock.Anything).Return(addProbeErr).Twice()

	uc := refreshservers.New(serverRepo, probeRepo, &logger)
	resp, err := uc.Execute(ctx, time.Now())

	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Count)

	serverRepo.AssertExpectations(t)
	probeRepo.AssertExpectations(t)
}
