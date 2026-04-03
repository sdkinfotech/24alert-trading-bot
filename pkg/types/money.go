package types

import (
	"fmt"
	"math"
)

// Quotation represents a price with units and fractional nano parts
// This is a placeholder - actual Quotation comes from generated proto files
type Quotation struct {
	Units int64
	Nano  int32
}

// MoneyValue represents an amount with currency
type MoneyValue struct {
	Currency string
	Value    *Quotation
}

// QuotationToFloat64 converts a Quotation to a float64 price
func QuotationToFloat64(q *Quotation) float64 {
	if q == nil {
		return 0
	}
	return float64(q.Units) + float64(q.Nano)/1e9
}

// Float64ToQuotation converts a float64 price to a Quotation
func Float64ToQuotation(f float64) *Quotation {
	units := int64(math.Floor(f))
	nano := int32(math.Round((f - float64(units)) * 1e9))

	// Handle floating point precision
	if nano >= 1e9 {
		units++
		nano = 0
	}
	if nano < 0 {
		units--
		nano = 0
	}

	return &Quotation{
		Units: units,
		Nano:  nano,
	}
}

// MoneyValueToFloat64 extracts the numeric value from a MoneyValue
func MoneyValueToFloat64(m *MoneyValue) float64 {
	if m == nil || m.Value == nil {
		return 0
	}
	return QuotationToFloat64(m.Value)
}

// NewMoneyValue creates a new MoneyValue from currency and float64 amount
func NewMoneyValue(currency string, amount float64) *MoneyValue {
	return &MoneyValue{
		Currency: currency,
		Value:    Float64ToQuotation(amount),
	}
}

// FormatPrice formats a Quotation as a readable price string
func FormatPrice(q *Quotation) string {
	if q == nil {
		return "0"
	}
	return fmt.Sprintf("%d.%09d", q.Units, q.Nano)
}

// FormatMoney formats a MoneyValue with currency
func FormatMoney(m *MoneyValue) string {
	if m == nil {
		return "0 (unknown)"
	}
	return fmt.Sprintf("%.2f %s", QuotationToFloat64(m.Value), m.Currency)
}

// CompareQuotations returns -1 if a < b, 0 if a == b, 1 if a > b
func CompareQuotations(a, b *Quotation) int {
	aVal := QuotationToFloat64(a)
	bVal := QuotationToFloat64(b)

	if aVal < bVal {
		return -1
	}
	if aVal > bVal {
		return 1
	}
	return 0
}

// AddQuotations adds two quotations
func AddQuotations(a, b *Quotation) *Quotation {
	result := QuotationToFloat64(a) + QuotationToFloat64(b)
	return Float64ToQuotation(result)
}

// SubtractQuotations subtracts b from a
func SubtractQuotations(a, b *Quotation) *Quotation {
	result := QuotationToFloat64(a) - QuotationToFloat64(b)
	return Float64ToQuotation(result)
}

// MultiplyQuotation multiplies a quotation by a scalar
func MultiplyQuotation(q *Quotation, scalar float64) *Quotation {
	result := QuotationToFloat64(q) * scalar
	return Float64ToQuotation(result)
}
