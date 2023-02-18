package application

import (
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/logging"
	"github.com/sergeii/swat4master/cmd/swat4master/persistence"
	"github.com/sergeii/swat4master/internal/services/discovery/finding"
	"github.com/sergeii/swat4master/internal/services/monitoring"
	"github.com/sergeii/swat4master/internal/services/probe"
	"github.com/sergeii/swat4master/internal/services/server"
	"github.com/sergeii/swat4master/internal/validation"
	"github.com/sergeii/swat4master/pkg/random"
)

var Module = fx.Module("application",
	fx.Provide(random.Seed),
	fx.Provide(logging.Provide),
	fx.Invoke(logging.NoGlobal),
	fx.Provide(validation.New),
	fx.Provide(persistence.Provide),
	fx.Provide(monitoring.NewMetricService),
	fx.Provide(finding.NewService),
	fx.Provide(server.NewService),
	fx.Provide(probe.NewService),
)
