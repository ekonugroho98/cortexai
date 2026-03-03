package service

import (
	"strings"
	"sync"
)

// PGPoolRegistry maps squad IDs to their PostgresService instances.
type PGPoolRegistry struct {
	mu    sync.RWMutex
	pools map[string]*PostgresService
}

// NewPGPoolRegistry creates an empty registry.
func NewPGPoolRegistry() *PGPoolRegistry {
	return &PGPoolRegistry{pools: make(map[string]*PostgresService)}
}

// Register adds a PostgresService for the given squad.
func (r *PGPoolRegistry) Register(squadID string, svc *PostgresService) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pools[squadID] = svc
}

// Get returns the PostgresService for the given squad, or nil if not registered.
func (r *PGPoolRegistry) Get(squadID string) *PostgresService {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.pools[squadID]
}

// CloseAll closes all registered PostgresService instances.
func (r *PGPoolRegistry) CloseAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var errs []string
	for id, svc := range r.pools {
		if err := svc.Close(); err != nil {
			errs = append(errs, id+": "+err.Error())
		}
	}
	r.pools = make(map[string]*PostgresService)
	if len(errs) > 0 {
		return &pgCloseError{msg: strings.Join(errs, "; ")}
	}
	return nil
}

type pgCloseError struct{ msg string }

func (e *pgCloseError) Error() string { return "close pg registries: " + e.msg }
