package server_test

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/entity/details"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
	"github.com/sergeii/swat4master/internal/persistence/memory"
	"github.com/sergeii/swat4master/internal/services/server"
	"github.com/sergeii/swat4master/internal/testutils"
	"github.com/sergeii/swat4master/pkg/gamespy/browsing/query"
)

func makeApp(tb fxtest.TB, extra ...fx.Option) {
	fxopts := []fx.Option{
		fx.Provide(clock.New),
		fx.Provide(memory.New),
		fx.Provide(
			server.NewService,
		),
		fx.NopLogger,
	}
	fxopts = append(fxopts, extra...)
	app := fxtest.New(tb, fxopts...)
	app.RequireStart().RequireStop()
}

func overrideClock(c clock.Clock) fx.Option {
	return fx.Decorate(
		func() clock.Clock {
			return c
		},
	)
}

func TestServerService_Create_OK(t *testing.T) {
	ctx := context.TODO()

	var repo servers.Repository
	var service *server.Service
	makeApp(t, fx.Populate(&repo, &service))

	svr := servers.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	svr, err := service.Create(ctx, svr, func(s *servers.Server) bool {
		return false
	})
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1", svr.GetDottedIP())
	assert.Equal(t, 10480, svr.GetGamePort())
	assert.Equal(t, 10481, svr.GetQueryPort())
	assert.Equal(t, 0, svr.GetVersion())
}

func TestServerService_Create_IgnoreConflict(t *testing.T) {
	ctx := context.TODO()

	var repo servers.Repository
	var service *server.Service
	makeApp(t, fx.Populate(&repo, &service))

	inserted := make(chan struct{})
	ignored := make(chan struct{})
	svr := servers.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)

	go func(s servers.Server) {
		<-inserted
		_, err := service.Create(ctx, s, func(s *servers.Server) bool {
			return false
		})
		require.ErrorIs(t, err, servers.ErrServerExists)
		close(ignored)
	}(svr)

	svr, err := service.Create(ctx, svr, func(s *servers.Server) bool {
		return false
	})
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1", svr.GetDottedIP())
	assert.Equal(t, 0, svr.GetVersion())
	close(inserted)

	<-ignored
	svr, err = repo.Get(ctx, svr.GetAddr())
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1", svr.GetDottedIP())
	assert.Equal(t, 0, svr.GetVersion())
}

func TestServerService_Create_ResolveConflict(t *testing.T) {
	ctx := context.TODO()

	var repo servers.Repository
	var service *server.Service
	makeApp(t, fx.Populate(&repo, &service))

	inserted := make(chan struct{})
	resolved := make(chan struct{})
	svr := servers.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)

	go func(s servers.Server) {
		<-inserted
		svr, err := service.Create(ctx, s, func(s *servers.Server) bool {
			s.UpdateDiscoveryStatus(ds.Master)
			return true
		})
		require.NoError(t, err)
		assert.Equal(t, "1.1.1.1", svr.GetDottedIP())
		assert.Equal(t, 1, svr.GetVersion())
		close(resolved)
	}(svr)

	svr, err := service.Create(ctx, svr, func(s *servers.Server) bool {
		return false
	})
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1", svr.GetDottedIP())
	assert.Equal(t, 0, svr.GetVersion())
	close(inserted)

	<-resolved
	svr, err = repo.Get(ctx, svr.GetAddr())
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1", svr.GetDottedIP())
	assert.Equal(t, 1, svr.GetVersion())
	assert.True(t, svr.HasDiscoveryStatus(ds.Master))
}

func TestServerService_Update_OK(t *testing.T) {
	ctx := context.TODO()

	var repo servers.Repository
	var service *server.Service
	makeApp(t, fx.Populate(&repo, &service))

	svr := servers.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	repo.Add(ctx, svr, servers.OnConflictIgnore) // nolint: errcheck
	assert.Equal(t, ds.New, svr.GetDiscoveryStatus())

	svr.UpdateDiscoveryStatus(ds.Master)
	svr, err := service.Update(ctx, svr, func(s *servers.Server) bool {
		return false
	})
	require.NoError(t, err)
	assert.Equal(t, 1, svr.GetVersion())
	assert.Equal(t, ds.Master, svr.GetDiscoveryStatus())
}

func TestServerService_Update_IgnoreConflict(t *testing.T) {
	ctx := context.TODO()

	var repo servers.Repository
	var service *server.Service
	makeApp(t, fx.Populate(&repo, &service))

	svr := servers.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	repo.Add(ctx, svr, servers.OnConflictIgnore) // nolint: errcheck
	assert.Equal(t, ds.New, svr.GetDiscoveryStatus())

	ignored := make(chan struct{})
	updated := make(chan struct{})

	go func(s servers.Server) {
		<-updated
		s.UpdateDiscoveryStatus(ds.Master)
		svr, err := service.Update(ctx, s, func(s *servers.Server) bool {
			return false
		})
		require.NoError(t, err)
		assert.Equal(t, ds.Details, svr.GetDiscoveryStatus())
		assert.Equal(t, 1, svr.GetVersion())
		close(ignored)
	}(svr)

	svr.UpdateDiscoveryStatus(ds.Details)
	svr, err := service.Update(ctx, svr, func(s *servers.Server) bool {
		return false
	})
	require.NoError(t, err)
	assert.Equal(t, 1, svr.GetVersion())
	assert.Equal(t, ds.Details, svr.GetDiscoveryStatus())
	close(updated)

	<-ignored
	svr, err = repo.Get(ctx, svr.GetAddr())
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1", svr.GetDottedIP())
	assert.Equal(t, 1, svr.GetVersion())
	assert.True(t, svr.HasDiscoveryStatus(ds.Details))
}

func TestServerService_Update_ResolveConflict(t *testing.T) {
	ctx := context.TODO()

	var repo servers.Repository
	var service *server.Service
	makeApp(t, fx.Populate(&repo, &service))

	svr := servers.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	repo.Add(ctx, svr, servers.OnConflictIgnore) // nolint: errcheck
	assert.Equal(t, ds.New, svr.GetDiscoveryStatus())

	resolved := make(chan struct{})
	updated := make(chan struct{})

	go func(s servers.Server) {
		<-updated
		s.UpdateDiscoveryStatus(ds.Master) // not applied due to conflict
		svr, err := service.Update(ctx, s, func(s *servers.Server) bool {
			s.UpdateDiscoveryStatus(ds.Port)
			return true
		})
		require.NoError(t, err)
		assert.Equal(t, ds.Details|ds.Port, svr.GetDiscoveryStatus())
		assert.Equal(t, 2, svr.GetVersion())
		close(resolved)
	}(svr)

	svr.UpdateDiscoveryStatus(ds.Details)
	svr, err := service.Update(ctx, svr, func(s *servers.Server) bool {
		return false
	})
	require.NoError(t, err)
	assert.Equal(t, 1, svr.GetVersion())
	assert.Equal(t, ds.Details, svr.GetDiscoveryStatus())
	close(updated)

	<-resolved
	svr, err = repo.Get(ctx, svr.GetAddr())
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1", svr.GetDottedIP())
	assert.Equal(t, 2, svr.GetVersion())
	assert.Equal(t, ds.Details|ds.Port, svr.GetDiscoveryStatus())
}

func TestServerService_CreateOrUpdate_Created(t *testing.T) {
	ctx := context.TODO()

	var repo servers.Repository
	var service *server.Service
	makeApp(t, fx.Populate(&repo, &service))

	svr := servers.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	svr.UpdateDiscoveryStatus(ds.Master)
	svr, err := service.CreateOrUpdate(ctx, svr, func(s *servers.Server) {
		panic("should not be called")
	})
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1", svr.GetDottedIP())
	assert.Equal(t, 10480, svr.GetGamePort())
	assert.Equal(t, 10481, svr.GetQueryPort())
	assert.Equal(t, 0, svr.GetVersion())
	assert.Equal(t, ds.Master, svr.GetDiscoveryStatus())
}

func TestServerService_CreateOrUpdate_Updated(t *testing.T) {
	ctx := context.TODO()

	var repo servers.Repository
	var service *server.Service
	makeApp(t, fx.Populate(&repo, &service))

	svr := servers.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	svr.UpdateDiscoveryStatus(ds.Master)
	repo.Add(ctx, svr, servers.OnConflictIgnore) // nolint: errcheck

	svr.UpdateDiscoveryStatus(ds.Details)
	svr, err := service.CreateOrUpdate(ctx, svr, func(s *servers.Server) {
		panic("should not be called")
	})
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1", svr.GetDottedIP())
	assert.Equal(t, 10480, svr.GetGamePort())
	assert.Equal(t, 10481, svr.GetQueryPort())
	assert.Equal(t, 1, svr.GetVersion())
	assert.Equal(t, ds.Master|ds.Details, svr.GetDiscoveryStatus())
}

func TestServerService_CreateOrUpdate_Race(t *testing.T) {
	ctx := context.TODO()

	var repo servers.Repository
	var service *server.Service
	makeApp(t, fx.Populate(&repo, &service))

	svr := servers.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	repo.Add(ctx, svr, servers.OnConflictIgnore) // nolint: errcheck

	wg := &sync.WaitGroup{}
	update := func(t *testing.T, svr servers.Server, status ds.DiscoveryStatus) {
		defer wg.Done()
		svr.UpdateDiscoveryStatus(status)
		svr, err := service.CreateOrUpdate(ctx, svr, func(s *servers.Server) {
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

	svr, _ = repo.Get(ctx, svr.GetAddr())
	assert.Equal(t, ds.Master|ds.Info|ds.Details|ds.Port, svr.GetDiscoveryStatus())
	assert.Equal(t, 4, svr.GetVersion())
}

func TestServerService_Get(t *testing.T) {
	ctx := context.TODO()

	var repo servers.Repository
	var service *server.Service
	makeApp(t, fx.Populate(&repo, &service))

	svr := servers.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	repo.Add(ctx, svr, servers.OnConflictIgnore) // nolint: errcheck

	svr, err := service.Get(ctx, svr.GetAddr())
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1", svr.GetDottedIP())
	assert.Equal(t, 10480, svr.GetGamePort())
	assert.Equal(t, 10481, svr.GetQueryPort())
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
			var repo servers.Repository
			var service *server.Service

			ctx := context.TODO()
			clockMock := clock.NewMock()

			makeApp(t, fx.Populate(&repo, &service), overrideClock(clockMock))

			svr1, _ := servers.New(net.ParseIP("1.1.1.1"), 10480, 10481)
			svr1.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
				"hostname":    "Swat4 Server",
				"hostport":    "10480",
				"mapname":     "A-Bomb Nightclub",
				"gamever":     "1.1",
				"gamevariant": "SWAT 4",
				"gametype":    "VIP Escort",
			}), clockMock.Now())
			svr1.UpdateDiscoveryStatus(ds.Master | ds.Info)
			svr1, _ = repo.Add(ctx, svr1, servers.OnConflictIgnore)

			clockMock.Add(time.Millisecond * 10)

			svr2, _ := servers.New(net.ParseIP("2.2.2.2"), 10480, 10481)
			svr2.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
				"hostname":    "Another Swat4 Server",
				"hostport":    "10480",
				"mapname":     "A-Bomb Nightclub",
				"gamever":     "1.0",
				"gamevariant": "SWAT 4",
				"gametype":    "Barricaded Suspects",
				"numplayers":  "12",
				"maxplayers":  "16",
			}), clockMock.Now())
			svr2.UpdateDiscoveryStatus(ds.Details | ds.Info)
			svr2, _ = repo.Add(ctx, svr2, servers.OnConflictIgnore)

			clockMock.Add(time.Millisecond * 10)

			svr3, _ := servers.New(net.ParseIP("3.3.3.3"), 10480, 10481)
			svr3.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
				"hostname":    "Awesome Swat4 Server",
				"hostport":    "10480",
				"mapname":     "A-Bomb Nightclub",
				"gamever":     "1.0",
				"gamevariant": "SWAT 4X",
				"gametype":    "Smash And Grab",
				"numplayers":  "1",
				"maxplayers":  "10",
			}), clockMock.Now())
			svr3.UpdateDiscoveryStatus(ds.Master | ds.Details | ds.Info)
			svr3, _ = repo.Add(ctx, svr3, servers.OnConflictIgnore)

			svr4, _ := servers.New(net.ParseIP("4.4.4.4"), 10480, 10481)
			svr4.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
				"hostname":    "Other Server",
				"hostport":    "10480",
				"mapname":     "A-Bomb Nightclub",
				"gamever":     "1.0",
				"gamevariant": "SWAT 4",
				"gametype":    "VIP Escort",
				"numplayers":  "14",
				"maxplayers":  "16",
			}), clockMock.Now())
			svr4.UpdateDiscoveryStatus(ds.Master)
			svr4, _ = repo.Add(ctx, svr4, servers.OnConflictIgnore)

			clockMock.Add(time.Millisecond * 10)

			filtered, err := service.FilterRecent(ctx, tt.recentness, tt.q, tt.status)
			require.NoError(t, err)
			testutils.AssertServers(t, tt.wantServers, filtered)
		})
	}
}
