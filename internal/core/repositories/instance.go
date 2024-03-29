package repositories

import (
	"context"
	"errors"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/entities/instance"
)

var ErrInstanceNotFound = errors.New("the requested instance was not found")

type InstanceRepository interface {
	Add(context.Context, instance.Instance) error
	GetByID(context.Context, string) (instance.Instance, error)
	GetByAddr(context.Context, addr.Addr) (instance.Instance, error)
	RemoveByID(context.Context, string) error
	RemoveByAddr(context.Context, addr.Addr) error
	Count(context.Context) (int, error)
}
