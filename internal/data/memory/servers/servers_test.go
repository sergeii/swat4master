package servers_test

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/entities/details"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/data/memory/servers"
	"github.com/sergeii/swat4master/internal/testutils"
)

func makeRepo() (*servers.Repository, *clock.Mock) {
	clockMock := clock.NewMock()
	return servers.New(clockMock), clockMock
}

func TestServerMemoryRepo_Add_New(t *testing.T) {
	ctx := context.TODO()
	repo, clockMock := makeRepo()

	svr, _ := server.New(net.ParseIP("1.1.1.1"), 10480, 10481)
	svr.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "Barricaded Suspects",
	}), clockMock.Now())
	svr, err := repo.Add(ctx, svr, repositories.ServerOnConflictIgnore)
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1", svr.GetDottedIP())
	assert.Equal(t, 10480, svr.GetGamePort())
	assert.Equal(t, 10481, svr.GetQueryPort())
	assert.Equal(t, 0, svr.GetVersion())

	other, _ := server.New(net.ParseIP("2.2.2.2"), 10480, 10481)
	other.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Another Swat4 Server",
		"hostport":    "10480",
		"mapname":     "The Wolcott Projects",
		"gamever":     "1.0",
		"gamevariant": "SWAT 4X",
		"gametype":    "VIP Escort",
	}), clockMock.Now())
	other, otherErr := repo.Add(ctx, other, repositories.ServerOnConflictIgnore)
	assert.Equal(t, "2.2.2.2", other.GetDottedIP())
	assert.Equal(t, 10480, other.GetGamePort())
	assert.Equal(t, 10481, other.GetQueryPort())
	assert.Equal(t, 0, other.GetVersion())

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

func TestServerMemoryRepo_Add_UpdateOnConflict(t *testing.T) {
	tests := []struct {
		name    string
		updated bool
	}{
		{
			"updated on conlfict",
			true,
		},
		{
			"conflict is ignored",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			repo, clockMock := makeRepo()

			svr, _ := server.New(net.ParseIP("1.1.1.1"), 10480, 10481)

			initialParams := details.MustNewInfoFromParams(map[string]string{
				"hostname":    "Swat4 Server",
				"hostport":    "10480",
				"mapname":     "A-Bomb Nightclub",
				"gamever":     "1.1",
				"gamevariant": "SWAT 4",
				"gametype":    "VIP Escort",
				"numplayers":  "16",
			})
			svr.UpdateInfo(initialParams, clockMock.Now())
			svr, err := repo.Add(ctx, svr, repositories.ServerOnConflictIgnore)
			require.NoError(t, err)

			addedSvr, err := repo.Get(ctx, addr.MustNewFromString("1.1.1.1", 10480))
			require.NoError(t, err)
			addedInfo := addedSvr.GetInfo()
			assert.Equal(t, "A-Bomb Nightclub", addedInfo.MapName)
			assert.Equal(t, 16, addedInfo.NumPlayers)

			newParams := details.MustNewInfoFromParams(map[string]string{
				"hostname":    "Swat4 Server",
				"hostport":    "10480",
				"mapname":     "Food Wall Restaurant",
				"gamever":     "1.1",
				"gamevariant": "SWAT 4",
				"gametype":    "VIP Escort",
				"numplayers":  "15",
			})
			svr.UpdateInfo(newParams, clockMock.Now())
			svr, addError := repo.Add(ctx, svr, func(s *server.Server) bool {
				s.UpdateInfo(newParams, clockMock.Now())
				return tt.updated
			})

			updatedSvr, _ := repo.Get(ctx, addedSvr.GetAddr())
			updatedInfo := updatedSvr.GetInfo()

			if tt.updated {
				require.NoError(t, addError)
				assert.Equal(t, "Food Wall Restaurant", updatedInfo.MapName)
				assert.Equal(t, 15, updatedInfo.NumPlayers)
				assert.Equal(t, 1, updatedSvr.GetVersion())
			} else {
				require.ErrorIs(t, addError, repositories.ErrServerExists)
				assert.Equal(t, "A-Bomb Nightclub", updatedInfo.MapName)
				assert.Equal(t, 16, updatedInfo.NumPlayers)
				assert.Equal(t, 0, updatedSvr.GetVersion())
			}
		})
	}
}

func TestServerMemoryRepo_Add_MultipleConflicts(t *testing.T) {
	ctx := context.TODO()
	repo, _ := makeRepo()

	wg := &sync.WaitGroup{}

	add := func(svr server.Server, status ds.DiscoveryStatus) {
		defer wg.Done()
		svr.UpdateDiscoveryStatus(status)
		_, err := repo.Add(ctx, svr, func(s *server.Server) bool {
			s.UpdateDiscoveryStatus(status)
			return true
		})
		require.NoError(t, err)
	}

	svr, _ := server.New(net.ParseIP("1.1.1.1"), 10480, 10481)

	wg.Add(4)
	go add(svr, ds.Master)
	go add(svr, ds.Info)
	go add(svr, ds.Details)
	go add(svr, ds.Port)
	wg.Wait()

	svr, err := repo.Get(ctx, svr.GetAddr())
	require.NoError(t, err)
	assert.Equal(t, 3, svr.GetVersion())
	assert.Equal(t, ds.Info|ds.Master|ds.Details|ds.Port, svr.GetDiscoveryStatus())
}

func TestServerMemoryRepo_Update_ResolveConflict(t *testing.T) {
	tests := []struct {
		name     string
		resolved bool
	}{
		{
			"conflict is resolved",
			true,
		},
		{
			"conflict is ignored",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			repo, clockMock := makeRepo()

			svr, _ := server.New(net.ParseIP("1.1.1.1"), 10480, 10481)
			svr.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
				"hostname":    "Swat4 Server",
				"hostport":    "10480",
				"mapname":     "A-Bomb Nightclub",
				"gamever":     "1.1",
				"gamevariant": "SWAT 4",
				"gametype":    "VIP Escort",
				"numplayers":  "16",
			}), clockMock.Now())
			_, err := repo.Add(ctx, svr, repositories.ServerOnConflictIgnore)
			require.NoError(t, err)

			gotOutdated := make(chan struct{})
			updatedMain := make(chan struct{})
			updatedParallel := make(chan struct{})
			go func(a addr.Addr, shouldResolve bool) {
				other, err := repo.Get(ctx, a)
				require.NoError(t, err)
				close(gotOutdated)
				<-updatedMain
				assert.Equal(t, 0, other.GetVersion())
				assert.Equal(t, ds.New, other.GetDiscoveryStatus())
				other.UpdateDiscoveryStatus(ds.Master)
				other, err = repo.Update(ctx, other, func(s *server.Server) bool {
					s.UpdateDiscoveryStatus(ds.Master)
					return shouldResolve
				})
				require.NoError(t, err)
				close(updatedParallel)
			}(svr.GetAddr(), tt.resolved)

			<-gotOutdated
			svr.UpdateDiscoveryStatus(ds.Info | ds.Details)
			svr, err = repo.Update(ctx, svr, func(s *server.Server) bool {
				panic("should never be called")
			})
			require.NoError(t, err)
			close(updatedMain)

			<-updatedParallel
			svr, err = repo.Get(ctx, svr.GetAddr())
			require.NoError(t, err)

			if tt.resolved {
				assert.Equal(t, 2, svr.GetVersion())
				assert.Equal(t, ds.Master|ds.Info|ds.Details, svr.GetDiscoveryStatus())
			} else {
				assert.Equal(t, 1, svr.GetVersion())
				assert.Equal(t, ds.Info|ds.Details, svr.GetDiscoveryStatus())
			}
		})
	}
}

func TestServerMemoryRepo_Update_MultipleConflicts(t *testing.T) {
	ctx := context.TODO()
	repo, clockMock := makeRepo()

	svr, _ := server.New(net.ParseIP("1.1.1.1"), 10480, 10481)
	svr.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "VIP Escort",
		"numplayers":  "16",
	}), clockMock.Now())
	_, err := repo.Add(ctx, svr, repositories.ServerOnConflictIgnore)
	require.NoError(t, err)

	wg := &sync.WaitGroup{}

	update := func(addr addr.Addr, status ds.DiscoveryStatus) {
		defer wg.Done()
		svr, err := repo.Get(ctx, addr)
		require.NoError(t, err)
		svr.UpdateDiscoveryStatus(status)
		svr, err = repo.Update(ctx, svr, func(s *server.Server) bool {
			s.UpdateDiscoveryStatus(status)
			return true
		})
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

func TestServerMemoryRepo_Remove_NoConflict(t *testing.T) {
	tests := []struct {
		name    string
		server  server.Server
		removed bool
	}{
		{
			name:    "positive case",
			server:  server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481),
			removed: true,
		},
		{
			name:    "unknown address",
			server:  server.MustNew(net.ParseIP("1.1.1.1"), 10580, 10581),
			removed: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			repo, _ := makeRepo()

			svr, _ := server.New(net.ParseIP("1.1.1.1"), 10480, 10481)
			repo.Add(ctx, svr, repositories.ServerOnConflictIgnore) // nolint: errcheck

			err := repo.Remove(ctx, tt.server, func(s *server.Server) bool {
				panic("should not be called")
			})
			assert.NoError(t, err)

			got, getErr := repo.Get(ctx, addr.MustNewFromString("1.1.1.1", 10480))
			if !tt.removed {
				assert.NoError(t, getErr)
				assert.Equal(t, "1.1.1.1", got.GetDottedIP())
			} else {
				assert.NoError(t, err)
				assert.ErrorIs(t, getErr, repositories.ErrServerNotFound)
			}
		})
	}
}

func TestServerMemoryRepo_Remove_ResolveConflict(t *testing.T) {
	tests := []struct {
		name     string
		resolved bool
	}{
		{
			"conflict is resolved",
			true,
		},
		{
			"conflict is ignored",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			repo, clockMock := makeRepo()
			svr, _ := server.New(net.ParseIP("1.1.1.1"), 10480, 10481)
			repo.Add(ctx, svr, repositories.ServerOnConflictIgnore) // nolint: errcheck

			obtained := make(chan struct{})
			updated := make(chan struct{})
			removed := make(chan struct{})

			go func(a addr.Addr, resolved bool) {
				outdated, err := repo.Get(ctx, a)
				require.NoError(t, err)
				assert.Equal(t, 0, outdated.GetVersion())
				close(obtained)

				<-updated
				err = repo.Remove(ctx, outdated, func(s *server.Server) bool {
					return resolved
				})
				require.NoError(t, err)
				close(removed)
			}(svr.GetAddr(), tt.resolved)

			<-obtained
			svr.Refresh(clockMock.Now())
			svr, err := repo.Update(ctx, svr, func(_ *server.Server) bool {
				panic("should not be called")
			})
			require.NoError(t, err)
			close(updated)

			<-removed
			got, getErr := repo.Get(ctx, addr.MustNewFromString("1.1.1.1", 10480))
			if !tt.resolved {
				assert.NoError(t, getErr)
				assert.Equal(t, "1.1.1.1", got.GetDottedIP())
				assert.Equal(t, 1, got.GetVersion())
			} else {
				assert.NoError(t, err)
				assert.ErrorIs(t, getErr, repositories.ErrServerNotFound)
			}
		})
	}
}

func TestServerMemoryRepo_Count(t *testing.T) {
	repo, _ := makeRepo()
	ctx := context.TODO()

	assertCount := func(expected int) {
		cnt, err := repo.Count(ctx)
		assert.NoError(t, err)
		assert.Equal(t, expected, cnt)
	}

	assertCount(0)

	server1 := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	repo.Add(ctx, server1, repositories.ServerOnConflictIgnore) // nolint: errcheck
	assertCount(1)

	server2 := server.MustNew(net.ParseIP("2.2.2.2"), 10480, 10481)
	repo.Add(ctx, server2, repositories.ServerOnConflictIgnore) // nolint: errcheck
	assertCount(2)

	_ = repo.Remove(ctx, server1, repositories.ServerOnConflictIgnore)
	assertCount(1)

	_ = repo.Remove(ctx, server2, repositories.ServerOnConflictIgnore)
	assertCount(0)

	// double remove
	_ = repo.Remove(ctx, server1, repositories.ServerOnConflictIgnore)
	assertCount(0)
}

func TestServerMemoryRepo_CountByStatus(t *testing.T) {
	repo, _ := makeRepo()
	ctx := context.TODO()

	// empty repo
	count, err := repo.CountByStatus(ctx)
	require.NoError(t, err)
	assert.Len(t, count, 0)

	// only server with no status
	svr0 := server.MustNew(net.ParseIP("1.10.1.10"), 10480, 10481)
	svr0.ClearDiscoveryStatus(ds.New)
	assert.Equal(t, ds.DiscoveryStatus(0), svr0.GetDiscoveryStatus())
	_, err = repo.Add(ctx, svr0, repositories.ServerOnConflictIgnore)
	require.NoError(t, err)
	count, err = repo.CountByStatus(ctx)
	require.NoError(t, err)
	assert.Len(t, count, 0)

	svr1, _ := server.New(net.ParseIP("1.1.1.1"), 10480, 10481)
	svr1.UpdateDiscoveryStatus(ds.Master | ds.Info)
	repo.Add(ctx, svr1, repositories.ServerOnConflictIgnore) // nolint: errcheck

	svr2, _ := server.New(net.ParseIP("2.2.2.2"), 10480, 10481)
	svr2.UpdateDiscoveryStatus(ds.Details | ds.Info)
	repo.Add(ctx, svr2, repositories.ServerOnConflictIgnore) // nolint: errcheck

	svr3, _ := server.New(net.ParseIP("3.3.3.3"), 10480, 10481)
	svr3.UpdateDiscoveryStatus(ds.Master | ds.Details | ds.Info)
	repo.Add(ctx, svr3, repositories.ServerOnConflictIgnore) // nolint: errcheck

	svr4, _ := server.New(net.ParseIP("4.4.4.4"), 10480, 10481)
	svr4.UpdateDiscoveryStatus(ds.NoDetails | ds.NoPort)
	repo.Add(ctx, svr4, repositories.ServerOnConflictIgnore) // nolint: errcheck

	svr5, _ := server.New(net.ParseIP("5.5.5.5"), 10480, 10481)
	svr5.UpdateDiscoveryStatus(ds.NoPort)
	repo.Add(ctx, svr5, repositories.ServerOnConflictIgnore) // nolint: errcheck

	count, err = repo.CountByStatus(ctx)
	require.NoError(t, err)
	assert.Len(t, count, 5)
	assert.Equal(t, 2, count[ds.Master])
	assert.Equal(t, 3, count[ds.Info])
	assert.Equal(t, 2, count[ds.Details])
	assert.Equal(t, 1, count[ds.NoDetails])
	assert.Equal(t, 2, count[ds.NoPort])
}

func TestServerMemoryRepo_Filter(t *testing.T) {
	tests := []struct {
		name        string
		fsfactory   func(clock.Clock, time.Time, time.Time, time.Time, time.Time, time.Time) repositories.ServerFilterSet
		wantServers []string
	}{
		{
			"no filter",
			func(_ clock.Clock, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet()
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
			func(_ clock.Clock, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().NoStatus(ds.Master)
			},
			[]string{
				"5.5.5.5:10480",
				"4.4.4.4:10480",
				"2.2.2.2:10480",
			},
		},
		{
			"exclude multiple status",
			func(_ clock.Clock, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().NoStatus(ds.PortRetry | ds.NoDetails)
			},
			[]string{
				"3.3.3.3:10480",
				"2.2.2.2:10480",
				"1.1.1.1:10480",
			},
		},
		{
			"include status",
			func(_ clock.Clock, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().WithStatus(ds.Master)
			},
			[]string{
				"3.3.3.3:10480",
				"1.1.1.1:10480",
			},
		},
		{
			"include multiple status",
			func(_ clock.Clock, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().WithStatus(ds.Master | ds.Details)
			},
			[]string{
				"3.3.3.3:10480",
			},
		},
		{
			"filter by multiple status",
			func(_ clock.Clock, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().WithStatus(ds.Master | ds.Details)
			},
			[]string{
				"3.3.3.3:10480",
			},
		},
		{
			"filter by update date - after",
			func(_ clock.Clock, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().UpdatedAfter(t4)
			},
			[]string{
				"5.5.5.5:10480",
				"4.4.4.4:10480",
			},
		},
		{
			"filter by update date - before",
			func(_ clock.Clock, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().UpdatedBefore(t4)
			},
			[]string{
				"3.3.3.3:10480",
				"2.2.2.2:10480",
				"1.1.1.1:10480",
			},
		},
		{
			"filter by update date - after and before",
			func(_ clock.Clock, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().
					UpdatedAfter(t4).
					UpdatedBefore(t5)
			},
			[]string{
				"4.4.4.4:10480",
			},
		},
		{
			"filter by update date - no match",
			func(c clock.Clock, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().UpdatedAfter(c.Now())
			},
			[]string{},
		},
		{
			"filter by refresh date - after",
			func(_ clock.Clock, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().ActiveAfter(t2)
			},
			[]string{
				"3.3.3.3:10480",
				"2.2.2.2:10480",
			},
		},
		{
			"filter by refresh date - before",
			func(_ clock.Clock, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().ActiveBefore(t3)
			},
			[]string{
				"2.2.2.2:10480",
				"1.1.1.1:10480",
			},
		},
		{
			"filter by refresh date - after and before",
			func(_ clock.Clock, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().
					ActiveAfter(t2).
					ActiveBefore(t3)
			},
			[]string{
				"2.2.2.2:10480",
			},
		},
		{
			"filter by refresh date - no match",
			func(c clock.Clock, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().ActiveAfter(c.Now())
			},
			[]string{},
		},
		{
			"filter by multiple fields",
			func(_ clock.Clock, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().
					UpdatedBefore(t5).
					ActiveAfter(t2).
					WithStatus(ds.Master)
			},
			[]string{
				"3.3.3.3:10480",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			repo, clockMock := makeRepo()

			t1 := clockMock.Now()
			clockMock.Add(time.Millisecond)

			svr1, _ := server.New(net.ParseIP("1.1.1.1"), 10480, 10481)
			svr1.UpdateDiscoveryStatus(ds.Master | ds.Info)
			svr1.Refresh(clockMock.Now())
			repo.Add(ctx, svr1, repositories.ServerOnConflictIgnore) // nolint: errcheck

			clockMock.Add(time.Millisecond)
			t2 := clockMock.Now()

			svr2, _ := server.New(net.ParseIP("2.2.2.2"), 10480, 10481)
			svr2.UpdateDiscoveryStatus(ds.Details | ds.Info)
			svr2.Refresh(clockMock.Now())
			repo.Add(ctx, svr2, repositories.ServerOnConflictIgnore) // nolint: errcheck

			clockMock.Add(time.Millisecond)
			t3 := clockMock.Now()

			svr3, _ := server.New(net.ParseIP("3.3.3.3"), 10480, 10481)
			svr3.UpdateDiscoveryStatus(ds.Master | ds.Details | ds.Info)
			svr3.Refresh(clockMock.Now())
			repo.Add(ctx, svr3, repositories.ServerOnConflictIgnore) // nolint: errcheck

			clockMock.Add(time.Millisecond)
			t4 := clockMock.Now()

			svr4, _ := server.New(net.ParseIP("4.4.4.4"), 10480, 10481)
			svr4.UpdateDiscoveryStatus(ds.NoDetails | ds.NoPort)
			repo.Add(ctx, svr4, repositories.ServerOnConflictIgnore) // nolint: errcheck

			clockMock.Add(time.Millisecond)
			t5 := clockMock.Now()

			svr5, _ := server.New(net.ParseIP("5.5.5.5"), 10480, 10481)
			svr5.UpdateDiscoveryStatus(ds.NoPort | ds.PortRetry)
			repo.Add(ctx, svr5, repositories.ServerOnConflictIgnore) // nolint: errcheck

			clockMock.Add(time.Millisecond)

			gotServers, err := repo.Filter(ctx, tt.fsfactory(clockMock, t1, t2, t3, t4, t5))
			require.NoError(t, err)
			testutils.AssertServers(t, tt.wantServers, gotServers)
		})
	}
}

func TestServerMemoryRepo_Filter_Empty(t *testing.T) {
	tests := []struct {
		name      string
		fsfactory func(clock.Clock) repositories.ServerFilterSet
	}{
		{
			"default filterset",
			func(clock.Clock) repositories.ServerFilterSet { return repositories.NewServerFilterSet() },
		},
		{
			"filter by no status",
			func(clock.Clock) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().NoStatus(ds.Master)
			},
		},
		{
			"filter by status",
			func(clock.Clock) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().WithStatus(ds.Master)
			},
		},
		{
			"filter by status and update date",
			func(c clock.Clock) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().WithStatus(ds.Master).UpdatedBefore(c.Now())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, clockMock := makeRepo()
			ctx := context.TODO()

			items, err := repo.Filter(ctx, tt.fsfactory(clockMock))
			require.NoError(t, err)
			assert.Equal(t, []server.Server{}, items)
		})
	}
}
