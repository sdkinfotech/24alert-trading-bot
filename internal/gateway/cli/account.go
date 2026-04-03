package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newAccountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Account commands",
	}

	cmd.AddCommand(
		newAccountListCmd(),
		newAccountMarginCmd(),
	)
	return cmd
}

func newAccountListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List accounts",
		RunE: func(_ *cobra.Command, _ []string) error {
			body, err := doGet(flagGatewayURL + "/api/v1/accounts")
			if err != nil {
				return err
			}
			printResult(body, []string{"ID", "TYPE", "NAME", "STATUS", "ACCESS_LEVEL", "OPENED"}, func(data any) [][]string {
				arr, ok := data.([]any)
				if !ok {
					return nil
				}
				var rows [][]string
				for _, item := range arr {
					m, _ := item.(map[string]any)
					rows = append(rows, []string{
						fmt.Sprint(m["id"]),
						fmt.Sprint(m["type"]),
						fmt.Sprint(m["name"]),
						fmt.Sprint(m["status"]),
						fmt.Sprint(m["access_level"]),
						fmt.Sprint(m["opened_date"]),
					})
				}
				return rows
			})
			return nil
		},
	}
	return cmd
}

func newAccountMarginCmd() *cobra.Command {
	var accountID string

	cmd := &cobra.Command{
		Use:   "margin",
		Short: "Get margin attributes",
		RunE: func(_ *cobra.Command, _ []string) error {
			reqURL := fmt.Sprintf("%s/api/v1/margin/%s", flagGatewayURL, accountID)
			body, err := doGet(reqURL)
			if err != nil {
				return err
			}
			printResult(body, []string{"LIQUID_PORTFOLIO", "STARTING_MARGIN", "MIN_MARGIN", "SUFFICIENCY", "MISSING", "CORRECTED"}, func(data any) [][]string {
				m, _ := data.(map[string]any)
				return [][]string{{
					fmt.Sprint(m["liquid_portfolio"]),
					fmt.Sprint(m["starting_margin"]),
					fmt.Sprint(m["minimal_margin"]),
					fmt.Sprint(m["funds_sufficiency_level"]),
					fmt.Sprint(m["amount_of_missing"]),
					fmt.Sprint(m["corrected_margin"]),
				}}
			})
			return nil
		},
	}

	cmd.Flags().StringVar(&accountID, "account-id", "", "account ID")
	_ = cmd.MarkFlagRequired("account-id")

	return cmd
}
