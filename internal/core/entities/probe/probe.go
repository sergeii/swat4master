package probe

import (
	"fmt"

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

type Probe struct {
	Addr    addr.Addr
	Port    int
	Goal    Goal
	Retries int
}

var Blank Probe // nolint: gochecknoglobals

func New(addr addr.Addr, port int, goal Goal) Probe {
	return Probe{
		Addr: addr,
		Port: port,
		Goal: goal,
	}
}

func (t *Probe) IncRetries(maxRetries int) (int, bool) {
	if t.Retries >= maxRetries {
		return t.Retries, false
	}
	t.Retries++
	return t.Retries, true
}
