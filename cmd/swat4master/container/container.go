package container

import (
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/internal/core/usecases/addserver"
	"github.com/sergeii/swat4master/internal/core/usecases/getserver"
	"github.com/sergeii/swat4master/internal/core/usecases/listservers"
)

type Container struct {
	GetServer   getserver.UseCase
	AddServer   addserver.UseCase
	ListServers listservers.UseCase
}

func New(
	getServerUseCase getserver.UseCase,
	addServerUseCase addserver.UseCase,
	listServersUseCase listservers.UseCase,
) Container {
	return Container{
		GetServer:   getServerUseCase,
		AddServer:   addServerUseCase,
		ListServers: listServersUseCase,
	}
}

var Module = fx.Module("container",
	fx.Provide(getserver.New),
	fx.Provide(addserver.New),
	fx.Provide(listservers.New),
	fx.Provide(New),
)
