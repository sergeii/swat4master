package memory

import (
	"container/list"
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/entity/addr"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
)

type server struct {
	Server    servers.Server
	UpdatedAt time.Time // most recent report time, either with a heartbeat or a keepalive message
}

type Repository struct {
	servers map[addr.Addr]*list.Element // ip:port -> instance item mapping (wrapped in a linked list element)
	history *list.List                  // history of reported servers with the most recent servers being in the front
	mutex   sync.RWMutex
}

func New() *Repository {
	repo := &Repository{
		servers: make(map[addr.Addr]*list.Element),
		history: list.New(),
	}
	return repo
}

func (mr *Repository) AddOrUpdate(_ context.Context, svr servers.Server) (servers.Server, error) {
	mr.mutex.Lock()
	defer mr.mutex.Unlock()

	var rep *server

	key := svr.GetAddr()
	// first check whether this instance has already been reported
	item, exists := mr.servers[key]
	if !exists {
		rep = &server{
			Server:    svr,
			UpdatedAt: time.Now(),
		}
		item = mr.history.PushFront(rep)
		mr.servers[key] = item
		return svr, nil
	}

	rep = item.Value.(*server) // nolint: forcetypeassert

	// only allow writes when the updated server's version
	// does not exceed the version of current saved version in the repository
	if rep.Server.GetVersion() > svr.GetVersion() {
		log.Warn().
			Stringer("server", svr).
			Int("ours", svr.GetVersion()).
			Int("theirs", rep.Server.GetVersion()).
			Msg("Unable to save server due to version conflict")
		return svr, servers.ErrVersionConflict
	}

	// bump the version counter
	// so this version of the server instance
	// maybe be only rewritten when other writers
	// are acknowledged with the changes
	svr.IncVersion()

	rep.Server = svr
	rep.UpdatedAt = time.Now()

	mr.history.MoveToFront(item)

	return svr, nil
}

func (mr *Repository) Update(
	ctx context.Context,
	unlocker servers.Unlocker,
	svr servers.Server,
) (servers.Server, error) {
	defer unlocker.Unlock()
	return mr.update(ctx, svr)
}

func (mr *Repository) update(_ context.Context, svr servers.Server) (servers.Server, error) {
	key := svr.GetAddr()

	item, exists := mr.servers[key]
	if !exists {
		return servers.Blank, servers.ErrServerNotFound
	}

	rep := item.Value.(*server) // nolint: forcetypeassert

	// only allow writes when the updated server's version
	// does not exceed the version of current saved version in the repository
	if rep.Server.GetVersion() > svr.GetVersion() {
		log.Warn().
			Stringer("server", svr).
			Int("ours", svr.GetVersion()).
			Int("theirs", rep.Server.GetVersion()).
			Msg("Unable to save server due to version conflict")
		return svr, servers.ErrVersionConflict
	}

	// bump the version counter
	// so this version of the server instance
	// maybe be only rewritten when other writers
	// are acknowledged with the changes
	svr.IncVersion()

	rep.Server = svr
	rep.UpdatedAt = time.Now()

	mr.history.MoveToFront(item)

	return svr, nil
}

func (mr *Repository) Remove(_ context.Context, svr servers.Server) error {
	mr.mutex.Lock()
	defer mr.mutex.Unlock()
	key := svr.GetAddr()
	item, ok := mr.servers[key]
	if !ok {
		return nil
	}
	delete(mr.servers, key)
	mr.history.Remove(item)
	return nil
}

func (mr *Repository) Get(ctx context.Context, addr addr.Addr) (servers.Server, error) {
	mr.mutex.RLock()
	defer mr.mutex.RUnlock()
	return mr.get(ctx, addr)
}

func (mr *Repository) GetForUpdate(
	ctx context.Context,
	addr addr.Addr,
) (servers.Server, servers.Unlocker, error) {
	mr.mutex.Lock()
	svr, err := mr.get(ctx, addr)
	if err != nil {
		mr.mutex.Unlock()
		return svr, nil, err
	}
	unlocker := NewUnlocker(&mr.mutex)
	return svr, unlocker, nil
}

func (mr *Repository) get(_ context.Context, addr addr.Addr) (servers.Server, error) {
	item, exists := mr.servers[addr]
	if !exists {
		return servers.Blank, servers.ErrServerNotFound
	}
	svr := item.Value.(*server) // nolint: forcetypeassert
	return svr.Server, nil
}

func (mr *Repository) Filter(_ context.Context, fs servers.FilterSet) ([]servers.Server, error) {
	mr.mutex.RLock()
	defer mr.mutex.RUnlock()

	before, byBefore := fs.GetBefore()
	after, byAfter := fs.GetAfter()
	withStatus, byWithStatus := fs.GetWithStatus()
	noStatus, byNoStatus := fs.GetNoStatus()

	// make the slice with enough size to fit all servers
	recent := make([]servers.Server, 0, len(mr.servers))
	for item := mr.history.Front(); item != nil; item = item.Next() {
		rep := item.Value.(*server) // nolint: forcetypeassert
		refreshedAt := rep.Server.GetRefreshedAt()
		// ignore servers whose info or details were updated before the specific date
		if byAfter && refreshedAt.Before(after) {
			// because servers in the list are sorted by update date
			// we can safely break here, as no more items would satisfy this condition
			break
		}
		// ignore servers added or updated after the specific date
		if byBefore && refreshedAt.After(before) {
			continue
		}
		// filter servers by discovery status
		if byWithStatus && !rep.Server.HasDiscoveryStatus(withStatus) {
			continue
		}
		if byNoStatus && !rep.Server.HasNoDiscoveryStatus(noStatus) {
			continue
		}
		recent = append(recent, rep.Server)
	}

	return recent, nil
}

func (mr *Repository) CleanNext(_ context.Context, before time.Time) (servers.Server, bool) {
	mr.mutex.Lock()
	defer mr.mutex.Unlock()

	oldest := mr.history.Back()
	if oldest == nil {
		return servers.Blank, false
	}

	item := oldest.Value.(*server) // nolint: forcetypeassert
	if item.UpdatedAt.After(before) {
		return servers.Blank, false
	}

	mr.history.Remove(oldest)
	delete(mr.servers, item.Server.GetAddr())

	log.Debug().
		Stringer("updated", item.UpdatedAt).
		Stringer("addr", item.Server.GetAddr()).
		Msg("Removed outdated server")

	return item.Server, true
}

func (mr *Repository) Count(context.Context) (int, error) {
	mr.mutex.RLock()
	defer mr.mutex.RUnlock()
	return len(mr.servers), nil
}

func (mr *Repository) CountByStatus(_ context.Context) (map[ds.DiscoveryStatus]int, error) {
	mr.mutex.RLock()
	defer mr.mutex.RUnlock()

	var bit ds.DiscoveryStatus
	counter := make(map[ds.DiscoveryStatus]int)

	for item := mr.history.Front(); item != nil; item = item.Next() {
		rep := item.Value.(*server) // nolint: forcetypeassert
		status := rep.Server.GetDiscoveryStatus()
		for bit = 1; bit <= status; bit <<= 1 {
			if status&bit == bit {
				counter[bit]++
			}
		}
	}

	return counter, nil
}
