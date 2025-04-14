package container

import (
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/internal/core/usecases/addserver"
	"github.com/sergeii/swat4master/internal/core/usecases/getserver"
	"github.com/sergeii/swat4master/internal/core/usecases/listservers"
	"github.com/sergeii/swat4master/internal/core/usecases/probeserver"
	"github.com/sergeii/swat4master/internal/core/usecases/refreshservers"
	"github.com/sergeii/swat4master/internal/core/usecases/removeserver"
	"github.com/sergeii/swat4master/internal/core/usecases/renewserver"
	"github.com/sergeii/swat4master/internal/core/usecases/reportserver"
	"github.com/sergeii/swat4master/internal/core/usecases/reviveservers"
	"github.com/sergeii/swat4master/internal/settings"
)

type UseCaseConfigs struct {
	fx.Out

	AddServerOptions      addserver.UseCaseOptions
	ReportServerOptions   reportserver.UseCaseOptions
	RefreshServersOptions refreshservers.UseCaseOptions
	ReviveServersOptions  reviveservers.UseCaseOptions
}

func NewUseCaseConfigs(settings settings.Settings) UseCaseConfigs {
	return UseCaseConfigs{
		AddServerOptions: addserver.UseCaseOptions{
			MaxProbeRetries: settings.DiscoveryRevivalRetries,
		},
		ReportServerOptions: reportserver.UseCaseOptions{
			MaxProbeRetries: settings.DiscoveryRevivalRetries,
		},
		RefreshServersOptions: refreshservers.UseCaseOptions{
			MaxProbeRetries: settings.DiscoveryRefreshRetries,
		},
		ReviveServersOptions: reviveservers.UseCaseOptions{
			MaxProbeRetries: settings.DiscoveryRevivalRetries,
		},
	}
}

type Container struct {
	AddServer      addserver.UseCase
	GetServer      getserver.UseCase
	ListServers    listservers.UseCase
	ProbeServer    probeserver.UseCase
	RefreshServers refreshservers.UseCase
	RemoveServer   removeserver.UseCase
	RenewServer    renewserver.UseCase
	ReportServer   reportserver.UseCase
	ReviveServers  reviveservers.UseCase
}

func NewContainer(
	addServerUseCase addserver.UseCase,
	getServerUseCase getserver.UseCase,
	listServersUseCase listservers.UseCase,
	probeServerUseCase probeserver.UseCase,
	refreshServersUseCase refreshservers.UseCase,
	removeServerUseCase removeserver.UseCase,
	renewServerUseCase renewserver.UseCase,
	reportServerUseCase reportserver.UseCase,
	reviveServersUseCase reviveservers.UseCase,
) Container {
	return Container{
		AddServer:      addServerUseCase,
		GetServer:      getServerUseCase,
		ListServers:    listServersUseCase,
		ProbeServer:    probeServerUseCase,
		RefreshServers: refreshServersUseCase,
		RemoveServer:   removeServerUseCase,
		RenewServer:    renewServerUseCase,
		ReportServer:   reportServerUseCase,
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
	fx.Provide(refreshservers.New),
	fx.Provide(reviveservers.New),
	fx.Provide(probeserver.New),
	fx.Provide(NewUseCaseConfigs),
	fx.Provide(NewContainer),
)
