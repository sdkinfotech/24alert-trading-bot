package marketdata

import (
	"sync"
	"time"
)

// InstrumentInfo holds instrument metadata.
type InstrumentInfo struct {
	UID            string
	FIGI           string
	Ticker         string
	ClassCode      string
	Name           string
	LotSize        int32
	InstrumentType string  // "share", "future", "bond", etc.
	MinPriceIncr   float64 // minimum price increment (tick size)
	Currency       string  // "rub", "usd", etc.
}

// IsFuture returns true for FORTS futures (ClassCode == "SPBFUT").
func (i InstrumentInfo) IsFuture() bool {
	return i.ClassCode == "SPBFUT"
}

// InstrumentCache is a thread-safe cache for instrument metadata keyed by UID.
type InstrumentCache struct {
	mu    sync.RWMutex
	items map[string]InstrumentInfo
}

func NewInstrumentCache() *InstrumentCache {
	return &InstrumentCache{items: make(map[string]InstrumentInfo)}
}

func (c *InstrumentCache) SetInstrument(info InstrumentInfo) {
	c.mu.Lock()
	c.items[info.UID] = info
	c.mu.Unlock()
}

func (c *InstrumentCache) GetInstrument(uid string) (InstrumentInfo, bool) {
	c.mu.RLock()
	info, ok := c.items[uid]
	c.mu.RUnlock()
	return info, ok
}

// PriceEntry stores a last-known price with a timestamp.
type PriceEntry struct {
	Price     float64
	UpdatedAt time.Time
}

// PriceCache is a thread-safe cache for last prices keyed by instrument UID.
type PriceCache struct {
	mu    sync.RWMutex
	items map[string]PriceEntry
}

func NewPriceCache() *PriceCache {
	return &PriceCache{items: make(map[string]PriceEntry)}
}

func (c *PriceCache) SetLastPrice(uid string, price float64) {
	c.mu.Lock()
	c.items[uid] = PriceEntry{Price: price, UpdatedAt: time.Now()}
	c.mu.Unlock()
}

func (c *PriceCache) GetLastPrice(uid string) (PriceEntry, bool) {
	c.mu.RLock()
	entry, ok := c.items[uid]
	c.mu.RUnlock()
	return entry, ok
}

func (c *PriceCache) GetAllPrices() map[string]PriceEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]PriceEntry, len(c.items))
	for k, v := range c.items {
		out[k] = v
	}
	return out
}
