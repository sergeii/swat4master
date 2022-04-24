package probers

import (
	"context"
	"time"

	"github.com/sergeii/swat4master/internal/core/servers"
)

type Prober interface {
	Probe(context.Context, servers.Server, int, time.Duration) (servers.Server, error)
	HandleRetry(servers.Server) servers.Server
	HandleFailure(servers.Server) servers.Server
}
