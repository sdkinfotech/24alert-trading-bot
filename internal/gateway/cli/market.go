package cli

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

func newMarketCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "market",
		Short: "Market data commands",
	}

	cmd.AddCommand(
		newMarketCandlesCmd(),
		newMarketBookCmd(),
		newMarketPriceCmd(),
		newMarketStatusCmd(),
	)
	return cmd
}

func newMarketCandlesCmd() *cobra.Command {
	var (
		instrument string
		from       string
		to         string
		interval   string
	)

	cmd := &cobra.Command{
		Use:   "candles",
		Short: "Get candles for an instrument",
		RunE: func(_ *cobra.Command, _ []string) error {
			params := url.Values{}
			params.Set("instrument_uid", instrument)
			if from != "" {
				params.Set("from", from)
			}
			if to != "" {
				params.Set("to", to)
			}
			params.Set("interval", interval)
			reqURL := fmt.Sprintf("%s/api/v1/candles?%s", flagGatewayURL, params.Encode())
			body, err := doGet(reqURL)
			if err != nil {
				return err
			}
			printResult(body, []string{"TIME", "OPEN", "HIGH", "LOW", "CLOSE", "VOLUME", "COMPLETE"}, func(data any) [][]string {
				arr, ok := data.([]any)
				if !ok {
					return nil
				}
				var rows [][]string
				for _, item := range arr {
					m, _ := item.(map[string]any)
					rows = append(rows, []string{
						fmt.Sprint(m["time"]),
						fmt.Sprint(m["open"]),
						fmt.Sprint(m["high"]),
						fmt.Sprint(m["low"]),
						fmt.Sprint(m["close"]),
						fmt.Sprint(m["volume"]),
						fmt.Sprint(m["is_complete"]),
					})
				}
				return rows
			})
			return nil
		},
	}

	cmd.Flags().StringVar(&instrument, "instrument", "", "instrument UID")
	cmd.Flags().StringVar(&from, "from", "", "start time RFC3339")
	cmd.Flags().StringVar(&to, "to", "", "end time RFC3339")
	cmd.Flags().StringVar(&interval, "interval", "1h", "candle interval")
	_ = cmd.MarkFlagRequired("instrument")

	return cmd
}

func newMarketBookCmd() *cobra.Command {
	var (
		instrument string
		depth      int32
	)

	cmd := &cobra.Command{
		Use:   "book",
		Short: "Get order book for an instrument",
		RunE: func(_ *cobra.Command, _ []string) error {
			reqURL := fmt.Sprintf("%s/api/v1/orderbook/%s?depth=%d", flagGatewayURL, instrument, depth)
			body, err := doGet(reqURL)
			if err != nil {
				return err
			}
			printResult(body, nil, nil)
			return nil
		},
	}

	cmd.Flags().StringVar(&instrument, "instrument", "", "instrument UID")
	cmd.Flags().Int32Var(&depth, "depth", 20, "order book depth")
	_ = cmd.MarkFlagRequired("instrument")

	return cmd
}

func newMarketPriceCmd() *cobra.Command {
	var instrument string

	cmd := &cobra.Command{
		Use:   "price",
		Short: "Get last price for an instrument",
		RunE: func(_ *cobra.Command, _ []string) error {
			reqURL := fmt.Sprintf("%s/api/v1/prices?instrument_uid=%s", flagGatewayURL, instrument)
			body, err := doGet(reqURL)
			if err != nil {
				return err
			}
			printResult(body, []string{"INSTRUMENT", "PRICE", "TIME"}, func(data any) [][]string {
				arr, ok := data.([]any)
				if !ok {
					return nil
				}
				var rows [][]string
				for _, item := range arr {
					m, _ := item.(map[string]any)
					rows = append(rows, []string{
						fmt.Sprint(m["instrument_uid"]),
						fmt.Sprint(m["price"]),
						fmt.Sprint(m["time"]),
					})
				}
				return rows
			})
			return nil
		},
	}

	cmd.Flags().StringVar(&instrument, "instrument", "", "instrument UID")
	_ = cmd.MarkFlagRequired("instrument")

	return cmd
}

func newMarketStatusCmd() *cobra.Command {
	var instrument string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Get trading status for an instrument",
		RunE: func(_ *cobra.Command, _ []string) error {
			reqURL := fmt.Sprintf("%s/api/v1/trading-status/%s", flagGatewayURL, instrument)
			body, err := doGet(reqURL)
			if err != nil {
				return err
			}
			printResult(body, []string{"INSTRUMENT", "STATUS", "LIMIT_AVAIL", "MARKET_AVAIL", "API_AVAIL"}, func(data any) [][]string {
				m, _ := data.(map[string]any)
				return [][]string{{
					fmt.Sprint(m["instrument_uid"]),
					fmt.Sprint(m["trading_status"]),
					fmt.Sprint(m["limit_order_available"]),
					fmt.Sprint(m["market_order_available"]),
					fmt.Sprint(m["api_trade_available"]),
				}}
			})
			return nil
		},
	}

	cmd.Flags().StringVar(&instrument, "instrument", "", "instrument UID")
	_ = cmd.MarkFlagRequired("instrument")

	return cmd
}
