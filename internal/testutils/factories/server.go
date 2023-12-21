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

func SaveServer(
	ctx context.Context,
	repo repositories.ServerRepository,
	svr server.Server,
) server.Server {
	savedSvr, _ := repo.Add(ctx, svr, repositories.ServerOnConflictIgnore)
	return savedSvr
}

func BuildNewServer(
	ip string,
	port int,
	queryPort int,
) server.Server {
	svr, _ := server.New(net.ParseIP(ip), port, queryPort)
	return svr
}

func BuildRandomServer() server.Server {
	randomIP := testutils.GenRandomIP()
	randPort := random.RandInt(1, 65534)
	return BuildNewServer(
		randomIP.String(),
		randPort,
		randPort+1,
	)
}

func BuildServerWithDetails(
	ip string,
	port int,
	queryPort int,
	det details.Details,
	status status.DiscoveryStatus,
) server.Server {
	svr := BuildNewServer(ip, port, queryPort)
	svr.UpdateDetails(det, time.Now())
	svr.UpdateDiscoveryStatus(status)
	return svr
}

func BuildServerWithStatus(
	ip string,
	port int,
	queryPort int,
	status status.DiscoveryStatus,
) server.Server {
	svr := BuildNewServer(ip, port, queryPort)
	svr.UpdateDiscoveryStatus(status)
	return svr
}

func BuildServerWithDefaultDetails(
	status status.DiscoveryStatus,
) server.Server {
	fields := map[string]string{
		"hostname":    "Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "VIP Escort",
	}
	svr := BuildNewServer("1.1.1.1", 10480, 10481)
	svr.UpdateDetails(details.MustNewDetailsFromParams(fields, nil, nil), time.Now())
	svr.UpdateDiscoveryStatus(status)
	return svr
}

func CreateServerWithDetails(
	ctx context.Context,
	repo repositories.ServerRepository,
	ip string,
	port int,
	queryPort int,
	det details.Details,
	status status.DiscoveryStatus,
) server.Server {
	svr := BuildServerWithDetails(ip, port, queryPort, det, status)
	SaveServer(ctx, repo, svr)
	return svr
}

func CreateServerWithStatus(
	ctx context.Context,
	repo repositories.ServerRepository,
	ip string,
	port int,
	queryPort int,
	status status.DiscoveryStatus,
) server.Server {
	svr := BuildServerWithStatus(ip, port, queryPort, status)
	SaveServer(ctx, repo, svr)
	return svr
}

func CreateServerWithDefaultDetails(
	ctx context.Context,
	repo repositories.ServerRepository,
	status status.DiscoveryStatus,
) server.Server {
	svr := BuildServerWithDefaultDetails(status)
	SaveServer(ctx, repo, svr)
	return svr
}
