package instances

import (
	"context"
	"errors"

	"github.com/sergeii/swat4master/internal/entity/addr"
)

var ErrInstanceNotFound = errors.New("the requested instance was not found")

type Repository interface {
	Add(context.Context, Instance) error
	GetByID(context.Context, string) (Instance, error)
	GetByAddr(context.Context, addr.Addr) (Instance, error)
	RemoveByID(context.Context, string) error
	RemoveByAddr(context.Context, addr.Addr) error
	Count(context.Context) (int, error)
}
