package udpserver

import (
	"context"
	"net"
	"net/netip"
)

type Handler interface {
	Handle(context.Context, *net.UDPConn, netip.AddrPort, []byte)
}

type FuncHandler func(context.Context, *net.UDPConn, netip.AddrPort, []byte)

func (f FuncHandler) Handle(
	ctx context.Context,
	conn *net.UDPConn,
	addr netip.AddrPort,
	payload []byte,
) {
	f(ctx, conn, addr, payload)
}

func HandleFunc(f func(context.Context, *net.UDPConn, netip.AddrPort, []byte)) FuncHandler {
	return f
}
