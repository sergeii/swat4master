package renewserver

import (
	"context"
	"errors"
	"net"

	"github.com/jonboulle/clockwork"

	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
)

var ErrUnknownInstanceID = errors.New("unknown instance id")

type UseCase struct {
	instanceRepo repositories.InstanceRepository
	serverRepo   repositories.ServerRepository
	clock        clockwork.Clock
}

func New(
	instanceRepo repositories.InstanceRepository,
	serverRepo repositories.ServerRepository,
	clock clockwork.Clock,
) UseCase {
	return UseCase{
		instanceRepo: instanceRepo,
		serverRepo:   serverRepo,
		clock:        clock,
	}
}

type Request struct {
	instanceID string
	ipAddr     net.IP
}

func NewRequest(instanceID string, ipAddr net.IP) Request {
	return Request{
		instanceID: instanceID,
		ipAddr:     ipAddr,
	}
}

func (uc UseCase) Execute(ctx context.Context, req Request) error {
	instance, err := uc.instanceRepo.GetByID(ctx, req.instanceID)
	if err != nil {
		return err
	}

	// the addressed must match, otherwise it could be a spoofing attempt
	if !instance.Addr.GetIP().Equal(req.ipAddr.To4()) {
		return ErrUnknownInstanceID
	}

	svr, err := uc.serverRepo.Get(ctx, instance.Addr)
	if err != nil {
		return err
	}

	// although keepalive request does not provide
	// any additional information about their server such
	// as player count or the scores,
	// we still want to bump the server,
	// so it keeps appearing in the list
	now := uc.clock.Now()
	svr.Refresh(now)

	if _, updateErr := uc.serverRepo.Update(ctx, svr, func(s *server.Server) bool {
		s.Refresh(now)
		return true
	}); updateErr != nil {
		return updateErr
	}

	return nil
}
