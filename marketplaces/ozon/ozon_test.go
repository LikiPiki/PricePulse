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

const exampleID = "1463825680"
const exampleURL = "https://www.ozon.ru/product/drip-kofe-sibaristica-mega-drip-max-efiopiya-kolumbiya-gvatemala-nabor-molotogo-kofe-v-drip-paketah-1463825680/?_bctx=CAQQ-9gj&at=1qal1SL8H10MOjCS6tdXl27ORJd8-2cR&hs=1&sh=-zH2O8iJ1Q"

func fixture(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("testdata/1463825680.html")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestResolveLink(t *testing.T) {
	adapter, err := New("standard")
	if err != nil {
		t.Fatal(err)
	}
	for _, link := range []string{exampleURL, "https://ozon.ru/product/1463825680/"} {
		if id, ok := adapter.ResolveLink(link); !ok || id != exampleID {
			t.Fatalf("ResolveLink(%q) = %q, %v", link, id, ok)
		}
	}
	for _, link := range []string{
		"https://ozon.ru.evil.test/product/1463825680/",
		"https://ozon.ru@evil.test/product/1463825680/",
		"http://ozon.ru/product/1463825680/",
		"https://ozon.ru:444/product/1463825680/",
		"https://ozon.ru/product/1463825680/reviews/",
		"https://ozon.ru/product/0/",
		"http://127.0.0.1/product/1463825680/",
	} {
		if _, ok := adapter.ResolveLink(link); ok {
			t.Fatalf("accepted unsupported link %q", link)
		}
	}
}

func TestParseExampleProduct(t *testing.T) {
	at := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	for mode, want := range map[string]int64{"standard": 124900, "card": 109900} {
		t.Run(mode, func(t *testing.T) {
			offer, err := Parse(strings.NewReader(fixture(t)), exampleID, mode, at)
			if err != nil {
				t.Fatal(err)
			}
			if offer.PriceMinor != want || offer.Currency != "RUB" || !offer.Available {
				t.Fatalf("unexpected price: %+v", offer)
			}
			if offer.ExternalID != exampleID || !strings.Contains(offer.Title, "Sibaristica") || offer.CollectedAt != at {
				t.Fatalf("unexpected product metadata: %+v", offer)
			}
			if offer.URL != "https://www.ozon.ru/product/1463825680/" || !strings.Contains(offer.Context, "price="+mode) {
				t.Fatalf("tracking URL or price context not normalized: %+v", offer)
			}
		})
	}
}

func TestParseRejectsAmbiguousOrMissingData(t *testing.T) {
	page := fixture(t)
	tests := map[string]string{
		"missing widget":       strings.ReplaceAll(page, "state-webPrice-", "state-other-"),
		"missing availability": strings.ReplaceAll(page, "&quot;isAvailable&quot;:true,", ""),
		"missing card price":   strings.ReplaceAll(page, "&quot;cardPrice&quot;:&quot;1 099 ₽&quot;,", ""),
		"wrong product":        strings.ReplaceAll(page, exampleID, "12345"),
		"invalid JSON":         strings.ReplaceAll(page, "&quot;isAvailable&quot;:true", "broken-json"),
		"conflicting widgets":  page + `<div id="state-webPrice-999" data-state='{"isAvailable":true,"price":"999 ₽"}'></div>`,
		"missing title":        strings.ReplaceAll(strings.ReplaceAll(page, "<h1>", "<p>"), "</h1>", "</p>"),
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(strings.NewReader(input), exampleID, "card", time.Now()); err == nil {
				t.Fatal("expected an error instead of a guessed price")
			}
		})
	}
	_, err := Parse(strings.NewReader("<title>Antibot Captcha</title>"), exampleID, "standard", time.Now())
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("captcha error = %v", err)
	}
}

func TestUnavailableIsNotAZeroPriceDiscount(t *testing.T) {
	page := strings.ReplaceAll(fixture(t), "&quot;isAvailable&quot;:true", "&quot;isAvailable&quot;:false")
	offer, err := Parse(strings.NewReader(page), exampleID, "standard", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if offer.Available || offer.PriceMinor != 0 {
		t.Fatalf("expected explicitly unavailable observation: %+v", offer)
	}
}

func TestParsePrice(t *testing.T) {
	for input, want := range map[string]int64{
		"1 249 ₽":           124900,
		"1\u00a0249\u00a0₽": 124900,
		"1\u202f249,50 ₽":   124950,
		"1249.5":            124950,
		"0,01 ₽":            1,
	} {
		got, err := ParsePrice(input)
		if err != nil || got != want {
			t.Errorf("ParsePrice(%q) = %d, %v; want %d", input, got, err, want)
		}
	}
	for _, input := range []string{"", "0 ₽", "-100 ₽", "от 100 ₽", "100 ₽ / месяц", "1 2 3 ₽", "1.234 ₽", "1e3", "1,000.00", "9999999999999999999999999 ₽"} {
		if _, err := ParsePrice(input); err == nil {
			t.Errorf("accepted invalid price %q", input)
		}
	}
}

func TestHTTPAdapter(t *testing.T) {
	page := fixture(t)
	for _, status := range []int{200, 403, 429, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/product/1463825680/" || r.URL.RawQuery != "" {
					t.Errorf("unexpected requested URL %s", r.URL)
				}
				w.WriteHeader(status)
				_, _ = io.WriteString(w, page)
			}))
			defer server.Close()
			adapter, _ := New("standard")
			adapter.baseURL = server.URL
			offer, err := adapter.GetOffer(context.Background(), exampleID)
			if status == 200 && (err != nil || offer.PriceMinor != 124900) {
				t.Fatalf("GetOffer() = %+v, %v", offer, err)
			}
			if status != 200 && err == nil {
				t.Fatal("HTTP failure must not produce an offer")
			}
			if (status == 403 || status == 429) && !errors.Is(err, ErrBlocked) {
				t.Fatalf("expected blocked error, got %v", err)
			}
		})
	}
}

func TestRedirectPolicy(t *testing.T) {
	adapter, _ := New("standard")
	for _, destination := range []string{"http://www.ozon.ru/product/1463825680/", "https://evil.test/"} {
		req, _ := http.NewRequest(http.MethodGet, destination, nil)
		if err := adapter.client.CheckRedirect(req, nil); !errors.Is(err, ErrBlocked) {
			t.Fatalf("accepted redirect to %s", destination)
		}
	}
	req, _ := http.NewRequest(http.MethodGet, "https://www.ozon.ru/product/1463825680/", nil)
	if err := adapter.client.CheckRedirect(req, make([]*http.Request, 3)); !errors.Is(err, ErrBlocked) {
		t.Fatal("redirect loop was not stopped")
	}
}
