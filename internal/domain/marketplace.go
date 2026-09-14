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
}
