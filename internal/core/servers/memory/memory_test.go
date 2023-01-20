package memory_test

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/internal/entity/details"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
	"github.com/sergeii/swat4master/internal/validation"

	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/core/servers/memory"
	"github.com/sergeii/swat4master/internal/entity/addr"
)

func TestMain(m *testing.M) {
	if err := validation.Register(); err != nil {
		panic(err)
	}
	m.Run()
}

func TestServerMemoryRepo_AddOrUpdate_NewInstance(t *testing.T) {
	ctx := context.TODO()
	repo := memory.New()

	svr, _ := servers.New(net.ParseIP("1.1.1.1"), 10480, 10481)
	svr.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "Barricaded Suspects",
	}))
	_, err := repo.AddOrUpdate(ctx, svr)
	require.NoError(t, err)

	other, _ := servers.New(net.ParseIP("2.2.2.2"), 10480, 10481)
	other.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Another Swat4 Server",
		"hostport":    "10480",
		"mapname":     "The Wolcott Projects",
		"gamever":     "1.0",
		"gamevariant": "SWAT 4X",
		"gametype":    "VIP Escort",
	}))
	_, otherErr := repo.AddOrUpdate(ctx, other)

	require.NoError(t, otherErr)

	addedSvr, err := repo.Get(ctx, addr.MustNewFromString("1.1.1.1", 10480))
	require.NoError(t, err)
	svrInfo := addedSvr.GetInfo()
	assert.Equal(t, "Swat4 Server", svrInfo.Hostname)

	addedOther, err := repo.Get(ctx, addr.MustNewFromString("2.2.2.2", 10480))
	require.NoError(t, err)
	otherInfo := addedOther.GetInfo()
	assert.Equal(t, "Another Swat4 Server", otherInfo.Hostname)
}

func TestServerMemoryRepo_AddOrUpdate_UpdateInstance(t *testing.T) {
	repo := memory.New()
	ctx := context.TODO()

	svr, _ := servers.New(net.ParseIP("1.1.1.1"), 10480, 10481)
	before := time.Now()
	svr.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "VIP Escort",
		"numplayers":  "16",
	}))
	_, err := repo.AddOrUpdate(ctx, svr)
	require.NoError(t, err)

	addedSvr, err := repo.Get(ctx, addr.MustNewFromString("1.1.1.1", 10480))
	require.NoError(t, err)
	addedInfo := addedSvr.GetInfo()
	assert.Equal(t, "A-Bomb Nightclub", addedInfo.MapName)
	assert.Equal(t, 16, addedInfo.NumPlayers)
	updatedSinceBefore, _ := repo.Filter(ctx, servers.NewFilterSet().After(before))
	assert.Len(t, updatedSinceBefore, 1)

	time.Sleep(time.Millisecond)
	after := time.Now()
	// instance is now unlisted
	reportedSinceAfter, _ := repo.Filter(ctx, servers.NewFilterSet().After(after))
	assert.Len(t, reportedSinceAfter, 0)

	svr.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Swat4 Server",
		"hostport":    "10480",
		"mapname":     "Food Wall Restaurant",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "VIP Escort",
		"numplayers":  "15",
	}))
	_, err = repo.AddOrUpdate(ctx, svr)
	require.NoError(t, err)

	updatedSvr, err := repo.Get(ctx, addr.MustNewFromString("1.1.1.1", 10480))
	updatedInfo := updatedSvr.GetInfo()
	require.NoError(t, err)
	assert.Equal(t, "Food Wall Restaurant", updatedInfo.MapName)
	assert.Equal(t, 15, updatedInfo.NumPlayers)

	// instance is listed again
	reportedSinceAfter, _ = repo.Filter(ctx, servers.NewFilterSet().After(after))
	assert.Len(t, reportedSinceAfter, 1)
}

func TestServerMemoryRepo_AddOrUpdate_VersionControl(t *testing.T) {
	ctx := context.TODO()
	repo := memory.New()

	svr, _ := servers.New(net.ParseIP("1.1.1.1"), 10480, 10481)
	svr.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "Barricaded Suspects",
	}))
	assert.Equal(t, 0, svr.GetVersion())
	svr, err := repo.AddOrUpdate(ctx, svr)
	require.NoError(t, err)
	// version is not incremented after insert
	assert.Equal(t, 0, svr.GetVersion())
	assert.Equal(t, "A-Bomb Nightclub", svr.GetInfo().MapName)

	svr.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Swat4 Server",
		"hostport":    "10480",
		"mapname":     "The Wolcott Projects",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "Barricaded Suspects",
	}))
	svr, err = repo.AddOrUpdate(ctx, svr)
	require.NoError(t, err)
	// version is incremented after update
	assert.Equal(t, 1, svr.GetVersion())
	assert.Equal(t, "The Wolcott Projects", svr.GetInfo().MapName)

	// version is incremented after every update
	svr.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Swat4 Server",
		"hostport":    "10480",
		"mapname":     "Food Wall Restaurant",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "Barricaded Suspects",
	}))
	svr, err = repo.AddOrUpdate(ctx, svr)
	require.NoError(t, err)
	// version is incremented after update
	assert.Equal(t, 2, svr.GetVersion())
	assert.Equal(t, "Food Wall Restaurant", svr.GetInfo().MapName)

	// version is saved in repo
	svr, err = repo.Get(ctx, svr.GetAddr())
	require.NoError(t, err)
	assert.Equal(t, 2, svr.GetVersion())
	assert.Equal(t, "Food Wall Restaurant", svr.GetInfo().MapName)

	// version is incremented after get+update
	svr.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Swat4 Server",
		"hostport":    "10480",
		"mapname":     "The Wolcott Projects",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "Barricaded Suspects",
	}))
	svr, err = repo.AddOrUpdate(ctx, svr)
	require.NoError(t, err)
	assert.Equal(t, 3, svr.GetVersion())
	assert.Equal(t, "The Wolcott Projects", svr.GetInfo().MapName)

	conflict, err := repo.Get(ctx, svr.GetAddr())
	require.NoError(t, err)
	assert.Equal(t, 3, conflict.GetVersion())
	assert.Equal(t, "The Wolcott Projects", conflict.GetInfo().MapName)

	svr, err = repo.AddOrUpdate(ctx, svr)
	require.NoError(t, err)
	assert.Equal(t, 4, svr.GetVersion())

	conflict, err = repo.AddOrUpdate(ctx, conflict)
	assert.ErrorIs(t, err, servers.ErrVersionConflict)
	assert.Equal(t, 3, conflict.GetVersion())
}

func TestServerMemoryRepo_Update_Exclusive(t *testing.T) {
	repo := memory.New()
	ctx := context.TODO()

	svr, _ := servers.New(net.ParseIP("1.1.1.1"), 10480, 10481)
	svr.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "VIP Escort",
		"numplayers":  "16",
	}))
	_, err := repo.AddOrUpdate(ctx, svr)
	require.NoError(t, err)

	svr, locker, err := repo.GetForUpdate(ctx, svr.GetAddr())
	require.NoError(t, err)

	updated := make(chan struct{})
	// a parallel update will wait for update to complete
	go func(a addr.Addr) {
		other, unlocker, err := repo.GetForUpdate(ctx, a)
		require.NoError(t, err)
		assert.Equal(t, 1, other.GetVersion())
		assert.Equal(t, ds.Info, other.GetDiscoveryStatus())
		other.UpdateDiscoveryStatus(ds.Details)
		other, err = repo.Update(ctx, unlocker, other)
		require.NoError(t, err)
		close(updated)
	}(svr.GetAddr())

	<-time.After(time.Millisecond * 10)
	svr.UpdateDiscoveryStatus(ds.Info)
	svr, err = repo.Update(ctx, locker, svr)
	require.NoError(t, err)

	<-updated
	svr, err = repo.Get(ctx, svr.GetAddr())
	require.NoError(t, err)
	assert.Equal(t, 2, svr.GetVersion())
	assert.Equal(t, ds.Info|ds.Details, svr.GetDiscoveryStatus())
}

func TestServerMemoryRepo_Update_ExclusiveAccess(t *testing.T) {
	repo := memory.New()
	ctx := context.TODO()

	svr, _ := servers.New(net.ParseIP("1.1.1.1"), 10480, 10481)
	svr.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "VIP Escort",
		"numplayers":  "16",
	}))
	_, err := repo.AddOrUpdate(ctx, svr)
	require.NoError(t, err)

	svr, locker, err := repo.GetForUpdate(ctx, svr.GetAddr())
	require.NoError(t, err)
	defer locker.Unlock()

	finished := make(chan struct{})
	// a parallel get will wait for update to complete
	go func(a addr.Addr) {
		other, err := repo.Get(ctx, a)
		require.NoError(t, err)
		assert.Equal(t, 1, other.GetVersion())
		assert.Equal(t, ds.Info, other.GetDiscoveryStatus())
		close(finished)
	}(svr.GetAddr())

	<-time.After(time.Millisecond * 10)
	svr.UpdateDiscoveryStatus(ds.Info)
	svr, err = repo.Update(ctx, locker, svr)
	require.NoError(t, err)

	<-finished

	svr, err = repo.Get(ctx, svr.GetAddr())
	require.NoError(t, err)
	assert.Equal(t, 1, svr.GetVersion())
	assert.Equal(t, ds.Info, svr.GetDiscoveryStatus())
}

func TestServerMemoryRepo_Update_Serialization(t *testing.T) {
	repo := memory.New()
	ctx := context.TODO()

	svr, _ := servers.New(net.ParseIP("1.1.1.1"), 10480, 10481)
	svr.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "VIP Escort",
		"numplayers":  "16",
	}))
	_, err := repo.AddOrUpdate(ctx, svr)
	require.NoError(t, err)

	wg := &sync.WaitGroup{}
	// a parallel get will wait for update to complete
	update := func(addr addr.Addr, status ds.DiscoveryStatus) {
		defer wg.Done()
		server, unlocker, err := repo.GetForUpdate(ctx, addr)
		require.NoError(t, err)
		server.UpdateDiscoveryStatus(status)
		server, err = repo.Update(ctx, unlocker, server)
		require.NoError(t, err)
	}

	wg.Add(4)
	go update(svr.GetAddr(), ds.Master)
	go update(svr.GetAddr(), ds.Info)
	go update(svr.GetAddr(), ds.Details)
	go update(svr.GetAddr(), ds.Port)
	wg.Wait()

	svr, err = repo.Get(ctx, svr.GetAddr())
	require.NoError(t, err)
	assert.Equal(t, 4, svr.GetVersion())
	assert.Equal(t, ds.Info|ds.Master|ds.Details|ds.Port, svr.GetDiscoveryStatus())
}

func TestServerMemoryRepo_Remove(t *testing.T) {
	tests := []struct {
		name    string
		server  servers.Server
		removed bool
	}{
		{
			name:    "positive case",
			server:  servers.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481),
			removed: true,
		},
		{
			name:    "unknown address",
			server:  servers.MustNew(net.ParseIP("1.1.1.1"), 10580, 10581),
			removed: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			before := time.Now()
			repo := memory.New()
			svr, _ := servers.New(net.ParseIP("1.1.1.1"), 10480, 10481)
			svr.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
				"hostname":    "Swat4 Server",
				"hostport":    "10480",
				"mapname":     "A-Bomb Nightclub",
				"gamever":     "1.1",
				"gamevariant": "SWAT 4",
				"gametype":    "Barricaded Suspects",
			}))
			repo.AddOrUpdate(ctx, svr) // nolint: errcheck

			reportedSinceBefore, _ := repo.Filter(ctx, servers.NewFilterSet().After(before))
			assert.Len(t, reportedSinceBefore, 1)

			err := repo.Remove(ctx, tt.server)
			assert.NoError(t, err)

			getInst, getErr := repo.Get(ctx, addr.MustNewFromString("1.1.1.1", 10480))
			reportedAfterRemove, _ := repo.Filter(ctx, servers.NewFilterSet().After(before))
			if !tt.removed {
				assert.NoError(t, getErr)
				assert.Equal(t, "1.1.1.1", getInst.GetDottedIP())
				assert.Len(t, reportedAfterRemove, 1)
			} else {
				assert.NoError(t, err)
				assert.ErrorIs(t, getErr, servers.ErrServerNotFound)
				assert.Len(t, reportedAfterRemove, 0)
			}
		})
	}
}

func TestServerMemoryRepo_CleanNext(t *testing.T) {
	repo := memory.New()
	ctx := context.TODO()

	before := time.Now()
	oldSvr := servers.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	oldSvr.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Old Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "Barricaded Suspects",
	}))
	repo.AddOrUpdate(ctx, oldSvr) // nolint: errcheck

	after := time.Now()
	newSvr := servers.MustNew(net.ParseIP("2.2.2.2"), 10480, 10481)
	newSvr.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "New Swat4 Server",
		"hostport":    "10480",
		"mapname":     "The Wolcott Projects",
		"gamever":     "1.0",
		"gamevariant": "SWAT 4X",
		"gametype":    "VIP Escort",
	}))
	repo.AddOrUpdate(ctx, newSvr) // nolint: errcheck

	// no servers are affected
	_, cleaned := repo.CleanNext(ctx, before)
	assert.False(t, cleaned)
	afterFirstClean, _ := repo.Filter(ctx, servers.NewFilterSet().After(before))
	assert.Len(t, afterFirstClean, 2)

	// the older instance was removed
	svr, cleaned := repo.CleanNext(ctx, after)
	assert.True(t, cleaned)
	assert.Equal(t, "1.1.1.1:10480", svr.GetAddr().String())
	afterSecondClean, _ := repo.Filter(ctx, servers.NewFilterSet().After(before))
	assert.Len(t, afterSecondClean, 1)
	assert.Equal(t, 0, cleanAll(repo, after))

	_, err := repo.Get(ctx, addr.MustNewFromString("1.1.1.1", 10480))
	assert.ErrorIs(t, err, servers.ErrServerNotFound)

	newSvr, err = repo.Get(ctx, addr.MustNewFromString("2.2.2.2", 10480))
	require.NoError(t, err)
	assert.Equal(t, "New Swat4 Server", newSvr.GetInfo().Hostname)

	// all servers are removed
	now := time.Now()
	svr, cleaned = repo.CleanNext(ctx, now)
	assert.True(t, cleaned)
	assert.Equal(t, "2.2.2.2:10480", svr.GetAddr().String())
	afterThirdClean, _ := repo.Filter(ctx, servers.NewFilterSet().After(before))
	assert.Len(t, afterThirdClean, 0)
	assert.Equal(t, 0, cleanAll(repo, now))

	_, err = repo.Get(ctx, addr.MustNewFromString("1.1.1.1", 10480))
	assert.ErrorIs(t, err, servers.ErrServerNotFound)

	_, err = repo.Get(ctx, addr.MustNewFromString("2.2.2.2", 10480))
	assert.ErrorIs(t, err, servers.ErrServerNotFound)
}

func TestServerMemoryRepo_CleanBefore_Multiple(t *testing.T) {
	repo := memory.New()
	ctx := context.TODO()

	before := time.Now()
	server1 := servers.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	repo.AddOrUpdate(ctx, server1) // nolint: errcheck

	server2 := servers.MustNew(net.ParseIP("2.2.2.2"), 10480, 10481)
	repo.AddOrUpdate(ctx, server2) // nolint: errcheck

	assert.Equal(t, 2, cleanAll(repo, time.Now()))

	afterFirstClean, _ := repo.Filter(ctx, servers.NewFilterSet().After(before))
	assert.Len(t, afterFirstClean, 0)

	_, err := repo.Get(ctx, addr.MustNewFromString("1.1.1.1", 10480))
	assert.ErrorIs(t, err, servers.ErrServerNotFound)

	_, err = repo.Get(ctx, addr.MustNewFromString("2.2.2.2", 10480))
	assert.ErrorIs(t, err, servers.ErrServerNotFound)
}

func TestServerMemoryRepo_CleanBefore_EmptyStorageNoError(t *testing.T) {
	repo := memory.New()
	_, cleaned := repo.CleanNext(context.TODO(), time.Now())
	assert.False(t, cleaned)
}

func TestServerMemoryRepo_Count(t *testing.T) {
	repo := memory.New()
	ctx := context.TODO()

	assertCount := func(expected int) {
		cnt, err := repo.Count(ctx)
		assert.NoError(t, err)
		assert.Equal(t, expected, cnt)
	}

	assertCount(0)

	server1 := servers.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	repo.AddOrUpdate(ctx, server1) // nolint: errcheck
	assertCount(1)

	server2 := servers.MustNew(net.ParseIP("2.2.2.2"), 10480, 10481)
	repo.AddOrUpdate(ctx, server2) // nolint: errcheck
	assertCount(2)

	_ = repo.Remove(ctx, server1)
	assertCount(1)

	_ = repo.Remove(ctx, server2)
	assertCount(0)

	// double remove
	_ = repo.Remove(ctx, server1)
	assertCount(0)
}

func TestServerMemoryRepo_CountByStatus(t *testing.T) {
	repo := memory.New()
	ctx := context.TODO()

	// empty repo
	count, err := repo.CountByStatus(ctx)
	require.NoError(t, err)
	assert.Len(t, count, 0)

	// only server with no status
	svr0 := servers.MustNew(net.ParseIP("1.10.1.10"), 10480, 10481)
	svr0.ClearDiscoveryStatus(ds.New)
	assert.Equal(t, ds.DiscoveryStatus(0), svr0.GetDiscoveryStatus())
	_, err = repo.AddOrUpdate(ctx, svr0)
	require.NoError(t, err)
	count, err = repo.CountByStatus(ctx)
	require.NoError(t, err)
	assert.Len(t, count, 0)

	svr1, _ := servers.New(net.ParseIP("1.1.1.1"), 10480, 10481)
	svr1.UpdateDiscoveryStatus(ds.Master | ds.Info)
	repo.AddOrUpdate(ctx, svr1) // nolint: errcheck

	svr2, _ := servers.New(net.ParseIP("2.2.2.2"), 10480, 10481)
	svr2.UpdateDiscoveryStatus(ds.Details | ds.Info)
	repo.AddOrUpdate(ctx, svr2) // nolint: errcheck

	svr3, _ := servers.New(net.ParseIP("3.3.3.3"), 10480, 10481)
	svr3.UpdateDiscoveryStatus(ds.Master | ds.Details | ds.Info)
	repo.AddOrUpdate(ctx, svr3) // nolint: errcheck

	svr4, _ := servers.New(net.ParseIP("4.4.4.4"), 10480, 10481)
	svr4.UpdateDiscoveryStatus(ds.NoDetails | ds.NoPort)
	repo.AddOrUpdate(ctx, svr4) // nolint: errcheck

	svr5, _ := servers.New(net.ParseIP("5.5.5.5"), 10480, 10481)
	svr5.UpdateDiscoveryStatus(ds.NoPort)
	repo.AddOrUpdate(ctx, svr5) // nolint: errcheck

	count, err = repo.CountByStatus(ctx)
	require.NoError(t, err)
	assert.Len(t, count, 5)
	assert.Equal(t, 2, count[ds.Master])
	assert.Equal(t, 3, count[ds.Info])
	assert.Equal(t, 2, count[ds.Details])
	assert.Equal(t, 1, count[ds.NoDetails])
	assert.Equal(t, 2, count[ds.NoPort])
}

func cleanAll(repo servers.Repository, before time.Time) int {
	var runs int
	for {
		_, cleaned := repo.CleanNext(context.TODO(), before)
		if !cleaned {
			break
		}
		runs++
	}
	return runs
}
