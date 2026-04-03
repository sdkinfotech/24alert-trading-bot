package idempotency

import (
	"strings"
	"testing"
)

func TestNewOrderID_Format(t *testing.T) {
	gen := NewOrderIDGenerator()
	id := gen.NewOrderID()

	if len(id) != 36 {
		t.Errorf("NewOrderID() length = %d, want 36", len(id))
	}

	// UUID v4 format: 8-4-4-4-12
	parts := strings.Split(id, "-")
	if len(parts) != 5 {
		t.Errorf("NewOrderID() has %d dash-separated parts, want 5", len(parts))
	}
}

func TestNewOrderID_Unique(t *testing.T) {
	gen := NewOrderIDGenerator()
	seen := make(map[string]bool)

	for i := 0; i < 1000; i++ {
		id := gen.NewOrderID()
		if seen[id] {
			t.Fatalf("duplicate ID at iteration %d: %s", i, id)
		}
		seen[id] = true
	}
}

func TestIsValidOrderID(t *testing.T) {
	gen := NewOrderIDGenerator()

	tests := []struct {
		name  string
		id    string
		valid bool
	}{
		{"valid uuid", gen.NewOrderID(), true},
		{"empty", "", false},
		{"too long", strings.Repeat("a", 37), false},
		{"not uuid", "not-a-valid-uuid-string", false},
		{"random string", "abc123", false},
		{"almost uuid", "12345678-1234-1234-1234-12345678901x", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gen.IsValidOrderID(tt.id)
			if got != tt.valid {
				t.Errorf("IsValidOrderID(%q) = %v, want %v", tt.id, got, tt.valid)
			}
		})
	}
}
