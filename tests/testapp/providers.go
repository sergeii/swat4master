package testapp

import (
	"context"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/internal/settings"
)

func ProvideSettings() settings.Settings {
	return settings.Settings{
		ServerLiveness:          time.Minute * 3,
		DiscoveryRevivalRetries: 2,
		DiscoveryRefreshRetries: 4,
	}
}

func ProvidePersistence(lc fx.Lifecycle) (*redis.Client, error) {
	mr, err := miniredis.Run()
	if err != nil {
		return nil, err
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			defer mr.Close()
			return rdb.Close()
		},
	})

	return rdb, nil
}

func NoLogging() *zerolog.Logger {
	logger := zerolog.Nop()
	return &logger
}
