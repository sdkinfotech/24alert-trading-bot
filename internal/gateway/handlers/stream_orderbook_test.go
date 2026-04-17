package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/24alert/trading-bot/pkg/logging"
)

// TestStreamOrderBook_MissingUIDs verifies that the endpoint returns 400 when
// the required `uids` query parameter is missing.
func TestStreamOrderBook_MissingUIDs(t *testing.T) {
	logger, err := logging.NewLogger("error", "text", "stdout", "")
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	h := NewStreamHandlers(nil, logger)

	r := chi.NewRouter()
	h.Routes(r)

	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/stream/orderbook")
	if err != nil {
		t.Fatalf("http get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

// TestStreamOrderBook_EmptyUIDs verifies that uids=,, (only whitespace/commas)
// is rejected with 400 rather than silently accepted.
func TestStreamOrderBook_EmptyUIDs(t *testing.T) {
	logger, err := logging.NewLogger("error", "text", "stdout", "")
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	h := NewStreamHandlers(nil, logger)

	r := chi.NewRouter()
	h.Routes(r)

	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/stream/orderbook?uids=%20,%20,")
	if err != nil {
		t.Fatalf("http get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 for whitespace-only uids, got %d", resp.StatusCode)
	}
}

// TestStreamOrderBookMsg_SnapshotJSON verifies that the wire JSON shape
// matches the contract expected by traderbook/services/market-data.
func TestStreamOrderBookMsg_SnapshotJSON(t *testing.T) {
	msg := StreamOrderBookMsg{
		Type:  "snapshot",
		UID:   "abcd-1234",
		Depth: 20,
		TS:    1713252200000,
		Bids: []StreamLevel{
			{Price: 28027.5, Quantity: 12540},
		},
		Asks: []StreamLevel{
			{Price: 28029.0, Quantity: 5430},
		},
	}
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)

	requiredFragments := []string{
		`"type":"snapshot"`,
		`"uid":"abcd-1234"`,
		`"depth":20`,
		`"ts":1713252200000`,
		`"price":28027.5`,
		`"quantity":12540`,
		`"price":28029`,
		`"quantity":5430`,
	}
	for _, frag := range requiredFragments {
		if !strings.Contains(got, frag) {
			t.Fatalf("missing %q in %s", frag, got)
		}
	}
}

// TestStreamOrderBookMsg_PingJSON verifies the exact shape of the ping frame
// so that the traderbook client can detect it without surprises.
func TestStreamOrderBookMsg_PingJSON(t *testing.T) {
	msg := StreamOrderBookMsg{Type: "ping"}
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	if got != `{"type":"ping"}` {
		t.Fatalf("unexpected ping json: %s", got)
	}
}
