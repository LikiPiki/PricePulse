package domain

import "errors"

// ValidateOffer prevents incomplete or invalid observations from entering history.
func ValidateOffer(offer Offer, externalID string) error {
	if externalID == "" || offer.ExternalID != externalID || offer.Title == "" || offer.Context == "" || offer.CollectedAt.IsZero() {
		return errors.New("invalid offer identity, metadata or observation time")
	}
	if len(offer.Title) > 500 || len(offer.URL) > 2000 || len(offer.Context) > 500 {
		return errors.New("offer metadata exceeds size limit")
	}

	// Supported currencies use two fractional digits.
	if offer.Currency != "RUB" && offer.Currency != "USD" && offer.Currency != "EUR" {
		return errors.New("unsupported currency")
	}
	if offer.PriceMinor < 0 || (offer.Available && offer.PriceMinor == 0) {
		return errors.New("invalid price")
	}
	return nil
}
