package addr

import (
	"errors"
	"net"
)

type PublicAddr struct {
	addr Addr
}

var BlankPublicAddr PublicAddr // nolint: gochecknoglobals

var ErrInvalidPublicIP = errors.New("invalid public IP address")

func NewPublicAddr(addr Addr) (PublicAddr, error) {
	ipv4 := net.IPv4(addr.IP[0], addr.IP[1], addr.IP[2], addr.IP[3])

	if ipv4.IsPrivate() || ipv4.IsLoopback() {
		return PublicAddr{}, ErrInvalidPublicIP
	}

	return PublicAddr{addr}, nil
}

func MustNewPublicAddr(addr Addr) PublicAddr {
	pa, err := NewPublicAddr(addr)
	if err != nil {
		panic(err)
	}
	return pa
}

func (pa PublicAddr) ToAddr() Addr {
	return pa.addr
}
