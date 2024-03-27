package application

import (
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/clock"
	"github.com/sergeii/swat4master/cmd/swat4master/container"
	"github.com/sergeii/swat4master/cmd/swat4master/logging"
	"github.com/sergeii/swat4master/cmd/swat4master/persistence"
	"github.com/sergeii/swat4master/internal/services/monitoring"
	"github.com/sergeii/swat4master/internal/services/probe"
	"github.com/sergeii/swat4master/internal/validation"
)

var Module = fx.Module("application",
	fx.Provide(logging.Provide),
	fx.Provide(clock.Provide),
	fx.Invoke(logging.NoGlobal),
	fx.Provide(validation.New),
	fx.Provide(persistence.Provide),
	fx.Provide(monitoring.NewMetricService),
	fx.Provide(probe.NewService),
	container.Module,
)
