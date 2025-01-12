package instancefactory

import (
	"context"
	"net"

	"github.com/sergeii/swat4master/internal/core/entities/instance"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/testutils"
	"github.com/sergeii/swat4master/pkg/random"
)

type BuildParams struct {
	ID   instance.Identifier
	IP   string
	Port int
}

type BuildOption func(*BuildParams)

func WithID(id instance.Identifier) BuildOption {
	return func(p *BuildParams) {
		p.ID = id
	}
}

func WithStringID(id string) BuildOption {
	return func(p *BuildParams) {
		p.ID = instance.MustNewID([]byte(id))
	}
}

func WithBytesID(id []byte) BuildOption {
	return func(p *BuildParams) {
		p.ID = instance.MustNewID(id)
	}
}

func WithRandomID() BuildOption {
	return func(p *BuildParams) {
		p.ID = instance.MustNewID(random.RandBytes(4))
	}
}

func WithServerAddress(ip string, port int) BuildOption {
	return func(p *BuildParams) {
		p.IP = ip
		p.Port = port
	}
}

func WithRandomServerAddress() BuildOption {
	return func(p *BuildParams) {
		p.IP = testutils.GenRandomIP().String()
		p.Port = random.RandInt(1, 65534)
	}
}

func Build(opts ...BuildOption) instance.Instance {
	params := BuildParams{
		ID:   instance.MustNewID([]byte("test")),
		IP:   "1.1.1.1",
		Port: 10480,
	}

	for _, opt := range opts {
		opt(&params)
	}

	return instance.MustNew(params.ID, net.ParseIP(params.IP), params.Port)
}

func Save(
	ctx context.Context,
	repo repositories.InstanceRepository,
	ins instance.Instance,
) instance.Instance {
	if err := repo.Add(ctx, ins); err != nil {
		panic(err)
	}
	return ins
}

func Create(
	ctx context.Context,
	repo repositories.InstanceRepository,
	opts ...BuildOption,
) instance.Instance {
	svr := Build(opts...)
	return Save(ctx, repo, svr)
}
