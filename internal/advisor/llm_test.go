package advisor

import "testing"

func TestParseAnalysisJSON(t *testing.T) {
	raw := `{"summary_md":"Рынок в балансе","structured":{"market_regime":"balance","conclusions":["цена у VWAP"],"next_watch":["пробой"],"confidence":0.7}}`
	out, err := parseAnalysisJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if out.SummaryMD == "" {
		t.Fatal("empty summary")
	}
	if len(out.Structured.Conclusions) != 1 {
		t.Fatalf("conclusions=%v", out.Structured.Conclusions)
	}
}

func TestParseAnalysisJSONWithFence(t *testing.T) {
	raw := "Here is JSON:\n```json\n{\"summary_md\":\"ok\",\"structured\":{\"conclusions\":[\"a\"]}}\n```"
	out, err := parseAnalysisJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if out.SummaryMD != "ok" {
		t.Fatalf("summary=%q", out.SummaryMD)
	}
}
