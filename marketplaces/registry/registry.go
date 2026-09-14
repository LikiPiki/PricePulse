// Package registry holds the explicitly configured marketplace adapters.
package registry

import (
	"fmt"
	"sort"

	"github.com/LikiPiki/PricePulse/internal/domain"
)

type Registry struct {
	items map[string]domain.Marketplace
}

func New(adapters ...domain.Marketplace) (*Registry, error) {
	r := &Registry{items: make(map[string]domain.Marketplace, len(adapters))}
	for _, adapter := range adapters {
		if err := r.Register(adapter); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (r *Registry) Register(adapter domain.Marketplace) error {
	if adapter == nil || adapter.Code() == "" {
		return fmt.Errorf("marketplace adapter must have a code")
	}
	if _, exists := r.items[adapter.Code()]; exists {
		return fmt.Errorf("marketplace %q is already registered", adapter.Code())
	}
	r.items[adapter.Code()] = adapter
	return nil
}

func (r *Registry) Get(code string) (domain.Marketplace, bool) {
	adapter, ok := r.items[code]
	return adapter, ok
}

func (r *Registry) Codes() []string {
	codes := make([]string, 0, len(r.items))
	for code := range r.items {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}
