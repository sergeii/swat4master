package keepalive

import (
	"context"
	"net"

	"github.com/sergeii/swat4master/internal/core/entities/master"
	"github.com/sergeii/swat4master/internal/core/usecases/renewserver"
	"github.com/sergeii/swat4master/internal/reporter"
)

type Handler struct {
	uc renewserver.UseCase
}

func New(
	dispatcher *reporter.Dispatcher,
	uc renewserver.UseCase,
) (Handler, error) {
	handler := Handler{
		uc: uc,
	}
	if err := dispatcher.Register(master.MsgKeepalive, handler); err != nil {
		return Handler{}, err
	}
	return handler, nil
}

func (h Handler) Handle(
	ctx context.Context,
	connAddr *net.UDPAddr,
	payload []byte,
) ([]byte, error) {
	instanceID, _, err := reporter.ParseInstanceID(payload)
	if err != nil {
		return nil, err
	}

	ucReq := renewserver.NewRequest(instanceID, connAddr.IP)
	if err := h.uc.Execute(ctx, ucReq); err != nil {
		return nil, err
	}

	return nil, nil
}
