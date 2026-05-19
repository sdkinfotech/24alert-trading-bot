// Command ai-trader-replay runs level_intraday paper logic on recorded SQLite ticks.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/24alert/trading-bot/internal/strategy"
)

func main() {
	sessionID := flag.String("session", "", "session id in ai_trader_record.db")
	evalOnly := flag.Bool("eval", false, "only run LLM signal accuracy eval")
	flag.Parse()
	if *sessionID == "" {
		fmt.Fprintln(os.Stderr, "usage: ai-trader-replay -session <id> [-eval]")
		os.Exit(2)
	}
	_ = os.Setenv("AI_TRADER_RECORD_ENABLED", "true")
	if *evalOnly {
		res, err := strategy.EvalTradeSignalsOnReplay(*sessionID, 5)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
		return
	}
	ticks, err := strategy.ReplayTicks(*sessionID, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("loaded %d ticks for session %s\n", len(ticks), *sessionID)
	res, err := strategy.EvalTradeSignalsOnReplay(*sessionID, 5)
	if err != nil {
		fmt.Fprintln(os.Stderr, "eval:", err)
	} else {
		fmt.Printf("signal eval: %+v\n", res)
	}
}
