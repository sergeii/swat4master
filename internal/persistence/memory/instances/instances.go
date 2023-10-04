package instances

import (
	"context"
	"sync"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/entities/instance"
	"github.com/sergeii/swat4master/internal/core/repositories"
)

type Repository struct {
	ids   map[string]instance.Instance
	addrs map[addr.Addr]instance.Instance
	mutex sync.RWMutex
}

func New() *Repository {
	repo := &Repository{
		ids:   make(map[string]instance.Instance),
		addrs: make(map[addr.Addr]instance.Instance),
	}
	return repo
}

func (r *Repository) Add(_ context.Context, instance instance.Instance) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	// check whether a server with this address has been reported under different instance id
	prev, exists := r.addrs[instance.Addr]
	// if so, remove the older instance
	if exists {
		delete(r.ids, prev.ID)
	}
	r.ids[instance.ID] = instance
	r.addrs[instance.Addr] = instance
	return nil
}

func (r *Repository) RemoveByID(_ context.Context, id string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	instance, exists := r.ids[id]
	if !exists {
		return nil
	}
	delete(r.addrs, instance.Addr)
	delete(r.ids, id)
	return nil
}

func (r *Repository) RemoveByAddr(_ context.Context, insAddr addr.Addr) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	instance, exists := r.addrs[insAddr]
	if !exists {
		return nil
	}
	delete(r.ids, instance.ID)
	delete(r.addrs, insAddr)
	return nil
}

func (r *Repository) GetByID(_ context.Context, id string) (instance.Instance, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	ins, exists := r.ids[id]
	if !exists {
		return instance.Blank, repositories.ErrInstanceNotFound
	}
	return ins, nil
}

func (r *Repository) GetByAddr(_ context.Context, insAddr addr.Addr) (instance.Instance, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	ins, exists := r.addrs[insAddr]
	if !exists {
		return instance.Blank, repositories.ErrInstanceNotFound
	}
	return ins, nil
}

func (r *Repository) Count(_ context.Context) (int, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return len(r.ids), nil
}
