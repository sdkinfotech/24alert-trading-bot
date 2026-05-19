package advisor

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// Store persists advisor memory in SQLite.
type Store struct {
	db *sql.DB
}

func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS advisor_sessions (
  session_id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  instrument_uid TEXT NOT NULL,
  ticker TEXT,
  mode TEXT,
  instruction TEXT,
  status TEXT NOT NULL DEFAULT 'running',
  started_at_ms INTEGER NOT NULL,
  stopped_at_ms INTEGER,
  updated_at_ms INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS micro_snapshots (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  session_id TEXT NOT NULL,
  captured_at_ms INTEGER NOT NULL,
  payload_json TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_micro_session_ts ON micro_snapshots(session_id, captured_at_ms DESC);

CREATE TABLE IF NOT EXISTS analysis_reports (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  timeframe TEXT NOT NULL,
  period_start_ms INTEGER NOT NULL,
  period_end_ms INTEGER NOT NULL,
  status TEXT NOT NULL,
  summary_md TEXT,
  structured_json TEXT,
  source_ids_json TEXT,
  model TEXT,
  prompt_version TEXT,
  error_message TEXT,
  created_at_ms INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_reports_session_tf_period ON analysis_reports(session_id, timeframe, period_end_ms);
CREATE INDEX IF NOT EXISTS idx_reports_session_tf ON analysis_reports(session_id, timeframe, period_end_ms DESC);

CREATE TABLE IF NOT EXISTS scheduler_state (
  session_id TEXT NOT NULL,
  timeframe TEXT NOT NULL,
  last_period_end_ms INTEGER NOT NULL,
  PRIMARY KEY (session_id, timeframe)
);

CREATE TABLE IF NOT EXISTS strategy_drafts (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  title TEXT NOT NULL,
  body TEXT NOT NULL,
  ticker TEXT,
  instrument_uid TEXT,
  created_at_ms INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS strategy_synthesis (
  session_id TEXT PRIMARY KEY,
  summary_md TEXT,
  structured_json TEXT,
  model TEXT,
  created_at_ms INTEGER NOT NULL
);
`
	_, err := s.db.Exec(schema)
	return err
}

func (s *Store) UpsertSession(ctx context.Context, sessionID, accountID, uid, ticker, mode, instruction, status string, startedAt time.Time) error {
	now := time.Now().UTC().UnixMilli()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO advisor_sessions (session_id, account_id, instrument_uid, ticker, mode, instruction, status, started_at_ms, updated_at_ms)
VALUES (?,?,?,?,?,?,?,?,?)
ON CONFLICT(session_id) DO UPDATE SET
  status=excluded.status,
  instruction=excluded.instruction,
  updated_at_ms=excluded.updated_at_ms`,
		sessionID, accountID, uid, ticker, mode, instruction, status, startedAt.UnixMilli(), now)
	return err
}

func (s *Store) MarkSessionStopped(ctx context.Context, sessionID string, stoppedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE advisor_sessions SET status='stopped', stopped_at_ms=?, updated_at_ms=? WHERE session_id=?`,
		stoppedAt.UnixMilli(), time.Now().UTC().UnixMilli(), sessionID)
	return err
}

func (s *Store) ListActiveSessions(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT session_id FROM advisor_sessions WHERE status='running'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) GetSession(ctx context.Context, sessionID string) (accountID, uid, ticker, instruction string, err error) {
	err = s.db.QueryRowContext(ctx, `
SELECT account_id, instrument_uid, COALESCE(ticker,''), COALESCE(instruction,'') FROM advisor_sessions WHERE session_id=?`,
		sessionID).Scan(&accountID, &uid, &ticker, &instruction)
	return
}

func (s *Store) InsertSnapshot(ctx context.Context, sessionID string, capturedAt time.Time, payload []byte) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO micro_snapshots (session_id, captured_at_ms, payload_json) VALUES (?,?,?)`,
		sessionID, capturedAt.UnixMilli(), string(payload))
	return err
}

func (s *Store) SnapshotsInRange(ctx context.Context, sessionID string, start, end time.Time) ([]MicroSnapshot, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, session_id, captured_at_ms, payload_json FROM micro_snapshots
WHERE session_id=? AND captured_at_ms >= ? AND captured_at_ms < ?
ORDER BY captured_at_ms ASC`,
		sessionID, start.UnixMilli(), end.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MicroSnapshot
	for rows.Next() {
		var snap MicroSnapshot
		var ms int64
		if err := rows.Scan(&snap.ID, &snap.SessionID, &ms, &snap.PayloadJSON); err != nil {
			return nil, err
		}
		snap.CapturedAt = time.UnixMilli(ms).UTC()
		out = append(out, snap)
	}
	return out, rows.Err()
}

func (s *Store) LastSnapshotAt(ctx context.Context, sessionID string) (time.Time, bool, error) {
	var ms sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
SELECT MAX(captured_at_ms) FROM micro_snapshots WHERE session_id=?`, sessionID).Scan(&ms)
	if err != nil || !ms.Valid {
		return time.Time{}, false, err
	}
	return time.UnixMilli(ms.Int64).UTC(), true, nil
}

func (s *Store) GetLastPeriodEnd(ctx context.Context, sessionID string, tf Timeframe) (time.Time, bool, error) {
	var ms int64
	err := s.db.QueryRowContext(ctx, `
SELECT last_period_end_ms FROM scheduler_state WHERE session_id=? AND timeframe=?`,
		sessionID, string(tf)).Scan(&ms)
	if err == sql.ErrNoRows {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return time.UnixMilli(ms).UTC(), true, nil
}

func (s *Store) SetLastPeriodEnd(ctx context.Context, sessionID string, tf Timeframe, end time.Time) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO scheduler_state (session_id, timeframe, last_period_end_ms) VALUES (?,?,?)
ON CONFLICT(session_id, timeframe) DO UPDATE SET last_period_end_ms=excluded.last_period_end_ms`,
		sessionID, string(tf), end.UnixMilli())
	return err
}

func (s *Store) GetReportByPeriod(ctx context.Context, sessionID string, tf Timeframe, periodEnd time.Time) (*AnalysisReport, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, session_id, timeframe, period_start_ms, period_end_ms, status, summary_md,
  structured_json, source_ids_json, model, prompt_version, error_message, created_at_ms
FROM analysis_reports WHERE session_id=? AND timeframe=? AND period_end_ms=?`,
		sessionID, string(tf), periodEnd.UnixMilli())
	return scanReportRow(row)
}

func (s *Store) ListFailedReports(ctx context.Context, olderThan time.Time, limit int) ([]AnalysisReport, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, session_id, timeframe, period_start_ms, period_end_ms, status, summary_md,
  structured_json, source_ids_json, model, prompt_version, error_message, created_at_ms
FROM analysis_reports WHERE status=? AND created_at_ms < ?
ORDER BY created_at_ms ASC LIMIT ?`,
		ReportStatusFailed, olderThan.UnixMilli(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReports(rows)
}

func (s *Store) ListFailedReportsForRunning(ctx context.Context, olderThan time.Time, limit int) ([]AnalysisReport, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT r.id, r.session_id, r.timeframe, r.period_start_ms, r.period_end_ms, r.status, r.summary_md,
  r.structured_json, r.source_ids_json, r.model, r.prompt_version, r.error_message, r.created_at_ms
FROM analysis_reports r
INNER JOIN advisor_sessions s ON s.session_id = r.session_id
WHERE r.status=? AND r.created_at_ms < ? AND s.status='running'
ORDER BY r.created_at_ms ASC LIMIT ?`,
		ReportStatusFailed, olderThan.UnixMilli(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReports(rows)
}

func (s *Store) GetSessionStatus(ctx context.Context, sessionID string) (string, error) {
	var status string
	err := s.db.QueryRowContext(ctx, `SELECT status FROM advisor_sessions WHERE session_id=?`, sessionID).Scan(&status)
	return status, err
}

func (s *Store) DeleteDraftsForSession(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM strategy_drafts WHERE session_id=?`, sessionID)
	return err
}

func (s *Store) ReportExists(ctx context.Context, sessionID string, tf Timeframe, periodEnd time.Time) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
SELECT COUNT(1) FROM analysis_reports WHERE session_id=? AND timeframe=? AND period_end_ms=?`,
		sessionID, string(tf), periodEnd.UnixMilli()).Scan(&n)
	return n > 0, err
}

func (s *Store) InsertReport(ctx context.Context, rep *AnalysisReport) error {
	if rep.ID == "" {
		rep.ID = uuid.NewString()
	}
	srcJSON, _ := json.Marshal(rep.SourceReportIDs)
	structJSON, _ := json.Marshal(rep.Structured)
	_, err := s.db.ExecContext(ctx, `
INSERT OR REPLACE INTO analysis_reports (
  id, session_id, timeframe, period_start_ms, period_end_ms, status, summary_md,
  structured_json, source_ids_json, model, prompt_version, error_message, created_at_ms
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		rep.ID, rep.SessionID, string(rep.Timeframe), rep.PeriodStart.UnixMilli(), rep.PeriodEnd.UnixMilli(),
		rep.Status, rep.SummaryMD, string(structJSON), string(srcJSON), rep.Model, rep.PromptVersion,
		rep.ErrorMessage, rep.CreatedAt.UnixMilli())
	return err
}

func (s *Store) ListReports(ctx context.Context, sessionID string, tf Timeframe, limit int) ([]AnalysisReport, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, session_id, timeframe, period_start_ms, period_end_ms, status, summary_md,
  structured_json, source_ids_json, model, prompt_version, error_message, created_at_ms
FROM analysis_reports WHERE session_id=? AND timeframe=?
ORDER BY period_end_ms DESC LIMIT ?`,
		sessionID, string(tf), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReports(rows)
}

func (s *Store) ReportsInRange(ctx context.Context, sessionID string, tf Timeframe, start, end time.Time) ([]AnalysisReport, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, session_id, timeframe, period_start_ms, period_end_ms, status, summary_md,
  structured_json, source_ids_json, model, prompt_version, error_message, created_at_ms
FROM analysis_reports WHERE session_id=? AND timeframe=? AND period_end_ms > ? AND period_end_ms <= ?
ORDER BY period_end_ms ASC`,
		sessionID, string(tf), start.UnixMilli(), end.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReports(rows)
}

func (s *Store) GetReport(ctx context.Context, reportID string) (*AnalysisReport, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, session_id, timeframe, period_start_ms, period_end_ms, status, summary_md,
  structured_json, source_ids_json, model, prompt_version, error_message, created_at_ms
FROM analysis_reports WHERE id=?`, reportID)
	return scanReportRow(row)
}

func scanReports(rows *sql.Rows) ([]AnalysisReport, error) {
	var out []AnalysisReport
	for rows.Next() {
		rep, err := scanReportFromRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *rep)
	}
	return out, rows.Err()
}

func scanReportFromRows(rows *sql.Rows) (*AnalysisReport, error) {
	var rep AnalysisReport
	var tf string
	var ps, pe, ca int64
	var structJSON, srcJSON string
	if err := rows.Scan(&rep.ID, &rep.SessionID, &tf, &ps, &pe, &rep.Status, &rep.SummaryMD,
		&structJSON, &srcJSON, &rep.Model, &rep.PromptVersion, &rep.ErrorMessage, &ca); err != nil {
		return nil, err
	}
	rep.Timeframe = Timeframe(tf)
	rep.PeriodStart = time.UnixMilli(ps).UTC()
	rep.PeriodEnd = time.UnixMilli(pe).UTC()
	rep.CreatedAt = time.UnixMilli(ca).UTC()
	_ = json.Unmarshal([]byte(structJSON), &rep.Structured)
	_ = json.Unmarshal([]byte(srcJSON), &rep.SourceReportIDs)
	return &rep, nil
}

func scanReportRow(row *sql.Row) (*AnalysisReport, error) {
	var rep AnalysisReport
	var tf string
	var ps, pe, ca int64
	var structJSON, srcJSON string
	if err := row.Scan(&rep.ID, &rep.SessionID, &tf, &ps, &pe, &rep.Status, &rep.SummaryMD,
		&structJSON, &srcJSON, &rep.Model, &rep.PromptVersion, &rep.ErrorMessage, &ca); err != nil {
		return nil, err
	}
	rep.Timeframe = Timeframe(tf)
	rep.PeriodStart = time.UnixMilli(ps).UTC()
	rep.PeriodEnd = time.UnixMilli(pe).UTC()
	rep.CreatedAt = time.UnixMilli(ca).UTC()
	_ = json.Unmarshal([]byte(structJSON), &rep.Structured)
	_ = json.Unmarshal([]byte(srcJSON), &rep.SourceReportIDs)
	return &rep, nil
}

func (s *Store) InsertDraft(ctx context.Context, d *StrategyDraft) error {
	if d.ID == "" {
		d.ID = uuid.NewString()
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO strategy_drafts (id, session_id, kind, title, body, ticker, instrument_uid, created_at_ms)
VALUES (?,?,?,?,?,?,?,?)`,
		d.ID, d.SessionID, d.Kind, d.Title, d.Body, d.Ticker, d.InstrumentUID, d.CreatedAt.UnixMilli())
	return err
}

func (s *Store) ListDrafts(ctx context.Context, sessionID string) ([]StrategyDraft, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, session_id, kind, title, body, ticker, instrument_uid, created_at_ms
FROM strategy_drafts WHERE session_id=? ORDER BY created_at_ms ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StrategyDraft
	for rows.Next() {
		var d StrategyDraft
		var ca int64
		if err := rows.Scan(&d.ID, &d.SessionID, &d.Kind, &d.Title, &d.Body, &d.Ticker, &d.InstrumentUID, &ca); err != nil {
			return nil, err
		}
		d.CreatedAt = time.UnixMilli(ca).UTC()
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) SaveSynthesis(ctx context.Context, syn *StrategySynthesis) error {
	structJSON, _ := json.Marshal(syn.Structured)
	_, err := s.db.ExecContext(ctx, `
INSERT INTO strategy_synthesis (session_id, summary_md, structured_json, model, created_at_ms)
VALUES (?,?,?,?,?)
ON CONFLICT(session_id) DO UPDATE SET summary_md=excluded.summary_md, structured_json=excluded.structured_json,
  model=excluded.model, created_at_ms=excluded.created_at_ms`,
		syn.SessionID, syn.SummaryMD, string(structJSON), syn.Model, syn.CreatedAt.UnixMilli())
	return err
}

func (s *Store) GetSynthesis(ctx context.Context, sessionID string) (*StrategySynthesis, error) {
	var syn StrategySynthesis
	var structJSON string
	var ca int64
	err := s.db.QueryRowContext(ctx, `
SELECT session_id, summary_md, structured_json, model, created_at_ms FROM strategy_synthesis WHERE session_id=?`,
		sessionID).Scan(&syn.SessionID, &syn.SummaryMD, &structJSON, &syn.Model, &ca)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	syn.CreatedAt = time.UnixMilli(ca).UTC()
	_ = json.Unmarshal([]byte(structJSON), &syn.Structured)
	syn.Drafts, _ = s.ListDrafts(ctx, sessionID)
	return &syn, nil
}

// SearchKnowledge lists reports for instrument across sessions (basic).
func (s *Store) SearchKnowledge(ctx context.Context, instrumentUID string, from, to time.Time, limit int) ([]AnalysisReport, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT r.id, r.session_id, r.timeframe, r.period_start_ms, r.period_end_ms, r.status, r.summary_md,
  r.structured_json, r.source_ids_json, r.model, r.prompt_version, r.error_message, r.created_at_ms
FROM analysis_reports r
JOIN advisor_sessions s ON s.session_id = r.session_id
WHERE s.instrument_uid=? AND r.period_end_ms >= ? AND r.period_end_ms <= ? AND r.status=?
ORDER BY r.period_end_ms DESC LIMIT ?`,
		instrumentUID, from.UnixMilli(), to.UnixMilli(), ReportStatusOK, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReports(rows)
}

func ParseStartedAt(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Now().UTC()
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC()
	}
	return time.Now().UTC()
}

func FormatError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if len(msg) > 500 {
		return msg[:500]
	}
	return msg
}
