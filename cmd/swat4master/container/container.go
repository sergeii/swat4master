package container

import (
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/internal/core/usecases/addserver"
	"github.com/sergeii/swat4master/internal/core/usecases/cleanservers"
	"github.com/sergeii/swat4master/internal/core/usecases/getserver"
	"github.com/sergeii/swat4master/internal/core/usecases/listservers"
	"github.com/sergeii/swat4master/internal/core/usecases/refreshservers"
	"github.com/sergeii/swat4master/internal/core/usecases/removeserver"
	"github.com/sergeii/swat4master/internal/core/usecases/renewserver"
	"github.com/sergeii/swat4master/internal/core/usecases/reportserver"
	"github.com/sergeii/swat4master/internal/core/usecases/reviveservers"
)

type Container struct {
	GetServer      getserver.UseCase
	AddServer      addserver.UseCase
	ListServers    listservers.UseCase
	ReportServer   reportserver.UseCase
	RenewServer    renewserver.UseCase
	RemoveServer   removeserver.UseCase
	CleanServers   cleanservers.UseCase
	RefreshServers refreshservers.UseCase
	ReviveServers  reviveservers.UseCase
}

func New(
	getServerUseCase getserver.UseCase,
	addServerUseCase addserver.UseCase,
	listServersUseCase listservers.UseCase,
	reportServerUseCase reportserver.UseCase,
	renewServerUseCase renewserver.UseCase,
	removeServerUseCase removeserver.UseCase,
	cleanServersUseCase cleanservers.UseCase,
	refreshServersUseCase refreshservers.UseCase,
	reviveServersUseCase reviveservers.UseCase,
) Container {
	return Container{
		GetServer:      getServerUseCase,
		AddServer:      addServerUseCase,
		ListServers:    listServersUseCase,
		ReportServer:   reportServerUseCase,
		RenewServer:    renewServerUseCase,
		RemoveServer:   removeServerUseCase,
		CleanServers:   cleanServersUseCase,
		RefreshServers: refreshServersUseCase,
		ReviveServers:  reviveServersUseCase,
	}
}

var Module = fx.Module("container",
	fx.Provide(getserver.New),
	fx.Provide(addserver.New),
	fx.Provide(listservers.New),
	fx.Provide(reportserver.New),
	fx.Provide(renewserver.New),
	fx.Provide(removeserver.New),
	fx.Provide(cleanservers.New),
	fx.Provide(refreshservers.New),
	fx.Provide(reviveservers.New),
	fx.Provide(New),
)
