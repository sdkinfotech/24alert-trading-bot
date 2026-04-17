package main

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(stopLossCmd, takeProfitCmd, stopLimitCmd, stopsCmd, cancelStopCmd)
}

var stopLossCmd = &cobra.Command{
	Use:   "stop-loss <instrument> <stop_price> [quantity]",
	Short: "Place stop-loss order (sell when price drops)",
	Long: `Place a stop-loss order.

When the market price reaches stop_price, a market sell order is placed.

Examples:
  trade stop-loss SBER 308.24             # sell 1 lot if price <= 308.24
  trade stop-loss SBER 308.24 5           # sell 5 lots
  trade stop-loss SBER 308.24 --dir buy   # buy stop-loss (short cover)`,
	Args: cobra.RangeArgs(2, 3),
	Run: func(cmd *cobra.Command, args []string) {
		dir, _ := cmd.Flags().GetString("dir")
		placeStop("stop_loss", dir, args, 0)
	},
}

var takeProfitCmd = &cobra.Command{
	Use:   "take-profit <instrument> <stop_price> [quantity]",
	Short: "Place take-profit order (sell when price rises)",
	Long: `Place a take-profit order.

When the market price reaches stop_price, a market sell order is placed.

Examples:
  trade take-profit SBER 320.81           # sell 1 lot if price >= 320.81
  trade take-profit SBER 320.81 5         # sell 5 lots`,
	Args: cobra.RangeArgs(2, 3),
	Run: func(cmd *cobra.Command, args []string) {
		dir, _ := cmd.Flags().GetString("dir")
		placeStop("take_profit", dir, args, 0)
	},
}

var stopLimitPrice float64

var stopLimitCmd = &cobra.Command{
	Use:   "stop-limit <instrument> <stop_price> -p <limit_price> [quantity]",
	Short: "Place stop-limit order",
	Long: `Place a stop-limit order.

When the market price reaches stop_price, a limit order at limit_price is placed.

Examples:
  trade stop-limit SBER 305.12 -p 301.98     # sell limit at 301.98 when price <= 305.12
  trade stop-limit SBER 305.12 -p 301.98 5   # 5 lots`,
	Args: cobra.RangeArgs(2, 3),
	Run: func(cmd *cobra.Command, args []string) {
		dir, _ := cmd.Flags().GetString("dir")
		if stopLimitPrice == 0 {
			fatal("--price is required for stop-limit")
		}
		placeStop("stop_limit", dir, args, stopLimitPrice)
	},
}

func init() {
	for _, cmd := range []*cobra.Command{stopLossCmd, takeProfitCmd, stopLimitCmd} {
		cmd.Flags().String("dir", "sell", "Direction: buy or sell")
	}
	stopLimitCmd.Flags().Float64VarP(&stopLimitPrice, "price", "p", 0, "Limit price for stop-limit (required)")
}

func placeStop(stopType, direction string, args []string, price float64) {
	uid := resolveInstrument(args[0])
	stopPrice, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		fatal("invalid stop_price: %s", args[1])
	}
	qty := int64(1)
	if len(args) > 2 {
		q, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			fatal("invalid quantity: %s", args[2])
		}
		qty = q
	}

	body := map[string]any{
		"account_id":      mustAccount(),
		"instrument_uid":  uid,
		"quantity":        qty,
		"direction":       direction,
		"stop_order_type": stopType,
		"stop_price":      stopPrice,
	}
	if price > 0 {
		body["price"] = price
	}

	data, err := doPost("/api/v1/stop-orders", body)
	if err != nil {
		fatal("%v", err)
	}
	var result struct {
		StopOrderID string `json:"stop_order_id"`
	}
	json.Unmarshal(data, &result)
	fmt.Printf("Stop order placed: %s\n", result.StopOrderID)
	fmt.Printf("  Type:      %s\n", stopType)
	fmt.Printf("  Direction: %s\n", direction)
	fmt.Printf("  Trigger:   %.2f\n", stopPrice)
	if price > 0 {
		fmt.Printf("  Limit:     %.2f\n", price)
	}
	fmt.Printf("  Quantity:  %d\n", qty)
}

var stopsCmd = &cobra.Command{
	Use:   "stops",
	Short: "List active stop-orders",
	Run: func(cmd *cobra.Command, args []string) {
		data, err := doGet("/api/v1/stop-orders?account_id=" + mustAccount())
		if err != nil {
			fatal("%v", err)
		}
		var orders []struct {
			ID        string  `json:"stop_order_id"`
			UID       string  `json:"instrument_uid"`
			Direction string  `json:"direction"`
			Type      string  `json:"stop_order_type"`
			Lots      int64   `json:"lots"`
			StopPrice float64 `json:"stop_price"`
			Price     float64 `json:"price"`
			Status    string  `json:"status"`
		}
		json.Unmarshal(data, &orders)
		if len(orders) == 0 {
			fmt.Println("No active stop-orders")
			return
		}
		fmt.Printf("%-38s %-6s %-12s %-8s %6s %10s %10s %s\n",
			"STOP_ORDER_ID", "DIR", "TYPE", "STATUS", "LOTS", "TRIGGER", "LIMIT", "INSTRUMENT")
		for _, o := range orders {
			lim := ""
			if o.Price > 0 {
				lim = fmt.Sprintf("%.2f", o.Price)
			}
			fmt.Printf("%-38s %-6s %-12s %-8s %6d %10.2f %10s %s\n",
				o.ID, shortDir(o.Direction), shortType(o.Type), shortStatus(o.Status),
				o.Lots, o.StopPrice, lim, shortUID(o.UID))
		}
	},
}

var cancelStopCmd = &cobra.Command{
	Use:   "cancel-stop <stop_order_id>",
	Short: "Cancel stop-order",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		data, err := doDelete(fmt.Sprintf("/api/v1/stop-orders/%s?account_id=%s", args[0], mustAccount()))
		if err != nil {
			fatal("%v", err)
		}
		var result struct {
			CancelledAt string `json:"cancelled_at"`
		}
		json.Unmarshal(data, &result)
		fmt.Printf("Stop order %s cancelled at %s\n", args[0], result.CancelledAt)
	},
}
