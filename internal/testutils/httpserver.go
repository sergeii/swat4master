package testutils

import (
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/api"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/tests/testapp"
)

type TestServerRepositories struct {
	Servers   repositories.ServerRepository
	Instances repositories.InstanceRepository
	Probes    repositories.ProbeRepository
}

func PrepareTestServer(tb fxtest.TB, extra ...fx.Option) (*httptest.Server, func()) {
	gin.SetMode(gin.ReleaseMode) // prevent gin from overwriting middlewares

	var router *gin.Engine
	fxopts := []fx.Option{
		fx.Provide(func() config.Config {
			return config.Config{
				HTTPListenAddr: "localhost:11337",
			}
		}),
		fx.Provide(testapp.ProvidePersistence),
		application.Module,
		api.Module,
		fx.Decorate(testapp.NoLogging),
		fx.NopLogger,
		fx.Populate(&router),
	}
	fxopts = append(fxopts, extra...)

	app := fxtest.New(tb, fxopts...)
	app.RequireStart()

	ts := httptest.NewServer(router)

	return ts, func() {
		defer app.RequireStop() // nolint: errcheck
		defer ts.Close()
	}
}

func PrepareTestServerWithRepos(
	tb fxtest.TB,
	extra ...fx.Option,
) (*httptest.Server, TestServerRepositories, func()) {
	var repos TestServerRepositories
	extra = append(
		extra,
		fx.Populate(&repos.Servers, &repos.Instances, &repos.Probes),
	)
	ts, cleanup := PrepareTestServer(tb, extra...)
	return ts, repos, cleanup
}
