package reporter

import (
	"context"
	"net/netip"
)

type Handler interface {
	Handle(ctx context.Context, connAddr netip.AddrPort, payload []byte) ([]byte, error)
}
