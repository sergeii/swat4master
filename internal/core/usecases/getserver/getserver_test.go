package getserver_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/core/usecases/getserver"
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

func TestGetServerUseCase_OK(t *testing.T) {
	ctx := context.TODO()

	svr := serverfactory.Build(serverfactory.WithDiscoveryStatus(ds.Details))

	mockRepo := new(MockServerRepository)
	mockRepo.On("Get", ctx, svr.Addr).Return(svr, nil)

	uc := getserver.New(mockRepo)
	got, err := uc.Execute(ctx, addr.MustNewPublicAddr(svr.Addr))

	assert.NoError(t, err)
	assert.Equal(t, 10481, got.QueryPort)
	assert.Equal(t, 10480, got.Info.HostPort)
	assert.Equal(t, "Swat4 Server", got.Info.Hostname)
	assert.Equal(t, "A-Bomb Nightclub", got.Info.MapName)
	assert.Equal(t, "VIP Escort", got.Info.GameType)
	assert.Equal(t, "SWAT 4", got.Info.GameVariant)
	assert.Equal(t, "1.1", got.Info.GameVersion)

	mockRepo.AssertExpectations(t)
}

func TestGetServerUseCase_NotFound(t *testing.T) {
	ctx := context.TODO()

	svrAddr := addr.MustNewFromDotted("1.1.1.1", 10480)

	mockRepo := new(MockServerRepository)
	mockRepo.On("Get", ctx, svrAddr).Return(server.Blank, repositories.ErrServerNotFound)

	uc := getserver.New(mockRepo)
	_, err := uc.Execute(ctx, addr.MustNewPublicAddr(svrAddr))

	assert.ErrorIs(t, err, getserver.ErrServerNotFound)

	mockRepo.AssertExpectations(t)
}

func TestGetServerUseCase_ValidateStatus(t *testing.T) {
	tests := []struct {
		name   string
		status ds.DiscoveryStatus
		want   bool
	}{
		{
			"positive case - only details",
			ds.Details,
			true,
		},
		{
			"positive case - mixed",
			ds.Details | ds.DetailsRetry | ds.NoPort,
			true,
		},
		{
			"no details",
			ds.Info | ds.Master | ds.NoDetails,
			false,
		},
		{
			"no details - only info",
			ds.Info,
			false,
		},
		{
			"no details - retry",
			ds.Info | ds.DetailsRetry,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()

			svr := serverfactory.Build(
				serverfactory.WithAddress("1.1.1.1", 10480),
				serverfactory.WithDiscoveryStatus(tt.status),
			)

			mockRepo := new(MockServerRepository)
			mockRepo.On("Get", ctx, svr.Addr).Return(svr, nil)

			uc := getserver.New(mockRepo)
			got, err := uc.Execute(ctx, addr.MustNewPublicAddr(svr.Addr))

			mockRepo.AssertExpectations(t)

			if tt.want {
				assert.NoError(t, err)
				assert.Equal(t, "1.1.1.1:10480", got.Addr.String())
				assert.Equal(t, 10481, got.QueryPort)
			} else {
				assert.ErrorIs(t, err, getserver.ErrServerHasNoDetails)
			}
		})
	}
}
