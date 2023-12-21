package server_test

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/sergeii/swat4master/internal/core/entities/details"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	repos "github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/persistence/memory"
	ss "github.com/sergeii/swat4master/internal/services/server"
	"github.com/sergeii/swat4master/internal/testutils"
	"github.com/sergeii/swat4master/pkg/gamespy/browsing/query"
)

func makeApp(tb fxtest.TB, extra ...fx.Option) {
	fxopts := []fx.Option{
		fx.Provide(clockwork.NewRealClock),
		fx.Provide(func(c clockwork.Clock) (repos.ServerRepository, repos.InstanceRepository, repos.ProbeRepository) {
			mem := memory.New(c)
			return mem.Servers, mem.Instances, mem.Probes
		}),
		fx.Provide(
			ss.NewService,
		),
		fx.NopLogger,
	}
	fxopts = append(fxopts, extra...)
	app := fxtest.New(tb, fxopts...)
	app.RequireStart().RequireStop()
}

func overrideClock(c clockwork.Clock) fx.Option {
	return fx.Decorate(
		func() clockwork.Clock {
			return c
		},
	)
}

func TestServerService_Update_OK(t *testing.T) {
	ctx := context.TODO()

	var repo repos.ServerRepository
	var service *ss.Service
	makeApp(t, fx.Populate(&repo, &service))

	svr := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	repo.Add(ctx, svr, repos.ServerOnConflictIgnore) // nolint: errcheck
	assert.Equal(t, ds.New, svr.DiscoveryStatus)

	svr.UpdateDiscoveryStatus(ds.Master)
	svr, err := service.Update(ctx, svr, func(s *server.Server) bool {
		return false
	})
	require.NoError(t, err)
	assert.Equal(t, 1, svr.Version)
	assert.Equal(t, ds.Master, svr.DiscoveryStatus)
}

func TestServerService_Update_IgnoreConflict(t *testing.T) {
	ctx := context.TODO()

	var repo repos.ServerRepository
	var service *ss.Service
	makeApp(t, fx.Populate(&repo, &service))

	svr := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	repo.Add(ctx, svr, repos.ServerOnConflictIgnore) // nolint: errcheck
	assert.Equal(t, ds.New, svr.DiscoveryStatus)

	ignored := make(chan struct{})
	updated := make(chan struct{})

	go func(s server.Server) {
		<-updated
		s.UpdateDiscoveryStatus(ds.Master)
		svr, err := service.Update(ctx, s, func(s *server.Server) bool {
			return false
		})
		require.NoError(t, err)
		assert.Equal(t, ds.Details, svr.DiscoveryStatus)
		assert.Equal(t, 1, svr.Version)
		close(ignored)
	}(svr)

	svr.UpdateDiscoveryStatus(ds.Details)
	svr, err := service.Update(ctx, svr, func(s *server.Server) bool {
		return false
	})
	require.NoError(t, err)
	assert.Equal(t, 1, svr.Version)
	assert.Equal(t, ds.Details, svr.DiscoveryStatus)
	close(updated)

	<-ignored
	svr, err = repo.Get(ctx, svr.Addr)
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1", svr.Addr.GetDottedIP())
	assert.Equal(t, 1, svr.Version)
	assert.True(t, svr.HasDiscoveryStatus(ds.Details))
}

func TestServerService_Update_ResolveConflict(t *testing.T) {
	ctx := context.TODO()

	var repo repos.ServerRepository
	var service *ss.Service
	makeApp(t, fx.Populate(&repo, &service))

	svr := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	repo.Add(ctx, svr, repos.ServerOnConflictIgnore) // nolint: errcheck
	assert.Equal(t, ds.New, svr.DiscoveryStatus)

	resolved := make(chan struct{})
	updated := make(chan struct{})

	go func(s server.Server) {
		<-updated
		s.UpdateDiscoveryStatus(ds.Master) // not applied due to conflict
		svr, err := service.Update(ctx, s, func(s *server.Server) bool {
			s.UpdateDiscoveryStatus(ds.Port)
			return true
		})
		require.NoError(t, err)
		assert.Equal(t, ds.Details|ds.Port, svr.DiscoveryStatus)
		assert.Equal(t, 2, svr.Version)
		close(resolved)
	}(svr)

	svr.UpdateDiscoveryStatus(ds.Details)
	svr, err := service.Update(ctx, svr, func(s *server.Server) bool {
		return false
	})
	require.NoError(t, err)
	assert.Equal(t, 1, svr.Version)
	assert.Equal(t, ds.Details, svr.DiscoveryStatus)
	close(updated)

	<-resolved
	svr, err = repo.Get(ctx, svr.Addr)
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1", svr.Addr.GetDottedIP())
	assert.Equal(t, 2, svr.Version)
	assert.Equal(t, ds.Details|ds.Port, svr.DiscoveryStatus)
}

func TestServerService_CreateOrUpdate_Created(t *testing.T) {
	ctx := context.TODO()

	var repo repos.ServerRepository
	var service *ss.Service
	makeApp(t, fx.Populate(&repo, &service))

	svr := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	svr.UpdateDiscoveryStatus(ds.Master)
	svr, err := service.CreateOrUpdate(ctx, svr, func(s *server.Server) {
		panic("should not be called")
	})
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1", svr.Addr.GetDottedIP())
	assert.Equal(t, 10480, svr.Addr.Port)
	assert.Equal(t, 10481, svr.QueryPort)
	assert.Equal(t, 0, svr.Version)
	assert.Equal(t, ds.Master, svr.DiscoveryStatus)
}

func TestServerService_CreateOrUpdate_Updated(t *testing.T) {
	ctx := context.TODO()

	var repo repos.ServerRepository
	var service *ss.Service
	makeApp(t, fx.Populate(&repo, &service))

	svr := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	svr.UpdateDiscoveryStatus(ds.Master)
	repo.Add(ctx, svr, repos.ServerOnConflictIgnore) // nolint: errcheck

	svr.UpdateDiscoveryStatus(ds.Details)
	svr, err := service.CreateOrUpdate(ctx, svr, func(s *server.Server) {
		panic("should not be called")
	})
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1", svr.Addr.GetDottedIP())
	assert.Equal(t, 10480, svr.Addr.Port)
	assert.Equal(t, 10481, svr.QueryPort)
	assert.Equal(t, 1, svr.Version)
	assert.Equal(t, ds.Master|ds.Details, svr.DiscoveryStatus)
}

func TestServerService_CreateOrUpdate_Race(t *testing.T) {
	ctx := context.TODO()

	var repo repos.ServerRepository
	var service *ss.Service
	makeApp(t, fx.Populate(&repo, &service))

	svr := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	repo.Add(ctx, svr, repos.ServerOnConflictIgnore) // nolint: errcheck

	wg := &sync.WaitGroup{}
	update := func(t *testing.T, svr server.Server, status ds.DiscoveryStatus) {
		defer wg.Done()
		svr.UpdateDiscoveryStatus(status)
		svr, err := service.CreateOrUpdate(ctx, svr, func(s *server.Server) {
			s.UpdateDiscoveryStatus(status)
		})
		require.NoError(t, err)
		assert.True(t, svr.HasDiscoveryStatus(status))
	}

	wg.Add(4)
	go update(t, svr, ds.Master)
	go update(t, svr, ds.Info)
	go update(t, svr, ds.Details)
	go update(t, svr, ds.Port)
	wg.Wait()

	svr, _ = repo.Get(ctx, svr.Addr)
	assert.Equal(t, ds.Master|ds.Info|ds.Details|ds.Port, svr.DiscoveryStatus)
	assert.Equal(t, 4, svr.Version)
}

func TestServerService_FilterRecent(t *testing.T) {
	tests := []struct {
		recentness  time.Duration
		q           query.Query
		status      ds.DiscoveryStatus
		wantServers []string
	}{
		{
			time.Millisecond * 100,
			query.Blank,
			ds.Master,
			[]string{"4.4.4.4:10480", "3.3.3.3:10480", "1.1.1.1:10480"},
		},
		{
			time.Millisecond * 100,
			query.MustNewFromString("gamevariant='SWAT 4'"),
			ds.Master,
			[]string{"4.4.4.4:10480", "1.1.1.1:10480"},
		},
		{
			time.Millisecond * 100,
			query.MustNewFromString("gamevariant='SWAT 4X'"),
			ds.Master,
			[]string{"3.3.3.3:10480"},
		},
		{
			time.Millisecond * 100,
			query.Blank,
			ds.Details,
			[]string{"3.3.3.3:10480", "2.2.2.2:10480"},
		},
		{
			time.Millisecond * 100,
			query.Blank,
			ds.Port,
			[]string{},
		},
		{
			time.Millisecond * 15,
			query.Blank,
			ds.Master,
			[]string{"4.4.4.4:10480", "3.3.3.3:10480"},
		},
		{
			time.Millisecond * 15,
			query.Blank,
			ds.Master | ds.Details,
			[]string{"3.3.3.3:10480"},
		},
		{
			time.Millisecond * 15,
			query.MustNewFromString("numplayers>10"),
			ds.Master,
			[]string{"4.4.4.4:10480"},
		},
		{
			time.Millisecond * 5,
			query.Blank,
			ds.Master,
			[]string{},
		},
	}

	for _, tt := range tests {
		testname := fmt.Sprintf("%s;%s;%s", tt.recentness, tt.q, tt.status)
		t.Run(testname, func(t *testing.T) {
			var repo repos.ServerRepository
			var service *ss.Service

			ctx := context.TODO()
			c := clockwork.NewFakeClock()
			makeApp(t, fx.Populate(&repo, &service), overrideClock(c))

			svr1 := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
			svr1.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
				"hostname":    "Swat4 Server",
				"hostport":    "10480",
				"mapname":     "A-Bomb Nightclub",
				"gamever":     "1.1",
				"gamevariant": "SWAT 4",
				"gametype":    "VIP Escort",
			}), c.Now())
			svr1.UpdateDiscoveryStatus(ds.Master | ds.Info)
			svr1, _ = repo.Add(ctx, svr1, repos.ServerOnConflictIgnore)

			c.Advance(time.Millisecond * 10)

			svr2 := server.MustNew(net.ParseIP("2.2.2.2"), 10480, 10481)
			svr2.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
				"hostname":    "Another Swat4 Server",
				"hostport":    "10480",
				"mapname":     "A-Bomb Nightclub",
				"gamever":     "1.0",
				"gamevariant": "SWAT 4",
				"gametype":    "Barricaded Suspects",
				"numplayers":  "12",
				"maxplayers":  "16",
			}), c.Now())
			svr2.UpdateDiscoveryStatus(ds.Details | ds.Info)
			svr2, _ = repo.Add(ctx, svr2, repos.ServerOnConflictIgnore)

			c.Advance(time.Millisecond * 10)

			svr3 := server.MustNew(net.ParseIP("3.3.3.3"), 10480, 10481)
			svr3.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
				"hostname":    "Awesome Swat4 Server",
				"hostport":    "10480",
				"mapname":     "A-Bomb Nightclub",
				"gamever":     "1.0",
				"gamevariant": "SWAT 4X",
				"gametype":    "Smash And Grab",
				"numplayers":  "1",
				"maxplayers":  "10",
			}), c.Now())
			svr3.UpdateDiscoveryStatus(ds.Master | ds.Details | ds.Info)
			svr3, _ = repo.Add(ctx, svr3, repos.ServerOnConflictIgnore)

			svr4 := server.MustNew(net.ParseIP("4.4.4.4"), 10480, 10481)
			svr4.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
				"hostname":    "Other Server",
				"hostport":    "10480",
				"mapname":     "A-Bomb Nightclub",
				"gamever":     "1.0",
				"gamevariant": "SWAT 4",
				"gametype":    "VIP Escort",
				"numplayers":  "14",
				"maxplayers":  "16",
			}), c.Now())
			svr4.UpdateDiscoveryStatus(ds.Master)
			svr4, _ = repo.Add(ctx, svr4, repos.ServerOnConflictIgnore)

			c.Advance(time.Millisecond * 10)

			filtered, err := service.FilterRecent(ctx, tt.recentness, tt.q, tt.status)
			require.NoError(t, err)
			testutils.AssertServers(t, tt.wantServers, filtered)
		})
	}
}
