package challenge

import (
	"context"
	"net"

	"github.com/sergeii/swat4master/internal/core/entities/master"
	"github.com/sergeii/swat4master/internal/reporter"
)

type Handler struct{}

func New(d *reporter.Dispatcher) (Handler, error) {
	handler := Handler{}
	if err := d.Register(master.MsgChallenge, handler); err != nil {
		return Handler{}, err
	}
	return handler, nil
}

func (h Handler) Handle(_ context.Context, _ *net.UDPAddr, payload []byte) ([]byte, error) {
	instanceID, _, err := reporter.ParseInstanceID(payload)
	if err != nil {
		return nil, err
	}
	resp := make([]byte, 0, 7)
	resp = append(resp, 0xfe, 0xfd, 0x0a)
	resp = append(resp, instanceID...)
	return resp, nil
}
