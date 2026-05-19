package strategy

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// AITraderRecorder persists full book+tape ticks for replay backtests.
type AITraderRecorder struct {
	mu   sync.Mutex
	db   *sql.DB
	path string
}

var globalAITraderRecorder *AITraderRecorder

func aiTraderRecorderPath() string {
	if p := os.Getenv("AI_TRADER_RECORD_DB"); p != "" {
		return p
	}
	return filepath.Join("data", "ai_trader_record.db")
}

func aiTraderRecorderEnabled() bool {
	v := os.Getenv("AI_TRADER_RECORD_ENABLED")
	return v == "1" || v == "true" || v == "yes"
}

func getAITraderRecorder() (*AITraderRecorder, error) {
	if !aiTraderRecorderEnabled() {
		return nil, nil
	}
	if globalAITraderRecorder != nil {
		return globalAITraderRecorder, nil
	}
	path := aiTraderRecorderPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS ai_trader_ticks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  session_id TEXT NOT NULL,
  instrument_uid TEXT NOT NULL,
  ticker TEXT,
  observed_at TEXT NOT NULL,
  kind TEXT NOT NULL,
  payload_json TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_ai_trader_ticks_session ON ai_trader_ticks(session_id, observed_at);
`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	globalAITraderRecorder = &AITraderRecorder{db: db, path: path}
	return globalAITraderRecorder, nil
}

func (rec *AITraderRecorder) RecordTick(sessionID, uid, ticker, kind string, payload any) error {
	if rec == nil || rec.db == nil {
		return nil
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	_, err = rec.db.Exec(
		`INSERT INTO ai_trader_ticks (session_id, instrument_uid, ticker, observed_at, kind, payload_json) VALUES (?,?,?,?,?,?)`,
		sessionID, uid, ticker, time.Now().UTC().Format(time.RFC3339Nano), kind, string(b),
	)
	return err
}

func (r *Runner) recordAITraderTick(s *AITraderSession, kind string, payload any) {
	rec, err := getAITraderRecorder()
	if err != nil || rec == nil || s == nil {
		return
	}
	_ = rec.RecordTick(s.ID, s.InstrumentID, s.Ticker, kind, payload)
}

// ReplayTicks loads ticks for a session in time order.
func ReplayTicks(sessionID string, limit int) ([]ReplayTick, error) {
	rec, err := getAITraderRecorder()
	if err != nil {
		return nil, err
	}
	if rec == nil || rec.db == nil {
		return nil, fmt.Errorf("recorder not enabled")
	}
	if limit <= 0 {
		limit = 100000
	}
	rows, err := rec.db.Query(
		`SELECT observed_at, kind, payload_json FROM ai_trader_ticks WHERE session_id=? ORDER BY id ASC LIMIT ?`,
		sessionID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ReplayTick
	for rows.Next() {
		var t ReplayTick
		if err := rows.Scan(&t.ObservedAt, &t.Kind, &t.PayloadJSON); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

type ReplayTick struct {
	ObservedAt  string `json:"observed_at"`
	Kind        string `json:"kind"`
	PayloadJSON string `json:"payload_json"`
}
