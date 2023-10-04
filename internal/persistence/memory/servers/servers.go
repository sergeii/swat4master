package servers

import (
	"container/list"
	"context"
	"sync"
	"time"

	"github.com/benbjohnson/clock"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
)

type serverItem struct {
	Server    server.Server
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
	svr server.Server,
	onConflict func(*server.Server) bool,
) (server.Server, error) {
	mr.mutex.Lock()
	defer mr.mutex.Unlock()

	var item *serverItem

	elem, exists := mr.servers[svr.Addr]
	if !exists {
		item = &serverItem{
			Server:    svr,
			UpdatedAt: mr.clock.Now(),
		}
		elem = mr.history.PushFront(item)
		mr.servers[svr.Addr] = elem
		return svr, nil
	}

	// in case the server already exists
	// let the caller decide what to do with the conflict
	item = elem.Value.(*serverItem) // nolint: forcetypeassert
	resolved := item.Server
	if !onConflict(&resolved) {
		// in case the caller has decided not to resolve the conflict, return error
		return server.Blank, repositories.ErrServerExists
	}
	svr = resolved
	return mr.update(ctx, elem, item, svr)
}

func (mr *Repository) Update(
	ctx context.Context,
	svr server.Server,
	onConflict func(*server.Server) bool,
) (server.Server, error) {
	mr.mutex.Lock()
	defer mr.mutex.Unlock()

	elem, exists := mr.servers[svr.Addr]
	if !exists {
		return server.Blank, repositories.ErrServerNotFound
	}

	item := elem.Value.(*serverItem) // nolint: forcetypeassert

	// only allow writes when the updated server's version
	// does not exceed the version of current saved version in the repository
	if item.Server.Version > svr.Version {
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
	item *serverItem,
	svr server.Server,
) (server.Server, error) {
	// bump the version counter
	// so this version of the server instance
	// maybe be only rewritten when other writers
	// are acknowledged with the changes
	svr.Version++

	item.Server = svr
	item.UpdatedAt = mr.clock.Now()

	mr.history.MoveToFront(elem)

	return svr, nil
}

func (mr *Repository) Remove(
	_ context.Context,
	svr server.Server,
	onConflict func(*server.Server) bool,
) error {
	mr.mutex.Lock()
	defer mr.mutex.Unlock()

	elem, exists := mr.servers[svr.Addr]
	if !exists {
		return nil
	}

	item := elem.Value.(*serverItem) // nolint: forcetypeassert
	// don't allow to remove servers with version greater than provided
	if item.Server.Version > svr.Version {
		// let the caller resolve the conflict
		if !onConflict(&item.Server) {
			return nil
		}
	}

	delete(mr.servers, svr.Addr)
	mr.history.Remove(elem)

	return nil
}

func (mr *Repository) Get(_ context.Context, addr addr.Addr) (server.Server, error) {
	mr.mutex.RLock()
	defer mr.mutex.RUnlock()
	item, exists := mr.servers[addr]
	if !exists {
		return server.Blank, repositories.ErrServerNotFound
	}
	svr := item.Value.(*serverItem) // nolint: forcetypeassert
	return svr.Server, nil
}

func (mr *Repository) Filter( // nolint: cyclop
	_ context.Context,
	fs repositories.ServerFilterSet,
) ([]server.Server, error) {
	mr.mutex.RLock()
	defer mr.mutex.RUnlock()

	activeBefore, byActiveBefore := fs.GetActiveBefore()
	activeAfter, byActiveAfter := fs.GetActiveAfter()
	updatedBefore, byUpdatedBefore := fs.GetUpdatedBefore()
	updatedAfter, byUpdatedAfter := fs.GetUpdatedAfter()
	withStatus, byWithStatus := fs.GetWithStatus()
	noStatus, byNoStatus := fs.GetNoStatus()

	// make the slice with enough size to fit all servers
	recent := make([]server.Server, 0, len(mr.servers))
	for item := mr.history.Front(); item != nil; item = item.Next() {
		rep := item.Value.(*serverItem) // nolint: forcetypeassert
		updatedAt := rep.UpdatedAt
		refreshedAt := rep.Server.RefreshedAt
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
		rep := item.Value.(*serverItem) // nolint: forcetypeassert
		status := rep.Server.DiscoveryStatus
		for bit = 1; bit <= status; bit <<= 1 {
			if status&bit == bit {
				counter[bit]++
			}
		}
	}

	return counter, nil
}
