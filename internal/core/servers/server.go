package servers

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/sergeii/swat4master/internal/entity/addr"
	"github.com/sergeii/swat4master/internal/entity/details"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
)

var ErrInvalidQueryPort = errors.New("invalid port number")

type Server struct {
	addr      addr.Addr
	queryPort int
	status    ds.DiscoveryStatus
	info      details.Info
	details   details.Details

	refreshedAt time.Time
	version     int // lamport clock counter
}

var Blank Server // nolint: gochecknoglobals

func New(ip net.IP, port, queryPort int) (Server, error) {
	svrAddr, err := addr.New(ip, port)
	if err != nil {
		return Blank, err
	}
	return NewFromAddr(svrAddr, queryPort)
}

func NewFromAddr(addr addr.Addr, queryPort int) (Server, error) {
	if queryPort < 1 || queryPort > 65535 {
		return Blank, ErrInvalidQueryPort
	}
	return Server{
		addr:      addr,
		queryPort: queryPort,
		status:    ds.New,
	}, nil
}

func MustNew(ip net.IP, port, queryPort int) Server {
	svr, err := New(ip, port, queryPort)
	if err != nil {
		panic(err)
	}
	return svr
}

func (gs *Server) GetAddr() addr.Addr {
	return gs.addr
}

func (gs *Server) GetIP() net.IP {
	return gs.addr.GetIP()
}

func (gs *Server) GetDottedIP() string {
	return gs.addr.GetDottedIP()
}

func (gs *Server) GetGamePort() int {
	return gs.addr.Port
}

func (gs *Server) GetQueryPort() int {
	return gs.queryPort
}

func (gs *Server) UpdateQueryPort(port int) {
	gs.queryPort = port
}

func (gs *Server) GetDiscoveryStatus() ds.DiscoveryStatus {
	return gs.status
}

func (gs *Server) HasDiscoveryStatus(status ds.DiscoveryStatus) bool {
	return (gs.status & status) == status
}

func (gs *Server) HasAnyDiscoveryStatus(status ds.DiscoveryStatus) bool {
	return (gs.status & status) > 0
}

func (gs *Server) HasNoDiscoveryStatus(status ds.DiscoveryStatus) bool {
	return (gs.status &^ status) == gs.status
}

func (gs *Server) UpdateDiscoveryStatus(status ds.DiscoveryStatus) {
	gs.status &^= ds.New
	gs.status |= status
}

func (gs *Server) ClearDiscoveryStatus(status ds.DiscoveryStatus) {
	gs.status &^= status
}

func (gs *Server) GetInfo() details.Info {
	return gs.info
}

func (gs *Server) UpdateInfo(info details.Info, updatedAt time.Time) {
	gs.info = info
	gs.refreshedAt = updatedAt
}

func (gs *Server) GetDetails() details.Details {
	return gs.details
}

func (gs *Server) UpdateDetails(det details.Details, updatedAt time.Time) {
	gs.details = det
	gs.info = det.Info
	gs.refreshedAt = updatedAt
}

func (gs *Server) Refresh(updatedAt time.Time) {
	gs.refreshedAt = updatedAt
}

func (gs *Server) GetRefreshedAt() time.Time {
	return gs.refreshedAt
}

func (gs *Server) GetVersion() int {
	return gs.version
}

func (gs *Server) IncVersion() {
	gs.version++
}

func (gs Server) String() string {
	return fmt.Sprintf("%s (%d)", gs.addr, gs.queryPort)
}
