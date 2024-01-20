package reporter

import (
	"context"
	"net"
)

type Handler interface {
	Handle(ctx context.Context, connAddr *net.UDPAddr, payload []byte) ([]byte, error)
}
