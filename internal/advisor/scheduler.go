package advisor

import (
	"context"
	"time"
)

var rollupTimeframes = []Timeframe{TF5m, TF15m, TF30m, TF1h, TF4h}

func (svc *Service) schedulerTick(ctx context.Context) {
	now := time.Now().UTC()
	ids, err := svc.store.ListActiveSessions(ctx)
	if err != nil {
		svc.log.Warn("advisor scheduler list", "error", err)
		return
	}
	for _, sid := range ids {
		for _, tf := range rollupTimeframes {
			svc.catchUpTimeframe(ctx, sid, tf, now)
		}
	}
	svc.retryFailedReports(ctx)
}

func (svc *Service) catchUpTimeframe(ctx context.Context, sessionID string, tf Timeframe, now time.Time) {
	closed := LastClosedPeriodEnd(now, tf)
	if closed.IsZero() {
		return
	}
	lastEnd, hasLast, _ := svc.store.GetLastPeriodEnd(ctx, sessionID, tf)
	for {
		var periodEnd time.Time
		if !hasLast {
			periodEnd = closed
		} else {
			periodEnd = lastEnd.Add(tf.Duration())
			periodEnd = AlignPeriodEnd(periodEnd.Add(-time.Millisecond), tf)
		}
		if periodEnd.After(closed) {
			break
		}
		if hasLast && !periodEnd.After(lastEnd) {
			break
		}
		if err := svc.runAgent(ctx, sessionID, tf, periodEnd); err != nil {
			svc.log.Warn("advisor agent", "session_id", sessionID, "tf", tf, "period_end", periodEnd, "error", err)
		}
		_ = svc.store.SetLastPeriodEnd(ctx, sessionID, tf, periodEnd)
		lastEnd = periodEnd
		hasLast = true
	}
}

func (svc *Service) retryFailedReports(ctx context.Context) {
	cutoff := time.Now().UTC().Add(-2 * time.Minute)
	failed, err := svc.store.ListFailedReports(ctx, cutoff, 10)
	if err != nil {
		return
	}
	for _, rep := range failed {
		_ = svc.runAgent(ctx, rep.SessionID, rep.Timeframe, rep.PeriodEnd)
	}
}
