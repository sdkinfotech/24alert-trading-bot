package strategy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

// notifyAdvisorRegister informs advisor-svc about a new AI trader session (non-blocking).
func notifyAdvisorRegister(s *AITraderSession) {
	if s == nil {
		return
	}
	body, _ := json.Marshal(map[string]string{
		"session_id":     s.ID,
		"account_id":     s.AccountID,
		"instrument_uid": s.InstrumentID,
		"ticker":         s.Ticker,
		"mode":           s.Mode,
		"instruction":    s.Instruction,
		"started_at":     s.StartedAt,
	})
	go advisorPOST("/advisor/sessions/register", body)
}

// notifyAdvisorFinalize triggers day/strategy rollup when session stops.
func notifyAdvisorFinalize(sessionID string) {
	if strings.TrimSpace(sessionID) == "" {
		return
	}
	go advisorPOST("/advisor/sessions/"+sessionID+"/finalize", []byte("{}"))
}

func advisorPOST(path string, body []byte) {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("ADVISOR_URL")), "/")
	if base == "" {
		base = "http://advisor-svc:9030"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+path, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Debug("advisor notify failed", "path", path, "error", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		slog.Debug("advisor notify bad status", "path", path, "status", resp.StatusCode, "body", strings.TrimSpace(string(b)))
	}
}
