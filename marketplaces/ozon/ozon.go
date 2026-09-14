// Package ozon reads public product HTML. The selector is experimental: Ozon can
// return an antibot challenge or change its undocumented page state at any time.
package ozon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/LikiPiki/PricePulse/internal/domain"
	"golang.org/x/net/html"
)

const PriceSelector = `div[id^="state-webPrice-"][data-state]`

var ErrBlocked = errors.New("ozon: access blocked by antibot; no price collected")
var ErrMarkup = errors.New("ozon: missing or ambiguous product/price state")
var productPath = regexp.MustCompile(`^/product/(?:[^/]+-)?([1-9][0-9]{0,18})/?$`)
var productID = regexp.MustCompile(`^[1-9][0-9]{0,18}$`)
var money = regexp.MustCompile(`^(?:[0-9]+|[0-9]{1,3}(?: [0-9]{3})+)(?:[,.][0-9]{1,2})?$`)

type Adapter struct {
	client  *http.Client
	mode    string
	baseURL string
}

// mode is standard or card. No fallback between them: that would mix conditions.
func New(mode string) (*Adapter, error) {
	if mode == "" {
		mode = "standard"
	}
	if mode != "standard" && mode != "card" {
		return nil, errors.New("OZON_PRICE_MODE must be standard or card")
	}
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Timeout: 20 * time.Second,
		Jar:     jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 || req.URL.Scheme != "https" || req.URL.Host != "www.ozon.ru" {
				return ErrBlocked
			}
			return nil
		},
	}

	return &Adapter{mode: mode, baseURL: "https://www.ozon.ru", client: client}, nil
}

func (a *Adapter) Code() string {
	return "ozon"
}

func (a *Adapter) ResolveLink(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || (u.Host != "ozon.ru" && u.Host != "www.ozon.ru") {
		return "", false
	}
	m := productPath.FindStringSubmatch(u.Path)
	if m == nil {
		return "", false
	}
	return m[1], true
}

func (a *Adapter) GetOffer(ctx context.Context, id string) (domain.Offer, error) {
	if !productID.MatchString(id) {
		return domain.Offer{}, errors.New("ozon: invalid product ID")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/product/"+id+"/", nil)
	if err != nil {
		return domain.Offer{}, err
	}
	req.Header.Set("User-Agent", "PricePulse/0.1 (+https://github.com/LikiPiki/PricePulse)")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9")
	resp, err := a.client.Do(req)
	if err != nil {
		if errors.Is(err, ErrBlocked) {
			return domain.Offer{}, ErrBlocked
		}
		return domain.Offer{}, errors.New("ozon: HTTP request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 403 || resp.StatusCode == 429 {
		return domain.Offer{}, ErrBlocked
	}
	if resp.StatusCode != 200 {
		return domain.Offer{}, fmt.Errorf("ozon: HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024+1))
	if err != nil {
		return domain.Offer{}, err
	}
	if len(b) > 8*1024*1024 {
		return domain.Offer{}, errors.New("ozon: response too large")
	}
	return Parse(bytes.NewReader(b), id, a.mode, time.Now().UTC())
}

// Parse only accepts a price state tied to the expected canonical product URL.
// Hash-based CSS classes, crossed-out prices, installments and recommendations
// deliberately have no role in price extraction.
func Parse(r io.Reader, id, mode string, at time.Time) (domain.Offer, error) {
	if !productID.MatchString(id) || (mode != "standard" && mode != "card") {
		return domain.Offer{}, ErrMarkup
	}
	doc, err := html.Parse(r)
	if err != nil {
		return domain.Offer{}, err
	}
	var title, canonical string
	var states []string
	blocked := false
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			attrs := map[string]string{}
			for _, a := range n.Attr {
				attrs[a.Key] = a.Val
			}
			if n.Data == "title" {
				s := strings.ToLower(nodeText(n))
				if strings.Contains(s, "captcha") || strings.Contains(s, "доступ ограничен") || strings.Contains(s, "access denied") {
					blocked = true
				}
			}
			if n.Data == "h1" {
				title = strings.TrimSpace(nodeText(n))
			}
			if n.Data == "link" && attrs["rel"] == "canonical" {
				canonical = attrs["href"]
			}
			if n.Data == "div" && strings.HasPrefix(attrs["id"], "state-webPrice-") && attrs["data-state"] != "" {
				states = append(states, attrs["data-state"])
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if blocked {
		return domain.Offer{}, ErrBlocked
	}
	a := &Adapter{}
	canonicalID, ok := a.ResolveLink(canonical)
	if !ok || canonicalID != id || title == "" || len(states) == 0 {
		return domain.Offer{}, ErrMarkup
	}
	var chosen *priceState
	for _, raw := range states {
		var p priceState
		if err = json.Unmarshal([]byte(raw), &p); err != nil || p.IsAvailable == nil {
			return domain.Offer{}, ErrMarkup
		}
		// Repeated desktop/mobile widgets are accepted only when their values agree.
		if chosen != nil && (chosen.Price != p.Price || chosen.CardPrice != p.CardPrice || *chosen.IsAvailable != *p.IsAvailable) {
			return domain.Offer{}, ErrMarkup
		}
		chosen = &p
	}
	o := domain.Offer{
		ExternalID:  id,
		URL:         "https://www.ozon.ru/product/" + id + "/",
		Title:       title,
		Currency:    "RUB",
		Available:   *chosen.IsAvailable,
		CollectedAt: at,
		Context:     "ozon;price=" + mode + ";region=anonymous-default;seller=unverified;variant=" + id,
	}
	if o.Available {
		value := chosen.Price
		if mode == "card" {
			value = chosen.CardPrice
		}
		o.PriceMinor, err = ParsePrice(value)
		if err != nil {
			return domain.Offer{}, fmt.Errorf("ozon: %w", err)
		}
	}
	if err = domain.ValidateOffer(o, id); err != nil {
		return domain.Offer{}, err
	}
	return o, nil
}

type priceState struct {
	Price       string `json:"price"`
	CardPrice   string `json:"cardPrice"`
	IsAvailable *bool  `json:"isAvailable"`
}

func nodeText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(nodeText(c))
	}
	return b.String()
}

// ParsePrice converts decimal rubles exactly, rejecting installments and ranges.
func ParsePrice(raw string) (int64, error) {
	s := strings.TrimSpace(strings.NewReplacer("\u00a0", " ", "\u202f", " ").Replace(raw))
	s = strings.TrimSpace(strings.TrimSuffix(s, "₽"))
	if !money.MatchString(s) {
		return 0, errors.New("invalid ruble price")
	}
	s = strings.ReplaceAll(strings.ReplaceAll(s, " ", ""), ",", ".")
	parts := strings.Split(s, ".")
	fraction := "00"
	if len(parts) == 2 {
		fraction = parts[1]
		if len(fraction) == 1 {
			fraction += "0"
		}
	}
	value, err := strconv.ParseInt(parts[0]+fraction, 10, 64)
	if err != nil || value <= 0 {
		return 0, errors.New("invalid or overflowing ruble price")
	}
	return value, nil
}
