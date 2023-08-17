package repositories

import (
	"context"
	"time"

	"github.com/sergeii/swat4master/internal/core/entities/probe"
)

var NC = time.Time{} // no constraint

type ProbeRepository interface {
	Add(context.Context, probe.Probe) error
	AddBetween(context.Context, probe.Probe, time.Time, time.Time) error
	Pop(context.Context) (probe.Probe, error)
	PopAny(context.Context) (probe.Probe, error)
	PopMany(context.Context, int) ([]probe.Probe, int, error)
	Count(context.Context) (int, error)
}
