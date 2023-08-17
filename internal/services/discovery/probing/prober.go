package probing

import (
	"context"
	"time"

	"github.com/sergeii/swat4master/internal/core/entities/server"
)

type Prober interface {
	Probe(context.Context, server.Server, int, time.Duration) (any, error)
	HandleSuccess(any, server.Server) server.Server
	HandleRetry(server.Server) server.Server
	HandleFailure(server.Server) server.Server
}
