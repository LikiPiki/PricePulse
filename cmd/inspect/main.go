// Inspect extracts a single Ozon offer without storing or sending anything.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/LikiPiki/PricePulse/internal/domain"
	"github.com/LikiPiki/PricePulse/marketplaces/ozon"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	link := flag.String("url", "", "canonical Ozon product URL")
	file := flag.String("html", "", "optional saved product HTML (offline parsing)")
	mode := flag.String("price", "standard", "standard or card")
	useBrowser := flag.Bool("browser", false, "load the page with Chrome/Chromium via chromedp")
	chrome := flag.String("chrome", "", "Chrome/Chromium executable path (otherwise auto-detected)")
	profile := flag.String("profile", "", "dedicated browser profile directory (default: temporary)")
	headless := flag.Bool("headless", false, "run browser without a visible window")
	flag.Parse()

	if *file != "" && *useBrowser {
		return fmt.Errorf("-html and -browser cannot be combined")
	}
	if !*useBrowser && (*chrome != "" || *profile != "" || *headless) {
		return fmt.Errorf("-chrome, -profile and -headless require -browser")
	}

	adapter, err := ozon.New(*mode)
	if err != nil {
		return err
	}
	id, ok := adapter.ResolveLink(*link)
	if !ok {
		return fmt.Errorf("provide a valid https://www.ozon.ru/product/... URL with -url")
	}

	var offer domain.Offer
	if *file != "" {
		input, openErr := os.Open(*file)
		if openErr != nil {
			return openErr
		}
		defer input.Close()
		offer, err = ozon.Parse(input, id, *mode, time.Now().UTC())
	} else if *useBrowser {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		offer, err = adapter.GetOfferBrowser(ctx, id, ozon.BrowserOptions{
			Executable:  *chrome,
			UserDataDir: *profile,
			Headless:    *headless,
		})
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		offer, err = adapter.GetOffer(ctx, id)
	}
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(offer)
}
