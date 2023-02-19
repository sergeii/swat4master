package probing

import (
	"context"
	"time"

	"github.com/sergeii/swat4master/internal/core/servers"
)

type Prober interface {
	Probe(context.Context, servers.Server, int, time.Duration) (any, error)
	HandleSuccess(any, servers.Server) servers.Server
	HandleRetry(servers.Server) servers.Server
	HandleFailure(servers.Server) servers.Server
}
