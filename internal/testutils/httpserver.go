package testutils

import (
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/rest"
	"github.com/sergeii/swat4master/internal/rest/api"
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
		fx.Provide(testapp.NoLogging),
		fx.Provide(testapp.ProvideSettings),
		fx.Provide(testapp.ProvidePersistence),
		application.Module,
		fx.Provide(api.New),
		fx.Provide(rest.NewRouter),
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
