package testredis

import (
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
		testutils.MustNoError(rdb.Close())
	})
	return rdb
}

func MakeClient(t *testing.T) *redis.Client {
	mr := miniredis.RunT(t)
	return MakeClientFromMini(t, mr)
}
