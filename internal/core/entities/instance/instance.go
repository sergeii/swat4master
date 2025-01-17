package instance

import (
	"fmt"
	"net"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
)

type Identifier [4]byte

func NewID(bytes []byte) (Identifier, error) {
	if len(bytes) != 4 {
		return Identifier{}, fmt.Errorf("instance ID must be 4 bytes long, got %d", len(bytes))
	}
	var id Identifier
	copy(id[:], bytes)
	return id, nil
}

func MustNewID(bytes []byte) Identifier {
	id, err := NewID(bytes)
	if err != nil {
		panic(err)
	}
	return id
}

func (id Identifier) Hex() string {
	return fmt.Sprintf("%x", id)
}

type Instance struct {
	ID   Identifier
	Addr addr.Addr
}

var Blank Instance // nolint: gochecknoglobals

func New(id Identifier, ip net.IP, port int) (Instance, error) {
	insAddr, err := addr.New(ip, port)
	if err != nil {
		return Blank, err
	}
	return Instance{id, insAddr}, nil
}

func MustNew(id Identifier, ip net.IP, port int) Instance {
	ins, err := New(id, ip, port)
	if err != nil {
		panic(err)
	}
	return ins
}
