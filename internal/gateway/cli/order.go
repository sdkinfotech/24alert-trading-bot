package cli

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newOrderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "order",
		Short: "Manage exchange orders",
	}

	cmd.AddCommand(
		newOrderPostCmd(),
		newOrderCancelCmd(),
		newOrderReplaceCmd(),
		newOrderListCmd(),
		newOrderStateCmd(),
	)
	return cmd
}

func newOrderPostCmd() *cobra.Command {
	var (
		instrument string
		qty        int64
		direction  string
		orderType  string
		price      float64
		accountID  string
	)

	cmd := &cobra.Command{
		Use:   "post",
		Short: "Place a new order",
		RunE: func(_ *cobra.Command, _ []string) error {
			payload, _ := json.Marshal(map[string]any{
				"instrument_uid": instrument,
				"quantity":       qty,
				"direction":      direction,
				"order_type":     orderType,
				"price":          price,
				"account_id":     accountID,
			})
			body, err := doRequest("POST", flagGatewayURL+"/api/v1/orders", bytes.NewReader(payload))
			if err != nil {
				return err
			}
			printResult(body, []string{"ORDER_ID", "STATUS", "LOTS_REQ", "LOTS_EXEC", "DIRECTION", "TYPE"}, func(data any) [][]string {
				m, _ := data.(map[string]any)
				return [][]string{{
					fmt.Sprint(m["order_id"]),
					fmt.Sprint(m["execution_status"]),
					fmt.Sprint(m["lots_requested"]),
					fmt.Sprint(m["lots_executed"]),
					fmt.Sprint(m["direction"]),
					fmt.Sprint(m["order_type"]),
				}}
			})
			return nil
		},
	}

	cmd.Flags().StringVar(&instrument, "instrument", "", "instrument UID")
	cmd.Flags().Int64Var(&qty, "qty", 0, "quantity in lots")
	cmd.Flags().StringVar(&direction, "direction", "buy", "buy or sell")
	cmd.Flags().StringVar(&orderType, "type", "limit", "order type: limit, market, bestprice")
	cmd.Flags().Float64Var(&price, "price", 0, "order price")
	cmd.Flags().StringVar(&accountID, "account-id", "", "account ID")
	_ = cmd.MarkFlagRequired("instrument")
	_ = cmd.MarkFlagRequired("account-id")

	return cmd
}

func newOrderCancelCmd() *cobra.Command {
	var orderID, accountID string

	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel an active order",
		RunE: func(_ *cobra.Command, _ []string) error {
			url := fmt.Sprintf("%s/api/v1/orders/%s?account_id=%s", flagGatewayURL, orderID, accountID)
			body, err := doRequest("DELETE", url, nil)
			if err != nil {
				return err
			}
			printResult(body, nil, nil)
			return nil
		},
	}

	cmd.Flags().StringVar(&orderID, "order-id", "", "order ID")
	cmd.Flags().StringVar(&accountID, "account-id", "", "account ID")
	_ = cmd.MarkFlagRequired("order-id")
	_ = cmd.MarkFlagRequired("account-id")

	return cmd
}

func newOrderReplaceCmd() *cobra.Command {
	var (
		orderID   string
		accountID string
		qty       int64
		price     float64
	)

	cmd := &cobra.Command{
		Use:   "replace",
		Short: "Replace an existing order",
		RunE: func(_ *cobra.Command, _ []string) error {
			payload, _ := json.Marshal(map[string]any{
				"quantity": qty,
				"price":    price,
			})
			url := fmt.Sprintf("%s/api/v1/orders/%s?account_id=%s", flagGatewayURL, orderID, accountID)
			body, err := doRequest("PUT", url, bytes.NewReader(payload))
			if err != nil {
				return err
			}
			printResult(body, nil, nil)
			return nil
		},
	}

	cmd.Flags().StringVar(&orderID, "order-id", "", "order ID")
	cmd.Flags().StringVar(&accountID, "account-id", "", "account ID")
	cmd.Flags().Int64Var(&qty, "qty", 0, "new quantity")
	cmd.Flags().Float64Var(&price, "price", 0, "new price")
	_ = cmd.MarkFlagRequired("order-id")
	_ = cmd.MarkFlagRequired("account-id")

	return cmd
}

func newOrderListCmd() *cobra.Command {
	var accountID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List active orders",
		RunE: func(_ *cobra.Command, _ []string) error {
			url := fmt.Sprintf("%s/api/v1/orders?account_id=%s", flagGatewayURL, accountID)
			body, err := doGet(url)
			if err != nil {
				return err
			}
			printResult(body, []string{"ORDER_ID", "INSTRUMENT", "DIR", "TYPE", "LOTS", "PRICE", "STATUS"}, func(data any) [][]string {
				arr, ok := data.([]any)
				if !ok {
					return nil
				}
				var rows [][]string
				for _, item := range arr {
					m, _ := item.(map[string]any)
					rows = append(rows, []string{
						fmt.Sprint(m["order_id"]),
						fmt.Sprint(m["instrument_uid"]),
						fmt.Sprint(m["direction"]),
						fmt.Sprint(m["order_type"]),
						fmt.Sprint(m["lots"]),
						fmt.Sprint(m["price"]),
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

func newOrderStateCmd() *cobra.Command {
	var orderID, accountID string

	cmd := &cobra.Command{
		Use:   "state",
		Short: "Get state of a specific order",
		RunE: func(_ *cobra.Command, _ []string) error {
			url := fmt.Sprintf("%s/api/v1/orders/%s?account_id=%s", flagGatewayURL, orderID, accountID)
			body, err := doGet(url)
			if err != nil {
				return err
			}
			printResult(body, []string{"ORDER_ID", "STATUS", "LOTS_REQ", "LOTS_EXEC", "DIRECTION", "TYPE", "INSTRUMENT"}, func(data any) [][]string {
				m, _ := data.(map[string]any)
				return [][]string{{
					fmt.Sprint(m["order_id"]),
					fmt.Sprint(m["execution_status"]),
					fmt.Sprint(m["lots_requested"]),
					fmt.Sprint(m["lots_executed"]),
					fmt.Sprint(m["direction"]),
					fmt.Sprint(m["order_type"]),
					fmt.Sprint(m["instrument_uid"]),
				}}
			})
			return nil
		},
	}

	cmd.Flags().StringVar(&orderID, "order-id", "", "order ID")
	cmd.Flags().StringVar(&accountID, "account-id", "", "account ID")
	_ = cmd.MarkFlagRequired("order-id")
	_ = cmd.MarkFlagRequired("account-id")

	return cmd
}
