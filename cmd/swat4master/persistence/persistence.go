package persistence

import (
	"context"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

type Persistence struct {
	fx.Out

	RedisClient *redis.Client
}

type Config struct {
	RedisURL string
}

func Provide(cfg Config, lc fx.Lifecycle) (Persistence, error) {
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return Persistence{}, err
	}

	redisClient := redis.NewClient(opts)

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			return redisClient.Close()
		},
	})

	persistence := Persistence{
		RedisClient: redisClient,
	}

	return persistence, nil
}
