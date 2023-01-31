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

func (mr *Repository) Add(
	ctx context.Context,
	svr servers.Server,
	onConflict func(*servers.Server) bool,
) (servers.Server, error) {
	mr.mutex.Lock()
	defer mr.mutex.Unlock()

	var item *server

	key := svr.GetAddr()
	elem, exists := mr.servers[key]
	if !exists {
		item = &server{
			Server:    svr,
			UpdatedAt: time.Now(),
		}
		elem = mr.history.PushFront(item)
		mr.servers[key] = elem
		return svr, nil
	}

	// in case the server already exists
	// let the caller decide what to do with the conflict
	item = elem.Value.(*server) // nolint: forcetypeassert
	resolved := item.Server
	if !onConflict(&resolved) {
		// in case the caller has decided not to resolve the conflict, return error
		return servers.Blank, servers.ErrServerExists
	}
	svr = resolved
	return mr.update(ctx, elem, item, svr)
}

func (mr *Repository) Update(
	ctx context.Context,
	svr servers.Server,
	onConflict func(*servers.Server) bool,
) (servers.Server, error) {
	mr.mutex.Lock()
	defer mr.mutex.Unlock()

	key := svr.GetAddr()
	elem, exists := mr.servers[key]
	if !exists {
		return servers.Blank, servers.ErrServerNotFound
	}

	item := elem.Value.(*server) // nolint: forcetypeassert

	// only allow writes when the updated server's version
	// does not exceed the version of current saved version in the repository
	if item.Server.GetVersion() > svr.GetVersion() {
		log.Debug().
			Stringer("server", svr).
			Int("ours", svr.GetVersion()).
			Int("theirs", item.Server.GetVersion()).
			Msg("Got version conflict saving server")
		resolved := item.Server
		// let the caller resolve the conflict
		if !onConflict(&resolved) {
			// return the newer version of the server
			// in case the caller has decided not to resolve the conflict
			return item.Server, nil
		}
		svr = resolved
	}

	return mr.update(ctx, elem, item, svr)
}

func (mr *Repository) update(
	_ context.Context,
	elem *list.Element,
	item *server,
	svr servers.Server,
) (servers.Server, error) {
	// bump the version counter
	// so this version of the server instance
	// maybe be only rewritten when other writers
	// are acknowledged with the changes
	svr.IncVersion()

	item.Server = svr
	item.UpdatedAt = time.Now()

	mr.history.MoveToFront(elem)

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

func (mr *Repository) Get(_ context.Context, addr addr.Addr) (servers.Server, error) {
	mr.mutex.RLock()
	defer mr.mutex.RUnlock()
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
