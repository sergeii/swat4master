package instancefactory

import (
	"context"

	"github.com/sergeii/swat4master/internal/core/entities/instance"
	"github.com/sergeii/swat4master/internal/core/repositories"
)

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
