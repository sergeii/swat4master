package servers_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/filterset"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/persistence/redis/redislock"
	"github.com/sergeii/swat4master/internal/persistence/redis/repositories/servers"
	tu "github.com/sergeii/swat4master/internal/testutils"
	"github.com/sergeii/swat4master/internal/testutils/factories/serverfactory"
	"github.com/sergeii/swat4master/internal/testutils/testredis"
)

type updated struct {
	Addr string
	Time float64
}

type storageState struct {
	Updates       []updated
	UpdatesKeys   []string
	Refreshes     []updated
	RefreshesKeys []string
	Statuses      map[string][]string
	Items         map[string]server.Server
}

func collectStorageState(ctx context.Context, rdb *redis.Client) storageState {
	zUpdatedMembers := tu.Must(rdb.ZRangeWithScores(ctx, "servers:updated", 0, -1).Result())
	updates := make([]updated, 0, len(zUpdatedMembers))
	updatesKeys := make([]string, 0, len(zUpdatedMembers))
	for _, m := range zUpdatedMembers {
		updates = append(updates, updated{Addr: m.Member.(string), Time: m.Score}) // nolint:forcetypeassert
		updatesKeys = append(updatesKeys, m.Member.(string))                       // nolint:forcetypeassert
	}

	zRefreshedMembers := tu.Must(rdb.ZRangeWithScores(ctx, "servers:refreshed", 0, -1).Result())
	refreshes := make([]updated, 0, len(zRefreshedMembers))
	refreshesKeys := make([]string, 0, len(zRefreshedMembers))
	for _, m := range zRefreshedMembers {
		refreshes = append(refreshes, updated{Addr: m.Member.(string), Time: m.Score}) // nolint:forcetypeassert
		refreshesKeys = append(refreshesKeys, m.Member.(string))                       // nolint:forcetypeassert
	}

	statuses := make(map[string][]string)
	statusKeys := tu.Must(rdb.Keys(ctx, "servers:status:*").Result())
	for _, k := range statusKeys {
		sStatusMembers := tu.Must(rdb.SMembers(ctx, k).Result())
		statusName, _ := strings.CutPrefix(k, "servers:status:")
		statuses[statusName] = sStatusMembers
	}

	hItems := tu.Must(rdb.HGetAll(ctx, "servers:items").Result())
	items := make(map[string]server.Server)
	for k, v := range hItems {
		var item server.Server
		tu.MustNoErr(json.Unmarshal([]byte(v), &item)) // nolint:musttag
		items[k] = item
	}

	return storageState{
		Updates:       updates,
		UpdatesKeys:   updatesKeys,
		Refreshes:     refreshes,
		RefreshesKeys: refreshesKeys,
		Statuses:      statuses,
		Items:         items,
	}
}

type testState struct {
	Clock *clockwork.FakeClock
	Redis *redis.Client
	Repo  *servers.Repository
}

func setup(t *testing.T) testState {
	c := clockwork.NewFakeClock()
	rdb := testredis.MakeClient(t)
	logger := zerolog.Nop()
	locker := redislock.NewManager(rdb, &logger)
	repo := servers.New(rdb, locker, c)
	return testState{Clock: c, Redis: rdb, Repo: repo}
}

func micro(t time.Time) time.Time {
	return t.Truncate(time.Microsecond)
}

func TestServersRedisRepo_Get_OK(t *testing.T) {
	ctx := context.TODO()
	ts := setup(t)
	then := ts.Clock.Now().UTC()

	// Given a repository with multiple servers in it
	svr1 := serverfactory.Build(
		serverfactory.WithAddress("1.1.1.1", 10480),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
		serverfactory.WithRefreshedAt(then),
	)
	svr2 := serverfactory.Build(
		serverfactory.WithAddress("2.2.2.2", 10580),
		serverfactory.WithDiscoveryStatus(ds.Details|ds.Info),
		serverfactory.WithRefreshedAt(then.Add(time.Second)),
	)
	svr3 := serverfactory.Build(
		serverfactory.WithAddress("3.3.3.3", 10480),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Details|ds.Info),
		serverfactory.WithRefreshedAt(then.Add(time.Hour)),
	)
	for _, svr := range []server.Server{svr1, svr2, svr3} {
		tu.Must(ts.Repo.Add(ctx, svr, repositories.ServerOnConflictIgnore))
	}

	// When getting one of the servers
	addr1 := addr.MustNewFromDotted("1.1.1.1", 10480)
	got1, err := ts.Repo.Get(ctx, addr1)
	// Then the server is expected to be returned
	require.NoError(t, err)
	assert.Equal(t, addr1, got1.Addr)
	assert.Equal(t, 1, got1.Version)
	assert.Equal(t, svr1.Info, got1.Info)
	assert.Equal(t, svr1.Details, got1.Details)
	assert.Equal(t, ds.Master|ds.Info, got1.DiscoveryStatus)
	assert.Equal(t, micro(then), micro(got1.RefreshedAt))

	// When getting another server
	addr2 := addr.MustNewFromDotted("3.3.3.3", 10480)
	got2, err := ts.Repo.Get(ctx, addr2)
	// Then the server is expected to be returned
	require.NoError(t, err)
	assert.Equal(t, addr2, got2.Addr)
	assert.Equal(t, 1, got2.Version)
	assert.Equal(t, svr3.Info, got2.Info)
	assert.Equal(t, svr3.Details, got2.Details)
	assert.Equal(t, ds.Master|ds.Details|ds.Info, got2.DiscoveryStatus)
	assert.Equal(t, micro(then.Add(time.Hour)), micro(got2.RefreshedAt))

	// When getting a non-existent server
	_, err = ts.Repo.Get(ctx, addr.MustNewFromDotted("1.1.1.1", 13480))
	// Then an error is expected
	require.ErrorIs(t, err, repositories.ErrServerNotFound)
}

func TestServersRedisRepo_Add_OK(t *testing.T) {
	ctx := context.TODO()
	ts := setup(t)
	then := ts.Clock.Now()

	// Given a repository with no servers in it

	// And a server to be added
	svr1 := serverfactory.Build(
		serverfactory.WithAddress("1.1.1.1", 10480),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
		serverfactory.WithRefreshedAt(ts.Clock.Now()),
	)

	// When adding a server to the repository a few moments later
	ts.Clock.Advance(time.Millisecond * 10)
	added1, err := ts.Repo.Add(ctx, svr1, repositories.ServerOnConflictIgnore)
	require.NoError(t, err)

	// Then the server is expected to be added to the repository with the correct version
	assert.Equal(t, 0, svr1.Version)
	assert.Equal(t, 1, added1.Version)

	// And the redis storage is expected to contain the server
	state := collectStorageState(ctx, ts.Redis)
	assert.ElementsMatch(t, []string{"1.1.1.1:10480"}, tu.MapKeys(state.Items))
	assert.ElementsMatch(t, []string{"1.1.1.1:10480"}, state.UpdatesKeys)
	assert.ElementsMatch(t, []string{"1.1.1.1:10480"}, state.RefreshesKeys)
	assert.ElementsMatch(t, []string{"1.1.1.1:10480"}, state.Statuses["master"])
	assert.ElementsMatch(t, []string{"1.1.1.1:10480"}, state.Statuses["info"])

	assert.EqualExportedValues(t, added1, state.Items["1.1.1.1:10480"])
	assert.Equal(t, float64(then.UnixNano()), state.Refreshes[0].Time)
	assert.Equal(t, float64(then.Add(time.Millisecond*10).UnixNano()), state.Updates[0].Time)

	// When adding more servers to the repository a few moments later
	svr2 := serverfactory.Build(
		serverfactory.WithAddress("2.2.2.2", 10580),
		serverfactory.WithDiscoveryStatus(ds.Details|ds.Info),
	)
	svr3 := serverfactory.Build(
		serverfactory.WithAddress("3.3.3.3", 10480),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Details|ds.Info),
		serverfactory.WithRefreshedAt(ts.Clock.Now()),
	)
	for _, svr := range []server.Server{svr2, svr3} {
		ts.Clock.Advance(time.Millisecond * 10)
		_, err = ts.Repo.Add(ctx, svr, repositories.ServerOnConflictIgnore)
		require.NoError(t, err)
	}

	// Then the storage is expected to contain all the servers
	state = collectStorageState(ctx, ts.Redis)

	assert.ElementsMatch(t, []string{"1.1.1.1:10480", "2.2.2.2:10580", "3.3.3.3:10480"}, tu.MapKeys(state.Items))
	assert.ElementsMatch(t, []string{"1.1.1.1:10480", "2.2.2.2:10580", "3.3.3.3:10480"}, state.UpdatesKeys)
	assert.ElementsMatch(t, []string{"1.1.1.1:10480", "3.3.3.3:10480"}, state.RefreshesKeys)
	assert.ElementsMatch(t, []string{"1.1.1.1:10480", "3.3.3.3:10480"}, state.Statuses["master"])
	assert.ElementsMatch(t, []string{"1.1.1.1:10480", "2.2.2.2:10580", "3.3.3.3:10480"}, state.Statuses["info"])
	assert.ElementsMatch(t, []string{"2.2.2.2:10580", "3.3.3.3:10480"}, state.Statuses["details"])

	assert.Equal(t, float64(then.Add(time.Millisecond*10).UnixNano()), state.Refreshes[1].Time)
	assert.Equal(t, float64(then.Add(time.Millisecond*20).UnixNano()), state.Updates[1].Time)
	assert.Equal(t, float64(then.Add(time.Millisecond*30).UnixNano()), state.Updates[2].Time)
}

func TestServersRedisRepo_Add_OnConflictIgnore(t *testing.T) {
	ctx := context.TODO()
	ts := setup(t)
	then := ts.Clock.Now()

	// Given a repository with a server in it
	svr := serverfactory.Build(
		serverfactory.WithAddress("1.1.1.1", 10480),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
		serverfactory.WithRefreshedAt(ts.Clock.Now()),
	)
	original, err := ts.Repo.Add(ctx, svr, repositories.ServerOnConflictIgnore)
	require.NoError(t, err)

	// And some time has passed
	ts.Clock.Advance(time.Second)

	// When adding the same server with a conflict resolution strategy set to ignore
	other := serverfactory.Build(
		serverfactory.WithAddress("1.1.1.1", 10480),
		serverfactory.WithDiscoveryStatus(ds.NoPort),
		serverfactory.WithRefreshedAt(ts.Clock.Now()),
	)
	_, err = ts.Repo.Add(ctx, other, func(s *server.Server) bool {
		s.UpdateDiscoveryStatus(ds.NoDetails | ds.NoPort)
		return false
	})

	// Then an error is expected
	require.ErrorIs(t, err, repositories.ErrServerExists)

	// And the server is expected to remain unchanged in the storage
	state := collectStorageState(ctx, ts.Redis)

	assert.ElementsMatch(t, []string{"1.1.1.1:10480"}, tu.MapKeys(state.Items))
	assert.EqualExportedValues(t, original, state.Items["1.1.1.1:10480"])
	assert.Equal(t, 1, original.Version)

	// And the server has the original statuses
	for _, wantStatus := range []string{"master", "info"} {
		assert.ElementsMatch(t, []string{"1.1.1.1:10480"}, state.Statuses[wantStatus])
	}
	for _, dontWantStatus := range []string{"no_port", "no_details"} {
		assert.Empty(t, state.Statuses[dontWantStatus])
	}

	// And the server retains the original refresh time and update time
	assert.ElementsMatch(t, []string{"1.1.1.1:10480"}, state.RefreshesKeys)
	assert.Equal(t, float64(then.UnixNano()), state.Refreshes[0].Time)
	assert.ElementsMatch(t, []string{"1.1.1.1:10480"}, state.UpdatesKeys)
	assert.Equal(t, float64(then.UnixNano()), state.Updates[0].Time)
}

func TestServersRedisRepo_Add_OnConflictUpdate(t *testing.T) {
	ctx := context.TODO()
	ts := setup(t)
	then := ts.Clock.Now()

	// Given a repository with a server in it
	svr := serverfactory.Build(
		serverfactory.WithAddress("1.1.1.1", 10480),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
		serverfactory.WithRefreshedAt(ts.Clock.Now()),
	)
	tu.Must(ts.Repo.Add(ctx, svr, repositories.ServerOnConflictIgnore))

	// And some time has passed
	ts.Clock.Advance(time.Second)

	// When adding the same server with a conflict resolution strategy set to resolve
	other := serverfactory.Build(
		serverfactory.WithAddress("1.1.1.1", 10480),
		serverfactory.WithDiscoveryStatus(ds.NoPort),
		serverfactory.WithRefreshedAt(ts.Clock.Now()),
	)
	added, err := ts.Repo.Add(ctx, other, func(s *server.Server) bool {
		s.UpdateDiscoveryStatus(ds.NoDetails | ds.NoPort)
		return true
	})

	// Then the server is expected to be added to the repository with the correct version
	require.NoError(t, err)
	assert.Equal(t, 2, added.Version)
	assert.Equal(t, ds.Master|ds.Info|ds.NoPort|ds.NoDetails, added.DiscoveryStatus)

	// And the redis storage is expected to contain the server
	state := collectStorageState(ctx, ts.Redis)
	assert.ElementsMatch(t, []string{"1.1.1.1:10480"}, tu.MapKeys(state.Items))
	assert.EqualExportedValues(t, added, state.Items["1.1.1.1:10480"])

	// And the server has the merged statuses
	for _, wantStatus := range []string{"master", "info", "no_port", "no_details"} {
		assert.ElementsMatch(t, []string{"1.1.1.1:10480"}, state.Statuses[wantStatus])
	}

	// And the server retains the original refresh time
	assert.ElementsMatch(t, []string{"1.1.1.1:10480"}, state.RefreshesKeys)
	assert.Equal(t, float64(then.UnixNano()), state.Refreshes[0].Time)
	// but the update time is updated
	assert.ElementsMatch(t, []string{"1.1.1.1:10480"}, state.UpdatesKeys)
	assert.Equal(t, float64(then.Add(time.Second).UnixNano()), state.Updates[0].Time)
}

func TestServersRedisRepo_Add_OnConflictUpdateConcurrently(t *testing.T) {
	ctx := context.TODO()
	ts := setup(t)

	// Given an empty repository and a server to be added
	svr := serverfactory.Build(
		serverfactory.WithAddress("1.1.1.1", 10480),
	)

	// When adding the server concurrently
	wg := &sync.WaitGroup{}
	add := func(svr server.Server, status ds.DiscoveryStatus) {
		defer wg.Done()
		svr.UpdateDiscoveryStatus(status)
		tu.Must(
			ts.Repo.Add(ctx, svr, func(s *server.Server) bool {
				s.UpdateDiscoveryStatus(status)
				return true
			}),
		)
	}

	wg.Add(4)
	go add(svr, ds.Master)
	go add(svr, ds.Info)
	go add(svr, ds.Details)
	go add(svr, ds.Port)
	wg.Wait()

	// Then the repository is expected to contain the server with the merged statuses
	state := collectStorageState(ctx, ts.Redis)
	stored := state.Items["1.1.1.1:10480"]
	assert.Equal(t, 4, stored.Version)
	assert.Equal(t, ds.Master|ds.Info|ds.Details|ds.Port, stored.DiscoveryStatus)
	for _, wantStatus := range []string{"master", "info", "details", "port"} {
		assert.ElementsMatch(t, []string{"1.1.1.1:10480"}, state.Statuses[wantStatus])
	}
}

func TestServersRedisRepo_Update_OK(t *testing.T) {
	ctx := context.TODO()
	ts := setup(t)
	then := ts.Clock.Now()

	// Given a repository with multiple servers in it
	svr1 := serverfactory.Build(
		serverfactory.WithAddress("1.1.1.1", 10480),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
	)
	svr2 := serverfactory.Build(
		serverfactory.WithAddress("2.2.2.2", 10580),
		serverfactory.WithDiscoveryStatus(ds.Details|ds.Info),
		serverfactory.WithRefreshedAt(then),
	)
	svr3 := serverfactory.Build(
		serverfactory.WithAddress("3.3.3.3", 10480),
		serverfactory.WithDiscoveryStatus(ds.NoPort),
		serverfactory.WithRefreshedAt(then),
	)
	for _, svr := range []*server.Server{&svr1, &svr2, &svr3} {
		*svr = tu.Must(ts.Repo.Add(ctx, *svr, repositories.ServerOnConflictIgnore))
	}

	// And some time has passed
	ts.Clock.Advance(time.Second)

	// When updating one of the servers
	svr2.UpdateDiscoveryStatus(ds.Master | ds.NoPort | ds.NoDetails)
	svr2.ClearDiscoveryStatus(ds.Details | ds.Info)
	svr2.Refresh(ts.Clock.Now())
	updated2, err := ts.Repo.Update(ctx, svr2, func(_ *server.Server) bool {
		panic("should not be called")
	})
	// Then the server is expected to be updated
	require.NoError(t, err)
	assert.Equal(t, 2, updated2.Version)
	assert.Equal(t, ds.Master|ds.NoPort|ds.NoDetails, updated2.DiscoveryStatus)

	// And the redis storage is expected to contain the updated server
	// and is expected to have the other server unchanged
	state := collectStorageState(ctx, ts.Redis)
	assert.ElementsMatch(t, []string{"1.1.1.1:10480", "2.2.2.2:10580", "3.3.3.3:10480"}, tu.MapKeys(state.Items))
	assert.EqualExportedValues(t, updated2, state.Items["2.2.2.2:10580"])
	assert.EqualExportedValues(t, svr1, state.Items["1.1.1.1:10480"])
	assert.EqualExportedValues(t, svr3, state.Items["3.3.3.3:10480"])

	assert.ElementsMatch(t, []string{"1.1.1.1:10480", "2.2.2.2:10580", "3.3.3.3:10480"}, state.UpdatesKeys)
	assert.Equal(t, float64(then.UnixNano()), state.Updates[0].Time)
	assert.Equal(t, "1.1.1.1:10480", state.Updates[0].Addr)
	assert.Equal(t, float64(then.UnixNano()), state.Updates[1].Time)
	assert.Equal(t, "3.3.3.3:10480", state.Updates[1].Addr)
	assert.Equal(t, float64(then.Add(time.Second).UnixNano()), state.Updates[2].Time)
	assert.Equal(t, "2.2.2.2:10580", state.Updates[2].Addr)

	assert.ElementsMatch(t, []string{"2.2.2.2:10580", "3.3.3.3:10480"}, state.RefreshesKeys)
	assert.Equal(t, float64(then.UnixNano()), state.Refreshes[0].Time)
	assert.Equal(t, "3.3.3.3:10480", state.Refreshes[0].Addr)
	assert.Equal(t, float64(then.Add(time.Second).UnixNano()), state.Refreshes[1].Time)
	assert.Equal(t, "2.2.2.2:10580", state.Refreshes[1].Addr)

	assert.ElementsMatch(t, []string{"1.1.1.1:10480", "2.2.2.2:10580"}, state.Statuses["master"])
	assert.ElementsMatch(t, []string{"1.1.1.1:10480"}, state.Statuses["info"])
	assert.ElementsMatch(t, []string{"2.2.2.2:10580", "3.3.3.3:10480"}, state.Statuses["no_port"])
	assert.ElementsMatch(t, []string{"2.2.2.2:10580"}, state.Statuses["no_details"])

	// When updating another server a few moments later
	ts.Clock.Advance(time.Second)

	svr3.UpdateDiscoveryStatus(ds.Master | ds.Info | ds.Port)
	svr3.ClearDiscoveryStatus(ds.NoPort)
	svr3.Refresh(time.Time{})
	updated3, err := ts.Repo.Update(ctx, svr3, func(_ *server.Server) bool {
		panic("should not be called")
	})
	// Then the server is expected to be updated
	require.NoError(t, err)
	assert.Equal(t, 2, updated3.Version)
	assert.Equal(t, ds.Master|ds.Info|ds.Port, updated3.DiscoveryStatus)

	// And the redis storage is expected to contain the updated server
	// and is expected to have the other server unchanged
	state = collectStorageState(ctx, ts.Redis)
	assert.ElementsMatch(t, []string{"1.1.1.1:10480", "2.2.2.2:10580", "3.3.3.3:10480"}, tu.MapKeys(state.Items))
	assert.EqualExportedValues(t, updated3, state.Items["3.3.3.3:10480"])
	assert.EqualExportedValues(t, svr1, state.Items["1.1.1.1:10480"])
	assert.EqualExportedValues(t, updated2, state.Items["2.2.2.2:10580"])

	assert.ElementsMatch(t, []string{"1.1.1.1:10480", "2.2.2.2:10580", "3.3.3.3:10480"}, state.UpdatesKeys)
	assert.Equal(t, float64(then.UnixNano()), state.Updates[0].Time)
	assert.Equal(t, "1.1.1.1:10480", state.Updates[0].Addr)
	assert.Equal(t, float64(then.Add(time.Second).UnixNano()), state.Updates[1].Time)
	assert.Equal(t, "2.2.2.2:10580", state.Updates[1].Addr)
	assert.Equal(t, float64(then.Add(time.Second*2).UnixNano()), state.Updates[2].Time)
	assert.Equal(t, "3.3.3.3:10480", state.Updates[2].Addr)

	assert.ElementsMatch(t, []string{"2.2.2.2:10580"}, state.RefreshesKeys)
	assert.Equal(t, float64(then.Add(time.Second).UnixNano()), state.Refreshes[0].Time)
	assert.Equal(t, "2.2.2.2:10580", state.Refreshes[0].Addr)

	assert.ElementsMatch(t, []string{"1.1.1.1:10480", "2.2.2.2:10580", "3.3.3.3:10480"}, state.Statuses["master"])
	assert.ElementsMatch(t, []string{"1.1.1.1:10480", "3.3.3.3:10480"}, state.Statuses["info"])
	assert.ElementsMatch(t, []string{"3.3.3.3:10480"}, state.Statuses["port"])
	assert.ElementsMatch(t, []string{"2.2.2.2:10580"}, state.Statuses["no_port"])
	assert.ElementsMatch(t, []string{"2.2.2.2:10580"}, state.Statuses["no_details"])
}

func TestServersRedisRepo_Update_OnConflictIgnore(t *testing.T) {
	ctx := context.TODO()
	ts := setup(t)
	then := ts.Clock.Now().UTC()

	// Given a repository with a server in it
	svr := serverfactory.Build(
		serverfactory.WithAddress("1.1.1.1", 10480),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
	)
	tu.Must(ts.Repo.Add(ctx, svr, repositories.ServerOnConflictIgnore))

	// When the server is being updated concurrently in another goroutine a few moments later
	ts.Clock.Advance(time.Second)

	ready := make(chan struct{})
	go func(a addr.Addr, now time.Time, ready chan struct{}) {
		conflicted, err := ts.Repo.Get(ctx, a)
		require.NoError(t, err)
		assert.Equal(t, 1, conflicted.Version)

		conflicted.UpdateDiscoveryStatus(ds.Port)
		conflicted.Refresh(now)
		tu.Must(ts.Repo.Update(ctx, conflicted, func(_ *server.Server) bool {
			panic("should not be called")
		}))
		close(ready)
	}(svr.Addr, ts.Clock.Now().UTC(), ready)

	<-ready

	// And the server is also being updated in the main goroutine a few moments later
	ts.Clock.Advance(time.Second)
	svr.Refresh(ts.Clock.Now())
	svr.UpdateDiscoveryStatus(ds.Details)
	notUpdated, err := ts.Repo.Update(ctx, svr, func(_ *server.Server) bool {
		return false
	})
	// Then no error is expected and the update is ignored
	require.NoError(t, err)
	assert.Equal(t, 2, notUpdated.Version)
	assert.Equal(t, micro(then.Add(time.Second)), micro(notUpdated.RefreshedAt))
	assert.Equal(t, ds.Master|ds.Info|ds.Port, notUpdated.DiscoveryStatus)

	// And the redis storage is expected to contain the server in the state from the concurrent update
	state := collectStorageState(ctx, ts.Redis)
	assert.ElementsMatch(t, []string{"1.1.1.1:10480"}, tu.MapKeys(state.Items))
	assert.EqualExportedValues(t, notUpdated, state.Items["1.1.1.1:10480"])

	assert.ElementsMatch(t, []string{"1.1.1.1:10480"}, state.UpdatesKeys)
	assert.ElementsMatch(t, []string{"1.1.1.1:10480"}, state.RefreshesKeys)
	assert.Equal(t, float64(then.Add(time.Second).UnixNano()), state.Updates[0].Time)
	assert.Equal(t, float64(then.Add(time.Second).UnixNano()), state.Refreshes[0].Time)
}

func TestServersRedisRepo_Update_OnConflictUpdate(t *testing.T) {
	ctx := context.TODO()
	ts := setup(t)
	then := ts.Clock.Now().UTC()

	// Given a repository with a server in it
	svr := serverfactory.Build(
		serverfactory.WithAddress("1.1.1.1", 10480),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
	)
	tu.Must(ts.Repo.Add(ctx, svr, repositories.ServerOnConflictIgnore))

	// When the server is being updated concurrently in another goroutine a few moments later
	ts.Clock.Advance(time.Second)

	ready := make(chan struct{})
	go func(a addr.Addr, now time.Time, ready chan struct{}) {
		conflicted, err := ts.Repo.Get(ctx, a)
		require.NoError(t, err)
		assert.Equal(t, 1, conflicted.Version)

		conflicted.UpdateDiscoveryStatus(ds.Port)
		conflicted.Refresh(now)
		tu.Must(ts.Repo.Update(ctx, conflicted, func(_ *server.Server) bool {
			panic("should not be called")
		}))
		close(ready)
	}(svr.Addr, ts.Clock.Now().UTC(), ready)

	<-ready

	// And the server is also being updated in the main goroutine a few moments later
	ts.Clock.Advance(time.Second)
	svr.Refresh(ts.Clock.Now())
	svr.UpdateDiscoveryStatus(ds.Details)
	updated, err := ts.Repo.Update(ctx, svr, func(s *server.Server) bool {
		s.UpdateDiscoveryStatus(ds.Details)
		return true
	})
	// Then no error is expected and the update is ignored
	require.NoError(t, err)
	assert.Equal(t, 3, updated.Version)
	assert.Equal(t, ds.Master|ds.Info|ds.Port|ds.Details, updated.DiscoveryStatus)
	assert.Equal(t, micro(then.Add(time.Second)), micro(updated.RefreshedAt))

	// And the redis storage is expected to reflect both updates
	state := collectStorageState(ctx, ts.Redis)
	assert.ElementsMatch(t, []string{"1.1.1.1:10480"}, tu.MapKeys(state.Items))
	assert.EqualExportedValues(t, updated, state.Items["1.1.1.1:10480"])

	assert.ElementsMatch(t, []string{"1.1.1.1:10480"}, state.UpdatesKeys)
	assert.ElementsMatch(t, []string{"1.1.1.1:10480"}, state.RefreshesKeys)
	assert.Equal(t, float64(then.Add(2*time.Second).UnixNano()), state.Updates[0].Time)
	// the refresh time was not updated during the conflict resolution
	assert.Equal(t, float64(then.Add(time.Second).UnixNano()), state.Refreshes[0].Time)
}

func TestServersRedisRepo_Update_OnConflictUpdateConcurrently(t *testing.T) {
	ctx := context.TODO()
	ts := setup(t)
	then := ts.Clock.Now().UTC()

	// Given a repository with a server in it
	svr := serverfactory.Build(
		serverfactory.WithAddress("1.1.1.1", 10480),
		serverfactory.WithDiscoveryStatus(ds.DetailsRetry),
		serverfactory.WithRefreshedAt(then),
	)
	tu.Must(ts.Repo.Add(ctx, svr, repositories.ServerOnConflictIgnore))

	ts.Clock.Advance(time.Second)

	// When updating the server concurrently in multiple goroutines
	wg := &sync.WaitGroup{}
	update := func(svr server.Server, status ds.DiscoveryStatus, refreshedAt time.Time) {
		defer wg.Done()
		svr.UpdateDiscoveryStatus(status)
		svr.Refresh(refreshedAt)
		tu.Must(
			ts.Repo.Update(ctx, svr, func(s *server.Server) bool {
				s.UpdateDiscoveryStatus(status)
				if refreshedAt.After(s.RefreshedAt) {
					s.Refresh(refreshedAt)
				}
				return true
			}),
		)
	}

	wg.Add(4)
	go update(svr, ds.Master, then.Add(3*time.Second))
	go update(svr, ds.Info, then.Add(time.Second))
	go update(svr, ds.Details, then.Add(2*time.Second))
	go update(svr, ds.Port, then.Add(2*time.Second))
	wg.Wait()

	// Then the repository is expected to contain the server in a state with successfully resolved conflicts
	state := collectStorageState(ctx, ts.Redis)
	stored := state.Items["1.1.1.1:10480"]
	assert.Equal(t, 5, stored.Version)
	assert.Equal(t, ds.Master|ds.Info|ds.Details|ds.DetailsRetry|ds.Port, stored.DiscoveryStatus)
	assert.Equal(t, micro(then.Add(3*time.Second)), micro(stored.RefreshedAt))
	for _, wantStatus := range []string{"master", "info", "details", "details_retry", "port"} {
		assert.ElementsMatch(t, []string{"1.1.1.1:10480"}, state.Statuses[wantStatus])
	}
}

func TestServersRedisRepo_Update_DoesNotExist(t *testing.T) {
	ctx := context.TODO()
	ts := setup(t)

	// Given a repository with no servers in it

	// And a server to be updated
	svr := serverfactory.Build(
		serverfactory.WithAddress("1.1.1.1", 10480),
	)

	// When updating the server
	_, err := ts.Repo.Update(ctx, svr, func(_ *server.Server) bool {
		panic("should not be called")
	})
	// Then an error is expected
	require.ErrorIs(t, err, repositories.ErrServerNotFound)

	// And the repository is expected to remain empty
	state := collectStorageState(ctx, ts.Redis)
	assert.Empty(t, state.Items)
	assert.Empty(t, state.UpdatesKeys)
	assert.Empty(t, state.RefreshesKeys)
	assert.Empty(t, state.Statuses)
}

func TestServersRedisRepo_Remove_OK(t *testing.T) {
	ctx := context.TODO()
	ts := setup(t)

	// Given a repository with multiple servers in it
	svr1 := serverfactory.Build(
		serverfactory.WithAddress("1.1.1.1", 10480),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
	)
	svr2 := serverfactory.Build(
		serverfactory.WithAddress("2.2.2.2", 10580),
		serverfactory.WithDiscoveryStatus(ds.Details|ds.Info),
	)
	svr3 := serverfactory.Build(
		serverfactory.WithAddress("3.3.3.3", 10480),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Details|ds.Info),
		serverfactory.WithRefreshedAt(ts.Clock.Now()),
	)

	for _, svr := range []*server.Server{&svr1, &svr2, &svr3} {
		*svr = tu.Must(ts.Repo.Add(ctx, *svr, repositories.ServerOnConflictIgnore))
	}

	// When removing one of the servers
	err := ts.Repo.Remove(ctx, svr2, repositories.ServerOnConflictIgnore)
	require.NoError(t, err)

	// Then the repository is expected to have the server removed and contain the remaining servers
	state := collectStorageState(ctx, ts.Redis)
	assert.ElementsMatch(t, []string{"1.1.1.1:10480", "3.3.3.3:10480"}, tu.MapKeys(state.Items))
	assert.ElementsMatch(t, []string{"1.1.1.1:10480", "3.3.3.3:10480"}, state.UpdatesKeys)
	assert.ElementsMatch(t, []string{"3.3.3.3:10480"}, state.RefreshesKeys)
	assert.ElementsMatch(t, []string{"1.1.1.1:10480", "3.3.3.3:10480"}, state.Statuses["master"])
	assert.ElementsMatch(t, []string{"1.1.1.1:10480", "3.3.3.3:10480"}, state.Statuses["info"])
	assert.ElementsMatch(t, []string{"3.3.3.3:10480"}, state.Statuses["details"])

	// When removing the remaining servers
	for _, svr := range []server.Server{svr1, svr3} {
		err = ts.Repo.Remove(ctx, svr, repositories.ServerOnConflictIgnore)
		require.NoError(t, err)
	}

	// Then the repository is expected to have no servers in it
	state = collectStorageState(ctx, ts.Redis)
	assert.Empty(t, state.Items)
	assert.Empty(t, state.UpdatesKeys)
	assert.Empty(t, state.RefreshesKeys)
	assert.Empty(t, state.Statuses)
}

func TestServersRedisRepo_Remove_OnConflictResolve(t *testing.T) {
	ctx := context.TODO()
	ts := setup(t)

	// Given a repository with multiple servers in it
	svr1 := serverfactory.Build(
		serverfactory.WithAddress("1.1.1.1", 10480),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
	)
	svr2 := serverfactory.Build(
		serverfactory.WithAddress("2.2.2.2", 10580),
		serverfactory.WithDiscoveryStatus(ds.Details|ds.Info),
	)
	svr3 := serverfactory.Build(
		serverfactory.WithAddress("3.3.3.3", 10480),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Details|ds.Info),
	)

	for _, svr := range []server.Server{svr1, svr2, svr3} {
		tu.Must(ts.Repo.Add(ctx, svr, repositories.ServerOnConflictIgnore))
	}

	// When removing one of the servers that is being updated concurrently...
	ready := make(chan struct{})

	go func(a addr.Addr, ready chan struct{}) {
		conflicted, err := ts.Repo.Get(ctx, a)
		require.NoError(t, err)
		assert.Equal(t, 1, conflicted.Version)
		conflicted.UpdateDiscoveryStatus(ds.Port | ds.DetailsRetry)
		tu.Must(ts.Repo.Update(ctx, conflicted, func(_ *server.Server) bool {
			panic("should not be called")
		}))
		close(ready)
	}(svr2.Addr, ready)

	<-ready
	// ...and the conflict resolution strategy is set to resolve
	err := ts.Repo.Remove(ctx, svr2, func(_ *server.Server) bool {
		return true
	})

	// Then no error is expected
	require.NoError(t, err)

	// And the repository is expected to have the server removed
	// accounting for the changes made by the concurrent update
	_, getErr := ts.Repo.Get(ctx, svr2.Addr)
	require.ErrorIs(t, getErr, repositories.ErrServerNotFound)

	// And the repository is expected to have the server to remain in the storage
	state := collectStorageState(ctx, ts.Redis)
	assert.ElementsMatch(
		t,
		[]string{"1.1.1.1:10480", "3.3.3.3:10480"},
		tu.MapKeys(state.Items),
	)
	assert.ElementsMatch(t, []string{"1.1.1.1:10480", "3.3.3.3:10480"}, tu.MapKeys(state.Items))
	assert.ElementsMatch(t, []string{"1.1.1.1:10480", "3.3.3.3:10480"}, state.UpdatesKeys)
	assert.Empty(t, state.RefreshesKeys)
	assert.ElementsMatch(t, []string{"1.1.1.1:10480", "3.3.3.3:10480"}, state.Statuses["master"])
	assert.ElementsMatch(t, []string{"1.1.1.1:10480", "3.3.3.3:10480"}, state.Statuses["info"])
	assert.ElementsMatch(t, []string{"3.3.3.3:10480"}, state.Statuses["details"])
	assert.Empty(t, state.Statuses["port"])
	assert.Empty(t, state.Statuses["details_retry"])
}

func TestServersRedisRepo_Remove_OnConflictIgnore(t *testing.T) {
	ctx := context.TODO()
	ts := setup(t)

	// Given a repository with multiple servers in it
	svr1 := serverfactory.Build(
		serverfactory.WithAddress("1.1.1.1", 10480),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
	)
	svr2 := serverfactory.Build(
		serverfactory.WithAddress("2.2.2.2", 10580),
		serverfactory.WithDiscoveryStatus(ds.Details|ds.Info),
	)
	svr3 := serverfactory.Build(
		serverfactory.WithAddress("3.3.3.3", 10480),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Details|ds.Info),
	)

	for _, svr := range []server.Server{svr1, svr2, svr3} {
		tu.Must(ts.Repo.Add(ctx, svr, repositories.ServerOnConflictIgnore))
	}

	// When removing one of the servers that is being updated concurrently...
	ready := make(chan struct{})

	go func(a addr.Addr, ready chan struct{}) {
		conflicted, err := ts.Repo.Get(ctx, a)
		require.NoError(t, err)
		assert.Equal(t, 1, conflicted.Version)
		tu.Must(ts.Repo.Update(ctx, conflicted, func(_ *server.Server) bool {
			panic("should not be called")
		}))
		close(ready)
	}(svr2.Addr, ready)

	<-ready
	// ...and the conflict resolution strategy is set to ignore
	err := ts.Repo.Remove(ctx, svr2, func(_ *server.Server) bool {
		return false
	})

	// Then no error is expected
	require.NoError(t, err)

	// And the repository is expected to have the server to remain
	// and its version to be incremented by the concurrent update
	got, getErr := ts.Repo.Get(ctx, svr2.Addr)
	require.NoError(t, getErr)
	assert.Equal(t, 2, got.Version)

	// And the repository is expected to have the server to remain in the storage
	state := collectStorageState(ctx, ts.Redis)
	assert.ElementsMatch(
		t,
		[]string{"1.1.1.1:10480", "2.2.2.2:10580", "3.3.3.3:10480"},
		tu.MapKeys(state.Items),
	)
}

func TestServersRedisRepo_Remove_OnNonExistentNoError(t *testing.T) {
	ctx := context.TODO()
	ts := setup(t)

	// Given a repository with multiple servers in it
	svr1 := serverfactory.Build(serverfactory.WithAddress("1.1.1.1", 10480))
	svr2 := serverfactory.Build(serverfactory.WithAddress("2.2.2.2", 10580))
	svr3 := serverfactory.Build(serverfactory.WithAddress("3.3.3.3", 10480))

	for _, svr := range []*server.Server{&svr1, &svr2, &svr3} {
		*svr = tu.Must(ts.Repo.Add(ctx, *svr, repositories.ServerOnConflictIgnore))
	}

	// When attempting to remove a server that does not exist
	svr := serverfactory.Build(serverfactory.WithAddress("4.4.4.4", 10480))
	err := ts.Repo.Remove(ctx, svr, repositories.ServerOnConflictIgnore)

	// Then no error is expected
	require.NoError(t, err)

	// And the repository is expected to remain unchanged
	state := collectStorageState(ctx, ts.Redis)
	assert.ElementsMatch(
		t,
		[]string{"1.1.1.1:10480", "2.2.2.2:10580", "3.3.3.3:10480"},
		tu.MapKeys(state.Items),
	)
}

func TestServersRedisRepo_Remove_OnEmptyStorageNoError(t *testing.T) {
	ctx := context.TODO()
	ts := setup(t)

	// Given an empty repository

	// When attempting to remove a server that does not exist
	svr := serverfactory.Build(serverfactory.WithAddress("1.1.1.1", 10480))

	// Then no error is expected
	err := ts.Repo.Remove(ctx, svr, repositories.ServerOnConflictIgnore)
	require.NoError(t, err)
}

func TestServersRedisRepo_Filter_OK(t *testing.T) {
	type Times struct {
		Before        time.Time
		Svr1UpdatedAt time.Time
		Svr2UpdatedAt time.Time
		Svr3UpdatedAt time.Time
		Svr4UpdatedAt time.Time
		Svr5UpdatedAt time.Time
		Now           time.Time
	}

	tests := []struct {
		name          string
		filterFactory func(times Times) filterset.ServerFilterSet
		wantServers   []string
	}{
		{
			"use no filters",
			func(_ Times) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet()
			},
			[]string{
				"5.5.5.5:10480",
				"4.4.4.4:10480",
				"3.3.3.3:10480",
				"2.2.2.2:10480",
				"1.1.1.1:10480",
			},
		},
		{
			"exclude status",
			func(_ Times) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet().NoStatus(ds.Master)
			},
			[]string{
				"5.5.5.5:10480",
				"4.4.4.4:10480",
				"2.2.2.2:10480",
			},
		},
		{
			"exclude multiple statuses",
			func(_ Times) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet().NoStatus(ds.PortRetry | ds.NoDetails)
			},
			[]string{
				"3.3.3.3:10480",
				"2.2.2.2:10480",
				"1.1.1.1:10480",
			},
		},
		{
			"filter by one status",
			func(_ Times) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet().WithStatus(ds.Master)
			},
			[]string{
				"3.3.3.3:10480",
				"1.1.1.1:10480",
			},
		},
		{
			"filter by multiple statuses",
			func(_ Times) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet().WithStatus(ds.Master | ds.Details)
			},
			[]string{
				"3.3.3.3:10480",
			},
		},
		{
			"filter by non-matching statuses",
			func(_ Times) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet().WithStatus(ds.Master | ds.Details | ds.Port)
			},
			[]string{},
		},
		{
			"filter by after update date",
			func(times Times) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet().UpdatedAfter(times.Svr4UpdatedAt)
			},
			[]string{
				"5.5.5.5:10480",
				"4.4.4.4:10480",
			},
		},
		{
			"filter by before update date",
			func(times Times) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet().UpdatedBefore(times.Svr4UpdatedAt)
			},
			[]string{
				"3.3.3.3:10480",
				"2.2.2.2:10480",
				"1.1.1.1:10480",
			},
		},
		{
			"filter by narrow after and before update date range",
			func(times Times) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet().
					UpdatedAfter(times.Svr4UpdatedAt).
					UpdatedBefore(times.Svr5UpdatedAt)
			},
			[]string{
				"4.4.4.4:10480",
			},
		},
		{
			"filter by wide after and before update date range",
			func(times Times) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet().
					UpdatedAfter(times.Before).
					UpdatedBefore(times.Now)
			},
			[]string{
				"5.5.5.5:10480",
				"4.4.4.4:10480",
				"3.3.3.3:10480",
				"2.2.2.2:10480",
				"1.1.1.1:10480",
			},
		},
		{
			"filter by impossible after and before update date range",
			func(times Times) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet().
					UpdatedAfter(times.Now).
					UpdatedBefore(times.Before)
			},
			[]string{},
		},
		{
			"filter by non-overlapping after and before update date range",
			func(times Times) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet().
					UpdatedAfter(times.Svr3UpdatedAt).
					UpdatedBefore(times.Svr3UpdatedAt)
			},
			[]string{},
		},
		{
			"filter by non-matching after update date in future",
			func(times Times) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet().UpdatedAfter(times.Now)
			},
			[]string{},
		},
		{
			"filter by non-matching before update date in past",
			func(times Times) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet().UpdatedBefore(times.Before)
			},
			[]string{},
		},
		{
			"filter by after refresh date",
			func(times Times) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet().ActiveAfter(times.Svr2UpdatedAt)
			},
			[]string{
				"3.3.3.3:10480",
				"2.2.2.2:10480",
			},
		},
		{
			"filter by before refresh date",
			func(times Times) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet().ActiveBefore(times.Svr3UpdatedAt)
			},
			[]string{
				"2.2.2.2:10480",
				"1.1.1.1:10480",
			},
		},
		{
			"filter by after and before refresh date",
			func(times Times) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet().
					ActiveAfter(times.Svr2UpdatedAt).
					ActiveBefore(times.Svr3UpdatedAt)
			},
			[]string{
				"2.2.2.2:10480",
			},
		},
		{
			"filter by non-matching after refresh date in future",
			func(times Times) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet().ActiveAfter(times.Now)
			},
			[]string{},
		},
		{
			"filter by non-matching before refresh date in past",
			func(times Times) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet().ActiveBefore(times.Before)
			},
			[]string{},
		},
		{
			"filter by multiple fields",
			func(times Times) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet().
					UpdatedBefore(times.Svr5UpdatedAt).
					ActiveAfter(times.Svr2UpdatedAt).
					WithStatus(ds.Master)
			},
			[]string{
				"3.3.3.3:10480",
			},
		},
		{
			"filter by multiple non-matching fields",
			func(times Times) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet().
					UpdatedBefore(times.Svr5UpdatedAt).
					ActiveAfter(times.Svr2UpdatedAt).
					NoStatus(ds.Master | ds.Info)
			},
			[]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			ts := setup(t)

			before := ts.Clock.Now()

			// Given multiple servers in the repository inserted at different times
			ts.Clock.Advance(time.Millisecond)
			t1 := ts.Clock.Now()
			svr1 := serverfactory.Build(
				serverfactory.WithAddress("1.1.1.1", 10480),
				serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
				serverfactory.WithRefreshedAt(t1),
			)
			tu.Must(ts.Repo.Add(ctx, svr1, repositories.ServerOnConflictIgnore))

			ts.Clock.Advance(time.Millisecond)
			t2 := ts.Clock.Now()
			svr2 := serverfactory.Build(
				serverfactory.WithAddress("2.2.2.2", 10480),
				serverfactory.WithDiscoveryStatus(ds.Details|ds.Info),
				serverfactory.WithRefreshedAt(t2),
			)
			tu.Must(ts.Repo.Add(ctx, svr2, repositories.ServerOnConflictIgnore))

			ts.Clock.Advance(time.Millisecond)
			t3 := ts.Clock.Now()
			svr3 := serverfactory.Build(
				serverfactory.WithAddress("3.3.3.3", 10480),
				serverfactory.WithDiscoveryStatus(ds.Master|ds.Details|ds.Info),
				serverfactory.WithRefreshedAt(t3),
			)
			tu.Must(ts.Repo.Add(ctx, svr3, repositories.ServerOnConflictIgnore))

			ts.Clock.Advance(time.Millisecond)
			t4 := ts.Clock.Now()
			svr4 := serverfactory.Build(
				serverfactory.WithAddress("4.4.4.4", 10480),
				serverfactory.WithDiscoveryStatus(ds.NoDetails|ds.NoPort),
			)
			tu.Must(ts.Repo.Add(ctx, svr4, repositories.ServerOnConflictIgnore))

			ts.Clock.Advance(time.Millisecond)
			t5 := ts.Clock.Now()
			svr5 := serverfactory.Build(
				serverfactory.WithAddress("5.5.5.5", 10480),
				serverfactory.WithDiscoveryStatus(ds.NoPort|ds.PortRetry),
			)
			tu.Must(ts.Repo.Add(ctx, svr5, repositories.ServerOnConflictIgnore))

			ts.Clock.Advance(time.Millisecond)
			after := ts.Clock.Now()

			// When filtering the servers based on different criteria
			filter := tt.filterFactory(Times{
				Before:        before,
				Svr1UpdatedAt: t1,
				Svr2UpdatedAt: t2,
				Svr3UpdatedAt: t3,
				Svr4UpdatedAt: t4,
				Svr5UpdatedAt: t5,
				Now:           after,
			})
			// Then the servers are expected to be filtered as expected
			result, err := ts.Repo.Filter(ctx, filter)
			require.NoError(t, err)

			gotServers := make([]string, len(result))
			for i, svr := range result {
				gotServers[i] = svr.Addr.String()
			}
			assert.ElementsMatch(t, tt.wantServers, gotServers)
		})
	}
}

func TestServersRedisRepo_Filter_NoRefreshedServers(t *testing.T) {
	tests := []struct {
		name          string
		filterFactory func(time.Time, time.Time) filterset.ServerFilterSet
	}{
		{
			"filter by after past refresh date",
			func(past, _ time.Time) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet().ActiveAfter(past)
			},
		},
		{
			"filter by before past refresh date",
			func(past, _ time.Time) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet().ActiveBefore(past)
			},
		},
		{
			"filter by after future refresh date",
			func(_, future time.Time) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet().ActiveAfter(future)
			},
		},
		{
			"filter by before future refresh date",
			func(_, future time.Time) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet().ActiveBefore(future)
			},
		},
		{
			"filter by after and before past refresh date",
			func(past, future time.Time) filterset.ServerFilterSet {
				return filterset.NewServerFilterSet().
					ActiveAfter(past).
					ActiveBefore(future)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			ts := setup(t)

			before := ts.Clock.Now()

			// Given multiple servers in the repository inserted at different times
			// without refresh dates set
			ts.Clock.Advance(time.Millisecond)
			svr1 := serverfactory.Build(
				serverfactory.WithAddress("1.1.1.1", 10480),
				serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
			)
			tu.Must(ts.Repo.Add(ctx, svr1, repositories.ServerOnConflictIgnore))

			ts.Clock.Advance(time.Millisecond)
			svr2 := serverfactory.Build(
				serverfactory.WithAddress("2.2.2.2", 10480),
				serverfactory.WithDiscoveryStatus(ds.Details|ds.Info),
			)
			tu.Must(ts.Repo.Add(ctx, svr2, repositories.ServerOnConflictIgnore))

			ts.Clock.Advance(time.Millisecond)
			svr3 := serverfactory.Build(
				serverfactory.WithAddress("3.3.3.3", 10480),
				serverfactory.WithDiscoveryStatus(ds.Master|ds.Details|ds.Info),
			)
			tu.Must(ts.Repo.Add(ctx, svr3, repositories.ServerOnConflictIgnore))

			ts.Clock.Advance(time.Millisecond)
			after := ts.Clock.Now()

			// When filtering the servers based on refresh date
			filter := tt.filterFactory(before, after)
			// Then no servers are expected to be returned
			result, err := ts.Repo.Filter(ctx, filter)
			require.NoError(t, err)
			assert.Equal(t, 0, len(result))
		})
	}
}

func TestServersRedisRepo_Filter_OnEmptyNoError(t *testing.T) {
	ctx := context.TODO()

	// Given an empty repository
	ts := setup(t)

	// When filtering the servers
	result, err := ts.Repo.Filter(ctx, filterset.NewServerFilterSet())

	// Then no error is expected
	require.NoError(t, err)
	// And the result is expected to be empty
	assert.Empty(t, result)
}

func TestServersRedisRepo_Count_OK(t *testing.T) {
	ctx := context.TODO()
	ts := setup(t)

	// Given a repository with multiple servers in it
	svr1 := serverfactory.Build(serverfactory.WithAddress("1.1.1.1", 10480))
	svr2 := serverfactory.Build(serverfactory.WithAddress("2.2.2.2", 10580))
	svr3 := serverfactory.Build(serverfactory.WithAddress("3.3.3.3", 10480))

	for _, svr := range []*server.Server{&svr1, &svr2, &svr3} {
		*svr = tu.Must(ts.Repo.Add(ctx, *svr, repositories.ServerOnConflictIgnore))
	}

	// When counting the objects in the repository
	count, err := ts.Repo.Count(ctx)
	// Then the count is expected to be 3
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	// When removing one of the servers
	tu.MustNoErr(ts.Repo.Remove(ctx, svr2, repositories.ServerOnConflictIgnore))

	// Then the remaining count is expected to be 2
	count, err = ts.Repo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// When removing the remaining servers
	for _, svr := range []server.Server{svr1, svr3} {
		err = ts.Repo.Remove(ctx, svr, repositories.ServerOnConflictIgnore)
		require.NoError(t, err)
	}
	// Then the count is expected to be 0
	count, err = ts.Repo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestServersRedisRepo_Count_OnEmptyStorageZero(t *testing.T) {
	ctx := context.TODO()
	ts := setup(t)

	// Given a repository with empty underlying storage

	// When counting the objects in the repository
	count, err := ts.Repo.Count(ctx)
	require.NoError(t, err)

	// Then the count is expected to be 0
	assert.Equal(t, 0, count)
}

func TestServersRedisRepo_CountByStatus_OK(t *testing.T) {
	ctx := context.TODO()
	ts := setup(t)

	// Given a repository with multiple servers in it
	svr1 := serverfactory.Build(
		serverfactory.WithAddress("1.1.1.1", 10480),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
	)
	svr2 := serverfactory.Build(
		serverfactory.WithAddress("2.2.2.2", 10580),
		serverfactory.WithDiscoveryStatus(ds.Details|ds.Info),
	)
	svr3 := serverfactory.Build(
		serverfactory.WithAddress("3.3.3.3", 10480),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Details|ds.Info),
	)
	svr4 := serverfactory.Build(
		serverfactory.WithAddress("4.4.4.4", 11480),
		serverfactory.WithDiscoveryStatus(ds.NoDetails|ds.NoPort),
	)
	svr5 := serverfactory.Build(
		serverfactory.WithAddress("5.5.5.5", 10880),
		serverfactory.WithDiscoveryStatus(ds.NoPort),
	)
	for _, svr := range []*server.Server{&svr1, &svr2, &svr3, &svr4, &svr5} {
		*svr = tu.Must(ts.Repo.Add(ctx, *svr, repositories.ServerOnConflictIgnore))
	}

	// When requesting the counts by status
	countByStatus, err := ts.Repo.CountByStatus(ctx)
	require.NoError(t, err)

	// Then the counts are expected to be in accordance with the servers' statuses
	expected := map[ds.DiscoveryStatus]int{
		ds.New:          0,
		ds.Master:       2,
		ds.Info:         3,
		ds.Details:      2,
		ds.DetailsRetry: 0,
		ds.NoDetails:    1,
		ds.Port:         0,
		ds.PortRetry:    0,
		ds.NoPort:       2,
	}
	assert.Equal(t, expected, countByStatus)

	// When removing one of the servers
	tu.MustNoErr(ts.Repo.Remove(ctx, svr2, repositories.ServerOnConflictIgnore))

	// Then the counts are expected to be updated
	countByStatus, err = ts.Repo.CountByStatus(ctx)
	require.NoError(t, err)
	expected = map[ds.DiscoveryStatus]int{
		ds.New:          0,
		ds.Master:       2,
		ds.Info:         2,
		ds.Details:      1,
		ds.DetailsRetry: 0,
		ds.NoDetails:    1,
		ds.Port:         0,
		ds.PortRetry:    0,
		ds.NoPort:       2,
	}
	assert.Equal(t, expected, countByStatus)

	// When some of the servers are updated
	svr3.ClearDiscoveryStatus(ds.Master | ds.Details | ds.Info)
	svr3.UpdateDiscoveryStatus(ds.NoDetails)

	svr5.ClearDiscoveryStatus(ds.NoPort)
	svr5.UpdateDiscoveryStatus(ds.Port)

	for _, svr := range []*server.Server{&svr3, &svr5} {
		*svr = tu.Must(ts.Repo.Update(ctx, *svr, repositories.ServerOnConflictIgnore))
	}

	// Then the counts are expected to be updated
	countByStatus, err = ts.Repo.CountByStatus(ctx)
	require.NoError(t, err)
	expected = map[ds.DiscoveryStatus]int{
		ds.New:          0,
		ds.Master:       1,
		ds.Info:         1,
		ds.Details:      0,
		ds.DetailsRetry: 0,
		ds.NoDetails:    2,
		ds.Port:         1,
		ds.PortRetry:    0,
		ds.NoPort:       1,
	}
	assert.Equal(t, expected, countByStatus)

	// When the remaining servers are removed
	for _, svr := range []server.Server{svr1, svr3, svr4, svr5} {
		tu.MustNoErr(ts.Repo.Remove(ctx, svr, repositories.ServerOnConflictIgnore))
	}

	// Then the counts are expected to be equal to zero
	countByStatus, err = ts.Repo.CountByStatus(ctx)
	require.NoError(t, err)
	expected = map[ds.DiscoveryStatus]int{
		ds.New:          0,
		ds.Master:       0,
		ds.Info:         0,
		ds.Details:      0,
		ds.DetailsRetry: 0,
		ds.NoDetails:    0,
		ds.Port:         0,
		ds.PortRetry:    0,
		ds.NoPort:       0,
	}
	assert.Equal(t, expected, countByStatus)
}

func TestServersRedisRepo_CountByStatus_OnEmptyStorageEmptyCounts(t *testing.T) {
	ctx := context.TODO()
	ts := setup(t)

	// Given a repository with empty underlying storage

	// When counting the objects in the repository
	countByStatus, err := ts.Repo.CountByStatus(ctx)
	require.NoError(t, err)

	// Then a map with empty counts is expected
	expected := map[ds.DiscoveryStatus]int{
		ds.New:          0,
		ds.Master:       0,
		ds.Info:         0,
		ds.Details:      0,
		ds.DetailsRetry: 0,
		ds.NoDetails:    0,
		ds.Port:         0,
		ds.PortRetry:    0,
		ds.NoPort:       0,
	}
	assert.Equal(t, expected, countByStatus)
}
