package instanceobserver_test

import (
	"context"
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/metrics"
	"github.com/sergeii/swat4master/internal/metrics/observers/instanceobserver"
)

type MockInstanceRepository struct {
	mock.Mock
	repositories.InstanceRepository
}

func (m *MockInstanceRepository) Count(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Get(0).(int), args.Error(1) // nolint: forcetypeassert
}

func TestInstanceObserver_Observe_OK(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()

	collector := metrics.New()

	instanceRepo := new(MockInstanceRepository)
	instanceRepo.On("Count", ctx).Return(37, nil)

	observer := instanceobserver.New(collector, instanceRepo, &logger)
	observer.Observe(ctx, collector)

	repoSizeValue := testutil.ToFloat64(collector.InstanceRepositorySize)
	assert.Equal(t, float64(37), repoSizeValue)

	instanceRepo.AssertExpectations(t)
}

func TestInstanceObserver_Observe_RepoFailure(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()

	collector := metrics.New()

	instanceRepo := new(MockInstanceRepository)
	instanceRepo.On("Count", ctx).Return(0, errors.New("repo failure"))

	observer := instanceobserver.New(collector, instanceRepo, &logger)
	observer.Observe(ctx, collector)

	repoSizeValue := testutil.ToFloat64(collector.InstanceRepositorySize)
	assert.Equal(t, float64(0), repoSizeValue)

	instanceRepo.AssertExpectations(t)
}
