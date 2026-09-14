package main

import (
	"log/slog"
	"os"

	"github.com/LikiPiki/PricePulse/internal/app"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	marketplaces, err := app.NewMarketplaceRegistry()
	if err != nil {
		logger.Error("initialize marketplace registry", "error", err)
		os.Exit(1)
	}

	// The next iteration will load tracked offers from storage and create snapshots.
	logger.Info("price collection run completed", "marketplaces", marketplaces.Codes())
}
