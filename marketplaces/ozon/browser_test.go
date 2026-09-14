package ozon

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// Opt-in integration test: launches a real browser against a local fixture.
// Normal go test does not require Chrome or make requests to Ozon.
func TestBrowserLoader(t *testing.T) {
	executable := os.Getenv("CHROME_BIN")
	if executable == "" {
		t.Skip("set CHROME_BIN to run the real-browser integration test")
	}

	page := fixture(t)
	tests := []struct {
		name    string
		html    string
		blocked bool
	}{
		{name: "product", html: page},
		{
			name: "dynamic product",
			html: strings.Replace(page, "<body>", `<body><script>
              document.addEventListener('DOMContentLoaded', () => {
                const widget = document.querySelector('[id^="state-webPrice-"]');
                widget.remove();
                setTimeout(() => document.body.appendChild(widget), 250);
              });
            </script>`, 1),
		},
		{name: "antibot", html: "<html><head><title>Antibot Captcha</title></head><body>Challenge</body></html>", blocked: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = io.WriteString(w, test.html)
			}))
			defer server.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			result, err := loadBrowserPage(ctx, server.URL, BrowserOptions{Executable: executable, Headless: true})
			if test.blocked {
				if !errors.Is(err, ErrBlocked) {
					t.Fatalf("expected ErrBlocked, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			offer, err := Parse(strings.NewReader(result), exampleID, "standard", time.Now())
			if err != nil || offer.PriceMinor != 124900 {
				t.Fatalf("rendered offer = %+v, error = %v", offer, err)
			}
		})
	}
}
