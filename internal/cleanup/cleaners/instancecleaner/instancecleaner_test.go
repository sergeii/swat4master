package instancecleaner_test

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/sergeii/swat4master/internal/cleanup"
	"github.com/sergeii/swat4master/internal/cleanup/cleaners/instancecleaner"
	"github.com/sergeii/swat4master/internal/core/entities/filterset"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/metrics"
)

type MockInstanceRepository struct {
	mock.Mock
	repositories.InstanceRepository
}

func (m *MockInstanceRepository) Clear(ctx context.Context, fs filterset.InstanceFilterSet) (int, error) {
	args := m.Called(ctx, fs)
	return args.Get(0).(int), args.Error(1) // nolint: forcetypeassert
}

func TestInstanceCleaner_Clean_OK(t *testing.T) {
	ctx := context.TODO()

	manager := cleanup.NewManager()
	collector := metrics.New()
	clock := clockwork.NewFakeClock()
	logger := zerolog.Nop()
	options := instancecleaner.Opts{
		Retention: time.Hour,
	}

	instanceRepo := new(MockInstanceRepository)
	instanceRepo.On("Clear", ctx, mock.Anything).Return(37, nil)

	cleaner := instancecleaner.New(
		manager,
		options,
		instanceRepo,
		clock,
		collector,
		&logger,
	)
	cleaner.Clean(ctx)

	instanceRepo.AssertCalled(
		t,
		"Clear",
		ctx,
		mock.MatchedBy(func(fs filterset.InstanceFilterSet) bool {
			updatedBefore, ok := fs.GetUpdatedBefore()
			wantUpdatedBefore := ok && updatedBefore.Equal(clock.Now().Add(-time.Hour))
			return wantUpdatedBefore
		}),
	)

	cleanerRemovalsWithInstancesValue := testutil.ToFloat64(collector.CleanerRemovals.WithLabelValues("instances"))
	assert.Equal(t, float64(37), cleanerRemovalsWithInstancesValue)
	cleanerErrorsWithInstancesValue := testutil.ToFloat64(collector.CleanerErrors.WithLabelValues("instances"))
	assert.Equal(t, float64(0), cleanerErrorsWithInstancesValue)
}

func TestInstanceCleaner_Clean_RepoError(t *testing.T) {
	ctx := context.TODO()

	manager := cleanup.NewManager()
	collector := metrics.New()
	clock := clockwork.NewFakeClock()
	logger := zerolog.Nop()
	options := instancecleaner.Opts{
		Retention: time.Hour,
	}

	instanceRepo := new(MockInstanceRepository)
	instanceRepo.On("Clear", ctx, mock.Anything).Return(0, assert.AnError)

	cleaner := instancecleaner.New(
		manager,
		options,
		instanceRepo,
		clock,
		collector,
		&logger,
	)
	cleaner.Clean(ctx)

	instanceRepo.AssertCalled(
		t,
		"Clear",
		ctx,
		mock.MatchedBy(func(fs filterset.InstanceFilterSet) bool {
			updatedBefore, ok := fs.GetUpdatedBefore()
			wantUpdatedBefore := ok && updatedBefore.Equal(clock.Now().Add(-time.Hour))
			return wantUpdatedBefore
		}),
	)

	cleanerRemovalsWithInstancesValue := testutil.ToFloat64(collector.CleanerRemovals.WithLabelValues("instances"))
	assert.Equal(t, float64(0), cleanerRemovalsWithInstancesValue)
	cleanerErrorsWithInstancesValue := testutil.ToFloat64(collector.CleanerErrors.WithLabelValues("instances"))
	assert.Equal(t, float64(1), cleanerErrorsWithInstancesValue)
}
