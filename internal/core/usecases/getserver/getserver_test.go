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

func TestGetServerUseCase_OK(t *testing.T) {
	ctx := context.TODO()

	svr := factories.BuildServerWithDefaultDetails(ds.Details)

	mockRepo := new(MockServerRepository)
	mockRepo.On("Get", ctx, svr.Addr).Return(svr, nil)

	uc := getserver.New(mockRepo)
	svr, err := uc.Execute(ctx, svr.Addr)

	assert.NoError(t, err)
	assert.Equal(t, 10481, svr.QueryPort)
	assert.Equal(t, 10480, svr.Info.HostPort)
	assert.Equal(t, "Swat4 Server", svr.Info.Hostname)
	assert.Equal(t, "A-Bomb Nightclub", svr.Info.MapName)
	assert.Equal(t, "VIP Escort", svr.Info.GameType)
	assert.Equal(t, "SWAT 4", svr.Info.GameVariant)
	assert.Equal(t, "1.1", svr.Info.GameVersion)

	mockRepo.AssertExpectations(t)
}

func TestGetServerUseCase_NotFound(t *testing.T) {
	ctx := context.TODO()

	svrAddr := addr.MustNewFromDotted("1.1.1.1", 10480)

	mockRepo := new(MockServerRepository)
	mockRepo.On("Get", ctx, svrAddr).Return(server.Blank, repositories.ErrServerNotFound)

	uc := getserver.New(mockRepo)
	_, err := uc.Execute(ctx, svrAddr)

	assert.ErrorIs(t, err, getserver.ErrServerNotFound)

	mockRepo.AssertExpectations(t)
}

func TestGetServerUseCase_ValidateStatus(t *testing.T) {
	ctx := context.TODO()

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
			svr := factories.BuildServerWithStatus(
				"1.1.1.1",
				10480,
				10481,
				tt.status,
			)

			mockRepo := new(MockServerRepository)
			mockRepo.On("Get", ctx, svr.Addr).Return(svr, nil)

			uc := getserver.New(mockRepo)
			svr, err := uc.Execute(ctx, svr.Addr)

			mockRepo.AssertExpectations(t)

			if tt.want {
				assert.NoError(t, err)
				assert.Equal(t, "1.1.1.1:10480", svr.Addr.String())
				assert.Equal(t, 10481, svr.QueryPort)
			} else {
				assert.ErrorIs(t, err, getserver.ErrServerHasNoDetails)
			}
		})
	}
}
