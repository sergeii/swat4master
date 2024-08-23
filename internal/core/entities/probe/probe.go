package probe

import (
	"fmt"
	"time"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
)

type Goal int

const (
	GoalDetails Goal = iota
	GoalPort
)

func (goal Goal) String() string {
	switch goal {
	case GoalDetails:
		return "details"
	case GoalPort:
		return "port"
	}
	return fmt.Sprintf("%d", goal)
}

var NC = time.Time{} // no constraint

type Probe struct {
	Addr       addr.Addr
	Port       int
	Goal       Goal
	Retries    int
	MaxRetries int
}

var Blank Probe // nolint: gochecknoglobals

func New(addr addr.Addr, port int, goal Goal, maxRetries int) Probe {
	return Probe{
		Addr:       addr,
		Port:       port,
		Goal:       goal,
		MaxRetries: maxRetries,
	}
}

func (t *Probe) IncRetries() (int, bool) {
	if t.Retries >= t.MaxRetries {
		return t.Retries, false
	}
	t.Retries++
	return t.Retries, true
}
