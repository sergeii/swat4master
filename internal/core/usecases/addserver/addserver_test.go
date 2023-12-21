package addserver_test

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/core/usecases/addserver"
	"github.com/sergeii/swat4master/internal/testutils/factories"
)

type MockServerRepository struct {
	mock.Mock
	repositories.ServerRepository
}

func (m *MockServerRepository) Get(ctx context.Context, addr addr.Addr) (server.Server, error) {
	args := m.Called(ctx, addr)
	return args.Get(0).(server.Server), args.Error(1) // nolint: forcetypeassert
}

func (m *MockServerRepository) Add(
	ctx context.Context,
	svr server.Server,
	onConflict func(*server.Server) bool,
) (server.Server, error) {
	args := m.Called(ctx, svr, onConflict)
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

func TestAddServerUseCase_ServerExists(t *testing.T) {
	ctx := context.TODO()

	tests := []struct {
		name      string
		status    ds.DiscoveryStatus
		wantErr   error
		wantProbe bool
	}{
		{
			"positive case - only details",
			ds.Details,
			nil,
			false,
		},
		{
			"positive case - mixed",
			ds.Details | ds.DetailsRetry | ds.Master,
			nil,
			false,
		},
		{
			"server discovery is already pending",
			ds.PortRetry,
			addserver.ErrServerDiscoveryInProgress,
			false,
		},
		{
			"server has no details but discovery is in progress",
			ds.DetailsRetry,
			addserver.ErrServerDiscoveryInProgress,
			false,
		},
		{
			"server has both details and port discovery in progress",
			ds.DetailsRetry | ds.PortRetry,
			addserver.ErrServerDiscoveryInProgress,
			false,
		},
		{
			"server has no discovered port",
			ds.NoPort,
			addserver.ErrServerHasNoQueryablePort,
			false,
		},
		{
			"server is reporting to master but has no port",
			ds.Master | ds.Info | ds.NoPort,
			addserver.ErrServerHasNoQueryablePort,
			false,
		},
		{
			"server has both no details and no port",
			ds.NoDetails | ds.NoPort,
			addserver.ErrServerHasNoQueryablePort,
			false,
		},
		{
			"server is reporting to master but has no details",
			ds.Info | ds.Master,
			addserver.ErrServerDiscoveryInProgress,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zerolog.Nop()

			svr := factories.BuildServerWithDefaultDetails(
				tt.status,
			)
			svrAddr := addr.MustNewFromDotted("1.1.1.1", 10480)

			serverRepo := new(MockServerRepository)
			serverRepo.On("Get", ctx, svr.Addr).Return(svr, nil)
			serverRepo.On("Update", ctx, mock.Anything, mock.Anything).Return(svr, nil)

			probeRepo := new(MockProbeRepository)
			probeRepo.On("AddBetween", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil)

			uc := addserver.New(serverRepo, probeRepo, &logger)
			addedSvr, err := uc.Execute(ctx, svrAddr)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, 10481, addedSvr.QueryPort)
				assert.Equal(t, 10480, addedSvr.Info.HostPort)
				assert.Equal(t, "Swat4 Server", addedSvr.Info.Hostname)
				assert.Equal(t, "A-Bomb Nightclub", addedSvr.Info.MapName)
				assert.Equal(t, "VIP Escort", addedSvr.Info.GameType)
				assert.Equal(t, "SWAT 4", addedSvr.Info.GameVariant)
				assert.Equal(t, "1.1", addedSvr.Info.GameVersion)
			}

			serverRepo.AssertCalled(t, "Get", ctx, svrAddr)

			if tt.wantProbe {
				serverRepo.AssertCalled(
					t,
					"Update",
					ctx,
					mock.MatchedBy(func(updatedSvr server.Server) bool {
						return updatedSvr.HasAnyDiscoveryStatus(ds.PortRetry)
					}),
					mock.Anything,
				)
				probeRepo.AssertCalled(
					t,
					"AddBetween",
					ctx,
					probe.New(svr.Addr, 10480, probe.GoalPort),
					repositories.NC,
					repositories.NC,
				)
			} else {
				serverRepo.AssertNotCalled(t, "Update", ctx, mock.Anything, mock.Anything)
				probeRepo.AssertNotCalled(t, "AddBetween", ctx, mock.Anything, mock.Anything, mock.Anything)
			}
		})
	}
}

func TestAddServerUseCase_ServerDoesNotExist(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()

	svrAddr := addr.MustNewFromDotted("1.1.1.1", 10480)
	newSvr := factories.BuildNewServer("1.1.1.1", 10480, 10481)

	serverRepo := new(MockServerRepository)
	serverRepo.On("Get", ctx, svrAddr).Return(server.Blank, repositories.ErrServerNotFound)
	serverRepo.On("Add", ctx, mock.Anything, mock.Anything).Return(newSvr, nil)
	serverRepo.On("Update", ctx, mock.Anything, mock.Anything).Return(newSvr, nil)

	probeRepo := new(MockProbeRepository)
	probeRepo.On("AddBetween", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	uc := addserver.New(serverRepo, probeRepo, &logger)
	_, err := uc.Execute(ctx, svrAddr)
	assert.ErrorIs(t, err, addserver.ErrServerDiscoveryInProgress)

	serverRepo.AssertCalled(t, "Get", ctx, svrAddr)
	serverRepo.AssertCalled(t, "Add", ctx, server.MustNewFromAddr(svrAddr, 10481), mock.Anything)
	serverRepo.AssertCalled(
		t,
		"Update",
		ctx,
		mock.MatchedBy(func(updatedSvr server.Server) bool {
			return updatedSvr.HasAnyDiscoveryStatus(ds.PortRetry)
		}),
		mock.Anything,
	)

	probeRepo.AssertCalled(
		t,
		"AddBetween",
		ctx,
		probe.New(svrAddr, 10480, probe.GoalPort),
		repositories.NC,
		repositories.NC,
	)
}
