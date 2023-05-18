package memory

import (
	"container/list"
	"context"
	"sync"
	"time"

	"github.com/benbjohnson/clock"

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
	clock   clock.Clock
	mutex   sync.RWMutex
}

func New(c clock.Clock) *Repository {
	repo := &Repository{
		servers: make(map[addr.Addr]*list.Element),
		history: list.New(),
		clock:   c,
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
			UpdatedAt: mr.clock.Now(),
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
	item.UpdatedAt = mr.clock.Now()

	mr.history.MoveToFront(elem)

	return svr, nil
}

func (mr *Repository) Remove(
	_ context.Context,
	svr servers.Server,
	onConflict func(*servers.Server) bool,
) error {
	mr.mutex.Lock()
	defer mr.mutex.Unlock()

	key := svr.GetAddr()
	elem, exists := mr.servers[key]
	if !exists {
		return nil
	}

	item := elem.Value.(*server) // nolint: forcetypeassert
	// don't allow to remove servers with version greater than provided
	if item.Server.GetVersion() > svr.GetVersion() {
		// let the caller resolve the conflict
		if !onConflict(&item.Server) {
			return nil
		}
	}

	delete(mr.servers, key)
	mr.history.Remove(elem)

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

func (mr *Repository) Filter( // nolint: cyclop
	_ context.Context,
	fs servers.FilterSet,
) ([]servers.Server, error) {
	mr.mutex.RLock()
	defer mr.mutex.RUnlock()

	activeBefore, byActiveBefore := fs.GetActiveBefore()
	activeAfter, byActiveAfter := fs.GetActiveAfter()
	updatedBefore, byUpdatedBefore := fs.GetUpdatedBefore()
	updatedAfter, byUpdatedAfter := fs.GetUpdatedAfter()
	withStatus, byWithStatus := fs.GetWithStatus()
	noStatus, byNoStatus := fs.GetNoStatus()

	// make the slice with enough size to fit all servers
	recent := make([]servers.Server, 0, len(mr.servers))
	for item := mr.history.Front(); item != nil; item = item.Next() {
		rep := item.Value.(*server) // nolint: forcetypeassert
		updatedAt := rep.UpdatedAt
		refreshedAt := rep.Server.GetRefreshedAt()
		if byUpdatedAfter && updatedAt.Before(updatedAfter) {
			// because servers in the list are sorted by update date
			// we can safely break here, as no more items would satisfy this condition
			break
		}
		switch {
		case byUpdatedBefore && !updatedAt.Before(updatedBefore):
			continue
		case byActiveAfter && (refreshedAt.IsZero() || refreshedAt.Before(activeAfter)):
			continue
		case byActiveBefore && (refreshedAt.IsZero() || !refreshedAt.Before(activeBefore)):
			continue
		case byWithStatus && !rep.Server.HasDiscoveryStatus(withStatus):
			continue
		case byNoStatus && !rep.Server.HasNoDiscoveryStatus(noStatus):
			continue
		}
		recent = append(recent, rep.Server)
	}

	return recent, nil
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
