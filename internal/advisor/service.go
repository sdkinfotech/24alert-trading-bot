package advisor

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const (
	defaultIngestInterval = 12 * time.Second
	defaultSnapMinGap     = 5 * time.Minute
	defaultSchedInterval  = 30 * time.Second
)

// Service coordinates ingest, scheduling, and HTTP API.
type Service struct {
	store  *Store
	runner *RunnerClient
	log    *slog.Logger

	ingestEvery time.Duration
	snapMinGap  time.Duration
	schedEvery  time.Duration

	mu       sync.Mutex
	finalize map[string]bool
}

func NewService(store *Store, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		store:       store,
		runner:      NewRunnerClient(),
		log:         log,
		ingestEvery: defaultIngestInterval,
		snapMinGap:  defaultSnapMinGap,
		schedEvery:  defaultSchedInterval,
		finalize:    map[string]bool{},
	}
}

func (svc *Service) Run(ctx context.Context) {
	ingestTicker := time.NewTicker(svc.ingestEvery)
	schedTicker := time.NewTicker(svc.schedEvery)
	defer ingestTicker.Stop()
	defer schedTicker.Stop()

	svc.ingestTick(ctx)
	svc.schedulerTick(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ingestTicker.C:
			svc.ingestTick(ctx)
		case <-schedTicker.C:
			svc.schedulerTick(ctx)
		}
	}
}

func (svc *Service) Register(ctx context.Context, req RegisterRequest) error {
	if strings.TrimSpace(req.SessionID) == "" {
		return errBadRequest("session_id required")
	}
	started := ParseStartedAt(req.StartedAt)
	return svc.store.UpsertSession(ctx, req.SessionID, req.AccountID, req.InstrumentUID, req.Ticker, req.Mode, req.Instruction, "running", started)
}

func (svc *Service) Finalize(ctx context.Context, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errBadRequest("session_id required")
	}
	svc.mu.Lock()
	if svc.finalize[sessionID] {
		svc.mu.Unlock()
		return nil
	}
	svc.finalize[sessionID] = true
	svc.mu.Unlock()

	_ = svc.store.MarkSessionStopped(ctx, sessionID, time.Now().UTC())
	go svc.runFinalize(context.Background(), sessionID)
	return nil
}

func (svc *Service) runFinalize(ctx context.Context, sessionID string) {
	now := time.Now().UTC()
	for _, tf := range []Timeframe{TF5m, TF15m, TF30m, TF1h, TF4h} {
		svc.catchUpTimeframe(ctx, sessionID, tf, now)
	}
	dayEnd := LastClosedPeriodEnd(now, TF1d)
	if dayEnd.IsZero() {
		dayEnd = AlignPeriodEnd(now, TF1d)
	}
	_ = svc.runAgent(ctx, sessionID, TF1d, dayEnd)
	_ = svc.runStrategyAgent(ctx, sessionID)
}

func (svc *Service) ingestTick(ctx context.Context) {
	ids, err := svc.store.ListActiveSessions(ctx)
	if err != nil {
		svc.log.Warn("advisor ingest list sessions", "error", err)
		return
	}
	for _, id := range ids {
		svc.ingestSession(ctx, id)
	}
}

func (svc *Service) ingestSession(ctx context.Context, sessionID string) {
	sess, err := svc.runner.GetSession(ctx, sessionID)
	if err != nil {
		svc.log.Debug("advisor ingest fetch session", "session_id", sessionID, "error", err)
		return
	}
	if strings.EqualFold(sess.Status, "stopped") {
		_ = svc.store.MarkSessionStopped(ctx, sessionID, time.Now().UTC())
		return
	}
	last, ok, _ := svc.store.LastSnapshotAt(ctx, sessionID)
	if ok && time.Since(last) < svc.snapMinGap {
		return
	}
	payload := map[string]any{
		"features":       sess.Features,
		"market_context": sess.MarketContext,
		"events":         lastNEvents(sess.Events, 40),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	if err := svc.store.InsertSnapshot(ctx, sessionID, time.Now().UTC(), raw); err != nil {
		svc.log.Warn("advisor snapshot insert", "session_id", sessionID, "error", err)
		return
	}
	AdvisorIngestSnapshotsTotal.Inc()
}

func lastNEvents(events []DecisionEvent, n int) []DecisionEvent {
	if len(events) <= n {
		return events
	}
	return events[len(events)-n:]
}
