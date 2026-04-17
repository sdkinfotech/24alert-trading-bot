package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"
)

var orderPrice float64
var orderBestprice bool

func init() {
	buyCmd.Flags().Float64VarP(&orderPrice, "price", "p", 0, "Limit price (omit for market order)")
	buyCmd.Flags().BoolVar(&orderBestprice, "best", false, "Use bestprice order type")
	sellCmd.Flags().Float64VarP(&orderPrice, "price", "p", 0, "Limit price (omit for market order)")
	sellCmd.Flags().BoolVar(&orderBestprice, "best", false, "Use bestprice order type")

	var replacePrice float64
	var replaceQty int64
	replaceCmd.Flags().Float64VarP(&replacePrice, "price", "p", 0, "New price (required)")
	replaceCmd.Flags().Int64VarP(&replaceQty, "qty", "q", 1, "New quantity")
	replaceCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("usage: trade replace <order_id> -p <price> [-q <qty>]")
		}
		p, _ := cmd.Flags().GetFloat64("price")
		q, _ := cmd.Flags().GetInt64("qty")
		if p == 0 {
			return fmt.Errorf("--price is required")
		}
		data, err := doPut(
			fmt.Sprintf("/api/v1/orders/%s?account_id=%s", args[0], mustAccount()),
			map[string]any{"quantity": q, "price": p},
		)
		if err != nil {
			fatal("%v", err)
		}
		printOrderResult(data)
		return nil
	}

	rootCmd.AddCommand(buyCmd, sellCmd, ordersCmd, orderCmd, cancelCmd, replaceCmd)
}

var buyCmd = &cobra.Command{
	Use:   "buy <instrument> [quantity]",
	Short: "Buy instrument (market/limit/bestprice)",
	Long: `Place a buy order.

Examples:
  trade buy SBER                  # market buy 1 lot
  trade buy SBER 5                # market buy 5 lots
  trade buy SBER -p 298.50        # limit buy at 298.50
  trade buy SBER --best           # bestprice buy`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		placeOrder("buy", args)
	},
}

var sellCmd = &cobra.Command{
	Use:   "sell <instrument> [quantity]",
	Short: "Sell instrument (market/limit/bestprice)",
	Long: `Place a sell order.

Examples:
  trade sell SBER                 # market sell 1 lot
  trade sell SBER 5               # market sell 5 lots
  trade sell SBER -p 320.00       # limit sell at 320.00`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		placeOrder("sell", args)
	},
}

func placeOrder(direction string, args []string) {
	uid := resolveInstrument(args[0])
	qty := int64(1)
	if len(args) > 1 {
		q, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			fatal("invalid quantity: %s", args[1])
		}
		qty = q
	}

	orderType := "market"
	if orderPrice > 0 {
		orderType = "limit"
	}
	if orderBestprice {
		orderType = "bestprice"
	}

	body := map[string]any{
		"account_id":     mustAccount(),
		"instrument_uid": uid,
		"quantity":       qty,
		"direction":      direction,
		"order_type":     orderType,
	}
	if orderPrice > 0 {
		body["price"] = orderPrice
	}

	data, err := doPost("/api/v1/orders", body)
	if err != nil {
		fatal("%v", err)
	}
	printOrderResult(data)
	orderPrice = 0
	orderBestprice = false
}

var ordersCmd = &cobra.Command{
	Use:   "orders",
	Short: "List active orders",
	Run: func(cmd *cobra.Command, args []string) {
		data, err := doGet("/api/v1/orders?account_id=" + mustAccount())
		if err != nil {
			fatal("%v", err)
		}
		var orders []struct {
			OrderID   string  `json:"order_id"`
			UID       string  `json:"instrument_uid"`
			Direction string  `json:"direction"`
			Type      string  `json:"order_type"`
			Lots      int64   `json:"lots"`
			Price     float64 `json:"price"`
			Status    string  `json:"status"`
		}
		json.Unmarshal(data, &orders)
		if len(orders) == 0 {
			fmt.Println("No active orders")
			return
		}
		fmt.Printf("%-15s %-8s %-10s %-8s %6s %10s %s\n",
			"ORDER_ID", "DIR", "TYPE", "STATUS", "LOTS", "PRICE", "INSTRUMENT")
		for _, o := range orders {
			fmt.Printf("%-15s %-8s %-10s %-8s %6d %10.2f %s\n",
				o.OrderID, shortDir(o.Direction), shortType(o.Type), shortStatus(o.Status),
				o.Lots, o.Price, shortUID(o.UID))
		}
	},
}

var orderCmd = &cobra.Command{
	Use:   "order <order_id>",
	Short: "Get order state",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		data, err := doGet(fmt.Sprintf("/api/v1/orders/%s?account_id=%s", args[0], mustAccount()))
		if err != nil {
			fatal("%v", err)
		}
		prettyJSON(data)
	},
}

var cancelCmd = &cobra.Command{
	Use:   "cancel <order_id>",
	Short: "Cancel active order",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		data, err := doDelete(fmt.Sprintf("/api/v1/orders/%s?account_id=%s", args[0], mustAccount()))
		if err != nil {
			fatal("%v", err)
		}
		var result struct {
			CancelledAt string `json:"cancelled_at"`
		}
		json.Unmarshal(data, &result)
		fmt.Printf("Order %s cancelled at %s\n", args[0], result.CancelledAt)
	},
}

var replaceCmd = &cobra.Command{
	Use:   "replace <order_id> -p <price> [-q <qty>]",
	Short: "Replace (modify) active limit order",
	Args:  cobra.ExactArgs(1),
}

func printOrderResult(data json.RawMessage) {
	var r struct {
		OrderID   string  `json:"order_id"`
		Status    string  `json:"execution_status"`
		LotsReq   int64   `json:"lots_requested"`
		LotsExec  int64   `json:"lots_executed"`
		Total     float64 `json:"total_price"`
		Direction string  `json:"direction"`
		Type      string  `json:"order_type"`
	}
	json.Unmarshal(data, &r)
	fmt.Printf("Order: %s\n", r.OrderID)
	fmt.Printf("  Status:    %s\n", shortStatus(r.Status))
	fmt.Printf("  Direction: %s\n", shortDir(r.Direction))
	fmt.Printf("  Type:      %s\n", shortType(r.Type))
	fmt.Printf("  Lots:      %d/%d executed\n", r.LotsExec, r.LotsReq)
	if r.Total > 0 {
		fmt.Printf("  Total:     %.2f RUB\n", r.Total)
	}
}

func shortDir(s string) string {
	s = trimPrefix(s, "ORDER_DIRECTION_")
	s = trimPrefix(s, "STOP_ORDER_DIRECTION_")
	return s
}

func shortType(s string) string {
	s = trimPrefix(s, "ORDER_TYPE_")
	s = trimPrefix(s, "STOP_ORDER_TYPE_")
	return s
}

func shortStatus(s string) string {
	s = trimPrefix(s, "EXECUTION_REPORT_STATUS_")
	s = trimPrefix(s, "STOP_ORDER_STATUS_")
	return s
}

func trimPrefix(s, prefix string) string {
	if len(s) > len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}

var reverseAliases map[string]string

func shortUID(uid string) string {
	if reverseAliases == nil {
		reverseAliases = make(map[string]string)
		for name, id := range aliases {
			reverseAliases[id] = name
		}
	}
	if name, ok := reverseAliases[uid]; ok {
		return name
	}
	if len(uid) > 12 {
		return uid[:12] + "..."
	}
	return uid
}

func buildQuery(params map[string]string) string {
	v := url.Values{}
	for k, val := range params {
		if val != "" {
			v.Set(k, val)
		}
	}
	q := v.Encode()
	if q != "" {
		return "?" + q
	}
	return ""
}
