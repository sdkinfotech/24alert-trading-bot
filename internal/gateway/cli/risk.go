package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRiskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "risk",
		Short: "Risk management commands",
	}

	cmd.AddCommand(
		newRiskStatusCmd(),
		newRiskResetCmd(),
	)
	return cmd
}

func newRiskStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Get risk subsystem status",
		RunE: func(_ *cobra.Command, _ []string) error {
			body, err := doGet(flagGatewayURL + "/api/v1/risk/status")
			if err != nil {
				return err
			}
			printResult(body, []string{"CB_TRIPPED", "FAILURES", "LAST_FAILURE", "THRESHOLD", "COOLDOWN"}, func(data any) [][]string {
				m, _ := data.(map[string]any)
				return [][]string{{
					fmt.Sprint(m["circuit_breaker_tripped"]),
					fmt.Sprint(m["failure_count"]),
					fmt.Sprint(m["last_failure"]),
					fmt.Sprint(m["threshold"]),
					fmt.Sprint(m["cooldown"]),
				}}
			})
			return nil
		},
	}
	return cmd
}

func newRiskResetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset circuit breaker",
		RunE: func(_ *cobra.Command, _ []string) error {
			body, err := doRequest("POST", flagGatewayURL+"/api/v1/risk/reset", nil)
			if err != nil {
				return err
			}
			printResult(body, nil, nil)
			return nil
		},
	}
	return cmd
}
