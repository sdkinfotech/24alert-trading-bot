package idempotency

import (
	"github.com/google/uuid"
)

// OrderIDGenerator generates idempotent order IDs
type OrderIDGenerator struct{}

// NewOrderIDGenerator creates a new ID generator
func NewOrderIDGenerator() *OrderIDGenerator {
	return &OrderIDGenerator{}
}

// NewOrderID generates a new UUID v4 for order idempotency
// T-Invest API requires max 36 characters
func (g *OrderIDGenerator) NewOrderID() string {
	return uuid.New().String()
}

// IsValidOrderID checks if the order ID meets T-Invest requirements
func (g *OrderIDGenerator) IsValidOrderID(id string) bool {
	if len(id) > 36 {
		return false
	}
	if len(id) == 0 {
		return false
	}
	// Try to parse as UUID for validation
	_, err := uuid.Parse(id)
	return err == nil
}
