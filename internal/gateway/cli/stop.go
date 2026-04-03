package cli

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Manage stop orders",
	}

	cmd.AddCommand(
		newStopPostCmd(),
		newStopListCmd(),
		newStopCancelCmd(),
	)
	return cmd
}

func newStopPostCmd() *cobra.Command {
	var (
		instrument    string
		qty           int64
		direction     string
		stopOrderType string
		stopPrice     float64
		price         float64
		accountID     string
	)

	cmd := &cobra.Command{
		Use:   "post",
		Short: "Place a new stop order",
		RunE: func(_ *cobra.Command, _ []string) error {
			payload, _ := json.Marshal(map[string]any{
				"instrument_uid":  instrument,
				"quantity":        qty,
				"direction":       direction,
				"stop_order_type": stopOrderType,
				"stop_price":      stopPrice,
				"price":           price,
				"account_id":      accountID,
			})
			body, err := doRequest("POST", flagGatewayURL+"/api/v1/stop-orders", bytes.NewReader(payload))
			if err != nil {
				return err
			}
			printResult(body, nil, nil)
			return nil
		},
	}

	cmd.Flags().StringVar(&instrument, "instrument", "", "instrument UID")
	cmd.Flags().Int64Var(&qty, "qty", 0, "quantity in lots")
	cmd.Flags().StringVar(&direction, "direction", "buy", "buy or sell")
	cmd.Flags().StringVar(&stopOrderType, "type", "stop_limit", "stop order type")
	cmd.Flags().Float64Var(&stopPrice, "stop-price", 0, "stop price")
	cmd.Flags().Float64Var(&price, "price", 0, "limit price")
	cmd.Flags().StringVar(&accountID, "account-id", "", "account ID")
	_ = cmd.MarkFlagRequired("instrument")
	_ = cmd.MarkFlagRequired("account-id")

	return cmd
}

func newStopListCmd() *cobra.Command {
	var accountID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List stop orders",
		RunE: func(_ *cobra.Command, _ []string) error {
			url := fmt.Sprintf("%s/api/v1/stop-orders?account_id=%s", flagGatewayURL, accountID)
			body, err := doGet(url)
			if err != nil {
				return err
			}
			printResult(body, []string{"STOP_ORDER_ID", "INSTRUMENT", "DIR", "TYPE", "LOTS", "STOP_PRICE", "STATUS"}, func(data any) [][]string {
				arr, ok := data.([]any)
				if !ok {
					return nil
				}
				var rows [][]string
				for _, item := range arr {
					m, _ := item.(map[string]any)
					rows = append(rows, []string{
						fmt.Sprint(m["stop_order_id"]),
						fmt.Sprint(m["instrument_uid"]),
						fmt.Sprint(m["direction"]),
						fmt.Sprint(m["stop_order_type"]),
						fmt.Sprint(m["lots"]),
						fmt.Sprint(m["stop_price"]),
						fmt.Sprint(m["status"]),
					})
				}
				return rows
			})
			return nil
		},
	}

	cmd.Flags().StringVar(&accountID, "account-id", "", "account ID")
	_ = cmd.MarkFlagRequired("account-id")

	return cmd
}

func newStopCancelCmd() *cobra.Command {
	var stopOrderID, accountID string

	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel a stop order",
		RunE: func(_ *cobra.Command, _ []string) error {
			url := fmt.Sprintf("%s/api/v1/stop-orders/%s?account_id=%s", flagGatewayURL, stopOrderID, accountID)
			body, err := doRequest("DELETE", url, nil)
			if err != nil {
				return err
			}
			printResult(body, nil, nil)
			return nil
		},
	}

	cmd.Flags().StringVar(&stopOrderID, "stop-order-id", "", "stop order ID")
	cmd.Flags().StringVar(&accountID, "account-id", "", "account ID")
	_ = cmd.MarkFlagRequired("stop-order-id")
	_ = cmd.MarkFlagRequired("account-id")

	return cmd
}
