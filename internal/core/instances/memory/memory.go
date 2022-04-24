package memory

import (
	"context"
	"sync"

	"github.com/sergeii/swat4master/internal/core/instances"
	"github.com/sergeii/swat4master/internal/entity/addr"
)

type Repository struct {
	ids   map[string]instances.Instance
	addrs map[addr.Addr]instances.Instance
	mutex sync.RWMutex
}

func New() *Repository {
	repo := &Repository{
		ids:   make(map[string]instances.Instance),
		addrs: make(map[addr.Addr]instances.Instance),
	}
	return repo
}

func (r *Repository) Add(ctx context.Context, instance instances.Instance) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	// check whether a server with this address has been reported under different instance id
	prev, exists := r.addrs[instance.GetAddr()]
	// if so, remove the older instance
	if exists {
		delete(r.ids, prev.GetID())
	}
	r.ids[instance.GetID()] = instance
	r.addrs[instance.GetAddr()] = instance
	return nil
}

func (r *Repository) RemoveByID(ctx context.Context, id string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	instance, exists := r.ids[id]
	if !exists {
		return nil
	}
	delete(r.addrs, instance.GetAddr())
	delete(r.ids, id)
	return nil
}

func (r *Repository) RemoveByAddr(ctx context.Context, insAddr addr.Addr) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	instance, exists := r.addrs[insAddr]
	if !exists {
		return nil
	}
	delete(r.ids, instance.GetID())
	delete(r.addrs, insAddr)
	return nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (instances.Instance, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	instance, exists := r.ids[id]
	if !exists {
		return instances.Blank, instances.ErrInstanceNotFound
	}
	return instance, nil
}

func (r *Repository) GetByAddr(ctx context.Context, insAddr addr.Addr) (instances.Instance, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	instance, exists := r.addrs[insAddr]
	if !exists {
		return instances.Blank, instances.ErrInstanceNotFound
	}
	return instance, nil
}

func (r *Repository) Count(ctx context.Context) (int, error) {
	return len(r.ids), nil
}
