package probes

import (
	"fmt"

	"github.com/sergeii/swat4master/internal/entity/addr"
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

type Target struct {
	addr    addr.Addr
	port    int
	goal    Goal
	retries int
}

var Blank Target // nolint: gochecknoglobals

func New(addr addr.Addr, port int, goal Goal) Target {
	return Target{
		addr: addr,
		port: port,
		goal: goal,
	}
}

func (t *Target) GetDottedIP() string {
	return t.addr.GetDottedIP()
}

func (t *Target) GetPort() int {
	return t.port
}

func (t *Target) GetAddr() addr.Addr {
	return t.addr
}

func (t *Target) GetGoal() Goal {
	return t.goal
}

func (t *Target) GetRetries() int {
	return t.retries
}

func (t *Target) IncRetries(maxRetries int) (int, bool) {
	if t.retries >= maxRetries {
		return t.retries, false
	}
	t.retries++
	return t.retries, true
}
