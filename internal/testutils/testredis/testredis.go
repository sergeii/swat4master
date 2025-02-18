package testredis

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/sergeii/swat4master/internal/testutils"
)

func MakeClientFromMini(t *testing.T, mr *miniredis.Miniredis) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	t.Cleanup(func() {
		testutils.MustNoErr(rdb.Close())
	})
	return rdb
}

func MakeRealClient(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	t.Cleanup(func() {
		client.FlushDB(context.Background())
		testutils.MustNoErr(client.Close())
	})
	return client
}

func MakeClient(t *testing.T) *redis.Client {
	mr := miniredis.RunT(t)
	return MakeClientFromMini(t, mr)
}
