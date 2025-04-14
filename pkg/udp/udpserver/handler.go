package udpserver

import (
	"context"
	"net"
)

type Handler interface {
	Handle(context.Context, *net.UDPConn, *net.UDPAddr, []byte)
}

type FuncHandler func(context.Context, *net.UDPConn, *net.UDPAddr, []byte)

func (f FuncHandler) Handle(
	ctx context.Context,
	conn *net.UDPConn,
	addr *net.UDPAddr,
	payload []byte,
) {
	f(ctx, conn, addr, payload)
}

func HandleFunc(f func(context.Context, *net.UDPConn, *net.UDPAddr, []byte)) FuncHandler {
	return f
}
