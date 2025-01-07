package probefactory

import (
	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/testutils"
	"github.com/sergeii/swat4master/pkg/random"
)

type BuildParams struct {
	IP         string
	Port       int
	ProbePort  int
	Goal       probe.Goal
	MaxRetries int
}

type BuildOption func(*BuildParams)

func WithServerAddress(ip string, port int) BuildOption {
	return func(p *BuildParams) {
		p.IP = ip
		p.Port = port
	}
}

func WithRandomServerAddress() BuildOption {
	return func(p *BuildParams) {
		randomIP := testutils.GenRandomIP()
		randPort := random.RandInt(1, 65534)
		p.IP = randomIP.String()
		p.Port = randPort
	}
}

func Build(opts ...BuildOption) probe.Probe {
	params := BuildParams{
		IP:         "1.1.1.1",
		Port:       10480,
		ProbePort:  10481,
		Goal:       probe.GoalDetails,
		MaxRetries: 0,
	}

	for _, opt := range opts {
		opt(&params)
	}

	prb := probe.New(
		addr.MustNewFromDotted(params.IP, params.Port),
		params.ProbePort,
		params.Goal,
		params.MaxRetries,
	)

	return prb
}
