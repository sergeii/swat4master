package serverfactory

import (
	"context"
	"net"
	"time"

	"github.com/sergeii/swat4master/internal/core/entities/details"
	"github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/testutils"
	"github.com/sergeii/swat4master/pkg/random"
)

type BuildParams struct {
	IP              string
	Port            int
	QueryPort       int
	DiscoveryStatus status.DiscoveryStatus
	Info            map[string]string
	Players         []map[string]string
	Objectives      []map[string]string
	RefreshedAt     time.Time
}

type BuildOption func(*BuildParams)

func WithAddress(ip string, port int) BuildOption {
	return func(p *BuildParams) {
		p.IP = ip
		p.Port = port
	}
}

func WithQueryPort(queryPort int) BuildOption {
	return func(p *BuildParams) {
		p.QueryPort = queryPort
	}
}

func WithRandomAddress() BuildOption {
	return func(p *BuildParams) {
		randomIP := testutils.GenRandomIP()
		randPort := random.RandInt(1, 65534)
		p.IP = randomIP.String()
		p.Port = randPort
		p.QueryPort = randPort + 1
	}
}

func WithDiscoveryStatus(status status.DiscoveryStatus) BuildOption {
	return func(p *BuildParams) {
		p.DiscoveryStatus = status
	}
}

func WithInfo(fields map[string]string) BuildOption {
	return func(p *BuildParams) {
		p.Info = fields
	}
}

func WithNoInfo() BuildOption {
	return func(p *BuildParams) {
		p.Info = nil
	}
}

func WithPlayers(players []map[string]string) BuildOption {
	return func(p *BuildParams) {
		p.Players = players
	}
}

func WithObjectives(objectives []map[string]string) BuildOption {
	return func(p *BuildParams) {
		p.Objectives = objectives
	}
}

func WithRefreshedAt(refreshedAt time.Time) BuildOption {
	return func(p *BuildParams) {
		p.RefreshedAt = refreshedAt
	}
}

func Build(opts ...BuildOption) server.Server {
	params := BuildParams{
		IP:              "1.1.1.1",
		Port:            10480,
		QueryPort:       10481,
		DiscoveryStatus: status.New,
		Info: map[string]string{
			"hostname":    "Swat4 Server",
			"hostport":    "10480",
			"mapname":     "A-Bomb Nightclub",
			"gamever":     "1.1",
			"gamevariant": "SWAT 4",
			"gametype":    "VIP Escort",
		},
		Players:    nil,
		Objectives: nil,
	}

	for _, opt := range opts {
		opt(&params)
	}

	svr := server.MustNew(net.ParseIP(params.IP), params.Port, params.QueryPort)

	svr.UpdateDetails(
		details.MustNewDetailsFromParams(
			params.Info,
			params.Players,
			params.Objectives,
		),
	)
	svr.UpdateDiscoveryStatus(params.DiscoveryStatus)
	svr.Refresh(params.RefreshedAt)

	return svr
}

func BuildRandom() server.Server {
	return Build(WithRandomAddress())
}

func Save(
	ctx context.Context,
	repo repositories.ServerRepository,
	svr server.Server,
) server.Server {
	savedSvr, err := repo.Add(ctx, svr, repositories.ServerOnConflictIgnore)
	if err != nil {
		panic(err)
	}
	return savedSvr
}

func Create(
	ctx context.Context,
	repo repositories.ServerRepository,
	opts ...BuildOption,
) server.Server {
	svr := Build(opts...)
	return Save(ctx, repo, svr)
}
