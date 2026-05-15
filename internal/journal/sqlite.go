package journal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// SQLite implements Journal on a local SQLite file.
type SQLite struct {
	db *sql.DB
}

// OpenSQLite opens (or creates) a SQLite journal at path.
func OpenSQLite(path string) (*SQLite, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("journal sqlite open: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(30 * time.Minute)

	schema := `
CREATE TABLE IF NOT EXISTS signals (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ts INTEGER NOT NULL,
  instance_id TEXT NOT NULL,
  instrument_uid TEXT NOT NULL,
  direction TEXT NOT NULL,
  quantity INTEGER NOT NULL,
  order_type TEXT,
  ref_price REAL,
  reason TEXT
);
CREATE TABLE IF NOT EXISTS orders (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ts INTEGER NOT NULL,
  instance_id TEXT NOT NULL,
  order_id TEXT NOT NULL UNIQUE,
  instrument_uid TEXT NOT NULL,
  direction TEXT NOT NULL,
  quantity INTEGER NOT NULL,
  order_type TEXT,
  ref_price REAL
);
CREATE TABLE IF NOT EXISTS executions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ts INTEGER NOT NULL,
  instance_id TEXT NOT NULL,
  order_id TEXT NOT NULL,
  instrument_uid TEXT NOT NULL,
  status TEXT NOT NULL,
  filled_qty INTEGER NOT NULL,
  avg_price REAL NOT NULL,
  message TEXT
);
CREATE TABLE IF NOT EXISTS strategy_state (
  instance_id TEXT PRIMARY KEY,
  ts INTEGER NOT NULL,
  state BLOB NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_exec_instance_ts ON executions(instance_id, ts DESC);
CREATE INDEX IF NOT EXISTS idx_orders_instance_ts ON orders(instance_id, ts DESC);
`
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("journal sqlite schema: %w", err)
	}
	return &SQLite{db: db}, nil
}

func (s *SQLite) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLite) RecordSignal(ctx context.Context, r SignalRecord) error {
	ts := r.CreatedAt
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO signals (ts, instance_id, instrument_uid, direction, quantity, order_type, ref_price, reason)
		VALUES (?,?,?,?,?,?,?,?)`,
		ts.UnixMilli(), r.InstanceID, r.InstrumentUID, r.Direction, r.Quantity, r.OrderType, r.RefPrice, r.Reason)
	return err
}

func (s *SQLite) RecordOrder(ctx context.Context, r OrderRecord) error {
	ts := r.CreatedAt
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO orders (ts, instance_id, order_id, instrument_uid, direction, quantity, order_type, ref_price)
		VALUES (?,?,?,?,?,?,?,?)`,
		ts.UnixMilli(), r.InstanceID, r.OrderID, r.InstrumentUID, r.Direction, r.Quantity, r.OrderType, r.RefPrice)
	return err
}

func (s *SQLite) RecordExecution(ctx context.Context, r ExecutionRecord) error {
	ts := r.CreatedAt
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO executions (ts, instance_id, order_id, instrument_uid, status, filled_qty, avg_price, message)
		VALUES (?,?,?,?,?,?,?,?)`,
		ts.UnixMilli(), r.InstanceID, r.OrderID, r.InstrumentUID, r.Status, r.FilledQty, r.AvgPrice, r.Message)
	return err
}

func (s *SQLite) ListRecentExecutions(ctx context.Context, instanceID string, limit int) ([]ExecutionRecord, error) {
	if limit <= 0 || limit > 5000 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT ts, instance_id, order_id, instrument_uid, status, filled_qty, avg_price, message
		FROM executions WHERE instance_id = ? ORDER BY id DESC LIMIT ?`, instanceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ExecutionRecord
	for rows.Next() {
		var tsMs int64
		var r ExecutionRecord
		if err := rows.Scan(&tsMs, &r.InstanceID, &r.OrderID, &r.InstrumentUID, &r.Status, &r.FilledQty, &r.AvgPrice, &r.Message); err != nil {
			return nil, err
		}
		r.CreatedAt = time.UnixMilli(tsMs).UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *SQLite) DailySummary(ctx context.Context, day time.Time) (DailySummary, error) {
	day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	start := day.UnixMilli()
	end := day.Add(24 * time.Hour).UnixMilli()

	var sum DailySummary
	sum.DayUTC = day

	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM signals WHERE ts >= ? AND ts < ?`, start, end).Scan(&sum.SignalsCount); err != nil {
		return sum, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders WHERE ts >= ? AND ts < ?`, start, end).Scan(&sum.OrdersCount); err != nil {
		return sum, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM executions WHERE ts >= ? AND ts < ?`, start, end).Scan(&sum.ExecutionsCount); err != nil {
		return sum, err
	}
	return sum, nil
}

func (s *SQLite) SaveStrategyState(ctx context.Context, instanceID string, state []byte) error {
	if instanceID == "" {
		return nil
	}
	ts := time.Now().UTC().UnixMilli()
	_, err := s.db.ExecContext(ctx, `INSERT INTO strategy_state(instance_id, ts, state) VALUES(?,?,?)
		ON CONFLICT(instance_id) DO UPDATE SET ts=excluded.ts, state=excluded.state`,
		instanceID, ts, state)
	return err
}

func (s *SQLite) LoadStrategyState(ctx context.Context, instanceID string) ([]byte, error) {
	if instanceID == "" {
		return nil, nil
	}
	var blob []byte
	err := s.db.QueryRowContext(ctx, `SELECT state FROM strategy_state WHERE instance_id = ?`, instanceID).Scan(&blob)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return blob, nil
}

var _ Journal = (*SQLite)(nil)
