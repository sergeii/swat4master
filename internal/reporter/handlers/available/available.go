package available

import (
	"context"
	"net"

	"github.com/sergeii/swat4master/internal/core/entities/master"
	"github.com/sergeii/swat4master/internal/reporter"
)

type Handler struct{}

func New(d *reporter.Dispatcher) (Handler, error) {
	handler := Handler{}
	if err := d.Register(master.MsgAvailable, handler); err != nil {
		return Handler{}, err
	}
	return handler, nil
}

func (h Handler) Handle(context.Context, *net.UDPAddr, []byte) ([]byte, error) {
	return master.ResponseIsAvailable, nil
}
