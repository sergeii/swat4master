package persistence

import (
	"context"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
)

type Persistence struct {
	fx.Out

	RedisClient *redis.Client
}

func Provide(cfg config.Config, lc fx.Lifecycle) (Persistence, error) {
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return Persistence{}, err
	}

	redisClient := redis.NewClient(opts)

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			return redisClient.Close()
		},
	})

	persistence := Persistence{
		RedisClient: redisClient,
	}

	return persistence, nil
}
