package domain

import "sync"

// DefaultRegistry is a thread-safe registry of collectors.
type DefaultRegistry struct {
	mu         sync.RWMutex
	collectors []Collector
}

// NewRegistry creates a new CollectorRegistry.
func NewRegistry() *DefaultRegistry {
	return &DefaultRegistry{}
}

// Register adds a collector.
func (r *DefaultRegistry) Register(c Collector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.collectors = append(r.collectors, c)
}

// Collectors returns a copy of the registered collectors.
func (r *DefaultRegistry) Collectors() []Collector {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Collector, len(r.collectors))
	copy(result, r.collectors)
	return result
}
