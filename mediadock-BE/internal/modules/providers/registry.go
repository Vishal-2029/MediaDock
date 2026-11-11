package providers

import (
	"fmt"
	"sync"
)

// Registry selects the appropriate Provider implementation for a submitted URL.
type Registry struct {
	mu        sync.RWMutex
	providers []Provider
	byID      map[PlatformID]Provider
}

// NewRegistry creates an empty Provider Registry.
func NewRegistry() *Registry {
	return &Registry{
		byID: make(map[PlatformID]Provider),
	}
}

// Register adds a provider to the registry in registration order.
// Platform-specific providers should be registered before generic fallback providers.
func (r *Registry) Register(p Provider) error {
	if p == nil {
		return fmt.Errorf("cannot register nil provider")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byID[p.ID()]; exists {
		return fmt.Errorf("provider %q already registered", p.ID())
	}
	r.byID[p.ID()] = p
	r.providers = append(r.providers, p)
	return nil
}

// Resolve selects the matching provider for rawURL.
func (r *Registry) Resolve(rawURL string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, p := range r.providers {
		if p.CanHandle(rawURL) {
			return p, nil
		}
	}
	return nil, ErrUnsupportedURL
}

// GetByID retrieves a registered provider by its PlatformID.
func (r *Registry) GetByID(id PlatformID) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.byID[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedURL, id)
	}
	return p, nil
}

// List returns metadata about all registered providers.
func (r *Registry) List() []PlatformID {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]PlatformID, 0, len(r.providers))
	for _, p := range r.providers {
		out = append(out, p.ID())
	}
	return out
}

