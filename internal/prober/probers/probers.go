package probers

import (
	"context"
	"time"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/entities/server"
)

type Prober interface {
	Probe(context.Context, addr.Addr, int, time.Duration) (any, error)
	HandleSuccess(any, server.Server) server.Server
	HandleRetry(server.Server) server.Server
	HandleFailure(server.Server) server.Server
}

type ForGoal map[probe.Goal]Prober
