package strategy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/24alert/trading-bot/pkg/config"
)

func TestManagementHandlerStatusAndFlattenRoutes(t *testing.T) {
	r := &Runner{
		byID: make(map[string]config.StrategyInstanceConfig),
	}
	// minimal runner — status works for unknown id via byID lookup failure
	h := NewManagementHandler(context.Background(), r)

	t.Run("status unknown", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/instances/missing/status", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("flatten not running", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/instances/missing/flatten", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusConflict && rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestFlattenResultJSON(t *testing.T) {
	b, err := json.Marshal(FlattenResult{Status: "ok", OrdersSubmitted: 1, InstanceID: "x"})
	if err != nil {
		t.Fatal(err)
	}
	var out FlattenResult
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.OrdersSubmitted != 1 {
		t.Fatalf("got %+v", out)
	}
}
