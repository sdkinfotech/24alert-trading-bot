package cli

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

func newPortfolioCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "portfolio",
		Short: "Portfolio and operations commands",
	}

	cmd.AddCommand(
		newPortfolioPositionsCmd(),
		newPortfolioInfoCmd(),
		newPortfolioLimitsCmd(),
		newPortfolioOpsCmd(),
	)
	return cmd
}

func newPortfolioPositionsCmd() *cobra.Command {
	var accountID string

	cmd := &cobra.Command{
		Use:   "positions",
		Short: "Get positions",
		RunE: func(_ *cobra.Command, _ []string) error {
			reqURL := fmt.Sprintf("%s/api/v1/positions?account_id=%s", flagGatewayURL, accountID)
			body, err := doGet(reqURL)
			if err != nil {
				return err
			}
			printResult(body, []string{"INSTRUMENT", "TYPE", "QTY", "AVG_PRICE", "CUR_PRICE", "YIELD", "CURRENCY"}, func(data any) [][]string {
				arr, ok := data.([]any)
				if !ok {
					return nil
				}
				var rows [][]string
				for _, item := range arr {
					m, _ := item.(map[string]any)
					rows = append(rows, []string{
						fmt.Sprint(m["instrument_uid"]),
						fmt.Sprint(m["instrument_type"]),
						fmt.Sprint(m["quantity"]),
						fmt.Sprint(m["average_price"]),
						fmt.Sprint(m["current_price"]),
						fmt.Sprint(m["expected_yield"]),
						fmt.Sprint(m["currency"]),
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

func newPortfolioInfoCmd() *cobra.Command {
	var accountID string

	cmd := &cobra.Command{
		Use:   "info",
		Short: "Get portfolio summary",
		RunE: func(_ *cobra.Command, _ []string) error {
			reqURL := fmt.Sprintf("%s/api/v1/portfolio?account_id=%s", flagGatewayURL, accountID)
			body, err := doGet(reqURL)
			if err != nil {
				return err
			}
			printResult(body, nil, nil)
			return nil
		},
	}

	cmd.Flags().StringVar(&accountID, "account-id", "", "account ID")
	_ = cmd.MarkFlagRequired("account-id")

	return cmd
}

func newPortfolioLimitsCmd() *cobra.Command {
	var accountID string

	cmd := &cobra.Command{
		Use:   "limits",
		Short: "Get withdrawal limits",
		RunE: func(_ *cobra.Command, _ []string) error {
			reqURL := fmt.Sprintf("%s/api/v1/limits?account_id=%s", flagGatewayURL, accountID)
			body, err := doGet(reqURL)
			if err != nil {
				return err
			}
			printResult(body, []string{"CURRENCY", "WITHDRAW", "BLOCKED"}, func(data any) [][]string {
				arr, ok := data.([]any)
				if !ok {
					return nil
				}
				var rows [][]string
				for _, item := range arr {
					m, _ := item.(map[string]any)
					rows = append(rows, []string{
						fmt.Sprint(m["currency"]),
						fmt.Sprint(m["withdraw_amount"]),
						fmt.Sprint(m["blocked_amount"]),
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

func newPortfolioOpsCmd() *cobra.Command {
	var (
		accountID  string
		instrument string
		from       string
		to         string
	)

	cmd := &cobra.Command{
		Use:   "ops",
		Short: "Get operations history",
		RunE: func(_ *cobra.Command, _ []string) error {
			params := url.Values{}
			params.Set("account_id", accountID)
			if instrument != "" {
				params.Set("instrument_uid", instrument)
			}
			if from != "" {
				params.Set("from", from)
			}
			if to != "" {
				params.Set("to", to)
			}
			reqURL := fmt.Sprintf("%s/api/v1/operations?%s", flagGatewayURL, params.Encode())
			body, err := doGet(reqURL)
			if err != nil {
				return err
			}
			printResult(body, []string{"ID", "TYPE", "STATE", "INSTRUMENT", "PAYMENT", "CURRENCY", "QTY", "DATE"}, func(data any) [][]string {
				m, _ := data.(map[string]any)
				ops, _ := m["operations"].([]any)
				var rows [][]string
				for _, item := range ops {
					o, _ := item.(map[string]any)
					rows = append(rows, []string{
						fmt.Sprint(o["id"]),
						fmt.Sprint(o["type"]),
						fmt.Sprint(o["state"]),
						fmt.Sprint(o["instrument_uid"]),
						fmt.Sprint(o["payment"]),
						fmt.Sprint(o["currency"]),
						fmt.Sprint(o["quantity"]),
						fmt.Sprint(o["date"]),
					})
				}
				return rows
			})
			return nil
		},
	}

	cmd.Flags().StringVar(&accountID, "account-id", "", "account ID")
	cmd.Flags().StringVar(&instrument, "instrument", "", "instrument UID")
	cmd.Flags().StringVar(&from, "from", "", "start time RFC3339")
	cmd.Flags().StringVar(&to, "to", "", "end time RFC3339")
	_ = cmd.MarkFlagRequired("account-id")

	return cmd
}
