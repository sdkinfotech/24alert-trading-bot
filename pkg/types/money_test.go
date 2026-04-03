package types

import (
	"math"
	"testing"
)

func TestQuotationToFloat64(t *testing.T) {
	tests := []struct {
		name string
		q    *Quotation
		want float64
	}{
		{"positive", &Quotation{Units: 100, Nano: 500_000_000}, 100.5},
		{"zero", &Quotation{Units: 0, Nano: 0}, 0},
		{"negative units", &Quotation{Units: -50, Nano: 0}, -50},
		{"units only", &Quotation{Units: 42, Nano: 0}, 42},
		{"nano only", &Quotation{Units: 0, Nano: 250_000_000}, 0.25},
		{"nil", nil, 0},
		{"large value", &Quotation{Units: 999_999_999, Nano: 999_999_999}, 999_999_999.999999999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := QuotationToFloat64(tt.q)
			if math.Abs(got-tt.want) > 1e-6 {
				t.Errorf("QuotationToFloat64(%+v) = %v, want %v", tt.q, got, tt.want)
			}
		})
	}
}

func TestFloat64ToQuotation(t *testing.T) {
	tests := []struct {
		name      string
		f         float64
		wantUnits int64
		wantNano  int32
	}{
		{"positive", 100.5, 100, 500_000_000},
		{"zero", 0, 0, 0},
		{"negative", -50.0, -50, 0},
		{"integer", 42.0, 42, 0},
		{"fractional", 0.25, 0, 250_000_000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Float64ToQuotation(tt.f)
			if got.Units != tt.wantUnits || got.Nano != tt.wantNano {
				t.Errorf("Float64ToQuotation(%v) = {%d, %d}, want {%d, %d}",
					tt.f, got.Units, got.Nano, tt.wantUnits, tt.wantNano)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	cases := []Quotation{
		{Units: 100, Nano: 500_000_000},
		{Units: 0, Nano: 0},
		{Units: 1, Nano: 1},
		{Units: 999, Nano: 999_000_000},
	}

	for _, orig := range cases {
		f := QuotationToFloat64(&orig)
		back := Float64ToQuotation(f)
		if math.Abs(QuotationToFloat64(back)-QuotationToFloat64(&orig)) > 1e-6 {
			t.Errorf("round-trip mismatch: orig=%+v → %v → %+v", orig, f, back)
		}
	}
}

func TestMoneyValueToFloat64(t *testing.T) {
	tests := []struct {
		name string
		m    *MoneyValue
		want float64
	}{
		{"nil money", nil, 0},
		{"nil value", &MoneyValue{Currency: "RUB", Value: nil}, 0},
		{"normal", &MoneyValue{Currency: "USD", Value: &Quotation{Units: 10, Nano: 500_000_000}}, 10.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MoneyValueToFloat64(tt.m)
			if math.Abs(got-tt.want) > 1e-6 {
				t.Errorf("MoneyValueToFloat64() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatPrice(t *testing.T) {
	tests := []struct {
		name string
		q    *Quotation
		want string
	}{
		{"nil", nil, "0"},
		{"zero", &Quotation{Units: 0, Nano: 0}, "0.000000000"},
		{"normal", &Quotation{Units: 100, Nano: 500_000_000}, "100.500000000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatPrice(tt.q)
			if got != tt.want {
				t.Errorf("FormatPrice() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCompareQuotations(t *testing.T) {
	tests := []struct {
		name string
		a, b *Quotation
		want int
	}{
		{"equal", &Quotation{10, 0}, &Quotation{10, 0}, 0},
		{"a < b", &Quotation{10, 0}, &Quotation{20, 0}, -1},
		{"a > b", &Quotation{20, 0}, &Quotation{10, 0}, 1},
		{"nano diff", &Quotation{10, 100_000_000}, &Quotation{10, 200_000_000}, -1},
		{"nil vs zero", nil, &Quotation{0, 0}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareQuotations(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("CompareQuotations() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestAddQuotations(t *testing.T) {
	a := &Quotation{Units: 10, Nano: 500_000_000}
	b := &Quotation{Units: 5, Nano: 300_000_000}
	result := AddQuotations(a, b)
	got := QuotationToFloat64(result)
	want := 15.8
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("AddQuotations() = %v, want %v", got, want)
	}
}

func TestSubtractQuotations(t *testing.T) {
	a := &Quotation{Units: 10, Nano: 500_000_000}
	b := &Quotation{Units: 5, Nano: 300_000_000}
	result := SubtractQuotations(a, b)
	got := QuotationToFloat64(result)
	want := 5.2
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("SubtractQuotations() = %v, want %v", got, want)
	}
}

func TestMultiplyQuotation(t *testing.T) {
	tests := []struct {
		name   string
		q      *Quotation
		scalar float64
		want   float64
	}{
		{"double", &Quotation{10, 0}, 2.0, 20.0},
		{"half", &Quotation{10, 0}, 0.5, 5.0},
		{"zero scalar", &Quotation{10, 0}, 0.0, 0.0},
		{"nil quotation", nil, 2.0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MultiplyQuotation(tt.q, tt.scalar)
			got := QuotationToFloat64(result)
			if math.Abs(got-tt.want) > 1e-6 {
				t.Errorf("MultiplyQuotation() = %v, want %v", got, tt.want)
			}
		})
	}
}
