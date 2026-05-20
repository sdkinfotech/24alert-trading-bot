// Command ai-trader-postmarket runs trade analyst post-market for a session id.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/24alert/trading-bot/internal/tradeanalyst"
)

func main() {
	sessionID := flag.String("session", "", "AI Trader session id (required)")
	journalPath := flag.String("journal", "", "override ai_trader_journal.jsonl path")
	flag.Parse()
	if *sessionID == "" {
		fmt.Fprintln(os.Stderr, "usage: ai-trader-postmarket -session <id>")
		os.Exit(2)
	}
	if *journalPath != "" {
		_ = os.Setenv("AI_TRADER_JOURNAL_PATH", *journalPath)
	}
	svc, err := tradeanalyst.NewService(nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	journal, err := tradeanalyst.LoadJournalEvents(svc.Store().JournalPath(), *sessionID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "journal:", err)
		os.Exit(1)
	}
	ticker := "unknown"
	for _, p := range splitID(*sessionID) {
		if len(p) == 4 || len(p) == 5 {
			ticker = p
			break
		}
	}
	in := tradeanalyst.SessionInput{
		SessionID: *sessionID,
		Ticker:    ticker,
		StartedAt: firstTime(journal),
		StoppedAt: lastTime(journal),
		StrategyKind: "level_intraday",
	}
	rep, err := tradeanalyst.AnalyzeSession(in, journal)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := svc.Store().SaveReport(rep); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if h := tradeanalyst.HintsFromReport(rep); h != nil {
		_ = svc.Store().SaveHints(h)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(rep)
}

func splitID(id string) []string {
	var out []string
	cur := ""
	for _, r := range id {
		if r == '-' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func firstTime(j []tradeanalyst.JournalEvent) string {
	if len(j) == 0 {
		return ""
	}
	return j[0].Time
}

func lastTime(j []tradeanalyst.JournalEvent) string {
	if len(j) == 0 {
		return ""
	}
	return j[len(j)-1].Time
}
