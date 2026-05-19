package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const instrumentCatalogTTL = 4 * time.Hour

// CatalogInstrument is a MOEX share or futures contract for AI Trader / dashboard pickers.
type CatalogInstrument struct {
	UID            string `json:"uid"`
	Ticker         string `json:"ticker"`
	Name           string `json:"name"`
	ClassCode      string `json:"class_code"`
	InstrumentType string `json:"instrument_type"`
	Exchange       string `json:"exchange"`
	Kind           string `json:"kind"` // share | future
}

type instrumentCatalog struct {
	mu        sync.RWMutex
	fetchedAt time.Time
	items     []CatalogInstrument
}

var globalInstrumentCatalog instrumentCatalog

func gatewayBaseURL() string {
	if u := strings.TrimSpace(os.Getenv("GATEWAY_URL")); u != "" {
		return strings.TrimRight(u, "/")
	}
	return "http://gateway:8080"
}

func (c *instrumentCatalog) ensure(ctx context.Context) error {
	c.mu.RLock()
	stale := len(c.items) == 0 || time.Since(c.fetchedAt) > instrumentCatalogTTL
	c.mu.RUnlock()
	if !stale {
		return nil
	}
	return c.refresh(ctx)
}

func (c *instrumentCatalog) refresh(ctx context.Context) error {
	shares, err := fetchGatewayInstruments(ctx, gatewayBaseURL()+"/api/v1/instruments/shares")
	if err != nil {
		return fmt.Errorf("shares: %w", err)
	}
	futures, err := fetchGatewayInstruments(ctx, gatewayBaseURL()+"/api/v1/instruments/futures")
	if err != nil {
		return fmt.Errorf("futures: %w", err)
	}
	items := make([]CatalogInstrument, 0, len(shares)+len(futures))
	for _, row := range shares {
		items = append(items, row.withKind("share"))
	}
	for _, row := range futures {
		items = append(items, row.withKind("future"))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Ticker == items[j].Ticker {
			return items[i].UID < items[j].UID
		}
		return items[i].Ticker < items[j].Ticker
	})

	c.mu.Lock()
	c.items = items
	c.fetchedAt = time.Now().UTC()
	c.mu.Unlock()
	return nil
}

type gatewayInstrument struct {
	UID            string `json:"uid"`
	Ticker         string `json:"ticker"`
	ClassCode      string `json:"class_code"`
	Name           string `json:"name"`
	Exchange       string `json:"exchange"`
	InstrumentType string `json:"instrument_type"`
}

func (g gatewayInstrument) withKind(kind string) CatalogInstrument {
	return CatalogInstrument{
		UID:            strings.TrimSpace(g.UID),
		Ticker:         strings.TrimSpace(g.Ticker),
		Name:           strings.TrimSpace(g.Name),
		ClassCode:      strings.TrimSpace(g.ClassCode),
		InstrumentType: strings.TrimSpace(g.InstrumentType),
		Exchange:       strings.TrimSpace(g.Exchange),
		Kind:           kind,
	}
}

func fetchGatewayInstruments(ctx context.Context, url string) ([]gatewayInstrument, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gateway %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out []gatewayInstrument
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func filterCatalogInstruments(items []CatalogInstrument, query, kind string, limit int) []CatalogInstrument {
	query = strings.ToLower(strings.TrimSpace(query))
	kind = strings.ToLower(strings.TrimSpace(kind))
	if limit <= 0 {
		limit = 40
	}
	if limit > 200 {
		limit = 200
	}

	var out []CatalogInstrument
	for _, item := range items {
		if kind != "" && kind != "all" && item.Kind != kind {
			continue
		}
		if query != "" && !catalogInstrumentMatches(item, query) {
			continue
		}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func catalogInstrumentMatches(item CatalogInstrument, query string) bool {
	for _, s := range []string{item.Ticker, item.Name, item.UID, item.ClassCode} {
		if strings.Contains(strings.ToLower(s), query) {
			return true
		}
	}
	return false
}

// SearchInstruments returns shares and/or futures from the gateway catalog (cached).
func SearchInstruments(ctx context.Context, query, kind string, limit int) ([]CatalogInstrument, error) {
	if err := globalInstrumentCatalog.ensure(ctx); err != nil {
		return nil, err
	}
	globalInstrumentCatalog.mu.RLock()
	items := append([]CatalogInstrument(nil), globalInstrumentCatalog.items...)
	globalInstrumentCatalog.mu.RUnlock()
	return filterCatalogInstruments(items, query, kind, limit), nil
}
