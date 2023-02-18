package probes

import (
	"context"
	"errors"
	"time"
)

var (
	ErrQueueIsEmpty     = errors.New("queue is empty")
	ErrTargetIsNotReady = errors.New("queue has waiting targets")
	ErrTargetHasExpired = errors.New("target item has expired")
)

var NC = time.Time{} // no constraint

type Repository interface {
	Add(context.Context, Target) error
	AddBetween(context.Context, Target, time.Time, time.Time) error
	Pop(context.Context) (Target, error)
	PopAny(context.Context) (Target, error)
	PopMany(context.Context, int) ([]Target, int, error)
	Count(context.Context) (int, error)
}
