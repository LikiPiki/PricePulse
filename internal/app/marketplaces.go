package app

import (
	"github.com/LikiPiki/PricePulse/marketplaces/registry"
)

// NewMarketplaceRegistry is the composition root for supported adapters.
// Add production adapters here explicitly as they are implemented.
func NewMarketplaceRegistry() (*registry.Registry, error) {
	return registry.New()
}
