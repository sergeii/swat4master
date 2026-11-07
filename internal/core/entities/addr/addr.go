package addr

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

type Addr struct {
	IP   [4]byte `json:"ip"`
	Port int     `json:"port"`
}

var Blank Addr //nolint: gochecknoglobals

var (
	ErrInvalidIP   = errors.New("invalid IP address")
	ErrInvalidPort = errors.New("invalid port number")
)

func New(ip net.IP, port int) (Addr, error) {
	// an invalid ip yields the zero netip.Addr, which is rejected as not IPv4
	nip, _ := netip.AddrFromSlice(ip)
	return NewFromNetip(nip.Unmap(), port)
}

func NewFromNetip(ip netip.Addr, port int) (Addr, error) {
	if port < 1 || port > 65535 {
		return Blank, ErrInvalidPort
	}

	switch {
	case !ip.Is4():
		return Blank, ErrInvalidIP
	case !ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback():
		return Blank, ErrInvalidIP
	}

	return Addr{IP: ip.As4(), Port: port}, nil
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

func NewFromDotted(ip string, port int) (Addr, error) {
	return New(net.ParseIP(ip), port)
}

func MustNewFromDotted(ip string, port int) Addr {
	addr, err := NewFromDotted(ip, port)
	if err != nil {
		panic(err)
	}
	return addr
}

func NewFromString(addrAndPort string) (Addr, error) {
	maybeIP, maybePort, ok := strings.Cut(addrAndPort, ":")
	if !ok || maybeIP == "" || maybePort == "" {
		return Blank, ErrInvalidIP
	}

	maybePortNumber, err := strconv.Atoi(maybePort)
	if err != nil {
		return Blank, ErrInvalidPort
	}

	return NewFromDotted(maybeIP, maybePortNumber)
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
