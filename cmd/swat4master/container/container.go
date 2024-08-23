package container

import (
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
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

type UseCaseConfigs struct {
	fx.Out

	AddServerOptions      addserver.UseCaseOptions
	ReportServerOptions   reportserver.UseCaseOptions
	RefreshServersOptions refreshservers.UseCaseOptions
	ReviveServersOptions  reviveservers.UseCaseOptions
}

func NewUseCaseConfigs(cfg config.Config) UseCaseConfigs {
	return UseCaseConfigs{
		AddServerOptions: addserver.UseCaseOptions{
			MaxProbeRetries: cfg.DiscoveryRevivalRetries,
		},
		ReportServerOptions: reportserver.UseCaseOptions{
			MaxProbeRetries: cfg.DiscoveryRevivalRetries,
		},
		RefreshServersOptions: refreshservers.UseCaseOptions{
			MaxProbeRetries: cfg.DiscoveryRefreshRetries,
		},
		ReviveServersOptions: reviveservers.UseCaseOptions{
			MaxProbeRetries: cfg.DiscoveryRevivalRetries,
		},
	}
}

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

func NewContainer(
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
	fx.Provide(NewUseCaseConfigs),
	fx.Provide(NewContainer),
)
