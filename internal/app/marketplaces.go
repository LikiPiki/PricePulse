package app

import (
	"os"

	"github.com/LikiPiki/PricePulse/marketplaces/ozon"
	"github.com/LikiPiki/PricePulse/marketplaces/registry"
)

// NewMarketplaceRegistry is the composition root for supported adapters.
// Add production adapters here explicitly as they are implemented.
func NewMarketplaceRegistry() (*registry.Registry, error) {
	adapter, err := ozon.New(os.Getenv("OZON_PRICE_MODE"))
	if err != nil {
		return nil, err
	}
	return registry.New(adapter)
}
