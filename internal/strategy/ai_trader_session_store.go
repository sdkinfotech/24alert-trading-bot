package strategy

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// aiTraderSessionSnapshot is persisted across runner restarts (no goroutine handles).
type aiTraderSessionSnapshot struct {
	ID            string                  `json:"id"`
	InstanceID    string                  `json:"instance_id,omitempty"`
	AccountID     string                  `json:"account_id"`
	InstrumentID  string                  `json:"instrument_uid"`
	Ticker        string                  `json:"ticker,omitempty"`
	StrategyKind  string                  `json:"strategy_kind"`
	Mode          string                  `json:"mode,omitempty"`
	Instruction   string                  `json:"instruction"`
	Limits        AITraderLimits          `json:"limits"`
	Status        string                  `json:"status"`
	Phase         string                  `json:"phase"`
	PhaseProgress AITraderPhaseProgress   `json:"phase_progress"`
	LevelPlaybook *LevelPlaybook          `json:"level_playbook,omitempty"`
	ActivePolicy  *DynamicTradingPolicy   `json:"active_policy,omitempty"`
	PaperState    *PaperTradingState      `json:"paper_state,omitempty"`
	ExecutionMode string                  `json:"execution_mode,omitempty"`
	LiveState     *LiveTradingState       `json:"live_state,omitempty"`
	StartedAt     string                  `json:"started_at"`
	UpdatedAt     string                  `json:"updated_at"`
	StoppedAt     string                  `json:"stopped_at,omitempty"`
	Features      *AITraderFeatures       `json:"features,omitempty"`
	MarketContext *AITraderMarketContext  `json:"market_context,omitempty"`
	LastDecision  *AITraderDecisionEvent  `json:"last_decision,omitempty"`
	Events        []AITraderDecisionEvent `json:"events,omitempty"`
	CollectFeed   []AITraderCollectEvent  `json:"collect_feed,omitempty"`
	LastTradeSignal *AITraderTradeSignal  `json:"last_trade_signal,omitempty"`
	SessionRegime   string                `json:"session_regime,omitempty"`
	MicroSignals    []AITraderMicroSignal `json:"micro_signals,omitempty"`
	LastPlaybookRefreshAt string          `json:"last_playbook_refresh_at,omitempty"`
	Depth                 int32             `json:"depth,omitempty"`
	ExecutionLog          []AITraderExecutionLogEntry `json:"execution_log,omitempty"`
}

type AITraderPersistedSummary struct {
	ID        string `json:"id"`
	Ticker    string `json:"ticker,omitempty"`
	Phase     string `json:"phase"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at"`
	AccountID string `json:"account_id"`
	InstrumentID string `json:"instrument_uid"`
}

type aiTraderSessionStore struct {
	mu   sync.Mutex
	db   *sql.DB
	path string
}

var globalAITraderSessionStore *aiTraderSessionStore

func aiTraderSessionsDBPath() string {
	if p := os.Getenv("AI_TRADER_SESSIONS_DB"); p != "" {
		return p
	}
	return filepath.Join("data", "ai_trader_sessions.db")
}

func aiTraderSessionStoreEnabled() bool {
	v := strings.TrimSpace(os.Getenv("AI_TRADER_SESSIONS_ENABLED"))
	if v == "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func getAITraderSessionStore() (*aiTraderSessionStore, error) {
	if !aiTraderSessionStoreEnabled() {
		return nil, nil
	}
	if globalAITraderSessionStore != nil {
		return globalAITraderSessionStore, nil
	}
	path := aiTraderSessionsDBPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS ai_trader_sessions (
  session_id TEXT PRIMARY KEY,
  status TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  snapshot_json TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_ai_trader_sessions_status ON ai_trader_sessions(status);
`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	globalAITraderSessionStore = &aiTraderSessionStore{db: db, path: path}
	return globalAITraderSessionStore, nil
}

func snapshotFromSession(s *AITraderSession, depth int32) *aiTraderSessionSnapshot {
	if s == nil {
		return nil
	}
	snap := &aiTraderSessionSnapshot{
		ID: s.ID, InstanceID: s.InstanceID, AccountID: s.AccountID, InstrumentID: s.InstrumentID,
		Ticker: s.Ticker, StrategyKind: s.StrategyKind, Mode: s.Mode, Instruction: s.Instruction,
		Limits: s.Limits, Status: s.Status, Phase: s.Phase, PhaseProgress: s.PhaseProgress,
		LevelPlaybook: s.LevelPlaybook, ActivePolicy: s.ActivePolicy, PaperState: s.PaperState,
		ExecutionMode: s.ExecutionMode, LiveState: s.LiveState, StartedAt: s.StartedAt,
		UpdatedAt: s.UpdatedAt, StoppedAt: s.StoppedAt, Features: s.Features,
		MarketContext: s.MarketContext, LastDecision: s.LastDecision,
		LastTradeSignal: s.LastTradeSignal, SessionRegime: s.SessionRegime,
		MicroSignals: s.MicroSignals, Depth: depth,
		ExecutionLog: append([]AITraderExecutionLogEntry(nil), s.ExecutionLog...),
	}
	if len(s.Events) > aiTraderMaxSessionEvents {
		snap.Events = append([]AITraderDecisionEvent(nil), s.Events[:aiTraderMaxSessionEvents]...)
	} else {
		snap.Events = append([]AITraderDecisionEvent(nil), s.Events...)
	}
	if len(s.CollectFeed) > 80 {
		snap.CollectFeed = append([]AITraderCollectEvent(nil), s.CollectFeed[:80]...)
	} else {
		snap.CollectFeed = append([]AITraderCollectEvent(nil), s.CollectFeed...)
	}
	if !s.lastPlaybookRefreshAt.IsZero() {
		snap.LastPlaybookRefreshAt = s.lastPlaybookRefreshAt.UTC().Format(time.RFC3339)
	}
	return snap
}

func (r *Runner) persistAITraderSessionSnapshot(s *AITraderSession, depth int32) {
	store, err := getAITraderSessionStore()
	if err != nil || store == nil || s == nil {
		return
	}
	if s.Status != "running" {
		return
	}
	snap := snapshotFromSession(s, depth)
	if snap == nil {
		return
	}
	b, err := json.Marshal(snap)
	if err != nil {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	store.mu.Lock()
	defer store.mu.Unlock()
	_, _ = store.db.Exec(
		`INSERT INTO ai_trader_sessions (session_id, status, updated_at, snapshot_json) VALUES (?,?,?,?)
		 ON CONFLICT(session_id) DO UPDATE SET status=excluded.status, updated_at=excluded.updated_at, snapshot_json=excluded.snapshot_json`,
		s.ID, s.Status, now, string(b),
	)
}

func (r *Runner) deleteAITraderSessionSnapshot(sessionID string) {
	store, err := getAITraderSessionStore()
	if err != nil || store == nil || sessionID == "" {
		return
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	_, _ = store.db.Exec(`DELETE FROM ai_trader_sessions WHERE session_id = ?`, sessionID)
}

func (r *Runner) listPersistedAITraderSessions() ([]AITraderPersistedSummary, error) {
	store, err := getAITraderSessionStore()
	if err != nil || store == nil {
		return nil, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	rows, err := store.db.Query(`SELECT session_id, status, updated_at, snapshot_json FROM ai_trader_sessions WHERE status = 'running' ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AITraderPersistedSummary
	for rows.Next() {
		var id, status, updated, raw string
		if err := rows.Scan(&id, &status, &updated, &raw); err != nil {
			continue
		}
		var snap aiTraderSessionSnapshot
		if json.Unmarshal([]byte(raw), &snap) != nil {
			continue
		}
		out = append(out, AITraderPersistedSummary{
			ID: id, Ticker: snap.Ticker, Phase: snap.Phase, Status: status,
			UpdatedAt: updated, AccountID: snap.AccountID, InstrumentID: snap.InstrumentID,
		})
	}
	return out, nil
}

func loadAITraderSessionSnapshot(sessionID string) (*aiTraderSessionSnapshot, error) {
	store, err := getAITraderSessionStore()
	if err != nil || store == nil {
		return nil, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	var raw string
	err = store.db.QueryRow(`SELECT snapshot_json FROM ai_trader_sessions WHERE session_id = ?`, sessionID).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var snap aiTraderSessionSnapshot
	if err := json.Unmarshal([]byte(raw), &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

func sessionFromSnapshot(snap *aiTraderSessionSnapshot) *AITraderSession {
	if snap == nil {
		return nil
	}
	s := &AITraderSession{
		ID: snap.ID, InstanceID: snap.InstanceID, AccountID: snap.AccountID,
		InstrumentID: snap.InstrumentID, Ticker: snap.Ticker, StrategyKind: snap.StrategyKind,
		Mode: snap.Mode, Instruction: snap.Instruction, Limits: snap.Limits,
		Status: "running", Phase: snap.Phase, PhaseProgress: snap.PhaseProgress,
		LevelPlaybook: snap.LevelPlaybook, ActivePolicy: snap.ActivePolicy,
		PaperState: snap.PaperState, ExecutionMode: snap.ExecutionMode, LiveState: snap.LiveState,
		StartedAt: snap.StartedAt, UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Features: snap.Features, MarketContext: snap.MarketContext, LastDecision: snap.LastDecision,
		Events: snap.Events, CollectFeed: snap.CollectFeed, LastTradeSignal: snap.LastTradeSignal,
		SessionRegime: snap.SessionRegime, MicroSignals: snap.MicroSignals,
		ExecutionLog: append([]AITraderExecutionLogEntry(nil), snap.ExecutionLog...),
		collectBuf: newAITraderCollectBuffer(), ctxState: newAITraderContextState(),
	}
	if snap.LastPlaybookRefreshAt != "" {
		if t, err := time.Parse(time.RFC3339, snap.LastPlaybookRefreshAt); err == nil {
			s.lastPlaybookRefreshAt = t
		}
	}
	return s
}

// ResumeAITraderSession restores a persisted session; reconnectOnly skips live/paper ticks until start-trading.
func (r *Runner) ResumeAITraderSession(parent context.Context, sessionID string, reconnectOnly bool) (*AITraderSession, error) {
	snap, err := loadAITraderSessionSnapshot(sessionID)
	if err != nil {
		return nil, fmt.Errorf("persisted session not found: %w", err)
	}
	s := sessionFromSnapshot(snap)
	if s == nil {
		return nil, fmt.Errorf("invalid snapshot")
	}
	depth := snap.Depth
	if depth <= 0 {
		depth = 50
	}
	sessionKey := aiTraderSessionKey(s.AccountID, s.InstrumentID)
	ctx, cancel := context.WithCancel(parent)
	if s.Limits.SessionTimeoutMinutes > 0 {
		ctx, cancel = context.WithTimeout(parent, time.Duration(s.Limits.SessionTimeoutMinutes)*time.Minute)
	}
	s.cancel = cancel
	if reconnectOnly {
		s.reconnectPaused = true
	}
	r.aiTrader.mu.Lock()
	if old := r.aiTrader.sessions[sessionKey]; old != nil && old.cancel != nil {
		old.cancel()
	}
	r.aiTrader.sessions[sessionKey] = s
	r.aiTrader.mu.Unlock()
	notifyAdvisorRegister(s)
	go r.runAITraderSession(ctx, s, depth)
	if !reconnectOnly && s.Phase == AITraderPhaseTrading {
		// trading loop already active via observe
	}
	return cloneAITraderSession(s), nil
}
