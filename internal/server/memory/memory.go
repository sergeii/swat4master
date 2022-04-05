package memory

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/internal/aggregate"
	"github.com/sergeii/swat4master/internal/server"
	"github.com/sergeii/swat4master/internal/server/memory/cleaner"
)

type reportedServer struct {
	Server     *aggregate.GameServer
	InstanceID string    // most recent instance id for this server
	ReportedAt time.Time // most recent report time, either with a heartbeat or a keepalive message
}

type Repository struct {
	servers   map[string]*reportedServer // ip:port -> server mapping, just to keep the server set unique
	instances map[string]string          // instance id -> ip:port mapping
	mutex     sync.RWMutex
}

type Option func(repo *Repository)

func WithCleaner(ctx context.Context, interval time.Duration, retention time.Duration) Option {
	return func(mr *Repository) {
		worker := cleaner.New(mr, interval, retention)
		worker.Run(ctx)
	}
}

func New(options ...Option) *Repository {
	repo := &Repository{
		servers:   make(map[string]*reportedServer),
		instances: make(map[string]string),
	}
	for _, opt := range options {
		opt(repo)
	}
	return repo
}

func (mr *Repository) Report(gs *aggregate.GameServer, instanceID string, params map[string]string) error {
	mr.mutex.Lock()
	defer mr.mutex.Unlock()
	// first check whether this instance has already been reported
	serverMappingKey := gs.GetAddr()
	repSvr, ok := mr.servers[serverMappingKey]
	if !ok {
		repSvr = &reportedServer{
			Server:     gs,
			InstanceID: instanceID,
		}
		mr.servers[serverMappingKey] = repSvr
	} else if repSvr.InstanceID != instanceID {
		// in case the server has been reported earlier with a different instance id
		// remove the obsolete instance link and replace it with the most recent one
		delete(mr.instances, repSvr.InstanceID)
		repSvr.InstanceID = instanceID
	}
	// keep instance id -> server relation, so the latter can be looked up with just the instance id
	mr.instances[instanceID] = serverMappingKey

	repSvr.ReportedAt = time.Now()
	repSvr.Server.Update(params)
	return nil
}

func (mr *Repository) Renew(serverIP string, instanceID string) error {
	mr.mutex.Lock()
	defer mr.mutex.Unlock()
	serverKey, ok := mr.instances[instanceID]
	if !ok {
		return server.ErrServerNotFound
	}
	repSvr, ok := mr.servers[serverKey]
	if !ok || repSvr.Server.GetDottedIP() != serverIP {
		return server.ErrServerNotFound
	}
	repSvr.ReportedAt = time.Now()
	return nil
}

func (mr *Repository) Remove(serverIP string, instanceID string) error {
	mr.mutex.Lock()
	defer mr.mutex.Unlock()
	serverKey, ok := mr.instances[instanceID]
	if !ok {
		return server.ErrServerNotFound
	}
	repSvr, ok := mr.servers[serverKey]
	if !ok || repSvr.Server.GetDottedIP() != serverIP {
		return server.ErrServerNotFound
	}
	delete(mr.servers, serverKey)
	delete(mr.instances, instanceID)
	return nil
}

func (mr *Repository) GetByInstanceID(instanceID string) (*aggregate.GameServer, error) {
	mr.mutex.RLock()
	defer mr.mutex.RUnlock()
	serverKey, ok := mr.instances[instanceID]
	if !ok {
		return nil, server.ErrServerNotFound
	}
	repSvr, ok := mr.servers[serverKey]
	if !ok {
		return nil, server.ErrServerNotFound
	}
	return repSvr.Server, nil
}

func (mr *Repository) GetReportedSince(since time.Time) ([]*aggregate.GameServer, error) {
	mr.mutex.RLock()
	defer mr.mutex.RUnlock()
	// make the slice with enough size to fit all servers
	servers := make([]*aggregate.GameServer, 0, len(mr.servers))
	for _, item := range mr.servers {
		if item.ReportedAt.After(since) {
			servers = append(servers, item.Server)
		}
	}
	return servers, nil
}

func (mr *Repository) Clean(ret time.Duration) {
	mr.mutex.Lock()
	defer mr.mutex.Unlock()

	cleanBefore := time.Now().Add(-ret)
	log.Info().Stringer("before", cleanBefore).Msg("Removing servers reported before date")

	count := 0
	total := 0
	for svrKey, item := range mr.servers {
		if item.ReportedAt.Before(cleanBefore) {
			log.Info().
				Stringer("reported", item.ReportedAt).
				Str("addr", item.Server.GetAddr()).
				Bytes("instance", []byte(item.InstanceID)).
				Msg("Deleting outdated server")
			delete(mr.instances, item.InstanceID)
			delete(mr.servers, svrKey)
			count++
		}
		total++
	}
	log.Info().
		Int("removed", count).Int("total", total).Dur("retention", ret).
		Msg("Finished removing outdated servers")
}
