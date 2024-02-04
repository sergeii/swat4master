package instance

import (
	"net"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
)

type Instance struct {
	ID   string
	Addr addr.Addr
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
