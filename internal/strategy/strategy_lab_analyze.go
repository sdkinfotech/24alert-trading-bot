package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

)

// StrategyLabAnalyzeRequest runs full matrix backtest + structured verdict.
type StrategyLabAnalyzeRequest struct {
	UID    string `json:"uid"`
	Ticker string `json:"ticker"`
	Days   int    `json:"days"`
	Lang   string `json:"lang"`
}

// StrategyLabAnalyzeResponse is the primary lab output (no LLM required).
type StrategyLabAnalyzeResponse struct {
	Ticker         string                   `json:"ticker"`
	UID            string                   `json:"uid"`
	Days           int                      `json:"days"`
	Verdict        string                   `json:"verdict"` // keep_prod | deploy_candidate | research_only | insufficient
	VerdictReason  string                   `json:"verdict_reason"`
	Recommendation string                   `json:"recommendation"` // keep_prod | apply | research_only | wait
	SummaryMD      string                   `json:"summary_md"`
	VsProductionMD string                   `json:"vs_production_md,omitempty"`
	ActionMD       string                   `json:"action_md,omitempty"`
	WarningsMD     string                   `json:"warnings_md,omitempty"`
	Candidate      *StrategyLabRunRow       `json:"candidate,omitempty"`
	Production     *StrategyLabRunRow       `json:"production,omitempty"`
	ConfigProd     *StrategyLabConfigProd   `json:"config_prod,omitempty"`
	FamilyLeaders  []StrategyLabFamilyLeader `json:"family_leaders"`
	TopRows        []StrategyLabRunRow      `json:"top_rows"`
	Rollout        StrategyLabRolloutPlan   `json:"rollout"`
	Matrix         json.RawMessage          `json:"matrix,omitempty"`
}

// StrategyLabFamilyLeader is the best backtest row per strategy family (human-facing).
type StrategyLabFamilyLeader struct {
	StrategyID    string            `json:"strategy_id"`
	Label         string            `json:"label"`
	Mode          string            `json:"mode,omitempty"`
	ParamsSummary string            `json:"params_summary"`
	PnL           float64           `json:"pnl"`
	DeltaVsProd   *float64          `json:"delta_vs_prod,omitempty"`
	MaxDrawdown   float64           `json:"max_drawdown"`
	Trades        int               `json:"trades"`
	Grade         string            `json:"grade"` // excellent | good | mixed | poor | research | insufficient | prod
	GradeLabel    string            `json:"grade_label"`
	VerdictLine   string            `json:"verdict_line"`
	LiveEligible  bool              `json:"live_eligible"`
	IsProduction  bool              `json:"is_production,omitempty"`
	Row           StrategyLabRunRow `json:"row"`
}

// StrategyLabConfigProd is the live instance from config.yaml for this instrument.
type StrategyLabConfigProd struct {
	InstanceID string            `json:"instance_id"`
	Type       string            `json:"type"`
	AccountID  string            `json:"account_id"`
	Enabled    bool              `json:"enabled"`
	Params     map[string]string `json:"params"`
}

// StrategyLabRolloutPlan is the mandatory production procedure.
type StrategyLabRolloutPlan struct {
	CanStageConfig   bool               `json:"can_stage_config"`
	CanEnableLive    bool               `json:"can_enable_live"`
	BlockedReasons   []string           `json:"blocked_reasons,omitempty"`
	Steps            []StrategyLabRolloutStep `json:"steps"`
	SmokeCommands    []string           `json:"smoke_commands,omitempty"`
}

// StrategyLabRolloutStep is one checklist item for operators.
type StrategyLabRolloutStep struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	DetailMD   string `json:"detail_md"`
	Automated  bool   `json:"automated"`
	APIAction  string `json:"api_action,omitempty"`
}

type labMatrixPayload struct {
	Days        int                    `json:"days"`
	Instruments []labMatrixInstrument  `json:"instruments"`
}

type labMatrixInstrument struct {
	Ticker          string             `json:"ticker"`
	UID             string             `json:"uid"`
	Error           string             `json:"error,omitempty"`
	Production      *StrategyLabRunRow `json:"production"`
	BestDeployable  *StrategyLabRunRow `json:"best_deployable"`
	BestResearch    *StrategyLabRunRow `json:"best_research"`
	Top10Overall    []StrategyLabRunRow `json:"top10_overall"`
	Top10Deployable []StrategyLabRunRow `json:"top10_deployable"`
}

const (
	labVerdictKeepProd        = "keep_prod"
	labVerdictDeployCandidate = "deploy_candidate"
	labVerdictResearchOnly    = "research_only"
	labVerdictInsufficient    = "insufficient"

	labMinTradesForVerdict = 8
	labMinPnLDeltaDeploy    = 2.0
	labMinTradesDeploy      = 10
)

func (r *Runner) StrategyLabAnalyze(ctx context.Context, req StrategyLabAnalyzeRequest) (*StrategyLabAnalyzeResponse, error) {
	ticker := strings.TrimSpace(strings.ToUpper(req.Ticker))
	uid := strings.TrimSpace(req.UID)
	if ticker == "" {
		return nil, fmt.Errorf("ticker is required")
	}
	days := req.Days
	if days <= 0 {
		days = 90
	}
	lang := strings.TrimSpace(req.Lang)
	if lang != "en" {
		lang = "ru"
	}

	raw, err := r.StrategyLabCompare(ctx, StrategyLabCompareRequest{Ticker: ticker, Days: days})
	if err != nil {
		return nil, err
	}
	var matrix labMatrixPayload
	if err := json.Unmarshal(raw, &matrix); err != nil {
		return nil, fmt.Errorf("parse matrix: %w", err)
	}
	var inst *labMatrixInstrument
	for i := range matrix.Instruments {
		if strings.EqualFold(matrix.Instruments[i].Ticker, ticker) {
			inst = &matrix.Instruments[i]
			break
		}
	}
	if inst == nil && len(matrix.Instruments) > 0 {
		inst = &matrix.Instruments[0]
	}
	if inst == nil {
		return nil, fmt.Errorf("no matrix result for %s", ticker)
	}
	if inst.Error != "" {
		return nil, fmt.Errorf("matrix: %s", inst.Error)
	}
	if uid == "" {
		uid = inst.UID
	}

	cfgProd := r.labConfigProdForUID(uid)
	out := buildLabAnalysis(ticker, uid, days, lang, inst, cfgProd)
	out.Matrix = raw
	return out, nil
}

func (r *Runner) labConfigProdForUID(uid string) *StrategyLabConfigProd {
	for _, inst := range r.strategiesCfg.Instances {
		for _, u := range inst.Instruments {
			if u == uid {
				params := make(map[string]string, len(inst.Params))
				for k, v := range inst.Params {
					params[k] = v
				}
				return &StrategyLabConfigProd{
					InstanceID: inst.ID,
					Type:       inst.Type,
					AccountID:  inst.AccountID,
					Enabled:    inst.Enabled,
					Params:     params,
				}
			}
		}
	}
	return nil
}

func buildLabAnalysis(ticker, uid string, days int, lang string, inst *labMatrixInstrument, cfgProd *StrategyLabConfigProd) *StrategyLabAnalyzeResponse {
	prod := inst.Production
	if prod != nil {
		p := *prod
		p.Mode = "prod"
		prod = &p
	}
	candidate := pickLabCandidate(inst)
	families := collectLabFamilyLeaders(inst, prod, lang)
	top := familiesToRows(families)

	verdict, reason := labVerdict(candidate, prod, cfgProd, lang)
	rec := labVerdictToRecommendation(verdict)
	summary, vsProd, actionMD := labMatrixNarrativeMD(ticker, days, lang, families, candidate, prod, cfgProd, verdict, reason)
	warnings := labStandardWarningsForRow(candidate, lang)
	warnings = strings.TrimSpace(warnings + "\n" + labAnalysisExtraWarnings(lang))

	rollout := buildRolloutPlan(verdict, candidate, cfgProd, lang)

	return &StrategyLabAnalyzeResponse{
		Ticker:         ticker,
		UID:            uid,
		Days:           days,
		Verdict:        verdict,
		VerdictReason:  reason,
		Recommendation: rec,
		SummaryMD:      summary,
		VsProductionMD: vsProd,
		ActionMD:       actionMD,
		WarningsMD:     warnings,
		Candidate:      candidate,
		Production:     prod,
		ConfigProd:     cfgProd,
		FamilyLeaders:  families,
		TopRows:        top,
		Rollout:        rollout,
	}
}

func labVerdictToRecommendation(verdict string) string {
	switch verdict {
	case labVerdictDeployCandidate:
		return "apply"
	case labVerdictResearchOnly:
		return "research_only"
	case labVerdictInsufficient:
		return "wait"
	default:
		return "keep_prod"
	}
}

func familiesToRows(families []StrategyLabFamilyLeader) []StrategyLabRunRow {
	out := make([]StrategyLabRunRow, 0, len(families))
	for _, f := range families {
		out = append(out, f.Row)
	}
	return out
}

func pickLabCandidate(inst *labMatrixInstrument) *StrategyLabRunRow {
	if inst.BestDeployable != nil {
		c := *inst.BestDeployable
		return &c
	}
	for _, r := range inst.Top10Deployable {
		if r.LiveEligible && r.Trades >= labMinTradesForVerdict && r.PnL > 0 {
			c := r
			return &c
		}
	}
	for _, r := range inst.Top10Overall {
		if r.Mode == "prod" || r.Mode == "prod_baseline" {
			continue
		}
		if r.LiveEligible && r.Trades >= labMinTradesForVerdict && r.PnL > 0 {
			c := r
			return &c
		}
	}
	if inst.BestResearch != nil {
		c := *inst.BestResearch
		return &c
	}
	if len(inst.Top10Overall) > 0 {
		c := inst.Top10Overall[0]
		return &c
	}
	return nil
}

func collectLabTopRows(inst *labMatrixInstrument, prod *StrategyLabRunRow, limit int) []StrategyLabRunRow {
	var rows []StrategyLabRunRow
	if prod != nil {
		rows = append(rows, *prod)
	}
	if inst.BestDeployable != nil {
		rows = append(rows, *inst.BestDeployable)
	}
	rows = append(rows, inst.Top10Deployable...)
	rows = append(rows, inst.Top10Overall...)
	seen := make(map[string]struct{})
	out := make([]StrategyLabRunRow, 0, limit)
	for _, r := range rows {
		key := rowKeyForDedup(r)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, r)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func rowKeyForDedup(r StrategyLabRunRow) string {
	return fmt.Sprintf("%s|%s|%v", r.Strategy, r.Mode, r.Params)
}

var labStrategyOrder = []string{
	"sma_crossover", "level_bounce", "orb_breakout", "ema_1h", "donchian_15m",
}

func collectLabFamilyLeaders(inst *labMatrixInstrument, prod *StrategyLabRunRow, lang string) []StrategyLabFamilyLeader {
	pool := make([]StrategyLabRunRow, 0, 64)
	pool = append(pool, inst.Top10Overall...)
	pool = append(pool, inst.Top10Deployable...)
	if inst.BestDeployable != nil {
		pool = append(pool, *inst.BestDeployable)
	}
	if inst.BestResearch != nil {
		pool = append(pool, *inst.BestResearch)
	}

	bestByStrategy := make(map[string]StrategyLabRunRow)
	for _, r := range pool {
		if r.Mode == "prod" || r.Mode == "prod_baseline" {
			continue
		}
		key := r.Strategy
		prev, ok := bestByStrategy[key]
		if !ok || labRowScore(r) > labRowScore(prev) {
			bestByStrategy[key] = r
		}
	}

	var out []StrategyLabFamilyLeader
	if prod != nil {
		out = append(out, rowToFamilyLeader(*prod, prod, lang, true))
	}
	for _, sid := range labStrategyOrder {
		r, ok := bestByStrategy[sid]
		if !ok {
			continue
		}
		out = append(out, rowToFamilyLeader(r, prod, lang, false))
	}
	return out
}

func labRowScore(r StrategyLabRunRow) float64 {
	if r.RiskScore != 0 {
		return r.RiskScore
	}
	if r.Trades < 5 {
		return -1e9
	}
	return r.Sharpe*r.PnL - 0.35*r.MaxDrawdown
}

func rowToFamilyLeader(r StrategyLabRunRow, prod *StrategyLabRunRow, lang string, isProd bool) StrategyLabFamilyLeader {
	grade, gradeLabel, line := labGradeForRow(r, prod, lang, isProd)
	var delta *float64
	if prod != nil && !isProd {
		d := r.PnL - prod.PnL
		delta = &d
	}
	return StrategyLabFamilyLeader{
		StrategyID:    r.Strategy,
		Label:         labStrategyLabel(r.Strategy, lang),
		Mode:          r.Mode,
		ParamsSummary: labParamsHumanSummary(r.Strategy, r.Params, lang),
		PnL:           r.PnL,
		DeltaVsProd:   delta,
		MaxDrawdown:   r.MaxDrawdown,
		Trades:        r.Trades,
		Grade:         grade,
		GradeLabel:    gradeLabel,
		VerdictLine:   line,
		LiveEligible:  r.LiveEligible,
		IsProduction:  isProd,
		Row:           r,
	}
}

func labStrategyLabel(strategyID, lang string) string {
	labelsRU := map[string]string{
		"sma_crossover": "SMA Crossover (1h)",
		"level_bounce":  "Отбой от уровней (15m)",
		"orb_breakout":  "ORB — пробой диапазона (15m)",
		"ema_1h":        "EMA пересечение (research)",
		"donchian_15m":  "Donchian пробой (research)",
	}
	labelsEN := map[string]string{
		"sma_crossover": "SMA Crossover (1h)",
		"level_bounce":  "Level Bounce (15m)",
		"orb_breakout":  "ORB range breakout (15m)",
		"ema_1h":        "EMA crossover (research)",
		"donchian_15m":  "Donchian breakout (research)",
	}
	if lang == "en" {
		if s, ok := labelsEN[strategyID]; ok {
			return s
		}
		return strategyID
	}
	if s, ok := labelsRU[strategyID]; ok {
		return s
	}
	return strategyID
}

func labGradeForRow(r StrategyLabRunRow, prod *StrategyLabRunRow, lang string, isProd bool) (grade, label, line string) {
	if isProd {
		return "prod", warnLine(lang, "База — текущий прод", "Baseline — current production"),
			warnLine(lang,
				fmt.Sprintf("Это ваш эталон в config: PnL %.1f пт. за период, %d сделок.", r.PnL, r.Trades),
				fmt.Sprintf("Config baseline: PnL %.1f pts, %d trades.", r.PnL, r.Trades))
	}
	if r.Trades < 5 {
		return "insufficient", warnLine(lang, "Мало данных", "Insufficient data"),
			warnLine(lang, fmt.Sprintf("Всего %d сделок — выводы ненадёжны.", r.Trades), fmt.Sprintf("Only %d trades — unreliable.", r.Trades))
	}
	if !r.LiveEligible {
		if r.PnL > 0 {
			return "research", warnLine(lang, "Только исследование", "Research only"),
				warnLine(lang,
					"Идея интересна в бэктесте, но в Go-runner на прод не ставится (ORB/EMA/Donchian или SMA без trailing).",
					"Interesting in backtest but not deployable on live runner.")
		}
		return "poor", warnLine(lang, "Не для прода", "Not for live"),
			warnLine(lang, "Убыточно или заблокировано для live.", "Unprofitable or blocked for live.")
	}
	if r.PnL <= 0 {
		return "poor", warnLine(lang, "Убыточно", "Unprofitable"),
			warnLine(lang, "PnL ≤ 0 за период — на прод не рассматриваем.", "PnL ≤ 0 — not for production.")
	}
	if prod == nil {
		return "good", warnLine(lang, "Прибыльно", "Profitable"),
			warnLine(lang, fmt.Sprintf("PnL +%.1f пт., %d сделок — без сравнения с прод.", r.PnL, r.Trades),
				fmt.Sprintf("PnL +%.1f pts, %d trades — no prod baseline.", r.PnL, r.Trades))
	}
	delta := r.PnL - prod.PnL
	ddRatio := 1.0
	if prod.MaxDrawdown > 0 {
		ddRatio = r.MaxDrawdown / prod.MaxDrawdown
	}
	switch {
	case delta >= 2 && ddRatio <= 1.2:
		return "excellent", warnLine(lang, "Лучше прод", "Beats production"),
			warnLine(lang,
				fmt.Sprintf("На %.1f пт. прибыльнее эталона при сопоставимой просадке — смысл рассмотреть смену параметров.", delta),
				fmt.Sprintf("%.1f pts better than baseline with similar drawdown — worth considering.", delta))
	case delta >= 0.5:
		return "good", warnLine(lang, "Чуть лучше прод", "Slightly better"),
			warnLine(lang,
				fmt.Sprintf("Прирост +%.1f пт. к бэктесту прод — скромное, но положительное отличие.", delta),
				fmt.Sprintf("+%.1f pts vs prod backtest — modest positive edge.", delta))
	case delta >= -1:
		return "mixed", warnLine(lang, "Паритет с продом", "On par with prod"),
			warnLine(lang,
				fmt.Sprintf("ΔPnL %+.1f пт. — по сути то же, что сейчас в config.", delta),
				fmt.Sprintf("ΔPnL %+.1f pts — roughly same as config.", delta))
	default:
		return "poor", warnLine(lang, "Хуже прод", "Worse than prod"),
			warnLine(lang,
				fmt.Sprintf("На %.1f пт. слабее эталона — менять прод нет смысла.", -delta),
				fmt.Sprintf("%.1f pts weaker than baseline — no reason to switch.", -delta))
	}
}

func labParamsHumanSummary(strategyID string, p map[string]string, lang string) string {
	if len(p) == 0 {
		return "—"
	}
	switch strategyID {
	case "sma_crossover":
		trail := p["trailing_stop_pct"]
		if trail != "" && trail != "0" {
			if lang == "en" {
				return fmt.Sprintf("fast %s / slow %s, trailing %s", p["fast_period"], p["slow_period"], trail)
			}
			return fmt.Sprintf("быстрая %s / медленная %s, трейлинг %s", p["fast_period"], p["slow_period"], trail)
		}
		if lang == "en" {
			return fmt.Sprintf("fast %s / slow %s (no trailing)", p["fast_period"], p["slow_period"])
		}
		return fmt.Sprintf("быстрая %s / медленная %s (без трейлинга)", p["fast_period"], p["slow_period"])
	case "level_bounce":
		if lang == "en" {
			return fmt.Sprintf("SL×%s TP×%s, cutoff %s:%s", p["sl_mult"], p["tp_mult"], p["cutoff_hour"], p["cutoff_min"])
		}
		return fmt.Sprintf("SL×%s TP×%s, cutoff %s:%s", p["sl_mult"], p["tp_mult"], p["cutoff_hour"], p["cutoff_min"])
	case "orb_breakout":
		if lang == "en" {
			return fmt.Sprintf("range %s candles, cutoff %s:%s", p["range_candles"], p["cutoff_hour"], p["cutoff_min"])
		}
		return fmt.Sprintf("диапазон %s свечей, cutoff %s:%s", p["range_candles"], p["cutoff_hour"], p["cutoff_min"])
	case "ema_1h":
		return fmt.Sprintf("EMA %s / %s", p["fast_period"], p["slow_period"])
	case "donchian_15m":
		if lang == "en" {
			return fmt.Sprintf("lookback %s, ATR stop %s", p["lookback"], p["atr_stop"])
		}
		return fmt.Sprintf("lookback %s, стоп ATR %s", p["lookback"], p["atr_stop"])
	default:
		return paramsSummary(p)
	}
}

func labMatrixNarrativeMD(
	ticker string, days int, lang string,
	families []StrategyLabFamilyLeader,
	candidate, prod *StrategyLabRunRow,
	cfgProd *StrategyLabConfigProd,
	verdict, reason string,
) (summary, vsProd, actionMD string) {
	var b strings.Builder
	verdictHuman := labVerdictHumanLabel(verdict, lang)
	if lang == "en" {
		b.WriteString(fmt.Sprintf("## %s — strategy comparison (~%d days)\n\n", ticker, days))
		b.WriteString(fmt.Sprintf("### Bottom line\n**%s** — %s\n\n", verdictHuman, reason))
		b.WriteString("We ran **different strategy families** (not 13 trailing tweaks of one SMA). One best config per family vs your production baseline.\n\n")
	} else {
		b.WriteString(fmt.Sprintf("## %s — сравнение семейств стратегий (~%d дн.)\n\n", ticker, days))
		b.WriteString(fmt.Sprintf("### Итог\n**%s** — %s\n\n", verdictHuman, reason))
		b.WriteString("Прогнали **разные типы стратегий** (не 13 строк с одним trailing у SMA). Ниже — лучший вариант **в каждом семействе** и сравнение с тем, что сейчас в config.\n\n")
	}

	if cfgProd != nil {
		if lang == "en" {
			b.WriteString(fmt.Sprintf("**Live config:** `%s` · %s · enabled=%v · %s\n\n",
				cfgProd.InstanceID, cfgProd.Type, cfgProd.Enabled, labParamsHumanSummary(cfgProd.Type, cfgProd.Params, lang)))
		} else {
			b.WriteString(fmt.Sprintf("**Сейчас в config.yaml:** `%s` · %s · enabled=%v · %s\n\n",
				cfgProd.InstanceID, cfgProd.Type, cfgProd.Enabled, labParamsHumanSummary(cfgProd.Type, cfgProd.Params, lang)))
		}
	}

	if lang == "en" {
		b.WriteString("| Family | Best config | PnL (pts) | vs prod | Trades | Verdict |\n")
		b.WriteString("|--------|-------------|-----------|---------|--------|--------|\n")
	} else {
		b.WriteString("| Семейство | Лучший вариант | PnL (пт.) | к прод | Сделок | Оценка |\n")
		b.WriteString("|-----------|----------------|-----------|--------|--------|--------|\n")
	}
	for _, f := range families {
		delta := "—"
		if f.DeltaVsProd != nil {
			delta = fmt.Sprintf("%+.1f", *f.DeltaVsProd)
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %.1f | %s | %d | **%s** |\n",
			f.Label, f.ParamsSummary, f.PnL, delta, f.Trades, f.GradeLabel))
	}
	b.WriteString("\n")

	if candidate != nil {
		if lang == "en" {
			b.WriteString(fmt.Sprintf("**Suggested candidate:** %s — %s — PnL **%.1f** pts, drawdown **%.1f**, **%d** trades.\n",
				candidate.Strategy, labParamsHumanSummary(candidate.Strategy, candidate.Params, lang),
				candidate.PnL, candidate.MaxDrawdown, candidate.Trades))
		} else {
			b.WriteString(fmt.Sprintf("**Кандидат на смену параметров:** %s — %s — PnL **%.1f** пт., просадка **%.1f**, **%d** сделок.\n",
				labStrategyLabel(candidate.Strategy, lang), labParamsHumanSummary(candidate.Strategy, candidate.Params, lang),
				candidate.PnL, candidate.MaxDrawdown, candidate.Trades))
		}
	}

	summary = b.String()
	vsProd = labAnalysisVsProdMD(candidate, prod, cfgProd, lang)

	var act strings.Builder
	switch verdict {
	case labVerdictDeployCandidate:
		if lang == "en" {
			act.WriteString("### What to do\n1. Review the family table — confirm the candidate beats production meaningfully.\n2. **Stage** config (`enabled: false`), then git → VPS → smoke → enable live only per policy.\n")
		} else {
			act.WriteString("### Что делать\n1. Проверьте таблицу семейств — кандидат должен быть **заметно** лучше строки «Сейчас на проде».\n2. **Записать в config** (`enabled: false`), затем git → VPS → smoke → live только по PRODUCTION_TRADING_POLICY.\n")
		}
		if candidate != nil {
			act.WriteString(fmt.Sprintf("\nПараметры для stage: `%s`", paramsSummary(candidate.Params)))
		}
	case labVerdictKeepProd:
		if lang == "en" {
			act.WriteString("### What to do\n**Keep production config.** Other families did not justify a change; do not chase Sharpe or tiny PnL deltas.\n")
		} else {
			act.WriteString("### Что делать\n**Оставить текущий прод.** Другие семейства не дали убедительного преимущества; не менять config из‑за +0.5 пт. или завышенного Sharpe.\n")
		}
	case labVerdictResearchOnly:
		if lang == "en" {
			act.WriteString("### What to do\nUse results for research only. Best row is not live-eligible — do not stage for trading.\n")
		} else {
			act.WriteString("### Что делать\nТолько для исследования. Лучший вариант **нельзя** на live (ORB/EMA/Donchian или SMA без trailing).\n")
		}
	default:
		if lang == "en" {
			act.WriteString("### What to do\nGather more data or extend the backtest window before changing production.\n")
		} else {
			act.WriteString("### Что делать\nМало данных или слабый сигнал — увеличьте период или повторите анализ позже.\n")
		}
	}
	actionMD = act.String()
	return summary, vsProd, actionMD
}

func labVerdictHumanLabel(verdict, lang string) string {
	if lang == "en" {
		switch verdict {
		case labVerdictDeployCandidate:
			return "Deploy candidate"
		case labVerdictResearchOnly:
			return "Research only"
		case labVerdictInsufficient:
			return "Insufficient data"
		default:
			return "Keep production"
		}
	}
	switch verdict {
	case labVerdictDeployCandidate:
		return "Имеет смысл рассмотреть выкат"
	case labVerdictResearchOnly:
		return "Только исследование"
	case labVerdictInsufficient:
		return "Недостаточно данных"
	default:
		return "Оставить прод"
	}
}

func labVerdict(candidate, prod *StrategyLabRunRow, cfgProd *StrategyLabConfigProd, lang string) (string, string) {
	if candidate == nil {
		return labVerdictInsufficient, warnLine(lang, "Нет результатов бэктеста.", "No backtest results.")
	}
	if !candidate.LiveEligible {
		if candidate.PnL > 0 {
			return labVerdictResearchOnly, warnLine(lang,
				"Лучший вариант только research (ORB/EMA/нет trailing).",
				"Best row is research-only.")
		}
		return labVerdictInsufficient, warnLine(lang, "Нет deployable кандидата.", "No deployable candidate.")
	}
	if candidate.Trades < labMinTradesForVerdict {
		return labVerdictInsufficient, warnLine(lang,
			fmt.Sprintf("Мало сделок (%d < %d).", candidate.Trades, labMinTradesForVerdict),
			fmt.Sprintf("Too few trades (%d).", candidate.Trades))
	}
	if candidate.PnL <= 0 {
		return labVerdictKeepProd, warnLine(lang, "Кандидат с отрицательным PnL.", "Candidate PnL is not positive.")
	}

	// Same as config prod params — no change needed.
	if cfgProd != nil && paramsSameSMA(candidate.Params, cfgProd.Params) {
		return labVerdictKeepProd, warnLine(lang,
			"Параметры кандидата совпадают с config.yaml.",
			"Candidate params match config.yaml.")
	}

	delta := 0.0
	if prod != nil {
		delta = candidate.PnL - prod.PnL
	}
	if prod != nil && delta < labMinPnLDeltaDeploy {
		return labVerdictKeepProd, warnLine(lang,
			fmt.Sprintf("Прирост к бэктесту прод %.2f пт. < порога %.1f пт.", delta, labMinPnLDeltaDeploy),
			fmt.Sprintf("PnL gain vs prod backtest %.2f < %.1f pts threshold.", delta, labMinPnLDeltaDeploy))
	}
	if prod != nil && candidate.MaxDrawdown > prod.MaxDrawdown*1.3 && delta < labMinPnLDeltaDeploy*2 {
		return labVerdictKeepProd, warnLine(lang,
			"Просадка существенно выше прод при умеренном приросте PnL.",
			"Drawdown much worse than prod for modest PnL gain.")
	}

	if candidate.Trades >= labMinTradesDeploy && candidate.PnL > 0 {
		return labVerdictDeployCandidate, warnLine(lang,
			"Кандидат проходит пороги бэктеста; выкат только по процедуре rollout.",
			"Candidate passes backtest gates; deploy only via rollout procedure.")
	}
	return labVerdictKeepProd, warnLine(lang, "Недостаточное преимущество над прод.", "Insufficient edge vs production.")
}

func labAnalysisSummaryMD(ticker string, days int, lang string, inst *labMatrixInstrument, candidate, prod *StrategyLabRunRow, cfgProd *StrategyLabConfigProd, verdict, reason string) string {
	var b strings.Builder
	if lang == "en" {
		b.WriteString(fmt.Sprintf("## %s — strategy analysis (~%d days)\n\n", ticker, days))
		b.WriteString(fmt.Sprintf("**Verdict:** `%s` — %s\n\n", verdict, reason))
		b.WriteString("Full matrix: SMA (periods + trailing), Level Bounce, ORB, EMA, Donchian vs production baseline.\n\n")
	} else {
		b.WriteString(fmt.Sprintf("## %s — анализ стратегий (~%d дн.)\n\n", ticker, days))
		b.WriteString(fmt.Sprintf("**Вердикт:** `%s` — %s\n\n", verdict, reason))
		b.WriteString("Полная матрица: SMA (периоды + trailing), Level Bounce, ORB, EMA, Donchian vs бэктест прод.\n\n")
	}
	if cfgProd != nil {
		if lang == "en" {
			b.WriteString(fmt.Sprintf("**Live config:** instance `%s`, type `%s`, enabled=%v — %s\n\n",
				cfgProd.InstanceID, cfgProd.Type, cfgProd.Enabled, paramsSummary(cfgProd.Params)))
		} else {
			b.WriteString(fmt.Sprintf("**Прод config:** instance `%s`, type `%s`, enabled=%v — %s\n\n",
				cfgProd.InstanceID, cfgProd.Type, cfgProd.Enabled, paramsSummary(cfgProd.Params)))
		}
	}
	if candidate != nil {
		if lang == "en" {
			b.WriteString(fmt.Sprintf("**Candidate:** %s / %s — PnL **%.2f** pts, DD **%.2f**, **%d** trades, params: %s\n",
				candidate.Strategy, candidate.Mode, candidate.PnL, candidate.MaxDrawdown, candidate.Trades, paramsSummary(candidate.Params)))
		} else {
			b.WriteString(fmt.Sprintf("**Кандидат:** %s / %s — PnL **%.2f** пт., просадка **%.2f**, **%d** сделок, параметры: %s\n",
				candidate.Strategy, candidate.Mode, candidate.PnL, candidate.MaxDrawdown, candidate.Trades, paramsSummary(candidate.Params)))
		}
	}
	if prod != nil {
		if lang == "en" {
			b.WriteString(fmt.Sprintf("\n**Prod backtest row:** PnL **%.2f**, **%d** trades.", prod.PnL, prod.Trades))
		} else {
			b.WriteString(fmt.Sprintf("\n**Бэктест прод (матрица):** PnL **%.2f** пт., **%d** сделок.", prod.PnL, prod.Trades))
		}
	}
	return b.String()
}

func labAnalysisVsProdMD(candidate, prod *StrategyLabRunRow, cfgProd *StrategyLabConfigProd, lang string) string {
	if candidate == nil {
		return ""
	}
	var parts []string
	if cfgProd != nil {
		parts = append(parts, warnLine(lang,
			fmt.Sprintf("config: fast=%s slow=%s trail=%s",
				cfgProd.Params["fast_period"], cfgProd.Params["slow_period"], cfgProd.Params["trailing_stop_pct"]),
			fmt.Sprintf("config: fast=%s slow=%s trail=%s",
				cfgProd.Params["fast_period"], cfgProd.Params["slow_period"], cfgProd.Params["trailing_stop_pct"])))
	}
	if prod != nil {
		parts = append(parts, warnLine(lang,
			fmt.Sprintf("бэктест прод: PnL %.2f, DD %.2f, %d сделок → кандидат ΔPnL **%+.2f** пт.",
				prod.PnL, prod.MaxDrawdown, prod.Trades, candidate.PnL-prod.PnL),
			fmt.Sprintf("prod backtest: PnL %.2f, DD %.2f, %d trades → candidate ΔPnL **%+.2f** pts",
				prod.PnL, prod.MaxDrawdown, prod.Trades, candidate.PnL-prod.PnL)))
	}
	return strings.Join(parts, "\n")
}

func labAnalysisExtraWarnings(lang string) string {
	return warnLine(lang,
		"Перед live: PRODUCTION_TRADING_POLICY.md — broker stop, watchdog, git→VPS deploy. Прямой «старт» из UI по умолчанию выключен.",
		"Before live: see PRODUCTION_TRADING_POLICY.md — broker stop, watchdog, git→VPS deploy. Direct UI live start is disabled by default.")
}

func labStandardWarningsForRow(sel *StrategyLabRunRow, lang string) string {
	if sel == nil {
		return ""
	}
	return labStandardWarnings(*sel, lang)
}

func buildRolloutPlan(verdict string, candidate *StrategyLabRunRow, cfgProd *StrategyLabConfigProd, lang string) StrategyLabRolloutPlan {
	plan := StrategyLabRolloutPlan{
		CanStageConfig: candidate != nil && candidate.LiveEligible && verdict != labVerdictResearchOnly,
		CanEnableLive:  false,
	}
	if verdict != labVerdictDeployCandidate {
		plan.BlockedReasons = append(plan.BlockedReasons, warnLine(lang,
			"Вердикт не deploy_candidate — live enable заблокирован.",
			"Verdict is not deploy_candidate — live enable blocked."))
	} else {
		plan.CanEnableLive = true
	}
	instanceID := "lab-NEW"
	if cfgProd != nil && cfgProd.InstanceID != "" {
		instanceID = cfgProd.InstanceID
	}

	if lang == "en" {
		plan.Steps = []StrategyLabRolloutStep{
			{ID: "review", Title: "Review analysis", DetailMD: "Confirm verdict, candidate params, warnings. PnL is price points, not account RUB.", Automated: false},
			{ID: "stage", Title: "Stage config (disabled)", DetailMD: fmt.Sprintf("Writes instance `%s` to config.yaml with **enabled: false**, reload runner. No live orders.", instanceID), Automated: true, APIAction: "POST /strategy-lab/apply phase=stage"},
			{ID: "git", Title: "Git commit & push", DetailMD: "Commit `config/config.yaml` (and code if needed). Push to origin. Production rollout via git only.", Automated: false},
			{ID: "vps", Title: "VPS deploy", DetailMD: "On srv03: `git pull`, `docker compose build strategy-runner`, `docker compose up -d strategy-runner`.", Automated: false},
			{ID: "reload", Title: "Config reload on VPS", DetailMD: "`curl -X POST http://127.0.0.1:9020/config/reload` after config is on the server.", Automated: false},
			{ID: "smoke", Title: "Smoke checks", DetailMD: "Flat positions, instances list, events — see smoke_commands.", Automated: false},
			{ID: "enable", Title: "Enable live (explicit)", DetailMD: "Only after smoke: apply phase=enable_live + confirm_live + STRATEGY_LAB_ALLOW_LIVE_START on runner.", Automated: true, APIAction: "POST /strategy-lab/apply phase=enable_live"},
		}
	} else {
		plan.Steps = []StrategyLabRolloutStep{
			{ID: "review", Title: "Проверить анализ", DetailMD: "Вердикт, параметры кандидата, предупреждения. PnL — пункты цены, не рубли на счёте.", Automated: false},
			{ID: "stage", Title: "Записать в config (disabled)", DetailMD: fmt.Sprintf("Instance `%s` в config.yaml с **enabled: false**, reload runner. Без боевых заявок.", instanceID), Automated: true, APIAction: "POST /strategy-lab/apply phase=stage"},
			{ID: "git", Title: "Git commit и push", DetailMD: "Закоммитить `config/config.yaml` (и код при необходимости). На прод — только через git.", Automated: false},
			{ID: "vps", Title: "Деплой на VPS", DetailMD: "На srv03: `git pull`, `docker compose build strategy-runner`, `docker compose up -d strategy-runner`.", Automated: false},
			{ID: "reload", Title: "Reload на VPS", DetailMD: "`curl -X POST http://127.0.0.1:9020/config/reload` после появления config на сервере.", Automated: false},
			{ID: "smoke", Title: "Smoke-проверки", DetailMD: "Flat позиции, /instances, events — см. smoke_commands.", Automated: false},
			{ID: "enable", Title: "Включить live (явно)", DetailMD: "После smoke: apply phase=enable_live + confirm_live + STRATEGY_LAB_ALLOW_LIVE_START на runner.", Automated: true, APIAction: "POST /strategy-lab/apply phase=enable_live"},
		}
	}
	if candidate != nil && cfgProd != nil {
		plan.SmokeCommands = []string{
			"curl -fsS http://127.0.0.1:9020/instances",
			fmt.Sprintf("curl -fsS http://127.0.0.1:9020/instances/%s/portfolio", cfgProd.InstanceID),
			fmt.Sprintf("curl -fsS 'http://127.0.0.1:9020/instances/%s/events?limit=20'", cfgProd.InstanceID),
			"docker compose -f deployments/docker-compose.yaml -p 24alert logs --tail=100 strategy-runner",
		}
	}
	return plan
}
