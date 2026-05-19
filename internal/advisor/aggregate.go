package advisor

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// SnapshotPayload is decoded micro snapshot JSON.
type SnapshotPayload struct {
	Features      map[string]any `json:"features"`
	MarketContext map[string]any `json:"market_context"`
	Events        []DecisionEvent `json:"events"`
}

func decodeSnapshotPayload(jsonStr string) SnapshotPayload {
	var p SnapshotPayload
	_ = json.Unmarshal([]byte(jsonStr), &p)
	return p
}

// FactsBundle is deterministic digest for LLM input.
type FactsBundle struct {
	Ticker          string
	PeriodLabel     string
	EventCount      int
	SnapshotCount   int
	SpreadMin       float64
	SpreadMax       float64
	ImbalanceAvg    float64
	TapeSummary     string
	PrintHighlights []string
	TimelineLines   []string
	SceneNotes      []string
	WallChanges     []string
	SpoofHints      []string
	IcebergHints    []string
	ThoughtLines    []string
	TextDigest      string
}

func BuildFactsFromSnapshots(snaps []MicroSnapshot, events []DecisionEvent, ticker string, periodLabel string) FactsBundle {
	fb := FactsBundle{
		Ticker:      ticker,
		PeriodLabel: periodLabel,
	}
	fb.SnapshotCount = len(snaps)
	fb.EventCount = len(events)

	var spreads []float64
	var imbalances []float64
	seenThought := map[string]bool{}

	for _, ev := range events {
		line := strings.TrimSpace(ev.Summary)
		if line == "" {
			line = strings.TrimSpace(ev.Reason)
		}
		if line != "" && !seenThought[line] {
			seenThought[line] = true
			fb.ThoughtLines = append(fb.ThoughtLines, line)
		}
	}

	for _, snap := range snaps {
		p := decodeSnapshotPayload(snap.PayloadJSON)
		if f, ok := p.Features["spread_bps"].(float64); ok {
			spreads = append(spreads, f)
		}
		if im, ok := p.Features["imbalance"].(float64); ok {
			imbalances = append(imbalances, im)
		}
		extractMarketContext(&fb, p.MarketContext)
	}

	if len(spreads) > 0 {
		fb.SpreadMin, fb.SpreadMax = minMax(spreads)
	}
	if len(imbalances) > 0 {
		fb.ImbalanceAvg = avg(imbalances)
	}

	fb.TextDigest = formatFactsDigest(fb)
	return fb
}

func extractMarketContext(fb *FactsBundle, mc map[string]any) {
	if mc == nil {
		return
	}
	if ts, ok := mc["tape_stats"].(map[string]any); ok {
		fb.TapeSummary = fmt.Sprintf("trades %.0f buy_vol %.0f sell_vol %.0f delta %.1f%% last %.4f",
			num(ts["trade_count"]), num(ts["buy_volume"]), num(ts["sell_volume"]), num(ts["delta_pct"])*100, num(ts["last_price"]))
	}
	if prints, ok := mc["recent_prints"].([]any); ok {
		for i, p := range prints {
			if i >= 8 {
				break
			}
			pm, _ := p.(map[string]any)
			fb.PrintHighlights = append(fb.PrintHighlights, fmt.Sprintf("%s %.0f @ %.4f", str(pm["side"]), num(pm["quantity"]), num(pm["price"])))
		}
	}
	if tl, ok := mc["book_timeline"].([]any); ok && len(tl) > 0 {
		start := 0
		if len(tl) > 5 {
			start = len(tl) - 5
		}
		for _, row := range tl[start:] {
			rm, _ := row.(map[string]any)
			fb.TimelineLines = append(fb.TimelineLines, fmt.Sprintf("mid %.4f spread %.1f bps imb %.2f bid %s ask %s",
				num(rm["mid"]), num(rm["spread_bps"]), num(rm["imbalance"]), str(rm["bid_wall"]), str(rm["ask_wall"])))
		}
	}
	if notes, ok := mc["scene_notes"].([]any); ok {
		for _, n := range notes {
			fb.SceneNotes = append(fb.SceneNotes, str(n))
		}
	}
	deriveWallSpoofIceberg(fb, mc)
}

// deriveWallSpoofIceberg adds deterministic hints from timeline + scene notes.
func deriveWallSpoofIceberg(fb *FactsBundle, mc map[string]any) {
	tl, ok := mc["book_timeline"].([]any)
	if !ok || len(tl) < 2 {
		return
	}
	type wallSnap struct {
		bidStr, askStr string
	}
	var snaps []wallSnap
	for _, row := range tl {
		rm, _ := row.(map[string]any)
		snaps = append(snaps, wallSnap{bidStr: str(rm["bid_wall"]), askStr: str(rm["ask_wall"])})
	}
	first, last := snaps[0], snaps[len(snaps)-1]
	if first.bidStr != "" && last.bidStr != "" && first.bidStr != last.bidStr {
		fb.WallChanges = append(fb.WallChanges, fmt.Sprintf("bid wall moved: %s → %s", first.bidStr, last.bidStr))
	}
	if first.askStr != "" && last.askStr != "" && first.askStr != last.askStr {
		fb.WallChanges = append(fb.WallChanges, fmt.Sprintf("ask wall moved: %s → %s", first.askStr, last.askStr))
	}
	for _, note := range fb.SceneNotes {
		low := strings.ToLower(note)
		if strings.Contains(low, "pull") || strings.Contains(low, "снят") || strings.Contains(low, "spoof") {
			fb.SpoofHints = append(fb.SpoofHints, note)
		}
		if strings.Contains(low, "iceberg") || strings.Contains(low, "айсберг") || strings.Contains(low, "reload") {
			fb.IcebergHints = append(fb.IcebergHints, note)
		}
	}
	if len(tl) >= 3 {
		mid0 := num((tl[0].(map[string]any))["mid"])
		midN := num((tl[len(tl)-1].(map[string]any))["mid"])
		if mid0 > 0 && math.Abs(midN-mid0)/mid0 < 0.0003 {
			for _, note := range fb.SceneNotes {
				if strings.Contains(strings.ToLower(note), "wall") || strings.Contains(strings.ToLower(note), "стен") {
					fb.SpoofHints = append(fb.SpoofHints, "price pinned near mid with wall activity: "+note)
				}
			}
		}
	}
}

func BuildFactsFromChildReports(reports []AnalysisReport, ticker, periodLabel string) FactsBundle {
	fb := FactsBundle{Ticker: ticker, PeriodLabel: periodLabel}
	for _, r := range reports {
		fb.ThoughtLines = append(fb.ThoughtLines, r.SummaryMD)
		for _, c := range r.Structured.Conclusions {
			fb.ThoughtLines = append(fb.ThoughtLines, c)
		}
	}
	fb.TextDigest = formatFactsDigest(fb)
	return fb
}

func formatFactsDigest(fb FactsBundle) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Instrument %s | period %s | snapshots %d | micro-events %d\n", fb.Ticker, fb.PeriodLabel, fb.SnapshotCount, fb.EventCount)
	if fb.SpreadMax > 0 {
		fmt.Fprintf(&b, "Spread bps: min %.1f max %.1f | avg imbalance %.3f\n", fb.SpreadMin, fb.SpreadMax, fb.ImbalanceAvg)
	}
	if fb.TapeSummary != "" {
		b.WriteString("Tape: " + fb.TapeSummary + "\n")
	}
	for _, p := range fb.PrintHighlights {
		b.WriteString("- print: " + p + "\n")
	}
	for _, t := range fb.TimelineLines {
		b.WriteString("- book: " + t + "\n")
	}
	for _, n := range fb.SceneNotes {
		b.WriteString("- scene: " + n + "\n")
	}
	for _, w := range fb.WallChanges {
		b.WriteString("- wall: " + w + "\n")
	}
	for _, s := range fb.SpoofHints {
		b.WriteString("- spoof?: " + s + "\n")
	}
	for _, i := range fb.IcebergHints {
		b.WriteString("- iceberg?: " + i + "\n")
	}
	for _, th := range fb.ThoughtLines {
		if len(fb.ThoughtLines) > 12 {
			break
		}
		b.WriteString("- thought: " + th + "\n")
	}
	return b.String()
}

func num(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	default:
		return 0
	}
}

func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func minMax(vals []float64) (float64, float64) {
	min, max := vals[0], vals[0]
	for _, v := range vals[1:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return min, max
}

func avg(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var s float64
	for _, v := range vals {
		s += v
	}
	return s / float64(len(vals))
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
