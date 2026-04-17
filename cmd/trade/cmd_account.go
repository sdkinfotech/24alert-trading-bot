package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var opsInstrument string
var opsFrom string
var opsTo string

func init() {
	operationsCmd.Flags().StringVar(&opsInstrument, "uid", "", "Filter by instrument UID or alias")
	operationsCmd.Flags().StringVar(&opsFrom, "from", "", "Start time (RFC3339)")
	operationsCmd.Flags().StringVar(&opsTo, "to", "", "End time (RFC3339)")

	rootCmd.AddCommand(accountsCmd, positionsCmd, portfolioCmd, marginCmd, limitsCmd, operationsCmd, riskCmd, riskResetCmd)
}

var accountsCmd = &cobra.Command{
	Use:   "accounts",
	Short: "List trading accounts",
	Run: func(cmd *cobra.Command, args []string) {
		data, err := doGet("/api/v1/accounts")
		if err != nil {
			fatal("%v", err)
		}
		var accs []struct {
			ID     string `json:"id"`
			Type   string `json:"type"`
			Name   string `json:"name"`
			Status string `json:"status"`
			Access string `json:"access_level"`
		}
		json.Unmarshal(data, &accs)
		fmt.Printf("%-15s %-25s %-30s %-15s %s\n", "ID", "NAME", "TYPE", "STATUS", "ACCESS")
		for _, a := range accs {
			fmt.Printf("%-15s %-25s %-30s %-15s %s\n",
				a.ID, a.Name,
				trimAccountPrefix(a.Type),
				trimAccountPrefix(a.Status),
				trimAccountPrefix(a.Access))
		}
	},
}

var positionsCmd = &cobra.Command{
	Use:   "positions",
	Short: "Show current positions",
	Run: func(cmd *cobra.Command, args []string) {
		data, err := doGet("/api/v1/positions?account_id=" + mustAccount())
		if err != nil {
			fatal("%v", err)
		}
		var positions []struct {
			UID       string  `json:"instrument_uid"`
			Type      string  `json:"instrument_type"`
			Qty       float64 `json:"quantity"`
			AvgPrice  float64 `json:"average_price"`
			CurPrice  float64 `json:"current_price"`
			Yield     float64 `json:"expected_yield"`
			Currency  string  `json:"currency"`
			Blocked   bool    `json:"blocked"`
		}
		json.Unmarshal(data, &positions)
		if len(positions) == 0 {
			fmt.Println("No open positions")
			return
		}
		fmt.Printf("%-14s %-8s %8s %10s %10s %10s %5s\n",
			"INSTRUMENT", "TYPE", "QTY", "AVG", "CURRENT", "P&L", "CUR")
		for _, p := range positions {
			blocked := ""
			if p.Blocked {
				blocked = " [BLOCKED]"
			}
			fmt.Printf("%-14s %-8s %8.0f %10.2f %10.2f %10.2f %5s%s\n",
				shortUID(p.UID), p.Type, p.Qty, p.AvgPrice, p.CurPrice, p.Yield, p.Currency, blocked)
		}
	},
}

var portfolioCmd = &cobra.Command{
	Use:   "portfolio",
	Short: "Portfolio summary",
	Run: func(cmd *cobra.Command, args []string) {
		data, err := doGet("/api/v1/portfolio?account_id=" + mustAccount())
		if err != nil {
			fatal("%v", err)
		}
		var p struct {
			Shares     float64 `json:"total_amount_shares"`
			Bonds      float64 `json:"total_amount_bonds"`
			ETF        float64 `json:"total_amount_etf"`
			Currencies float64 `json:"total_amount_currencies"`
			Futures    float64 `json:"total_amount_futures"`
			Yield      float64 `json:"expected_yield"`
		}
		json.Unmarshal(data, &p)

		total := p.Shares + p.Bonds + p.ETF + p.Currencies + p.Futures
		fmt.Printf("Shares:     %12.2f RUB\n", p.Shares)
		fmt.Printf("Bonds:      %12.2f RUB\n", p.Bonds)
		fmt.Printf("ETF:        %12.2f RUB\n", p.ETF)
		fmt.Printf("Currencies: %12.2f RUB\n", p.Currencies)
		fmt.Printf("Futures:    %12.2f RUB\n", p.Futures)
		fmt.Println("─────────────────────────")
		fmt.Printf("Total:      %12.2f RUB\n", total)
		fmt.Printf("P&L:        %12.2f RUB\n", p.Yield)
	},
}

var marginCmd = &cobra.Command{
	Use:   "margin",
	Short: "Show margin attributes",
	Run: func(cmd *cobra.Command, args []string) {
		data, err := doGet("/api/v1/margin/" + mustAccount())
		if err != nil {
			fatal("%v", err)
		}
		var m struct {
			Liquid   float64 `json:"liquid_portfolio"`
			Starting float64 `json:"starting_margin"`
			Minimal  float64 `json:"minimal_margin"`
			Level    float64 `json:"funds_sufficiency_level"`
			Missing  float64 `json:"amount_of_missing"`
		}
		json.Unmarshal(data, &m)
		fmt.Printf("Liquid portfolio:  %12.2f RUB\n", m.Liquid)
		fmt.Printf("Starting margin:   %12.2f RUB\n", m.Starting)
		fmt.Printf("Minimal margin:    %12.2f RUB\n", m.Minimal)
		fmt.Printf("Sufficiency level: %12.2f\n", m.Level)
		if m.Missing > 0 {
			fmt.Printf("Missing funds:     %12.2f RUB\n", m.Missing)
		}
	},
}

var limitsCmd = &cobra.Command{
	Use:   "limits",
	Short: "Show withdraw limits",
	Run: func(cmd *cobra.Command, args []string) {
		data, err := doGet("/api/v1/limits?account_id=" + mustAccount())
		if err != nil {
			fatal("%v", err)
		}
		var limits []struct {
			Currency string  `json:"currency"`
			Blocked  float64 `json:"blocked_amount"`
			Withdraw float64 `json:"withdraw_amount"`
		}
		json.Unmarshal(data, &limits)
		if len(limits) == 0 {
			fmt.Println("No limits data")
			return
		}
		fmt.Printf("%-8s %12s %12s\n", "CUR", "WITHDRAW", "BLOCKED")
		for _, l := range limits {
			fmt.Printf("%-8s %12.2f %12.2f\n", l.Currency, l.Withdraw, l.Blocked)
		}
	},
}

var operationsCmd = &cobra.Command{
	Use:   "operations",
	Short: "Show operations history",
	Long: `Show operations (trades, dividends, etc).

Examples:
  trade operations                        # last 24h
  trade operations --uid SBER             # filter by instrument
  trade operations --from 2026-04-01T00:00:00Z`,
	Run: func(cmd *cobra.Command, args []string) {
		params := map[string]string{
			"account_id": mustAccount(),
		}
		if opsInstrument != "" {
			params["instrument_uid"] = resolveInstrument(opsInstrument)
		}
		if opsFrom != "" {
			params["from"] = opsFrom
		}
		if opsTo != "" {
			params["to"] = opsTo
		}
		data, err := doGet("/api/v1/operations" + buildQuery(params))
		if err != nil {
			fatal("%v", err)
		}
		var page struct {
			Operations []struct {
				ID       string    `json:"id"`
				UID      string    `json:"instrument_uid"`
				Type     string    `json:"type"`
				State    string    `json:"state"`
				Payment  float64   `json:"payment"`
				Currency string    `json:"currency"`
				Qty      int64     `json:"quantity"`
				Date     time.Time `json:"date"`
			} `json:"operations"`
			HasNext bool `json:"has_next"`
		}
		json.Unmarshal(data, &page)

		if page.Operations == nil {
			var ops []struct {
				ID       string    `json:"id"`
				UID      string    `json:"instrument_uid"`
				Type     string    `json:"type"`
				State    string    `json:"state"`
				Payment  float64   `json:"payment"`
				Currency string    `json:"currency"`
				Qty      int64     `json:"quantity"`
				Date     time.Time `json:"date"`
			}
			json.Unmarshal(data, &ops)
			page.Operations = ops
		}

		if len(page.Operations) == 0 {
			fmt.Println("No operations")
			return
		}
		fmt.Printf("%-20s %-14s %-20s %-10s %8s %12s %5s\n",
			"DATE", "INSTRUMENT", "TYPE", "STATE", "QTY", "PAYMENT", "CUR")
		for _, o := range page.Operations {
			fmt.Printf("%-20s %-14s %-20s %-10s %8d %12.2f %5s\n",
				o.Date.Format("2006-01-02 15:04"),
				shortUID(o.UID), o.Type, o.State,
				o.Qty, o.Payment, o.Currency)
		}
		if page.HasNext {
			fmt.Println("\n... more operations available")
		}
	},
}

var riskCmd = &cobra.Command{
	Use:   "risk",
	Short: "Show risk / circuit breaker status",
	Run: func(cmd *cobra.Command, args []string) {
		data, err := doGet("/api/v1/risk/status")
		if err != nil {
			fatal("%v", err)
		}
		var r struct {
			Tripped   bool   `json:"circuit_breaker_tripped"`
			Failures  int    `json:"failure_count"`
			Threshold int    `json:"threshold"`
			Cooldown  string `json:"cooldown"`
		}
		json.Unmarshal(data, &r)

		status := "OK"
		if r.Tripped {
			status = "TRIPPED (orders blocked!)"
		}
		fmt.Printf("Circuit breaker: %s\n", status)
		fmt.Printf("Failures:        %d / %d\n", r.Failures, r.Threshold)
		if r.Cooldown != "" {
			fmt.Printf("Cooldown:        %s\n", r.Cooldown)
		}
	},
}

var riskResetCmd = &cobra.Command{
	Use:   "risk-reset",
	Short: "Reset circuit breaker",
	Run: func(cmd *cobra.Command, args []string) {
		_, err := doPost("/api/v1/risk/reset", nil)
		if err != nil {
			fatal("%v", err)
		}
		fmt.Println("Circuit breaker reset")
	},
}

func trimAccountPrefix(s string) string {
	for _, prefix := range []string{"ACCOUNT_TYPE_", "ACCOUNT_STATUS_", "ACCOUNT_ACCESS_LEVEL_"} {
		if len(s) > len(prefix) && s[:len(prefix)] == prefix {
			return s[len(prefix):]
		}
	}
	return s
}
