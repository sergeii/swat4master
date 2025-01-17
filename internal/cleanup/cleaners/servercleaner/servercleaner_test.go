package servercleaner_test

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

	"github.com/sergeii/swat4master/internal/cleanup"
	"github.com/sergeii/swat4master/internal/cleanup/cleaners/servercleaner"
	"github.com/sergeii/swat4master/internal/core/entities/filterset"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
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

func (m *MockServerRepository) Remove(
	ctx context.Context,
	svr server.Server,
	onConflict func(*server.Server) bool,
) error {
	args := m.Called(ctx, svr, onConflict)
	return args.Error(0)
}

// func TestCleanServersUseCase_RemoveErrors(t *testing.T) {
//	ctx := context.TODO()
//	logger := zerolog.Nop()
//
//	until := time.Now().Add(-24 * time.Hour) // Example time filter
//
//	svr1 := serverfactory.BuildRandom()
//	svr2 := serverfactory.BuildRandom()
//	svr3 := serverfactory.BuildRandom()
//	outdatedServers := []server.Server{svr1, svr2, svr3}
//
//	serverRepo := new(MockServerRepository)
//	serverRepo.On("Count", ctx).Return(3, nil).Once()
//	serverRepo.On("Count", ctx).Return(2, nil).Once()
//	serverRepo.On("Filter", ctx, mock.Anything).Return(outdatedServers, nil).Once()
//	serverRepo.On("Remove", ctx, svr1, mock.Anything).Return(nil).Once()
//	serverRepo.On("Remove", ctx, svr2, mock.Anything).Return(nil).Once()
//	serverRepo.On("Remove", ctx, svr3, mock.Anything).Return(errors.New("error")).Once()
//
//	uc := cleanservers.New(serverRepo, &logger)
//	response, err := uc.Execute(ctx, until)
//
//	assert.NoError(t, err)
//	assert.Equal(t, 2, response.Count)
//	assert.Equal(t, 1, response.Errors)
//
//	serverRepo.AssertExpectations(t)
//	serverRepo.AssertNumberOfCalls(t, "Remove", 3)
// }

func TestServerCleaner_Clean_OK(t *testing.T) {
	ctx := context.TODO()

	manager := cleanup.NewManager()
	collector := metrics.New()
	clock := clockwork.NewFakeClock()
	logger := zerolog.Nop()
	options := servercleaner.Opts{
		Retention: time.Hour * 24,
	}

	outdatedServers := []server.Server{
		serverfactory.BuildRandom(),
		serverfactory.BuildRandom(),
	}

	serverRepo := new(MockServerRepository)
	serverRepo.On("Filter", ctx, mock.Anything).Return(outdatedServers, nil).Once()
	serverRepo.On("Remove", ctx, mock.Anything, mock.Anything).Return(nil).Times(2)

	cleaner := servercleaner.New(
		manager,
		options,
		serverRepo,
		clock,
		collector,
		&logger,
	)
	cleaner.Clean(ctx)

	serverRepo.AssertExpectations(t)
	serverRepo.AssertCalled(
		t,
		"Filter",
		ctx,
		mock.MatchedBy(func(fs filterset.ServerFilterSet) bool {
			updatedBefore, ok := fs.GetUpdatedBefore()
			wantUpdatedBefore := ok && updatedBefore.Equal(clock.Now().Add(-time.Hour*24))
			return wantUpdatedBefore
		}),
	)
	for _, svr := range outdatedServers {
		serverRepo.AssertCalled(t, "Remove", ctx, svr, mock.Anything)
	}

	cleanerRemovalsWithServersValue := testutil.ToFloat64(collector.CleanerRemovals.WithLabelValues("servers"))
	assert.Equal(t, float64(2), cleanerRemovalsWithServersValue)
	cleanerErrorsWithServersValue := testutil.ToFloat64(collector.CleanerErrors.WithLabelValues("servers"))
	assert.Equal(t, float64(0), cleanerErrorsWithServersValue)
}

func TestServerCleaner_Clean_NothingToClean(t *testing.T) {
	ctx := context.TODO()

	manager := cleanup.NewManager()
	collector := metrics.New()
	clock := clockwork.NewFakeClock()
	logger := zerolog.Nop()
	options := servercleaner.Opts{
		Retention: time.Hour * 24,
	}

	serverRepo := new(MockServerRepository)
	serverRepo.On("Filter", ctx, mock.Anything).Return([]server.Server{}, nil).Once()

	cleaner := servercleaner.New(
		manager,
		options,
		serverRepo,
		clock,
		collector,
		&logger,
	)
	cleaner.Clean(ctx)

	serverRepo.AssertExpectations(t)
	serverRepo.AssertNotCalled(t, "Remove", mock.Anything, mock.Anything)

	cleanerRemovalsWithServersValue := testutil.ToFloat64(collector.CleanerRemovals.WithLabelValues("servers"))
	assert.Equal(t, float64(0), cleanerRemovalsWithServersValue)
	cleanerErrorsWithServersValue := testutil.ToFloat64(collector.CleanerErrors.WithLabelValues("servers"))
	assert.Equal(t, float64(0), cleanerErrorsWithServersValue)
}

func TestServerCleaner_Clean_RepoErrors(t *testing.T) {
	ctx := context.TODO()

	manager := cleanup.NewManager()
	collector := metrics.New()
	clock := clockwork.NewFakeClock()
	logger := zerolog.Nop()
	options := servercleaner.Opts{
		Retention: time.Hour * 24,
	}

	svr1 := serverfactory.BuildRandom()
	svr2 := serverfactory.BuildRandom()
	svr3 := serverfactory.BuildRandom()
	outdatedServers := []server.Server{svr1, svr2, svr3}

	serverRepo := new(MockServerRepository)
	serverRepo.On("Filter", ctx, mock.Anything).Return(outdatedServers, nil).Once()
	serverRepo.On("Remove", ctx, svr1, mock.Anything).Return(nil).Once()
	serverRepo.On("Remove", ctx, svr2, mock.Anything).Return(nil).Once()
	serverRepo.On("Remove", ctx, svr3, mock.Anything).Return(errors.New("error")).Once()

	cleaner := servercleaner.New(
		manager,
		options,
		serverRepo,
		clock,
		collector,
		&logger,
	)
	cleaner.Clean(ctx)

	serverRepo.AssertExpectations(t)
	serverRepo.AssertNumberOfCalls(t, "Remove", 3)

	cleanerRemovalsWithServersValue := testutil.ToFloat64(collector.CleanerRemovals.WithLabelValues("servers"))
	assert.Equal(t, float64(2), cleanerRemovalsWithServersValue)
	cleanerErrorsWithServersValue := testutil.ToFloat64(collector.CleanerErrors.WithLabelValues("servers"))
	assert.Equal(t, float64(1), cleanerErrorsWithServersValue)
}
