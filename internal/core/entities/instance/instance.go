package instance

import (
	"net"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
)

type Instance struct {
	id   string
	addr addr.Addr
}

var Blank Instance // nolint: gochecknoglobals

func New(id string, ip net.IP, port int) (Instance, error) {
	insAddr, err := addr.New(ip, port)
	if err != nil {
		return Blank, err
	}
	return Instance{id, insAddr}, nil
}

func MustNew(id string, ip net.IP, port int) Instance {
	ins, err := New(id, ip, port)
	if err != nil {
		panic(err)
	}
	return ins
}

func (ins Instance) GetID() string {
	return ins.id
}

func (ins Instance) GetIP() net.IP {
	return ins.addr.GetIP()
}

func (ins Instance) GetDottedIP() string {
	return ins.addr.GetDottedIP()
}

func (ins Instance) GetPort() int {
	return ins.addr.Port
}

func (ins Instance) GetAddr() addr.Addr {
	return ins.addr
}
