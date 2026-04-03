package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	flagConfig     string
	flagOutput     string
	flagGatewayURL string
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "24alert",
		Short: "24Alert trading bot CLI",
		Long:  "Command-line interface for the 24Alert trading gateway.\nIf no subcommand is given, the HTTP gateway server starts.",
	}

	root.PersistentFlags().StringVar(&flagConfig, "config", "config/config.yaml", "path to config file")
	root.PersistentFlags().StringVar(&flagOutput, "output", "table", "output format: table or json")
	root.PersistentFlags().StringVar(&flagGatewayURL, "gateway-url", "http://localhost:8080", "gateway base URL")

	root.AddCommand(
		newOrderCmd(),
		newStopCmd(),
		newMarketCmd(),
		newPortfolioCmd(),
		newAccountCmd(),
		newRiskCmd(),
	)

	return root
}

func doGet(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func doRequest(method, url string, bodyReader io.Reader) ([]byte, error) {
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func printResult(body []byte, headers []string, rowFn func(data any) [][]string) {
	if flagOutput == "json" {
		fmt.Println(string(body))
		return
	}

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		fmt.Println(string(body))
		return
	}

	if rowFn != nil && headers != nil {
		var data any
		if err := json.Unmarshal(envelope.Data, &data); err == nil {
			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			for _, h := range headers {
				fmt.Fprintf(tw, "%s\t", h)
			}
			fmt.Fprintln(tw)
			for _, row := range rowFn(data) {
				for _, cell := range row {
					fmt.Fprintf(tw, "%s\t", cell)
				}
				fmt.Fprintln(tw)
			}
			tw.Flush()
			return
		}
	}

	var indented []byte
	indented, _ = json.MarshalIndent(envelope.Data, "", "  ")
	fmt.Println(string(indented))
}
