// Package domain contains business types and contracts independent from delivery
// and infrastructure details.
package domain

import (
	"context"
	"time"
)

// Marketplace is an adapter for one external marketplace.
// Implementations must translate provider-specific data to Offer.
type Marketplace interface {
	Code() string
	GetOffer(ctx context.Context, externalID string) (Offer, error)
}

// LinkResolver recognizes canonical product links without fetching user URLs.
type LinkResolver interface {
	ResolveLink(rawURL string) (externalID string, ok bool)
}

// Offer is a normalized offer returned by a marketplace.
// PriceMinor is stored in the smallest unit of Currency (kopecks for RUB).
type Offer struct {
	ExternalID  string    `json:"external_id"`
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	PriceMinor  int64     `json:"price_minor"`
	Currency    string    `json:"currency"`
	Available   bool      `json:"available"`
	CollectedAt time.Time `json:"collected_at"`
	// Context describes price conditions and any unverified identity dimensions.
	// Anonymous public pages do not guarantee a fixed seller or delivery region.
	Context string `json:"context"`
}
