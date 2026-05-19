package advisor

import (
	"context"
	"time"
)

func (svc *Service) shouldRetryFailedReport(ctx context.Context, rep AnalysisReport) bool {
	st, err := svc.store.GetSessionStatus(ctx, rep.SessionID)
	if err != nil || st != "running" {
		return false
	}
	// Exponential-ish spacing: at least 60s since failure before retry.
	if time.Since(rep.CreatedAt) < 60*time.Second {
		return false
	}
	return true
}
