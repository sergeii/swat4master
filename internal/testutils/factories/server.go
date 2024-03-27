package factories

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

type BuildServerParams struct {
	IP              string
	Port            int
	QueryPort       int
	DiscoveryStatus status.DiscoveryStatus
	Info            map[string]string
	Players         []map[string]string
	Objectives      []map[string]string
}

type BuildServerOption func(*BuildServerParams)

func WithAddress(ip string, port int) BuildServerOption {
	return func(p *BuildServerParams) {
		p.IP = ip
		p.Port = port
	}
}

func WithQueryPort(queryPort int) BuildServerOption {
	return func(p *BuildServerParams) {
		p.QueryPort = queryPort
	}
}

func WithDiscoveryStatus(status status.DiscoveryStatus) BuildServerOption {
	return func(p *BuildServerParams) {
		p.DiscoveryStatus = status
	}
}

func WithInfo(fields map[string]string) BuildServerOption {
	return func(p *BuildServerParams) {
		p.Info = fields
	}
}

func WithNoInfo() BuildServerOption {
	return func(p *BuildServerParams) {
		p.Info = nil
	}
}

func WithPlayers(players []map[string]string) BuildServerOption {
	return func(p *BuildServerParams) {
		p.Players = players
	}
}

func WithObjectives(objectives []map[string]string) BuildServerOption {
	return func(p *BuildServerParams) {
		p.Objectives = objectives
	}
}

func BuildServer(opts ...BuildServerOption) server.Server {
	params := BuildServerParams{
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

	svr.UpdateDetails(details.MustNewDetailsFromParams(params.Info, params.Players, params.Objectives), time.Now())
	svr.UpdateDiscoveryStatus(params.DiscoveryStatus)

	return svr
}

func BuildRandomServer() server.Server {
	randomIP := testutils.GenRandomIP()
	randPort := random.RandInt(1, 65534)
	return BuildServer(
		WithAddress(randomIP.String(), randPort),
		WithQueryPort(randPort+1),
	)
}

func SaveServer(
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

func CreateServer(
	ctx context.Context,
	repo repositories.ServerRepository,
	opts ...BuildServerOption,
) server.Server {
	svr := BuildServer(opts...)
	return SaveServer(ctx, repo, svr)
}
