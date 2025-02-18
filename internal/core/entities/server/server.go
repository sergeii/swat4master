package server

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/entities/details"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
)

var ErrInvalidQueryPort = errors.New("invalid port number")

type Server struct {
	Addr            addr.Addr
	QueryPort       int
	DiscoveryStatus ds.DiscoveryStatus
	Info            details.Info
	Details         details.Details

	RefreshedAt time.Time
	Version     int // lamport clock counter
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
		Addr:            addr,
		QueryPort:       queryPort,
		DiscoveryStatus: ds.New,
	}, nil
}

func MustNew(ip net.IP, port, queryPort int) Server {
	svr, err := New(ip, port, queryPort)
	if err != nil {
		panic(err)
	}
	return svr
}

func MustNewFromAddr(addr addr.Addr, queryPort int) Server {
	svr, err := NewFromAddr(addr, queryPort)
	if err != nil {
		panic(err)
	}
	return svr
}

func (gs *Server) HasDiscoveryStatus(status ds.DiscoveryStatus) bool {
	return (gs.DiscoveryStatus & status) == status
}

func (gs *Server) HasAnyDiscoveryStatus(status ds.DiscoveryStatus) bool {
	return (gs.DiscoveryStatus & status) > 0
}

func (gs *Server) HasNoDiscoveryStatus(status ds.DiscoveryStatus) bool {
	return (gs.DiscoveryStatus &^ status) == gs.DiscoveryStatus
}

func (gs *Server) UpdateDiscoveryStatus(status ds.DiscoveryStatus) {
	gs.DiscoveryStatus &^= ds.New
	gs.DiscoveryStatus |= status
}

func (gs *Server) ClearDiscoveryStatus(status ds.DiscoveryStatus) {
	gs.DiscoveryStatus &^= status
}

func (gs *Server) UpdateInfo(info details.Info) {
	gs.Info = info
}

func (gs *Server) UpdateDetails(det details.Details) {
	gs.Details = det
	gs.Info = det.Info
}

func (gs *Server) Refresh(updatedAt time.Time) {
	gs.RefreshedAt = updatedAt
}

func (gs Server) String() string {
	return fmt.Sprintf("%s (%d)", gs.Addr, gs.QueryPort)
}
