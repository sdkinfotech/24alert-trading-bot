package advisor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// RunnerClient fetches AI trader sessions from strategy-runner.
type RunnerClient struct {
	base   string
	client *http.Client
}

func NewRunnerClient() *RunnerClient {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("STRATEGY_RUNNER_URL")), "/")
	if base == "" {
		base = "http://24alert-strategy-runner:9020"
	}
	return &RunnerClient{
		base: base,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *RunnerClient) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/ai-trader/sessions/"+sessionID, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("runner %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var s Session
	if err := json.Unmarshal(body, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (c *RunnerClient) ListSessions(ctx context.Context) ([]Session, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/ai-trader/sessions", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("runner %d", resp.StatusCode)
	}
	var out []Session
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}
