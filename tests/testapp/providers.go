package testapp

import (
	"context"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

func ProvidePersistence(lc fx.Lifecycle) (*redis.Client, error) {
	mr, err := miniredis.Run()
	if err != nil {
		return nil, err
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
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
