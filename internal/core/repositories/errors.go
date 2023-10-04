package repositories

import (
	"errors"
)

var (
	ErrServerNotFound = errors.New("the requested server was not found")
	ErrServerExists   = errors.New("server already exists")

	ErrInstanceNotFound = errors.New("the requested instance was not found")

	ErrProbeQueueIsEmpty = errors.New("queue is empty")
	ErrProbeIsNotReady   = errors.New("queue has waiting probes")
	ErrProbeHasExpired   = errors.New("probe has expired")
)
