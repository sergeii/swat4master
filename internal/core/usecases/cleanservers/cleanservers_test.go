package cleanservers_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/entities/filterset"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/core/usecases/cleanservers"
	"github.com/sergeii/swat4master/internal/testutils/factories"
)

type MockServerRepository struct {
	mock.Mock
	repositories.ServerRepository
}

func (m *MockServerRepository) Count(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *MockServerRepository) Filter(
	ctx context.Context,
	fs filterset.FilterSet,
) ([]server.Server, error) {
	args := m.Called(ctx, fs)
	return args.Get(0).([]server.Server), args.Error(1) // nolint: forcetypeassert
}

func (m *MockServerRepository) Remove(
	ctx context.Context,
	svr server.Server,
	onConflict func(*server.Server) bool,
) error {
	args := m.Called(ctx, svr, onConflict)
	return args.Error(0)
}

type MockInstanceRepository struct {
	mock.Mock
	repositories.InstanceRepository
}

func (m *MockInstanceRepository) RemoveByAddr(ctx context.Context, addr addr.Addr) error {
	args := m.Called(ctx, addr)
	return args.Error(0)
}

func TestCleanServersUseCase_Success(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()

	until := time.Now().Add(-24 * time.Hour) // Example time filter

	outdatedServers := []server.Server{
		factories.BuildRandomServer(),
		factories.BuildRandomServer(),
	}

	serverRepo := new(MockServerRepository)
	serverRepo.On("Count", ctx).Return(10, nil).Once()
	serverRepo.On("Count", ctx).Return(8, nil).Once()
	serverRepo.On("Filter", ctx, mock.Anything).Return(outdatedServers, nil).Once()
	serverRepo.On("Remove", ctx, mock.Anything, mock.Anything).Return(nil).Times(2)

	instanceRepo := new(MockInstanceRepository)
	instanceRepo.On("RemoveByAddr", ctx, mock.Anything).Return(nil).Times(2)

	uc := cleanservers.New(serverRepo, instanceRepo, &logger)
	response, err := uc.Execute(ctx, until)

	assert.NoError(t, err)
	assert.Equal(t, 2, response.Count)
	assert.Equal(t, 0, response.Errors)

	serverRepo.AssertExpectations(t)
	instanceRepo.AssertExpectations(t)

	serverRepo.AssertCalled(
		t,
		"Filter",
		ctx,
		mock.MatchedBy(func(fs filterset.FilterSet) bool {
			updatedBefore, _ := fs.GetUpdatedBefore()
			return updatedBefore.Equal(until)
		}),
	)
	for _, svr := range outdatedServers {
		serverRepo.AssertCalled(t, "Remove", ctx, svr, mock.Anything)
		instanceRepo.AssertCalled(t, "RemoveByAddr", ctx, svr.Addr)
	}
}

func TestCleanServersUseCase_NothingToClean(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()

	until := time.Now().Add(-24 * time.Hour) // Example time filter

	serverRepo := new(MockServerRepository)
	serverRepo.On("Count", ctx).Return(0, nil).Times(2)
	serverRepo.On("Filter", ctx, mock.Anything).Return([]server.Server{}, nil).Once()

	instanceRepo := new(MockInstanceRepository)

	uc := cleanservers.New(serverRepo, instanceRepo, &logger)
	response, err := uc.Execute(ctx, until)

	assert.NoError(t, err)
	assert.Equal(t, 0, response.Count)
	assert.Equal(t, 0, response.Errors)

	serverRepo.AssertExpectations(t)
	instanceRepo.AssertExpectations(t)

	serverRepo.AssertNotCalled(t, "Remove", mock.Anything, mock.Anything, mock.Anything)
	instanceRepo.AssertNotCalled(t, "RemoveByAddr", mock.Anything, mock.Anything)
}

func TestCleanServersUseCase_RemoveErrors(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()

	until := time.Now().Add(-24 * time.Hour) // Example time filter

	svr1 := factories.BuildRandomServer()
	svr2 := factories.BuildRandomServer()
	svr3 := factories.BuildRandomServer()
	outdatedServers := []server.Server{svr1, svr2, svr3}

	serverRepo := new(MockServerRepository)
	serverRepo.On("Count", ctx).Return(3, nil).Once()
	serverRepo.On("Count", ctx).Return(2, nil).Once()
	serverRepo.On("Filter", ctx, mock.Anything).Return(outdatedServers, nil).Once()
	serverRepo.On("Remove", ctx, svr2, mock.Anything).Return(nil).Once()
	serverRepo.On("Remove", ctx, svr3, mock.Anything).Return(errors.New("error")).Once()

	instanceRepo := new(MockInstanceRepository)
	instanceRepo.On("RemoveByAddr", ctx, svr1.Addr).Return(errors.New("error")).Once()
	instanceRepo.On("RemoveByAddr", ctx, svr2.Addr).Return(nil).Once()
	instanceRepo.On("RemoveByAddr", ctx, svr3.Addr).Return(nil).Once()

	uc := cleanservers.New(serverRepo, instanceRepo, &logger)
	response, err := uc.Execute(ctx, until)

	assert.NoError(t, err)
	assert.Equal(t, 1, response.Count)
	assert.Equal(t, 2, response.Errors)

	serverRepo.AssertExpectations(t)
	instanceRepo.AssertExpectations(t)

	serverRepo.AssertNumberOfCalls(t, "Remove", 2)
	instanceRepo.AssertNumberOfCalls(t, "RemoveByAddr", 3)
}

func TestCleanServersUseCase_CountError(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()

	until := time.Now().Add(-24 * time.Hour) // Example time filter
	countErr := errors.New("error")

	serverRepo := new(MockServerRepository)
	serverRepo.On("Count", ctx).Return(0, countErr).Once()

	instanceRepo := new(MockInstanceRepository)

	uc := cleanservers.New(serverRepo, instanceRepo, &logger)
	response, err := uc.Execute(ctx, until)

	assert.ErrorIs(t, err, countErr)
	assert.Equal(t, cleanservers.NoResponse, response)

	serverRepo.AssertExpectations(t)
	instanceRepo.AssertExpectations(t)

	serverRepo.AssertNumberOfCalls(t, "Filter", 0)
	serverRepo.AssertNumberOfCalls(t, "Remove", 0)
	instanceRepo.AssertNumberOfCalls(t, "RemoveByAddr", 0)
}
