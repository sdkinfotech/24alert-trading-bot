package advisor

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (svc *Service) runAgent(ctx context.Context, sessionID string, tf Timeframe, periodEnd time.Time) error {
	exists, err := svc.store.ReportExists(ctx, sessionID, tf, periodEnd)
	if err != nil {
		return err
	}
	if exists {
		rep, err := svc.store.GetReportByPeriod(ctx, sessionID, tf, periodEnd)
		if err != nil {
			return err
		}
		if rep.Status == ReportStatusOK {
			return nil
		}
		if rep.Status == ReportStatusFailed && time.Since(rep.CreatedAt) < 2*time.Minute {
			return nil
		}
	}

	_, uid, ticker, instruction, err := svc.store.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if ticker == "" {
		ticker = shortUID(uid)
	}
	periodStart := PeriodStart(periodEnd, tf)
	periodLabel := fmt.Sprintf("%s–%s MSK", periodStart.In(MSK()).Format("15:04"), periodEnd.In(MSK()).Format("15:04"))

	var rep AnalysisReport
	var childIDs []string
	var childSummaries []string

	if tf == TF5m {
		snaps, err := svc.store.SnapshotsInRange(ctx, sessionID, periodStart, periodEnd)
		if err != nil {
			return err
		}
		sess, _ := svc.runner.GetSession(ctx, sessionID)
		var events []DecisionEvent
		if sess != nil {
			events = filterEventsByTime(sess.Events, periodStart, periodEnd)
		}
		facts := BuildFactsFromSnapshots(snaps, events, ticker, periodLabel)
		if len(snaps) == 0 && len(events) == 0 {
			return nil
		}
		rep, _, err = runAnalysisLLM(ctx, tf, ticker, instruction, facts, nil)
		if err != nil {
			return svc.saveFailedReport(ctx, sessionID, tf, periodStart, periodEnd, err)
		}
	} else {
		var childReports []AnalysisReport
		for _, ctf := range tf.ChildTimeframes() {
			reports, err := svc.store.ReportsInRange(ctx, sessionID, ctf, periodStart, periodEnd)
			if err != nil {
				return err
			}
			childReports = append(childReports, reports...)
		}
		for _, r := range childReports {
			if r.Status != ReportStatusOK {
				continue
			}
			childIDs = append(childIDs, r.ID)
			childSummaries = append(childSummaries, fmt.Sprintf("[%s] %s", r.Timeframe, r.SummaryMD))
		}
		if len(childSummaries) == 0 {
			return nil
		}
		facts := BuildFactsFromChildReports(childReports, ticker, periodLabel)
		var model string
		rep, model, err = runAnalysisLLM(ctx, tf, ticker, instruction, facts, childSummaries)
		if err != nil {
			return svc.saveFailedReport(ctx, sessionID, tf, periodStart, periodEnd, err)
		}
		_ = model
	}

	rep.SessionID = sessionID
	rep.Timeframe = tf
	rep.PeriodStart = periodStart
	rep.PeriodEnd = periodEnd
	rep.SourceReportIDs = childIDs
	if rep.CreatedAt.IsZero() {
		rep.CreatedAt = time.Now().UTC()
	}
	if err := svc.store.InsertReport(ctx, &rep); err != nil {
		return err
	}
	AdvisorReportsTotal.WithLabelValues(string(tf), ReportStatusOK).Inc()
	return nil
}

func (svc *Service) runStrategyAgent(ctx context.Context, sessionID string) error {
	_, uid, ticker, instruction, err := svc.store.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if ticker == "" {
		ticker = shortUID(uid)
	}
	now := time.Now().UTC()
	dayEnd := LastClosedPeriodEnd(now, TF1d)
	dayRep, err := svc.store.GetReportByPeriod(ctx, sessionID, TF1d, dayEnd)
	if err != nil {
		dayRep = nil
	}
	if dayRep == nil || dayRep.Status != ReportStatusOK {
		_ = svc.runAgent(ctx, sessionID, TF1d, dayEnd)
		dayRep, _ = svc.store.GetReportByPeriod(ctx, sessionID, TF1d, dayEnd)
	}
	if dayRep == nil {
		return fmt.Errorf("day report missing")
	}
	var all []AnalysisReport
	for _, tf := range []Timeframe{TF5m, TF15m, TF30m, TF1h, TF4h, TF1d} {
		reps, _ := svc.store.ListReports(ctx, sessionID, tf, 50)
		all = append(all, reps...)
	}
	syn, err := runStrategyLLM(ctx, ticker, instruction, *dayRep, all)
	if err != nil {
		AdvisorLLMErrorsTotal.Inc()
		return err
	}
	syn.SessionID = sessionID
	if err := svc.store.SaveSynthesis(ctx, &syn); err != nil {
		return err
	}
	_ = svc.store.DeleteDraftsForSession(ctx, sessionID)
	for i := range syn.Drafts {
		syn.Drafts[i].SessionID = sessionID
		syn.Drafts[i].Ticker = ticker
		syn.Drafts[i].InstrumentUID = uid
		syn.Drafts[i].CreatedAt = time.Now().UTC()
		if err := svc.store.InsertDraft(ctx, &syn.Drafts[i]); err != nil {
			svc.log.Warn("advisor draft insert", "error", err)
		}
	}
	strRep := AnalysisReport{
		SessionID:   sessionID,
		Timeframe:   TFStrategy,
		PeriodStart: PeriodStart(dayEnd, TF1d),
		PeriodEnd:   dayEnd,
		Status:      ReportStatusOK,
		SummaryMD:   syn.SummaryMD,
		Structured:  syn.Structured,
		Model:       syn.Model,
		PromptVersion: advisorPromptVer,
		CreatedAt:   syn.CreatedAt,
		SourceReportIDs: []string{dayRep.ID},
	}
	return svc.store.InsertReport(ctx, &strRep)
}

func (svc *Service) saveFailedReport(ctx context.Context, sessionID string, tf Timeframe, start, end time.Time, err error) error {
	AdvisorLLMErrorsTotal.Inc()
	rep := AnalysisReport{
		SessionID:     sessionID,
		Timeframe:     tf,
		PeriodStart:   start,
		PeriodEnd:     end,
		Status:        ReportStatusFailed,
		ErrorMessage:  FormatError(err),
		PromptVersion: advisorPromptVer,
		CreatedAt:     time.Now().UTC(),
	}
	_ = svc.store.InsertReport(ctx, &rep)
	AdvisorReportsTotal.WithLabelValues(string(tf), ReportStatusFailed).Inc()
	return err
}

func filterEventsByTime(events []DecisionEvent, start, end time.Time) []DecisionEvent {
	var out []DecisionEvent
	for _, ev := range events {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(ev.Time))
		if err != nil {
			continue
		}
		t = t.UTC()
		if !t.Before(start) && t.Before(end) {
			out = append(out, ev)
		}
	}
	return out
}

func shortUID(uid string) string {
	uid = strings.TrimSpace(uid)
	if len(uid) > 8 {
		return uid[:8]
	}
	return uid
}
