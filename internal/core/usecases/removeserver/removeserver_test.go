package removeserver_test

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/entities/instance"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/core/usecases/removeserver"
	"github.com/sergeii/swat4master/internal/testutils"
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

func (m *MockInstanceRepository) GetByID(ctx context.Context, instanceID string) (instance.Instance, error) {
	args := m.Called(ctx, instanceID)
	return args.Get(0).(instance.Instance), args.Error(1) // nolint: forcetypeassert
}

func (m *MockInstanceRepository) RemoveByID(ctx context.Context, instanceID string) error {
	args := m.Called(ctx, instanceID)
	return args.Error(0)
}

func TestRemoveServerUseCase_Success(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()

	svr := serverfactory.BuildRandom()
	inst := instance.MustNew("foo", svr.Addr.GetIP(), svr.Addr.Port)

	serverRepo := new(MockServerRepository)
	serverRepo.On("Get", ctx, svr.Addr).Return(svr, nil)
	serverRepo.On("Remove", ctx, svr, mock.Anything).Return(nil)

	instanceRepo := new(MockInstanceRepository)
	instanceRepo.On("GetByID", ctx, "foo").Return(inst, nil)
	instanceRepo.On("RemoveByID", ctx, "foo").Return(nil)

	uc := removeserver.New(serverRepo, instanceRepo, &logger)
	err := uc.Execute(ctx, removeserver.NewRequest("foo", svr.Addr))
	assert.NoError(t, err)

	serverRepo.AssertCalled(t, "Get", ctx, svr.Addr)
	instanceRepo.AssertCalled(t, "GetByID", ctx, "foo")
	serverRepo.AssertCalled(t, "Remove", ctx, svr, mock.Anything)
	instanceRepo.AssertCalled(t, "RemoveByID", ctx, "foo")
}

func TestRemoveServerUseCase_ServerAlreadyDeleted(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()

	svr := serverfactory.BuildRandom()

	serverRepo := new(MockServerRepository)
	serverRepo.On("Get", ctx, svr.Addr).Return(server.Blank, repositories.ErrServerNotFound)

	instanceRepo := new(MockInstanceRepository)

	uc := removeserver.New(serverRepo, instanceRepo, &logger)
	err := uc.Execute(ctx, removeserver.NewRequest("foo", svr.Addr))
	assert.ErrorIs(t, err, removeserver.ErrServerNotFound)

	serverRepo.AssertCalled(t, "Get", ctx, svr.Addr)
	instanceRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
	serverRepo.AssertNotCalled(t, "Remove", mock.Anything, mock.Anything, mock.Anything)
	instanceRepo.AssertNotCalled(t, "RemoveByID", mock.Anything, mock.Anything)
}

func TestRemoveServerUseCase_InstanceAlreadyDeleted(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()

	svr := serverfactory.BuildRandom()

	serverRepo := new(MockServerRepository)
	serverRepo.On("Get", ctx, svr.Addr).Return(svr, nil)
	serverRepo.On("Remove", ctx, svr, mock.Anything).Return(nil)

	instanceRepo := new(MockInstanceRepository)
	instanceRepo.On("GetByID", ctx, "foo").Return(instance.Blank, repositories.ErrInstanceNotFound)

	uc := removeserver.New(serverRepo, instanceRepo, &logger)
	err := uc.Execute(ctx, removeserver.NewRequest("foo", svr.Addr))
	assert.ErrorIs(t, err, removeserver.ErrInstanceNotFound)

	serverRepo.AssertCalled(t, "Get", ctx, svr.Addr)
	instanceRepo.AssertCalled(t, "GetByID", ctx, "foo")
	serverRepo.AssertNotCalled(t, "Remove", mock.Anything, mock.Anything, mock.Anything)
	instanceRepo.AssertNotCalled(t, "RemoveByID", mock.Anything, mock.Anything)
}

func TestRemoveServerUseCase_InstanceAddrDoesNotMatch(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()

	svr := serverfactory.BuildRandom()
	inst := instance.MustNew("foo", testutils.GenRandomIP(), svr.Addr.Port)

	serverRepo := new(MockServerRepository)
	serverRepo.On("Get", ctx, svr.Addr).Return(svr, nil)
	serverRepo.On("Remove", ctx, svr, mock.Anything).Return(nil)

	instanceRepo := new(MockInstanceRepository)
	instanceRepo.On("GetByID", ctx, "foo").Return(inst, nil)
	instanceRepo.On("RemoveByID", ctx, "foo").Return(nil)

	uc := removeserver.New(serverRepo, instanceRepo, &logger)
	err := uc.Execute(ctx, removeserver.NewRequest("foo", svr.Addr))
	assert.ErrorIs(t, err, removeserver.ErrInstanceAddrMismatch)

	serverRepo.AssertCalled(t, "Get", ctx, svr.Addr)
	instanceRepo.AssertCalled(t, "GetByID", ctx, "foo")
	serverRepo.AssertNotCalled(t, "Remove", mock.Anything, mock.Anything, mock.Anything)
	instanceRepo.AssertNotCalled(t, "RemoveByID", mock.Anything, mock.Anything)
}
