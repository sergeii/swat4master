package server

import (
	"errors"
	"time"

	"github.com/sergeii/swat4master/internal/aggregate"
)

var ErrServerNotFound = errors.New("the requested server was not found")

type Repository interface {
	Report(server *aggregate.GameServer, InstanceID string, params map[string]string) error
	Renew(IPAddr string, InstanceID string) error
	Remove(IPAddr string, InstanceID string) error
	GetByInstanceID(InstanceID string) (*aggregate.GameServer, error)
	GetReportedSince(since time.Time) ([]*aggregate.GameServer, error)
}
