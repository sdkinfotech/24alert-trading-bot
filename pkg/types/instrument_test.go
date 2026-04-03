package types

import "testing"

func TestNewInstrumentIdentifier(t *testing.T) {
	ii := NewInstrumentIdentifier("uid1", "figi1", "SBER", "TQBR")
	if ii.UID != "uid1" || ii.FIGI != "figi1" || ii.Ticker != "SBER" || ii.ClassCode != "TQBR" {
		t.Fatalf("unexpected fields: %+v", ii)
	}
}

func TestInstrumentClassChecks(t *testing.T) {
	tests := []struct {
		classCode  string
		wantType   string
		wantStock  bool
		wantFuture bool
		wantOption bool
		wantBond   bool
		wantCurr   bool
		wantETF    bool
	}{
		{"TQBR", "stock", true, false, false, false, false, false},
		{"SPYF", "stock", true, false, false, false, false, false},
		{"SPBFUT", "future", false, true, false, false, false, false},
		{"SPBOPT", "option", false, false, true, false, false, false},
		{"TQOB", "bond", false, false, false, true, false, false},
		{"CURS", "currency", false, false, false, false, true, false},
		{"TQTF", "etf", false, false, false, false, false, true},
		{"SOMETHING", "unknown", false, false, false, false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.classCode, func(t *testing.T) {
			ii := NewInstrumentIdentifier("", "", "", tt.classCode)
			if ii.IsStock() != tt.wantStock {
				t.Errorf("IsStock() = %v, want %v", ii.IsStock(), tt.wantStock)
			}
			if ii.IsFuture() != tt.wantFuture {
				t.Errorf("IsFuture() = %v, want %v", ii.IsFuture(), tt.wantFuture)
			}
			if ii.IsOption() != tt.wantOption {
				t.Errorf("IsOption() = %v, want %v", ii.IsOption(), tt.wantOption)
			}
			if ii.IsBond() != tt.wantBond {
				t.Errorf("IsBond() = %v, want %v", ii.IsBond(), tt.wantBond)
			}
			if ii.IsCurrency() != tt.wantCurr {
				t.Errorf("IsCurrency() = %v, want %v", ii.IsCurrency(), tt.wantCurr)
			}
			if ii.IsETF() != tt.wantETF {
				t.Errorf("IsETF() = %v, want %v", ii.IsETF(), tt.wantETF)
			}
			if got := ii.InstrumentType(); got != tt.wantType {
				t.Errorf("InstrumentType() = %q, want %q", got, tt.wantType)
			}
		})
	}
}

func TestPrimaryIdentifier(t *testing.T) {
	tests := []struct {
		name string
		uid  string
		figi string
		tick string
		want string
	}{
		{"uid present", "uid1", "figi1", "SBER", "uid1"},
		{"no uid, figi present", "", "figi1", "SBER", "figi1"},
		{"only ticker", "", "", "SBER", "SBER"},
		{"all empty", "", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ii := NewInstrumentIdentifier(tt.uid, tt.figi, tt.tick, "")
			if got := ii.PrimaryIdentifier(); got != tt.want {
				t.Errorf("PrimaryIdentifier() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInstrumentString(t *testing.T) {
	tests := []struct {
		name string
		uid  string
		figi string
		tick string
		want string
	}{
		{"ticker present", "uid1", "figi1", "SBER", "SBER"},
		{"no ticker, uid present", "uid1", "figi1", "", "uid1"},
		{"no ticker no uid", "", "figi1", "", "figi1"},
		{"all empty", "", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ii := NewInstrumentIdentifier(tt.uid, tt.figi, tt.tick, "")
			if got := ii.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
