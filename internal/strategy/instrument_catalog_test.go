package strategy

import "testing"

func TestFilterCatalogInstruments(t *testing.T) {
	items := []CatalogInstrument{
		{UID: "u1", Ticker: "SBER", Name: "Сбер", Kind: "share"},
		{UID: "u2", Ticker: "SiH6", Name: "Silver", Kind: "future"},
		{UID: "u3", Ticker: "BMM6", Name: "Brent", Kind: "future"},
	}
	got := filterCatalogInstruments(items, "bm", "future", 10)
	if len(got) != 1 || got[0].Ticker != "BMM6" {
		t.Fatalf("unexpected filter: %+v", got)
	}
	got = filterCatalogInstruments(items, "sber", "all", 10)
	if len(got) != 1 || got[0].Kind != "share" {
		t.Fatalf("unexpected share filter: %+v", got)
	}
}
