package server

import (
	"context"
	"net"
)

type Handler interface {
	Handle(context.Context, *net.TCPConn)
}

type FuncHandler func(context.Context, *net.TCPConn)

func (f FuncHandler) Handle(ctx context.Context, conn *net.TCPConn) {
	f(ctx, conn)
}

func HandleFunc(f func(context.Context, *net.TCPConn)) FuncHandler {
	return f
}
