package ozon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/LikiPiki/PricePulse/internal/domain"
)

// BrowserOptions configures a separate Chrome/Chromium process. An empty
// UserDataDir uses a temporary profile, never the user's everyday browser profile.
type BrowserOptions struct {
	Executable  string
	UserDataDir string
	Headless    bool
}

// GetOfferBrowser renders the canonical product page and reuses the HTML parser.
// A browser is an alternative page loader, not a guarantee of antibot acceptance.
func (a *Adapter) GetOfferBrowser(ctx context.Context, id string, options BrowserOptions) (domain.Offer, error) {
	if !productID.MatchString(id) {
		return domain.Offer{}, errors.New("ozon: invalid product ID")
	}

	page, err := loadBrowserPage(ctx, "https://www.ozon.ru/product/"+id+"/", options)
	if err != nil {
		return domain.Offer{}, err
	}
	return Parse(strings.NewReader(page), id, a.mode, time.Now().UTC())
}

func loadBrowserPage(ctx context.Context, link string, options BrowserOptions) (string, error) {
	allocatorOptions := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("headless", options.Headless),
		chromedp.Flag("enable-automation", true),
		// Keep the browser sandbox enabled, including when invoked as root.
		chromedp.Flag("no-sandbox", false),
	}
	if options.Executable != "" {
		allocatorOptions = append(allocatorOptions, chromedp.ExecPath(options.Executable))
	}
	if options.UserDataDir != "" {
		allocatorOptions = append(allocatorOptions, chromedp.UserDataDir(options.UserDataDir))
	}

	allocatorCtx, cancelAllocator := chromedp.NewExecAllocator(ctx, allocatorOptions...)
	defer cancelAllocator()
	browserCtx, cancelBrowser := chromedp.NewContext(allocatorCtx)
	defer cancelBrowser()

	var state, page string
	err := chromedp.Run(browserCtx,
		chromedp.Navigate(link),
		chromedp.Poll(pageStateExpression, &state,
			chromedp.WithPollingInterval(500*time.Millisecond),
			chromedp.WithPollingTimeout(30*time.Second),
		),
		chromedp.OuterHTML("html", &page, chromedp.ByQuery),
	)
	if err != nil {
		if errors.Is(err, chromedp.ErrPollingTimeout) {
			return "", fmt.Errorf("%w: browser did not find the price widget within 30 seconds", ErrMarkup)
		}
		return "", fmt.Errorf("ozon: browser load failed: %w", err)
	}
	if state == "blocked" {
		return "", ErrBlocked
	}
	return page, nil
}

// Wait for asynchronously rendered product state, but stop on a visible challenge.
// No CAPTCHA interaction, fingerprint changes or browser security overrides.
const pageStateExpression = `(() => {
  const title = document.title.toLowerCase();
  if (/captcha|доступ ограничен|access denied|antibot/.test(title)) {
    return "blocked";
  }
  const price = document.querySelector('div[id^="state-webPrice-"][data-state]');
  const canonical = document.querySelector('link[rel="canonical"]');
  const heading = document.querySelector('h1');
  return price && canonical && heading ? "ready" : false;
})()`
