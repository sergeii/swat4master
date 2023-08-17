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
	addr    addr.Addr
	port    int
	goal    Goal
	retries int
}

var Blank Probe // nolint: gochecknoglobals

func New(addr addr.Addr, port int, goal Goal) Probe {
	return Probe{
		addr: addr,
		port: port,
		goal: goal,
	}
}

func (t *Probe) GetDottedIP() string {
	return t.addr.GetDottedIP()
}

func (t *Probe) GetPort() int {
	return t.port
}

func (t *Probe) GetAddr() addr.Addr {
	return t.addr
}

func (t *Probe) GetGoal() Goal {
	return t.goal
}

func (t *Probe) GetRetries() int {
	return t.retries
}

func (t *Probe) IncRetries(maxRetries int) (int, bool) {
	if t.retries >= maxRetries {
		return t.retries, false
	}
	t.retries++
	return t.retries, true
}
