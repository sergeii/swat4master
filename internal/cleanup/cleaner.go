package cleanup

import (
	"context"
	"sync"
)

type Cleaner interface {
	Clean(ctx context.Context)
}

type Manager struct {
	mutex    sync.Mutex
	cleaners []Cleaner
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) AddCleaner(c Cleaner) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.cleaners = append(m.cleaners, c)
}

func (m *Manager) Clean(ctx context.Context) {
	for _, c := range m.cleaners {
		go c.Clean(ctx)
	}
}
