package addr

import (
	"errors"
	"fmt"
	"net"
)

type Addr struct {
	IP   [4]byte
	Port int
}

var Blank Addr // nolint: gochecknoglobals

var (
	ErrInvalidServerIP   = errors.New("invalid IP address")
	ErrInvalidServerPort = errors.New("invalid port number")
)

func New(ip net.IP, port int) (Addr, error) {
	if port < 1 || port > 65535 {
		return Blank, ErrInvalidServerPort
	}

	if len(ip) == 0 {
		return Blank, ErrInvalidServerIP
	}

	ipv4 := ip.To4()
	if ipv4 == nil || !ipv4.IsGlobalUnicast() || ipv4.IsPrivate() {
		return Blank, ErrInvalidServerIP
	}

	addr := Addr{Port: port}
	copy(addr.IP[:], ipv4)

	return addr, nil
}

func NewForTesting(ip net.IP, port int) Addr {
	ipv4 := ip.To4()
	if ipv4 == nil {
		panic("invalid ip")
	}
	addr := Addr{Port: port}
	copy(addr.IP[:], ipv4)
	return addr
}

func NewFromString(ip string, port int) (Addr, error) {
	return New(net.ParseIP(ip), port)
}

func MustNewFromString(ip string, port int) Addr {
	addr, err := NewFromString(ip, port)
	if err != nil {
		panic(err)
	}
	return addr
}

func (a Addr) GetIP() net.IP {
	return net.IPv4(a.IP[0], a.IP[1], a.IP[2], a.IP[3]).To4()
}

func (a Addr) GetDottedIP() string {
	return fmt.Sprintf("%d.%d.%d.%d", a.IP[0], a.IP[1], a.IP[2], a.IP[3])
}

func (a Addr) String() string {
	return fmt.Sprintf("%s:%d", a.GetDottedIP(), a.Port)
}
