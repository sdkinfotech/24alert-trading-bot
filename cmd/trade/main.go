package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	serverURL string
	accountID string
)

var aliases = map[string]string{
	"sber":        "e6123145-9665-43e0-8413-cd61b8aa9b13",
	"gazp":        "962e2a95-02a9-4171-abd7-aa198dbe643a",
	"vtbr":        "8e2b0325-0292-4654-8a18-4f63ed3b0e09",
	"lkoh":        "4fba23fc-e34a-42f4-a987-4c1e8e9d66ba",
	"kzu6":        "3da86f70-25a7-4092-bc1b-05cf84dad9fe",
	"gold_call":   "66f3eea6-35f2-4347-8f1d-8f082588801c",
	"gl11700cd6b": "66f3eea6-35f2-4347-8f1d-8f082588801c",
}

func resolveInstrument(s string) string {
	if uid, ok := aliases[strings.ToLower(s)]; ok {
		return uid
	}
	return s
}

var rootCmd = &cobra.Command{
	Use:   "trade",
	Short: "24Alert Trading CLI",
	Long: `CLI for 24Alert Trading API.

Supports orders, stop-orders, market data, portfolio and risk management.

Environment variables:
  TRADE_SERVER   API server URL  (default http://176.123.160.234:8080)
  TRADE_ACCOUNT  Account ID      (auto-detected if not set)

Instrument aliases (use instead of UID):
  SBER        Сбербанк (акция)
  KZU6        Казахский тенге (фьючерс)
  GOLD_CALL   Золото CALL опцион

Examples:
  trade buy SBER
  trade sell SBER 2
  trade buy SBER -p 298.50
  trade stop-loss SBER 308.24
  trade price SBER
  trade book SBER --depth 5
  trade candles SBER --interval 5min
  trade orders
  trade positions`,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&serverURL, "server", "s", "", "API server URL (env: TRADE_SERVER)")
	rootCmd.PersistentFlags().StringVarP(&accountID, "account", "a", "", "Account ID (env: TRADE_ACCOUNT)")

	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if serverURL == "" {
			serverURL = os.Getenv("TRADE_SERVER")
		}
		if serverURL == "" {
			serverURL = "http://176.123.160.234:8080"
		}
		if accountID == "" {
			accountID = os.Getenv("TRADE_ACCOUNT")
		}
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
