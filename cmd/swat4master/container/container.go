package container

import (
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/internal/core/usecases/addserver"
	"github.com/sergeii/swat4master/internal/core/usecases/getserver"
	"github.com/sergeii/swat4master/internal/core/usecases/listservers"
	"github.com/sergeii/swat4master/internal/core/usecases/removeserver"
	"github.com/sergeii/swat4master/internal/core/usecases/renewserver"
	"github.com/sergeii/swat4master/internal/core/usecases/reportserver"
)

type Container struct {
	GetServer    getserver.UseCase
	AddServer    addserver.UseCase
	ListServers  listservers.UseCase
	ReportServer reportserver.UseCase
	RenewServer  renewserver.UseCase
	RemoveServer removeserver.UseCase
}

func New(
	getServerUseCase getserver.UseCase,
	addServerUseCase addserver.UseCase,
	listServersUseCase listservers.UseCase,
	reportServerUseCase reportserver.UseCase,
	renewServerUseCase renewserver.UseCase,
	removeServerUseCase removeserver.UseCase,
) Container {
	return Container{
		GetServer:    getServerUseCase,
		AddServer:    addServerUseCase,
		ListServers:  listServersUseCase,
		ReportServer: reportServerUseCase,
		RenewServer:  renewServerUseCase,
		RemoveServer: removeServerUseCase,
	}
}

var Module = fx.Module("container",
	fx.Provide(getserver.New),
	fx.Provide(addserver.New),
	fx.Provide(listservers.New),
	fx.Provide(reportserver.New),
	fx.Provide(renewserver.New),
	fx.Provide(removeserver.New),
	fx.Provide(New),
)
