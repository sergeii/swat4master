package persistence

import (
	"context"

	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
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

	// Disable maintenance notifications
	// https://github.com/redis/go-redis/issues/3536
	opts.MaintNotificationsConfig = &maintnotifications.Config{
		Mode: maintnotifications.ModeDisabled,
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
