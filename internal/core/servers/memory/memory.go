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

func (mr *Repository) AddOrUpdate(_ context.Context, svr servers.Server) error {
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
	} else {
		mr.history.MoveToFront(item)
		rep = item.Value.(*server) // nolint: forcetypeassert
		rep.Server = svr
		rep.UpdatedAt = time.Now()
	}

	return nil
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

func (mr *Repository) GetByAddr(_ context.Context, addr addr.Addr) (servers.Server, error) {
	mr.mutex.RLock()
	defer mr.mutex.RUnlock()
	item, ok := mr.servers[addr]
	if !ok {
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
		// ignore servers added or updated before the specific date
		if byAfter && rep.UpdatedAt.Before(after) {
			break
		}
		// ignore servers added or updated after the specific date
		if byBefore && rep.UpdatedAt.After(before) {
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
