package strategy

import (
	"fmt"
	"sync"
)

// Factory builds a new strategy instance (stateless constructor).
type Factory func() (Strategy, error)

// Registry maps strategy type names to constructors.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]Factory),
	}
}

// Register adds a built-in strategy factory.
func (r *Registry) Register(name string, f Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = f
}

// Create instantiates a strategy by type name.
func (r *Registry) Create(name string) (Strategy, error) {
	r.mu.RLock()
	f, ok := r.factories[name]
	r.mu.RUnlock()
	if !ok || f == nil {
		return nil, fmt.Errorf("unknown strategy type %q", name)
	}
	return f()
}
