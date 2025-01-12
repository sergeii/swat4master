package repositories

import (
	"context"
	"errors"

	"github.com/sergeii/swat4master/internal/core/entities/filterset"
	"github.com/sergeii/swat4master/internal/core/entities/instance"
)

var ErrInstanceNotFound = errors.New("the requested instance was not found")

type InstanceRepository interface {
	Add(context.Context, instance.Instance) error
	Get(context.Context, string) (instance.Instance, error)
	Remove(context.Context, string) error
	Clear(context.Context, filterset.InstanceFilterSet) (int, error)
	Count(context.Context) (int, error)
}
