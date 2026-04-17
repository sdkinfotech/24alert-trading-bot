package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var candleInterval string
var candleFrom string
var candleTo string
var bookDepth int

func init() {
	candlesCmd.Flags().StringVarP(&candleInterval, "interval", "i", "1h", "Candle interval: 1min, 5min, 15min, 1h, day, week, month")
	candlesCmd.Flags().StringVar(&candleFrom, "from", "", "Start time (RFC3339). Default: -24h")
	candlesCmd.Flags().StringVar(&candleTo, "to", "", "End time (RFC3339). Default: now")
	bookCmd.Flags().IntVarP(&bookDepth, "depth", "d", 10, "Orderbook depth (5, 10, 20)")

	rootCmd.AddCommand(priceCmd, bookCmd, candlesCmd, tradingStatusCmd)
}

var priceCmd = &cobra.Command{
	Use:   "price <instrument>",
	Short: "Get last price",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		uid := resolveInstrument(args[0])
		data, err := doGet("/api/v1/prices?instrument_uid=" + uid)
		if err != nil {
			fatal("%v", err)
		}
		var prices []struct {
			UID   string  `json:"instrument_uid"`
			Price float64 `json:"price"`
		}
		json.Unmarshal(data, &prices)
		if len(prices) == 0 {
			fmt.Println("No price data")
			return
		}
		fmt.Printf("%.4f\n", prices[0].Price)
	},
}

var bookCmd = &cobra.Command{
	Use:   "book <instrument>",
	Short: "Show orderbook (bids/asks)",
	Long: `Show orderbook for an instrument.

Examples:
  trade book SBER              # default depth 10
  trade book SBER -d 5         # depth 5
  trade book GOLD_CALL -d 20   # options — check orderbook for price`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		uid := resolveInstrument(args[0])
		data, err := doGet(fmt.Sprintf("/api/v1/orderbook/%s?depth=%d", uid, bookDepth))
		if err != nil {
			fatal("%v", err)
		}
		var book struct {
			Bids  []struct{ Price float64; Quantity int64 } `json:"bids"`
			Asks  []struct{ Price float64; Quantity int64 } `json:"asks"`
			Last  float64                                   `json:"last_price"`
			Close float64                                   `json:"close_price"`
		}
		json.Unmarshal(data, &book)

		maxRows := len(book.Asks)
		if len(book.Bids) > maxRows {
			maxRows = len(book.Bids)
		}

		fmt.Printf("Last: %.4f  Close: %.4f\n\n", book.Last, book.Close)
		fmt.Printf("%10s %10s  |  %-10s %-10s\n", "ASK_QTY", "ASK", "BID", "BID_QTY")
		fmt.Println(strings.Repeat("-", 50))

		for i := len(book.Asks) - 1; i >= 0; i-- {
			fmt.Printf("%10d %10.4f  |  %10s %10s\n",
				book.Asks[i].Quantity, book.Asks[i].Price, "", "")
		}
		fmt.Println(strings.Repeat("=", 50))
		for _, b := range book.Bids {
			fmt.Printf("%10s %10s  |  %-10.4f %-10d\n",
				"", "", b.Price, b.Quantity)
		}
		fmt.Printf("\nSpread: %.4f\n", book.Asks[0].Price-book.Bids[0].Price)
	},
}

var candlesCmd = &cobra.Command{
	Use:   "candles <instrument>",
	Short: "Get candles (OHLCV)",
	Long: `Get historical candles.

Intervals: 1min, 5min, 15min, 1h, day, week, month

Examples:
  trade candles SBER                        # 1h candles, last 24h
  trade candles SBER -i 5min                # 5min candles
  trade candles SBER -i day --from 2026-01-01T00:00:00Z`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		uid := resolveInstrument(args[0])
		params := map[string]string{
			"instrument_uid": uid,
			"interval":       candleInterval,
		}
		if candleFrom != "" {
			params["from"] = candleFrom
		}
		if candleTo != "" {
			params["to"] = candleTo
		}
		data, err := doGet("/api/v1/candles" + buildQuery(params))
		if err != nil {
			fatal("%v", err)
		}
		var candles []struct {
			Open     float64   `json:"open"`
			High     float64   `json:"high"`
			Low      float64   `json:"low"`
			Close    float64   `json:"close"`
			Volume   int64     `json:"volume"`
			Time     time.Time `json:"time"`
			Complete bool      `json:"is_complete"`
		}
		json.Unmarshal(data, &candles)
		if len(candles) == 0 {
			fmt.Println("No candle data")
			return
		}
		fmt.Printf("%-20s %10s %10s %10s %10s %10s %s\n",
			"TIME", "OPEN", "HIGH", "LOW", "CLOSE", "VOLUME", "OK")
		for _, c := range candles {
			done := " "
			if c.Complete {
				done = "+"
			}
			fmt.Printf("%-20s %10.4f %10.4f %10.4f %10.4f %10d %s\n",
				c.Time.Format("2006-01-02 15:04"), c.Open, c.High, c.Low, c.Close, c.Volume, done)
		}
		fmt.Printf("\n%d candles (%s)\n", len(candles), candleInterval)
	},
}

var tradingStatusCmd = &cobra.Command{
	Use:     "status <instrument>",
	Aliases: []string{"trading-status"},
	Short:   "Get trading status for instrument",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		uid := resolveInstrument(args[0])
		data, err := doGet("/api/v1/trading-status/" + uid)
		if err != nil {
			fatal("%v", err)
		}
		var s struct {
			Status string `json:"trading_status"`
			Limit  bool   `json:"limit_order_available"`
			Market bool   `json:"market_order_available"`
			API    bool   `json:"api_trade_available"`
		}
		json.Unmarshal(data, &s)

		status := strings.TrimPrefix(s.Status, "SECURITY_TRADING_STATUS_")
		fmt.Printf("Status:  %s\n", status)
		fmt.Printf("Limit:   %s\n", boolIcon(s.Limit))
		fmt.Printf("Market:  %s\n", boolIcon(s.Market))
		fmt.Printf("API:     %s\n", boolIcon(s.API))
	},
}

func boolIcon(b bool) string {
	if b {
		return "YES"
	}
	return "NO"
}

func intArg(args []string, idx int, def int) int {
	if idx >= len(args) {
		return def
	}
	v, err := strconv.Atoi(args[idx])
	if err != nil {
		return def
	}
	return v
}
