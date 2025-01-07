package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/sergeii/swat4master/internal/core/entities/probe"
)

var ErrProbeQueueIsEmpty = errors.New("queue is empty")

var NC = time.Time{} // no constraint

type ProbeRepository interface {
	Add(context.Context, probe.Probe) error
	AddBetween(context.Context, probe.Probe, time.Time, time.Time) error
	Pop(context.Context) (probe.Probe, error)
	Peek(context.Context) (probe.Probe, error)
	PopMany(context.Context, int) ([]probe.Probe, int, error)
	Count(context.Context) (int, error)
}
