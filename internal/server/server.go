package server

import (
	"errors"
	"time"

	"github.com/sergeii/swat4master/internal/aggregate"
)

var ErrServerNotFound = errors.New("the requested server was not found")

type Repository interface {
	Report(server *aggregate.GameServer, instanceID string, params map[string]string) error
	Renew(IPAddr string, instanceID string) error
	Remove(IPAddr string, instanceID string) error
	GetByInstanceID(instanceID string) (*aggregate.GameServer, error)
	GetReportedSince(since time.Time) ([]*aggregate.GameServer, error)
}
