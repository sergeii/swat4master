package servers_test

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/entities/details"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/persistence/memory/servers"
	"github.com/sergeii/swat4master/internal/testutils"
)

func TestServerMemoryRepo_Add_New(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	repo := servers.New(c)

	svr := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	svr.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "Barricaded Suspects",
	}), c.Now())
	svr, err := repo.Add(ctx, svr, repositories.ServerOnConflictIgnore)
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1", svr.Addr.GetDottedIP())
	assert.Equal(t, 10480, svr.Addr.Port)
	assert.Equal(t, 10481, svr.QueryPort)
	assert.Equal(t, 0, svr.Version)

	other := server.MustNew(net.ParseIP("2.2.2.2"), 10480, 10481)
	other.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Another Swat4 Server",
		"hostport":    "10480",
		"mapname":     "The Wolcott Projects",
		"gamever":     "1.0",
		"gamevariant": "SWAT 4X",
		"gametype":    "VIP Escort",
	}), c.Now())
	other, otherErr := repo.Add(ctx, other, repositories.ServerOnConflictIgnore)
	assert.Equal(t, "2.2.2.2", other.Addr.GetDottedIP())
	assert.Equal(t, 10480, other.Addr.Port)
	assert.Equal(t, 10481, other.QueryPort)
	assert.Equal(t, 0, other.Version)

	require.NoError(t, otherErr)

	addedSvr, err := repo.Get(ctx, addr.MustNewFromDotted("1.1.1.1", 10480))
	require.NoError(t, err)
	assert.Equal(t, "Swat4 Server", addedSvr.Info.Hostname)

	addedOther, err := repo.Get(ctx, addr.MustNewFromDotted("2.2.2.2", 10480))
	require.NoError(t, err)
	assert.Equal(t, "Another Swat4 Server", addedOther.Info.Hostname)
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
			c := clockwork.NewFakeClock()
			repo := servers.New(c)

			svr := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)

			initialParams := details.MustNewInfoFromParams(map[string]string{
				"hostname":    "Swat4 Server",
				"hostport":    "10480",
				"mapname":     "A-Bomb Nightclub",
				"gamever":     "1.1",
				"gamevariant": "SWAT 4",
				"gametype":    "VIP Escort",
				"numplayers":  "16",
			})
			svr.UpdateInfo(initialParams, c.Now())
			svr, err := repo.Add(ctx, svr, repositories.ServerOnConflictIgnore)
			require.NoError(t, err)

			addedSvr, err := repo.Get(ctx, addr.MustNewFromDotted("1.1.1.1", 10480))
			require.NoError(t, err)
			assert.Equal(t, "A-Bomb Nightclub", addedSvr.Info.MapName)
			assert.Equal(t, 16, addedSvr.Info.NumPlayers)

			newParams := details.MustNewInfoFromParams(map[string]string{
				"hostname":    "Swat4 Server",
				"hostport":    "10480",
				"mapname":     "Food Wall Restaurant",
				"gamever":     "1.1",
				"gamevariant": "SWAT 4",
				"gametype":    "VIP Escort",
				"numplayers":  "15",
			})
			svr.UpdateInfo(newParams, c.Now())
			svr, addError := repo.Add(ctx, svr, func(s *server.Server) bool {
				s.UpdateInfo(newParams, c.Now())
				return tt.updated
			})

			updatedSvr, _ := repo.Get(ctx, addedSvr.Addr)

			if tt.updated {
				require.NoError(t, addError)
				assert.Equal(t, "Food Wall Restaurant", updatedSvr.Info.MapName)
				assert.Equal(t, 15, updatedSvr.Info.NumPlayers)
				assert.Equal(t, 1, updatedSvr.Version)
			} else {
				require.ErrorIs(t, addError, repositories.ErrServerExists)
				assert.Equal(t, "A-Bomb Nightclub", updatedSvr.Info.MapName)
				assert.Equal(t, 16, updatedSvr.Info.NumPlayers)
				assert.Equal(t, 0, updatedSvr.Version)
			}
		})
	}
}

func TestServerMemoryRepo_Add_MultipleConflicts(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	repo := servers.New(c)

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

	svr := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)

	wg.Add(4)
	go add(svr, ds.Master)
	go add(svr, ds.Info)
	go add(svr, ds.Details)
	go add(svr, ds.Port)
	wg.Wait()

	svr, err := repo.Get(ctx, svr.Addr)
	require.NoError(t, err)
	assert.Equal(t, 3, svr.Version)
	assert.Equal(t, ds.Info|ds.Master|ds.Details|ds.Port, svr.DiscoveryStatus)
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
			c := clockwork.NewFakeClock()
			repo := servers.New(c)

			svr := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
			svr.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
				"hostname":    "Swat4 Server",
				"hostport":    "10480",
				"mapname":     "A-Bomb Nightclub",
				"gamever":     "1.1",
				"gamevariant": "SWAT 4",
				"gametype":    "VIP Escort",
				"numplayers":  "16",
			}), c.Now())
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
				assert.Equal(t, 0, other.Version)
				assert.Equal(t, ds.New, other.DiscoveryStatus)
				other.UpdateDiscoveryStatus(ds.Master)
				other, err = repo.Update(ctx, other, func(s *server.Server) bool {
					s.UpdateDiscoveryStatus(ds.Master)
					return shouldResolve
				})
				require.NoError(t, err)
				close(updatedParallel)
			}(svr.Addr, tt.resolved)

			<-gotOutdated
			svr.UpdateDiscoveryStatus(ds.Info | ds.Details)
			svr, err = repo.Update(ctx, svr, func(s *server.Server) bool {
				panic("should never be called")
			})
			require.NoError(t, err)
			close(updatedMain)

			<-updatedParallel
			svr, err = repo.Get(ctx, svr.Addr)
			require.NoError(t, err)

			if tt.resolved {
				assert.Equal(t, 2, svr.Version)
				assert.Equal(t, ds.Master|ds.Info|ds.Details, svr.DiscoveryStatus)
			} else {
				assert.Equal(t, 1, svr.Version)
				assert.Equal(t, ds.Info|ds.Details, svr.DiscoveryStatus)
			}
		})
	}
}

func TestServerMemoryRepo_Update_MultipleConflicts(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	repo := servers.New(c)

	svr := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	svr.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "VIP Escort",
		"numplayers":  "16",
	}), c.Now())
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
	go update(svr.Addr, ds.Master)
	go update(svr.Addr, ds.Info)
	go update(svr.Addr, ds.Details)
	go update(svr.Addr, ds.Port)
	wg.Wait()

	svr, err = repo.Get(ctx, svr.Addr)
	require.NoError(t, err)
	assert.Equal(t, 4, svr.Version)
	assert.Equal(t, ds.Info|ds.Master|ds.Details|ds.Port, svr.DiscoveryStatus)
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
			c := clockwork.NewFakeClock()
			repo := servers.New(c)

			svr := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
			repo.Add(ctx, svr, repositories.ServerOnConflictIgnore) // nolint: errcheck

			err := repo.Remove(ctx, tt.server, func(s *server.Server) bool {
				panic("should not be called")
			})
			assert.NoError(t, err)

			got, getErr := repo.Get(ctx, addr.MustNewFromDotted("1.1.1.1", 10480))
			if !tt.removed {
				assert.NoError(t, getErr)
				assert.Equal(t, "1.1.1.1", got.Addr.GetDottedIP())
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
			c := clockwork.NewFakeClock()
			repo := servers.New(c)

			svr := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
			repo.Add(ctx, svr, repositories.ServerOnConflictIgnore) // nolint: errcheck

			obtained := make(chan struct{})
			updated := make(chan struct{})
			removed := make(chan struct{})

			go func(a addr.Addr, resolved bool) {
				outdated, err := repo.Get(ctx, a)
				require.NoError(t, err)
				assert.Equal(t, 0, outdated.Version)
				close(obtained)

				<-updated
				err = repo.Remove(ctx, outdated, func(s *server.Server) bool {
					return resolved
				})
				require.NoError(t, err)
				close(removed)
			}(svr.Addr, tt.resolved)

			<-obtained
			svr.Refresh(c.Now())
			svr, err := repo.Update(ctx, svr, func(_ *server.Server) bool {
				panic("should not be called")
			})
			require.NoError(t, err)
			close(updated)

			<-removed
			got, getErr := repo.Get(ctx, addr.MustNewFromDotted("1.1.1.1", 10480))
			if !tt.resolved {
				assert.NoError(t, getErr)
				assert.Equal(t, "1.1.1.1", got.Addr.GetDottedIP())
				assert.Equal(t, 1, got.Version)
			} else {
				assert.NoError(t, err)
				assert.ErrorIs(t, getErr, repositories.ErrServerNotFound)
			}
		})
	}
}

func TestServerMemoryRepo_Count(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	repo := servers.New(c)

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
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	repo := servers.New(c)

	// empty repo
	count, err := repo.CountByStatus(ctx)
	require.NoError(t, err)
	assert.Len(t, count, 0)

	// only server with no status
	svr0 := server.MustNew(net.ParseIP("1.10.1.10"), 10480, 10481)
	svr0.ClearDiscoveryStatus(ds.New)
	assert.Equal(t, ds.DiscoveryStatus(0), svr0.DiscoveryStatus)
	_, err = repo.Add(ctx, svr0, repositories.ServerOnConflictIgnore)
	require.NoError(t, err)
	count, err = repo.CountByStatus(ctx)
	require.NoError(t, err)
	assert.Len(t, count, 0)

	svr1 := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	svr1.UpdateDiscoveryStatus(ds.Master | ds.Info)
	repo.Add(ctx, svr1, repositories.ServerOnConflictIgnore) // nolint: errcheck

	svr2 := server.MustNew(net.ParseIP("2.2.2.2"), 10480, 10481)
	svr2.UpdateDiscoveryStatus(ds.Details | ds.Info)
	repo.Add(ctx, svr2, repositories.ServerOnConflictIgnore) // nolint: errcheck

	svr3 := server.MustNew(net.ParseIP("3.3.3.3"), 10480, 10481)
	svr3.UpdateDiscoveryStatus(ds.Master | ds.Details | ds.Info)
	repo.Add(ctx, svr3, repositories.ServerOnConflictIgnore) // nolint: errcheck

	svr4 := server.MustNew(net.ParseIP("4.4.4.4"), 10480, 10481)
	svr4.UpdateDiscoveryStatus(ds.NoDetails | ds.NoPort)
	repo.Add(ctx, svr4, repositories.ServerOnConflictIgnore) // nolint: errcheck

	svr5 := server.MustNew(net.ParseIP("5.5.5.5"), 10480, 10481)
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
		fsfactory   func(time.Time, time.Time, time.Time, time.Time, time.Time, time.Time) repositories.ServerFilterSet
		wantServers []string
	}{
		{
			"no filter",
			func(_ time.Time, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
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
			func(_ time.Time, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
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
			func(_ time.Time, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
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
			func(_ time.Time, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().WithStatus(ds.Master)
			},
			[]string{
				"3.3.3.3:10480",
				"1.1.1.1:10480",
			},
		},
		{
			"include multiple status",
			func(_ time.Time, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().WithStatus(ds.Master | ds.Details)
			},
			[]string{
				"3.3.3.3:10480",
			},
		},
		{
			"filter by multiple status",
			func(_ time.Time, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().WithStatus(ds.Master | ds.Details)
			},
			[]string{
				"3.3.3.3:10480",
			},
		},
		{
			"filter by update date - after",
			func(_ time.Time, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().UpdatedAfter(t4)
			},
			[]string{
				"5.5.5.5:10480",
				"4.4.4.4:10480",
			},
		},
		{
			"filter by update date - before",
			func(_ time.Time, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
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
			func(_ time.Time, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
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
			func(now time.Time, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().UpdatedAfter(now)
			},
			[]string{},
		},
		{
			"filter by refresh date - after",
			func(_ time.Time, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().ActiveAfter(t2)
			},
			[]string{
				"3.3.3.3:10480",
				"2.2.2.2:10480",
			},
		},
		{
			"filter by refresh date - before",
			func(_ time.Time, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().ActiveBefore(t3)
			},
			[]string{
				"2.2.2.2:10480",
				"1.1.1.1:10480",
			},
		},
		{
			"filter by refresh date - after and before",
			func(_ time.Time, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
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
			func(now time.Time, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().ActiveAfter(now)
			},
			[]string{},
		},
		{
			"filter by multiple fields",
			func(_ time.Time, t1, t2, t3, t4, t5 time.Time) repositories.ServerFilterSet {
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
			c := clockwork.NewFakeClock()
			repo := servers.New(c)

			t1 := c.Now()
			c.Advance(time.Millisecond)

			svr1 := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
			svr1.UpdateDiscoveryStatus(ds.Master | ds.Info)
			svr1.Refresh(c.Now())
			repo.Add(ctx, svr1, repositories.ServerOnConflictIgnore) // nolint: errcheck

			c.Advance(time.Millisecond)
			t2 := c.Now()

			svr2 := server.MustNew(net.ParseIP("2.2.2.2"), 10480, 10481)
			svr2.UpdateDiscoveryStatus(ds.Details | ds.Info)
			svr2.Refresh(c.Now())
			repo.Add(ctx, svr2, repositories.ServerOnConflictIgnore) // nolint: errcheck

			c.Advance(time.Millisecond)
			t3 := c.Now()

			svr3 := server.MustNew(net.ParseIP("3.3.3.3"), 10480, 10481)
			svr3.UpdateDiscoveryStatus(ds.Master | ds.Details | ds.Info)
			svr3.Refresh(c.Now())
			repo.Add(ctx, svr3, repositories.ServerOnConflictIgnore) // nolint: errcheck

			c.Advance(time.Millisecond)
			t4 := c.Now()

			svr4 := server.MustNew(net.ParseIP("4.4.4.4"), 10480, 10481)
			svr4.UpdateDiscoveryStatus(ds.NoDetails | ds.NoPort)
			repo.Add(ctx, svr4, repositories.ServerOnConflictIgnore) // nolint: errcheck

			c.Advance(time.Millisecond)
			t5 := c.Now()

			svr5 := server.MustNew(net.ParseIP("5.5.5.5"), 10480, 10481)
			svr5.UpdateDiscoveryStatus(ds.NoPort | ds.PortRetry)
			repo.Add(ctx, svr5, repositories.ServerOnConflictIgnore) // nolint: errcheck

			c.Advance(time.Millisecond)

			gotServers, err := repo.Filter(ctx, tt.fsfactory(c.Now(), t1, t2, t3, t4, t5))
			require.NoError(t, err)
			testutils.AssertServers(t, tt.wantServers, gotServers)
		})
	}
}

func TestServerMemoryRepo_Filter_Empty(t *testing.T) {
	tests := []struct {
		name      string
		fsfactory func(time.Time) repositories.ServerFilterSet
	}{
		{
			"default filterset",
			func(time.Time) repositories.ServerFilterSet { return repositories.NewServerFilterSet() },
		},
		{
			"filter by no status",
			func(time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().NoStatus(ds.Master)
			},
		},
		{
			"filter by status",
			func(time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().WithStatus(ds.Master)
			},
		},
		{
			"filter by status and update date",
			func(t time.Time) repositories.ServerFilterSet {
				return repositories.NewServerFilterSet().WithStatus(ds.Master).UpdatedBefore(t)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			c := clockwork.NewFakeClock()
			repo := servers.New(c)

			items, err := repo.Filter(ctx, tt.fsfactory(time.Now()))
			require.NoError(t, err)
			assert.Equal(t, []server.Server{}, items)
		})
	}
}
