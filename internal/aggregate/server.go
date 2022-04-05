package aggregate

import (
	"errors"
	"fmt"
	"net"
)

var (
	ErrInvalidGameServerIP   = errors.New("invalid IP address")
	ErrInvalidGameServerPort = errors.New("invalid port number")
)

type GameServer struct {
	ip        net.IP
	port      int
	queryPort int
	params    map[string]string
}

func NewGameServer(ip net.IP, port, queryPort int) (*GameServer, error) {
	if port < 1 || port > 65535 {
		return nil, ErrInvalidGameServerPort
	}
	if queryPort < 1 || queryPort > 65535 {
		return nil, ErrInvalidGameServerPort
	}
	if ip == nil || len(ip) == 0 {
		return nil, ErrInvalidGameServerIP
	}
	ipv4 := ip.To4()
	if ipv4 == nil || !ipv4.IsGlobalUnicast() || ipv4.IsPrivate() {
		return nil, ErrInvalidGameServerIP
	}
	return &GameServer{ip: ipv4, port: port, queryPort: queryPort}, nil
}

func (gs *GameServer) GetAddr() string {
	return fmt.Sprintf("%s:%d", gs.ip, gs.port)
}

func (gs *GameServer) GetIP() net.IP {
	return gs.ip
}

func (gs *GameServer) GetDottedIP() string {
	return gs.ip.To4().String()
}

func (gs *GameServer) GetGamePort() int {
	return gs.port
}

func (gs *GameServer) GetQueryPort() int {
	return gs.queryPort
}

func (gs *GameServer) GetReportedParams() map[string]string {
	return gs.params
}

func (gs *GameServer) Update(params map[string]string) {
	gs.params = params
}
