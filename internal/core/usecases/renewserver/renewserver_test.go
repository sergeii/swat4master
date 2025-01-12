package renewserver_test

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/entities/instance"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/core/usecases/renewserver"
	"github.com/sergeii/swat4master/internal/testutils"
	"github.com/sergeii/swat4master/internal/testutils/factories/serverfactory"
)

const DEADBEEF = "\xde\xad\xbe\xef"

type b []byte

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
	return svr, args.Error(0)
}

type MockInstanceRepository struct {
	mock.Mock
	repositories.InstanceRepository
}

func (m *MockInstanceRepository) Get(ctx context.Context, instanceID instance.Identifier) (instance.Instance, error) {
	args := m.Called(ctx, instanceID)
	return args.Get(0).(instance.Instance), args.Error(1) // nolint: forcetypeassert
}

func TestRenewServerUseCase_Success(t *testing.T) {
	ctx := context.TODO()
	clock := clockwork.NewFakeClock()

	svr := serverfactory.BuildRandom()
	instID := instance.MustNewID(b(DEADBEEF))
	inst := instance.MustNew(instID, svr.Addr.GetIP(), svr.Addr.Port)

	clock.Advance(time.Second)
	passedTime := clock.Now()

	serverRepo := new(MockServerRepository)
	serverRepo.On("Get", ctx, svr.Addr).Return(svr, nil)
	serverRepo.On("Update", ctx, mock.Anything, mock.Anything).Return(nil)

	instanceRepo := new(MockInstanceRepository)
	instanceRepo.On("Get", ctx, inst.ID).Return(inst, nil)

	uc := renewserver.New(instanceRepo, serverRepo, clock)
	err := uc.Execute(ctx, renewserver.NewRequest(b(DEADBEEF), svr.Addr.GetIP()))
	assert.NoError(t, err)

	updatedSvr := svr
	updatedSvr.RefreshedAt = passedTime

	instanceRepo.AssertCalled(t, "Get", ctx, inst.ID)
	serverRepo.AssertCalled(t, "Get", ctx, svr.Addr)
	serverRepo.AssertCalled(t, "Update", ctx, updatedSvr, mock.Anything)
}

func TestRenewServerUseCase_InstanceNotFound(t *testing.T) {
	ctx := context.TODO()
	clock := clockwork.NewFakeClock()

	svr := serverfactory.BuildRandom()
	instID := instance.MustNewID(b(DEADBEEF))
	inst := instance.MustNew(instID, svr.Addr.GetIP(), svr.Addr.Port)

	serverRepo := new(MockServerRepository)
	serverRepo.On("Get", ctx, svr.Addr).Return(svr, nil)

	instanceRepo := new(MockInstanceRepository)
	instanceRepo.On("Get", ctx, inst.ID).Return(instance.Blank, repositories.ErrInstanceNotFound)

	uc := renewserver.New(instanceRepo, serverRepo, clock)
	err := uc.Execute(ctx, renewserver.NewRequest(b(DEADBEEF), svr.Addr.GetIP()))
	assert.ErrorIs(t, err, repositories.ErrInstanceNotFound)

	instanceRepo.AssertCalled(t, "Get", ctx, inst.ID)
	serverRepo.AssertNotCalled(t, "Get", ctx, svr.Addr)
	serverRepo.AssertNotCalled(t, "Update", ctx, mock.Anything, mock.Anything)
}

func TestRenewServerUseCase_ServerNotFound(t *testing.T) {
	ctx := context.TODO()
	clock := clockwork.NewFakeClock()

	svr := serverfactory.BuildRandom()
	instID := instance.MustNewID(b(DEADBEEF))
	inst := instance.MustNew(instID, svr.Addr.GetIP(), svr.Addr.Port)

	serverRepo := new(MockServerRepository)
	serverRepo.On("Get", ctx, svr.Addr).Return(server.Blank, repositories.ErrServerNotFound)

	instanceRepo := new(MockInstanceRepository)
	instanceRepo.On("Get", ctx, inst.ID).Return(inst, nil)

	uc := renewserver.New(instanceRepo, serverRepo, clock)
	err := uc.Execute(ctx, renewserver.NewRequest(b(DEADBEEF), svr.Addr.GetIP()))
	assert.ErrorIs(t, err, repositories.ErrServerNotFound)

	instanceRepo.AssertCalled(t, "Get", ctx, inst.ID)
	serverRepo.AssertCalled(t, "Get", ctx, svr.Addr)
	serverRepo.AssertNotCalled(t, "Update", ctx, mock.Anything, mock.Anything)
}

func TestRenewServerUseCase_InstanceAddressMismatch(t *testing.T) {
	ctx := context.TODO()
	clock := clockwork.NewFakeClock()

	svr := serverfactory.BuildRandom()
	instID := instance.MustNewID(b(DEADBEEF))
	inst := instance.MustNew(instID, testutils.GenRandomIP(), svr.Addr.Port)

	serverRepo := new(MockServerRepository)
	serverRepo.On("Get", ctx, svr.Addr).Return(server.Blank, repositories.ErrServerNotFound)

	instanceRepo := new(MockInstanceRepository)
	instanceRepo.On("Get", ctx, inst.ID).Return(inst, nil)

	uc := renewserver.New(instanceRepo, serverRepo, clock)
	err := uc.Execute(ctx, renewserver.NewRequest(b(DEADBEEF), svr.Addr.GetIP()))
	assert.ErrorIs(t, err, renewserver.ErrUnknownInstanceID)

	instanceRepo.AssertCalled(t, "Get", ctx, inst.ID)
	serverRepo.AssertNotCalled(t, "Get", ctx, mock.Anything)
	serverRepo.AssertNotCalled(t, "Update", ctx, mock.Anything, mock.Anything)
}
