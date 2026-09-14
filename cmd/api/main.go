package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
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

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /v1/marketplaces", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": marketplaces.Codes()})
	})

	addr := ":8080"
	logger.Info("API server started", "address", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Error("API server stopped", "error", err)
		os.Exit(1)
	}
}
