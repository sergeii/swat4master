package servers

import (
	"container/list"
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/filterset"
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
	clock   clockwork.Clock
	mutex   sync.RWMutex
}

func New(clock clockwork.Clock) *Repository {
	repo := &Repository{
		servers: make(map[addr.Addr]*list.Element),
		history: list.New(),
		clock:   clock,
	}
	return repo
}

func (r *Repository) Add(
	ctx context.Context,
	svr server.Server,
	onConflict func(*server.Server) bool,
) (server.Server, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	var item *serverItem

	elem, exists := r.servers[svr.Addr]
	if !exists {
		item = &serverItem{
			Server:    svr,
			UpdatedAt: r.clock.Now(),
		}
		elem = r.history.PushFront(item)
		r.servers[svr.Addr] = elem
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
	return r.update(ctx, elem, item, svr)
}

func (r *Repository) Update(
	ctx context.Context,
	svr server.Server,
	onConflict func(*server.Server) bool,
) (server.Server, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	elem, exists := r.servers[svr.Addr]
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

	return r.update(ctx, elem, item, svr)
}

func (r *Repository) update(
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
	item.UpdatedAt = r.clock.Now()

	r.history.MoveToFront(elem)

	return svr, nil
}

func (r *Repository) AddOrUpdate(
	ctx context.Context,
	svr server.Server,
	onConflictDo func(*server.Server),
) (server.Server, error) {
	repoOnConflict := func(s *server.Server) bool {
		onConflictDo(s)
		// we never want to fail an update
		return true
	}
	if _, err := r.Get(ctx, svr.Addr); err != nil {
		switch {
		case errors.Is(err, repositories.ErrServerNotFound):
			return r.Add(ctx, svr, repoOnConflict)
		default:
			return server.Blank, err
		}
	}
	return r.Update(ctx, svr, repoOnConflict)
}

func (r *Repository) Remove(
	_ context.Context,
	svr server.Server,
	onConflict func(*server.Server) bool,
) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	elem, exists := r.servers[svr.Addr]
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

	delete(r.servers, svr.Addr)
	r.history.Remove(elem)

	return nil
}

func (r *Repository) Get(_ context.Context, addr addr.Addr) (server.Server, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	item, exists := r.servers[addr]
	if !exists {
		return server.Blank, repositories.ErrServerNotFound
	}
	svr := item.Value.(*serverItem) // nolint: forcetypeassert
	return svr.Server, nil
}

func (r *Repository) Filter( // nolint: cyclop
	_ context.Context,
	fs filterset.FilterSet,
) ([]server.Server, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	activeBefore, byActiveBefore := fs.GetActiveBefore()
	activeAfter, byActiveAfter := fs.GetActiveAfter()
	updatedBefore, byUpdatedBefore := fs.GetUpdatedBefore()
	updatedAfter, byUpdatedAfter := fs.GetUpdatedAfter()
	withStatus, byWithStatus := fs.GetWithStatus()
	noStatus, byNoStatus := fs.GetNoStatus()

	// make the slice with enough size to fit all servers
	recent := make([]server.Server, 0, len(r.servers))
	for item := r.history.Front(); item != nil; item = item.Next() {
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
			{
				continue
			}
		}
		recent = append(recent, rep.Server)
	}

	return recent, nil
}

func (r *Repository) Count(context.Context) (int, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return len(r.servers), nil
}

func (r *Repository) CountByStatus(_ context.Context) (map[ds.DiscoveryStatus]int, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var bit ds.DiscoveryStatus
	counter := make(map[ds.DiscoveryStatus]int)

	for item := r.history.Front(); item != nil; item = item.Next() {
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
