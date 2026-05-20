package tradeanalyst

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

// Store persists trade rounds, session reports, and per-instrument hints.
type Store struct {
	mu   sync.Mutex
	db   *sql.DB
	path string
}

func dbPath() string {
	if p := os.Getenv("AI_TRADER_ANALYST_DB"); p != "" {
		return p
	}
	return filepath.Join("data", "ai_trader_trade_analyst.db")
}

func journalPath() string {
	if p := os.Getenv("AI_TRADER_JOURNAL_PATH"); p != "" {
		return p
	}
	return filepath.Join("data", "ai_trader_journal.jsonl")
}

// OpenStore opens or creates the analyst SQLite database.
func OpenStore() (*Store, error) {
	path := dbPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, path: path}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS trade_rounds (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  ticker TEXT NOT NULL,
  instrument_uid TEXT,
  payload_json TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_trade_rounds_session ON trade_rounds(session_id);
CREATE INDEX IF NOT EXISTS idx_trade_rounds_ticker ON trade_rounds(ticker);

CREATE TABLE IF NOT EXISTS session_reports (
  session_id TEXT PRIMARY KEY,
  ticker TEXT NOT NULL,
  generated_at TEXT NOT NULL,
  payload_json TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_session_reports_ticker ON session_reports(ticker, generated_at);

CREATE TABLE IF NOT EXISTS instrument_stats (
  ticker TEXT PRIMARY KEY,
  payload_json TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS trading_hints (
  ticker TEXT PRIMARY KEY,
  payload_json TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
`)
	return err
}

func (s *Store) SaveReport(rep *SessionReport) error {
	if rep == nil || rep.SessionID == "" {
		return fmt.Errorf("empty report")
	}
	b, err := json.Marshal(rep)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err = s.db.Exec(
		`INSERT INTO session_reports (session_id, ticker, generated_at, payload_json) VALUES (?,?,?,?)
		 ON CONFLICT(session_id) DO UPDATE SET ticker=excluded.ticker, generated_at=excluded.generated_at, payload_json=excluded.payload_json`,
		rep.SessionID, rep.Ticker, rep.GeneratedAt, string(b),
	)
	if err != nil {
		return err
	}
	for _, tr := range rep.TradeRounds {
		tb, _ := json.Marshal(tr)
		_, _ = s.db.Exec(
			`INSERT INTO trade_rounds (id, session_id, ticker, instrument_uid, payload_json, created_at) VALUES (?,?,?,?,?,?)
			 ON CONFLICT(id) DO UPDATE SET payload_json=excluded.payload_json`,
			tr.ID, tr.SessionID, tr.Ticker, tr.InstrumentUID, string(tb), rep.GeneratedAt,
		)
	}
	return s.updateInstrumentStatsLocked(rep)
}

func (s *Store) updateInstrumentStatsLocked(rep *SessionReport) error {
	st, _ := s.getInstrumentStatsLocked(rep.Ticker)
	st.Ticker = rep.Ticker
	st.SessionsAnalyzed++
	st.TotalRounds += len(rep.TradeRounds)
	st.LastReportAt = rep.GeneratedAt
	if rep.WinRate > 0 {
		st.WinRate = (st.WinRate*float64(st.SessionsAnalyzed-1) + rep.WinRate) / float64(st.SessionsAnalyzed)
	}
	var holdSum float64
	for _, tr := range rep.TradeRounds {
		holdSum += tr.HoldMinutes
	}
	if len(rep.TradeRounds) > 0 {
		st.AvgHoldMinutes = (st.AvgHoldMinutes*float64(st.SessionsAnalyzed-1) + holdSum/float64(len(rep.TradeRounds))) / float64(st.SessionsAnalyzed)
	}
	st.AvgRealizedRUB = (st.AvgRealizedRUB*float64(st.SessionsAnalyzed-1) + rep.RealizedRUB) / float64(st.SessionsAnalyzed)
	st.Lessons = appendUnique(st.Lessons, rep.Recommendations...)
	if len(st.Lessons) > 20 {
		st.Lessons = st.Lessons[len(st.Lessons)-20:]
	}
	b, _ := json.Marshal(st)
	_, err := s.db.Exec(
		`INSERT INTO instrument_stats (ticker, payload_json, updated_at) VALUES (?,?,?)
		 ON CONFLICT(ticker) DO UPDATE SET payload_json=excluded.payload_json, updated_at=excluded.updated_at`,
		st.Ticker, string(b), time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

func appendUnique(dst []string, add ...string) []string {
	seen := make(map[string]bool)
	for _, x := range dst {
		seen[x] = true
	}
	for _, x := range add {
		x = trimNote(x)
		if x == "" || seen[x] {
			continue
		}
		seen[x] = true
		dst = append(dst, x)
	}
	return dst
}

func (s *Store) SaveHints(h *TradingHints) error {
	if h == nil || h.Ticker == "" {
		return fmt.Errorf("empty hints")
	}
	h.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	b, err := json.Marshal(h)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err = s.db.Exec(
		`INSERT INTO trading_hints (ticker, payload_json, updated_at) VALUES (?,?,?)
		 ON CONFLICT(ticker) DO UPDATE SET payload_json=excluded.payload_json, updated_at=excluded.updated_at`,
		h.Ticker, string(b), h.UpdatedAt,
	)
	return err
}

func (s *Store) GetHints(ticker string) (*TradingHints, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var raw string
	err := s.db.QueryRow(`SELECT payload_json FROM trading_hints WHERE ticker = ?`, ticker).Scan(&raw)
	if err != nil {
		return nil, false
	}
	var h TradingHints
	if json.Unmarshal([]byte(raw), &h) != nil {
		return nil, false
	}
	return &h, true
}

func (s *Store) GetReport(sessionID string) (*SessionReport, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var raw string
	err := s.db.QueryRow(`SELECT payload_json FROM session_reports WHERE session_id = ?`, sessionID).Scan(&raw)
	if err != nil {
		return nil, false
	}
	var rep SessionReport
	if json.Unmarshal([]byte(raw), &rep) != nil {
		return nil, false
	}
	return &rep, true
}

func (s *Store) ListReportsByTicker(ticker string, limit int) ([]SessionReport, error) {
	if limit <= 0 {
		limit = 20
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(
		`SELECT payload_json FROM session_reports WHERE ticker = ? ORDER BY generated_at DESC LIMIT ?`,
		ticker, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SessionReport
	for rows.Next() {
		var raw string
		if rows.Scan(&raw) != nil {
			continue
		}
		var rep SessionReport
		if json.Unmarshal([]byte(raw), &rep) == nil {
			out = append(out, rep)
		}
	}
	return out, nil
}

func (s *Store) ListRoundsByTicker(ticker string, limit int) ([]TradeRound, error) {
	if limit <= 0 {
		limit = 50
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(
		`SELECT payload_json FROM trade_rounds WHERE ticker = ? ORDER BY created_at DESC LIMIT ?`,
		ticker, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TradeRound
	for rows.Next() {
		var raw string
		if rows.Scan(&raw) != nil {
			continue
		}
		var tr TradeRound
		if json.Unmarshal([]byte(raw), &tr) == nil {
			out = append(out, tr)
		}
	}
	return out, nil
}

func (s *Store) getInstrumentStatsLocked(ticker string) (InstrumentStats, error) {
	var st InstrumentStats
	st.Ticker = ticker
	var raw string
	err := s.db.QueryRow(`SELECT payload_json FROM instrument_stats WHERE ticker = ?`, ticker).Scan(&raw)
	if err != nil {
		return st, err
	}
	_ = json.Unmarshal([]byte(raw), &st)
	return st, nil
}

func (s *Store) GetInstrumentStats(ticker string) (*InstrumentStats, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.getInstrumentStatsLocked(ticker)
	if err != nil {
		return nil, false
	}
	return &st, true
}

func (s *Store) JournalPath() string { return journalPath() }
