package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/24alert/trading-bot/pkg/llm"
	"github.com/24alert/trading-bot/pkg/metrics"
)

// LabParamMap accepts string or numeric JSON in params (Python backtest uses numbers).
type LabParamMap map[string]string

func (p *LabParamMap) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*p = LabParamMap{}
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	out := make(LabParamMap, len(raw))
	for k, v := range raw {
		out[k] = labParamJSONValueToString(v)
	}
	*p = out
	return nil
}

func labParamJSONValueToString(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return n.String()
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		if f == float64(int64(f)) {
			return strconv.FormatInt(int64(f), 10)
		}
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return strconv.FormatBool(b)
	}
	return strings.TrimSpace(string(raw))
}

// StrategyLabOptimization describes what the optimize endpoint actually ran.
type StrategyLabOptimization struct {
	Kind              string    `json:"kind,omitempty"`
	FixedFast         int       `json:"fixed_fast,omitempty"`
	FixedSlow         int       `json:"fixed_slow,omitempty"`
	TrailingGridPct   []float64 `json:"trailing_grid_pct,omitempty"`
	Step1BestRiskScore float64  `json:"step1_best_risk_score,omitempty"`
	NoteRU            string    `json:"note_ru,omitempty"`
	NoteEN            string    `json:"note_en,omitempty"`
}

// StrategyLabInterpretRequest is sent after compare/optimize results are shown.
type StrategyLabInterpretRequest struct {
	Ticker       string                  `json:"ticker"`
	Days         int                     `json:"days"`
	Lang         string                  `json:"lang"`
	Strategy     string                  `json:"strategy,omitempty"`
	Rows         []StrategyLabRunRow     `json:"rows"`
	Selected     *StrategyLabRunRow      `json:"selected,omitempty"`
	Production   *StrategyLabRunRow      `json:"production,omitempty"`
	Optimization *StrategyLabOptimization `json:"optimization,omitempty"`
}

// StrategyLabInterpretResponse is human-readable guidance for the operator.
type StrategyLabInterpretResponse struct {
	SummaryMD      string `json:"summary_md"`
	VsProductionMD string `json:"vs_production_md,omitempty"`
	Recommendation string `json:"recommendation"` // apply | keep_prod | research_only | wait
	ApplyHintMD    string `json:"apply_hint_md,omitempty"`
	WarningsMD     string `json:"warnings_md,omitempty"`
	LLMModel       string `json:"llm_model,omitempty"`
	LLMFallback    bool   `json:"llm_fallback,omitempty"`
}

type strategyLabLLMOutput struct {
	SummaryMD      string `json:"summary_md"`
	VsProductionMD string `json:"vs_production_md"`
	Recommendation string `json:"recommendation"`
	ApplyHintMD    string `json:"apply_hint_md"`
	WarningsMD     string `json:"warnings_md"`
}

func strategyLabModel() string {
	if m := strings.TrimSpace(os.Getenv("STRATEGY_LAB_MODEL")); m != "" {
		return m
	}
	return assistantModel()
}

func strategyLabModelFallbacks() []string {
	if v := strings.TrimSpace(os.Getenv("STRATEGY_LAB_MODEL_FALLBACKS")); v != "" {
		return strings.Split(v, ",")
	}
	return assistantModelFallbacks()
}

func strategyLabLLMEnabled() bool {
	v := strings.TrimSpace(os.Getenv("STRATEGY_LAB_LLM_ENABLED"))
	if v != "" {
		return strings.EqualFold(v, "true") || v == "1"
	}
	return assistantEnabled()
}

func (r *Runner) StrategyLabInterpret(ctx context.Context, req StrategyLabInterpretRequest) (*StrategyLabInterpretResponse, error) {
	ticker := strings.TrimSpace(req.Ticker)
	if ticker == "" {
		return nil, fmt.Errorf("ticker is required")
	}
	if len(req.Rows) == 0 {
		return nil, fmt.Errorf("rows are required")
	}
	days := req.Days
	if days <= 0 {
		days = 90
	}
	lang := strings.TrimSpace(req.Lang)
	if lang != "en" {
		lang = "ru"
	}

	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	if !strategyLabLLMEnabled() {
		out := strategyLabFallbackInterpret(req, lang)
		out.LLMFallback = true
		return out, nil
	}

	facts := buildStrategyLabFacts(req, days, lang)
	factsJSON, _ := json.MarshalIndent(facts, "", "  ")

	system := strategyLabSystemPrompt(lang)
	user := fmt.Sprintf("Тикер %s, период %d дней.\nДанные бэктеста (JSON):\n%s", ticker, days, string(factsJSON))

	res, err := llm.CompleteJSON(ctx, llm.JSONCompletionRequest{
		Service:   metrics.LLMServiceStrategyLab,
		Model:     strategyLabModel(),
		Fallbacks: strategyLabModelFallbacks(),
		System:    system,
		User:      user,
		MaxTokens: 2048,
		Timeout:   85 * time.Second,
	})
	if err != nil {
		r.logger.Warn("strategy lab llm failed, using fallback", "error", err)
		out := strategyLabFallbackInterpret(req, lang)
		out.LLMFallback = true
		return out, nil
	}

	var raw strategyLabLLMOutput
	content := strings.TrimSpace(res.Content)
	if i := strings.Index(content, "{"); i > 0 {
		content = content[i:]
	}
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		r.logger.Warn("strategy lab llm parse failed", "error", err)
		out := strategyLabFallbackInterpret(req, lang)
		out.LLMFallback = true
		out.LLMModel = res.Model
		return out, nil
	}

	out := &StrategyLabInterpretResponse{
		SummaryMD:      strings.TrimSpace(raw.SummaryMD),
		VsProductionMD: strings.TrimSpace(raw.VsProductionMD),
		Recommendation: sanitizeLabRecommendation(raw.Recommendation),
		ApplyHintMD:    strings.TrimSpace(raw.ApplyHintMD),
		WarningsMD:     strings.TrimSpace(raw.WarningsMD),
		LLMModel:       res.Model,
	}
	if out.SummaryMD == "" {
		fb := strategyLabFallbackInterpret(req, lang)
		out.SummaryMD = fb.SummaryMD
		out.VsProductionMD = fb.VsProductionMD
		out.Recommendation = fb.Recommendation
		out.ApplyHintMD = fb.ApplyHintMD
		out.WarningsMD = fb.WarningsMD
		out.LLMFallback = true
	}
	return out, nil
}

func strategyLabSystemPrompt(lang string) string {
	langNote := "Ответ на русском."
	if lang == "en" {
		langNote = "Answer in English."
	}
	return `Ты аналитик бэктеста торговых стратегий 24alert на MOEX фьючерсах.
PnL — в пунктах цены фьючерса, не рубли на счёте. Sharpe в бэктесте завышен (мало сделок, неверная annualization).
Комиссии, ГО, проскальзывание, broker stop и watchdog не моделируются. Прошлое ≠ будущее. Не обещай прибыль.

Если optimization.kind = sma_two_step: явно напиши, что таблица — одна пара fast/slow и перебор trailing, НЕ разные стратегии.
Сравни лучший trailing с production (params + PnL). Не рекомендуй apply только из-за +0.5 пт PnL.
После safety incident (2026-05) live apply только если параметры осмысленно лучше прод и trailing > 0.

` + langNote + `
Строго JSON:
{
  "summary_md": "структурированный markdown: методика, таблица trailing или топ конфигов, вывод",
  "vs_production_md": "сравнение с prod: fast/slow/trailing и PnL/DD/сделок",
  "recommendation": "apply|keep_prod|research_only|wait",
  "apply_hint_md": "конкретные params или «не выкатывать»",
  "warnings_md": "ограничения бэктеста и риски"
}
recommendation:
- apply — осмысленно лучше прод по PnL/risk и deployable
- keep_prod — прод не хуже или смена fast/slow не обоснована
- research_only — только research
- wait — мало сделок или неясно`
}

func sanitizeLabRecommendation(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "apply", "keep_prod", "research_only", "wait":
		return strings.ToLower(strings.TrimSpace(s))
	default:
		return "wait"
	}
}

type strategyLabFacts struct {
	Ticker       string                   `json:"ticker"`
	Days         int                      `json:"days"`
	Strategy     string                   `json:"strategy_filter,omitempty"`
	AnalysisKind string                   `json:"analysis_kind,omitempty"`
	Optimization *StrategyLabOptimization `json:"optimization,omitempty"`
	Rows         []strategyLabFactsRow    `json:"rows"`
	Selected     *strategyLabFactsRow     `json:"selected,omitempty"`
	Production   *strategyLabFactsRow     `json:"production,omitempty"`
	Notes        []string                 `json:"notes"`
}

type strategyLabFactsRow struct {
	Strategy      string            `json:"strategy"`
	Mode          string            `json:"mode,omitempty"`
	Params        map[string]string `json:"params"`
	PnL           float64           `json:"pnl"`
	Sharpe        float64           `json:"sharpe"`
	MaxDrawdown   float64           `json:"max_drawdown"`
	Trades        int               `json:"trades"`
	LiveEligible  bool              `json:"live_eligible"`
	Quality       string            `json:"quality,omitempty"`
	RiskScore     float64           `json:"risk_score,omitempty"`
}

func buildStrategyLabFacts(req StrategyLabInterpretRequest, days int, lang string) strategyLabFacts {
	f := strategyLabFacts{
		Ticker:       req.Ticker,
		Days:         days,
		Strategy:     req.Strategy,
		AnalysisKind: detectLabAnalysisKind(req.Rows, req.Optimization),
		Optimization: req.Optimization,
		Notes: []string{
			"PnL in futures price points, not RUB PnL on account.",
			"Commission and margin not included.",
			"Backtest Sharpe is not a reliable annual figure.",
		},
	}
	if lang == "ru" {
		f.Notes = []string{
			"PnL в пунктах цены фьючерса, не рубли на счёте.",
			"Комиссии и ГО не учтены.",
			"Sharpe в бэктесте не годовой показатель с биржи.",
		}
	}
	for _, r := range req.Rows {
		f.Rows = append(f.Rows, rowToFacts(r))
	}
	if req.Selected != nil {
		x := rowToFacts(*req.Selected)
		f.Selected = &x
	}
	if req.Production != nil {
		x := rowToFacts(*req.Production)
		f.Production = &x
	}
	return f
}

func rowToFacts(r StrategyLabRunRow) strategyLabFactsRow {
	return strategyLabFactsRow{
		Strategy:     r.Strategy,
		Mode:         r.Mode,
		Params:       map[string]string(r.Params),
		PnL:          r.PnL,
		Sharpe:       r.Sharpe,
		MaxDrawdown:  r.MaxDrawdown,
		Trades:       r.Trades,
		LiveEligible: r.LiveEligible,
		Quality:      r.Quality,
		RiskScore:    r.RiskScore,
	}
}

func strategyLabFallbackInterpret(req StrategyLabInterpretRequest, lang string) *StrategyLabInterpretResponse {
	return labBuildStructuredInterpret(req, lang)
}

func detectLabAnalysisKind(rows []StrategyLabRunRow, opt *StrategyLabOptimization) string {
	if opt != nil && opt.Kind == "sma_two_step" {
		return "sma_trailing_sweep"
	}
	if len(rows) == 0 {
		return "empty"
	}
	nonProd := 0
	sameFS := true
	var fast, slow string
	for _, r := range rows {
		if r.Mode == "prod" || r.Mode == "prod_baseline" {
			continue
		}
		nonProd++
		f := r.Params["fast_period"]
		s := r.Params["slow_period"]
		if fast == "" {
			fast, slow = f, s
		} else if f != fast || s != slow {
			sameFS = false
		}
	}
	if nonProd >= 3 && sameFS && fast != "" {
		allTrail := true
		for _, r := range rows {
			if r.Mode == "prod" || r.Mode == "prod_baseline" {
				continue
			}
			if r.Mode != "sma_trailing" && r.Mode != "" {
				allTrail = false
				break
			}
		}
		if allTrail {
			return "sma_trailing_sweep"
		}
	}
	diverse := map[string]struct{}{}
	for _, r := range rows {
		if r.Mode == "prod" || r.Mode == "prod_baseline" {
			continue
		}
		diverse[r.Strategy+"/"+r.Mode] = struct{}{}
	}
	if len(diverse) > 2 {
		return "matrix_compare"
	}
	return "generic"
}

func labBuildStructuredInterpret(req StrategyLabInterpretRequest, lang string) *StrategyLabInterpretResponse {
	best := pickBestLabRow(req.Rows)
	sel := best
	if req.Selected != nil {
		sel = *req.Selected
	}
	prod := req.Production
	if prod == nil {
		for _, r := range req.Rows {
			if r.Mode == "prod" || r.Mode == "prod_baseline" {
				cp := r
				prod = &cp
				break
			}
		}
	}

	kind := detectLabAnalysisKind(req.Rows, req.Optimization)
	rec, recReason := labRecommend(sel, prod, kind, lang)
	warnings := labStandardWarnings(sel, lang)
	if kind == "sma_trailing_sweep" {
		warnings = strings.TrimSpace(warnings + "\n" + warnLine(lang,
			"Для полного сравнения семейств стратегий используйте «Сравнить все стратегии», не только «Оптимизировать».",
			"Use «Compare all strategies» for a full matrix, not optimize alone."))
	}

	summary := labSummaryMD(req, kind, best, sel, lang)
	vsProd := labVsProdDetailed(prod, sel, best, kind, lang)
	applyHint := labApplyHintDetailed(sel, prod, rec, lang)

	return &StrategyLabInterpretResponse{
		SummaryMD:      summary,
		VsProductionMD: vsProd,
		Recommendation: rec,
		ApplyHintMD:    applyHint + "\n\n_" + recReason + "_",
		WarningsMD:     warnings,
		LLMFallback:    true,
	}
}

func labRecommend(sel StrategyLabRunRow, prod *StrategyLabRunRow, kind, lang string) (string, string) {
	if sel.Trades < 5 {
		return "wait", warnLine(lang, "Мало сделок для вывода.", "Too few trades.")
	}
	if !sel.LiveEligible {
		if sel.PnL > 0 {
			return "research_only", warnLine(lang, "Вариант не для live.", "Not live-eligible.")
		}
		return "wait", warnLine(lang, "Нет прибыльного deployable варианта.", "No profitable deployable row.")
	}
	if prod == nil {
		if sel.PnL > 0 {
			return "wait", warnLine(lang, "Нет бэктеста текущего прод — сравнение неполное.", "No production baseline backtest.")
		}
		return "wait", warnLine(lang, "Нет данных.", "Insufficient data.")
	}

	// Same fast/slow as prod: only trailing tuning — need meaningful PnL edge and similar or lower DD.
	sameParams := paramsSameSMA(sel.Params, prod.Params)
	deltaPnL := sel.PnL - prod.PnL
	deltaDD := sel.MaxDrawdown - prod.MaxDrawdown

	if sameParams {
		if deltaPnL > 0.3 && deltaDD <= prod.MaxDrawdown*1.15 {
			return "keep_prod", warnLine(lang,
				"Параметры совпадают с продом — менять config не нужно.",
				"Params match production — no config change needed.")
		}
		return "keep_prod", warnLine(lang, "Уже на проде с этими параметрами.", "Already on production params.")
	}

	// Different fast/slow — higher bar after two-step optimize (periods picked without trailing).
	if kind == "sma_trailing_sweep" {
		if deltaPnL < 1.5 {
			return "keep_prod", warnLine(lang,
				fmt.Sprintf("Оптимизатор сменил fast/slow; прирост PnL %.2f пт. слишком мал для смены прод.", deltaPnL),
				fmt.Sprintf("Optimizer changed fast/slow; PnL gain %.2f pts is too small to change production.", deltaPnL))
		}
		if deltaDD > prod.MaxDrawdown*1.25 && deltaPnL < 3 {
			return "keep_prod", warnLine(lang,
				"Просадка выше прод при умеренном приросте PnL.",
				"Drawdown worse than prod for modest PnL gain.")
		}
	}

	if deltaPnL >= 1.5 && sel.Sharpe >= prod.Sharpe*0.85 && sel.Trades >= prod.Trades-3 {
		return "apply", warnLine(lang,
			"Бэктест лучше прод по PnL; перед live — политика safety TASK-027.",
			"Backtest beats prod on PnL; check TASK-027 safety before live.")
	}
	return "keep_prod", warnLine(lang,
		fmt.Sprintf("Прод не хуже: ΔPnL %+.2f пт., ΔDD %+.2f.", deltaPnL, deltaDD),
		fmt.Sprintf("Keep prod: ΔPnL %+.2f pts, ΔDD %+.2f.", deltaPnL, deltaDD))
}

func paramsSameSMA(a, b map[string]string) bool {
	if a == nil || b == nil {
		return false
	}
	for _, k := range []string{"fast_period", "slow_period", "trailing_stop_pct", "initial_stop_swing_bars"} {
		if strings.TrimSpace(a[k]) != strings.TrimSpace(b[k]) {
			return false
		}
	}
	return true
}

func labStandardWarnings(sel StrategyLabRunRow, lang string) string {
	var parts []string
	parts = append(parts, warnLine(lang,
		"Бэктест: пункты цены, без комиссий/ГО; Sharpe завышен на малой выборке.",
		"Backtest: price points, no fees/margin; Sharpe inflated on small samples."))
	if sel.Trades < 15 {
		parts = append(parts, warnLine(lang,
			fmt.Sprintf("Всего %d сделок за период — выводы предварительные.", sel.Trades),
			fmt.Sprintf("Only %d trades in window — preliminary.", sel.Trades)))
	}
	return strings.Join(parts, "\n")
}

func labSummaryMD(req StrategyLabInterpretRequest, kind string, best, sel StrategyLabRunRow, lang string) string {
	if kind == "sma_trailing_sweep" {
		return labSummaryTrailingSweep(req, best, sel, lang)
	}
	if lang == "en" {
		return fmt.Sprintf("**%s** — ~%d days backtest.\n\nBest row: **%s** / %s — PnL **%.2f**, %d trades, params: %s.\n\nSelected: PnL **%.2f**, %d trades, params: %s.",
			req.Ticker, req.Days, best.Strategy, best.Mode, best.PnL, best.Trades, paramsSummary(best.Params),
			sel.PnL, sel.Trades, paramsSummary(sel.Params))
	}
	return fmt.Sprintf("**%s** — бэктест ~%d дн.\n\nЛучшая строка: **%s** / %s — PnL **%.2f** пт., %d сделок, параметры: %s.\n\nВыбрано: PnL **%.2f**, %d сделок, параметры: %s.",
		req.Ticker, req.Days, best.Strategy, best.Mode, best.PnL, best.Trades, paramsSummary(best.Params),
		sel.PnL, sel.Trades, paramsSummary(sel.Params))
}

func labSummaryTrailingSweep(req StrategyLabInterpretRequest, best, sel StrategyLabRunRow, lang string) string {
	fast := best.Params["fast_period"]
	slow := best.Params["slow_period"]
	if req.Optimization != nil && req.Optimization.FixedFast > 0 {
		fast = fmt.Sprintf("%d", req.Optimization.FixedFast)
		slow = fmt.Sprintf("%d", req.Optimization.FixedSlow)
	}
	note := ""
	if req.Optimization != nil {
		if lang == "en" && req.Optimization.NoteEN != "" {
			note = req.Optimization.NoteEN
		} else if req.Optimization.NoteRU != "" {
			note = req.Optimization.NoteRU
		}
	}
	if note == "" {
		note = warnLine(lang,
			"Таблица — одна пара SMA и перебор trailing 0.3%–1.5%, не разные стратегии.",
			"Table is one SMA pair and trailing sweep 0.3%–1.5%, not different strategies.")
	}

	table := labTrailingTableMD(req.Rows, lang)
	bestTrail := best.Params["trailing_stop_pct"]

	if lang == "en" {
		return fmt.Sprintf(
			"## What the optimizer did\n%s\n\n**Fixed SMA:** fast=%s, slow=%s on ~%d days.\n\n## Trailing sensitivity\n%s\n\n## Best in grid\nTrailing **%s** — PnL **%.2f** pts, DD **%.2f**, **%d** trades (Sharpe %.2f is not exchange-grade).\n\n**Selected:** trailing **%s**, PnL **%.2f**.",
			note, fast, slow, req.Days, table, bestTrail, best.PnL, best.MaxDrawdown, best.Trades, best.Sharpe,
			sel.Params["trailing_stop_pct"], sel.PnL,
		)
	}
	return fmt.Sprintf(
		"## Что сделал оптимизатор\n%s\n\n**Зафиксированная пара SMA:** fast=%s, slow=%s за ~%d дн.\n\n## Чувствительность к trailing\n%s\n\n## Лучший в сетке\nTrailing **%s** — PnL **%.2f** пт., просадка **%.2f**, **%d** сделок (Sharpe %.2f — ориентир, не биржевой).\n\n**Выбранная строка:** trailing **%s**, PnL **%.2f** пт.",
		note, fast, slow, req.Days, table, bestTrail, best.PnL, best.MaxDrawdown, best.Trades, best.Sharpe,
		sel.Params["trailing_stop_pct"], sel.PnL,
	)
}

func labTrailingTableMD(rows []StrategyLabRunRow, lang string) string {
	var sweep []StrategyLabRunRow
	for _, r := range rows {
		if r.Mode == "prod" || r.Mode == "prod_baseline" {
			continue
		}
		if r.Mode == "sma_trailing" || (r.Mode == "" && r.Params["trailing_stop_pct"] != "") {
			sweep = append(sweep, r)
		}
	}
	if len(sweep) == 0 {
		return "—"
	}
	// sort by trailing pct ascending
	for i := 0; i < len(sweep); i++ {
		for j := i + 1; j < len(sweep); j++ {
			if sweep[j].Params["trailing_stop_pct"] < sweep[i].Params["trailing_stop_pct"] {
				sweep[i], sweep[j] = sweep[j], sweep[i]
			}
		}
	}
	hdr := "| trailing | PnL | DD | сделок |\n|----------|-----|-----|--------|\n"
	if lang == "en" {
		hdr = "| trailing | PnL | DD | trades |\n|----------|-----|-----|--------|\n"
	}
	var b strings.Builder
	b.WriteString(hdr)
	for _, r := range sweep {
		tr := r.Params["trailing_stop_pct"]
		if tr == "" {
			tr = "—"
		}
		b.WriteString(fmt.Sprintf("| %s | %.2f | %.2f | %d |\n", tr, r.PnL, r.MaxDrawdown, r.Trades))
	}
	return b.String()
}

func labVsProdDetailed(prod *StrategyLabRunRow, sel, best StrategyLabRunRow, kind, lang string) string {
	if prod == nil {
		return warnLine(lang,
			"**Прод:** бэктест текущего config не передан. Запустите «Сравнить все» или проверьте `config.yaml`.",
			"**Production:** no baseline backtest in response.")
	}
	if lang == "en" {
		return fmt.Sprintf(
			"**Production config** (backtest): fast=%s slow=%s trailing=%s — PnL **%.2f**, DD **%.2f**, **%d** trades.\n\n"+
				"**Best in optimizer grid:** fast=%s slow=%s trailing=%s — PnL **%.2f** (Δ **%+.2f** vs prod).\n\n"+
				"**Selected row:** fast=%s slow=%s trailing=%s — PnL **%.2f** (Δ **%+.2f** vs prod).",
			prod.Params["fast_period"], prod.Params["slow_period"], prod.Params["trailing_stop_pct"],
			prod.PnL, prod.MaxDrawdown, prod.Trades,
			best.Params["fast_period"], best.Params["slow_period"], best.Params["trailing_stop_pct"],
			best.PnL, best.PnL-prod.PnL,
			sel.Params["fast_period"], sel.Params["slow_period"], sel.Params["trailing_stop_pct"],
			sel.PnL, sel.PnL-prod.PnL,
		)
	}
	return fmt.Sprintf(
		"**Сейчас в config (бэктест):** fast=%s slow=%s trailing=%s — PnL **%.2f** пт., просадка **%.2f**, **%d** сделок.\n\n"+
			"**Лучший в сетке оптимизатора:** fast=%s slow=%s trailing=%s — PnL **%.2f** (Δ **%+.2f** пт. к прод).\n\n"+
			"**Выбранная строка:** fast=%s slow=%s trailing=%s — PnL **%.2f** (Δ **%+.2f** пт. к прод).",
		prod.Params["fast_period"], prod.Params["slow_period"], prod.Params["trailing_stop_pct"],
		prod.PnL, prod.MaxDrawdown, prod.Trades,
		best.Params["fast_period"], best.Params["slow_period"], best.Params["trailing_stop_pct"],
		best.PnL, best.PnL-prod.PnL,
		sel.Params["fast_period"], sel.Params["slow_period"], sel.Params["trailing_stop_pct"],
		sel.PnL, sel.PnL-prod.PnL,
	)
}

func labApplyHintDetailed(sel StrategyLabRunRow, prod *StrategyLabRunRow, rec, lang string) string {
	switch rec {
	case "apply":
		return labApplyHint(sel, lang)
	case "keep_prod":
		if prod != nil {
			return warnLine(lang,
				fmt.Sprintf("**Не менять config.** Оставить прод: %s", paramsSummary(prod.Params)),
				fmt.Sprintf("**Do not change config.** Keep production: %s", paramsSummary(prod.Params)))
		}
		return warnLine(lang, "**Не выкатывать** — оставить текущий прод.", "**Do not deploy** — keep production.")
	case "research_only":
		return warnLine(lang, "Только исследование, шаг «Запуск» не для live.", "Research only — do not use Launch for live.")
	default:
		return warnLine(lang, "Подождите: «Сравнить все стратегии» или больше данных.", "Wait — run full compare or gather more data.")
	}
}

func pickBestLabRow(rows []StrategyLabRunRow) StrategyLabRunRow {
	best := rows[0]
	bestScore := -1e18
	for _, r := range rows {
		if r.Mode == "prod" {
			continue
		}
		sc := r.RiskScore
		if sc == 0 && r.Trades >= 5 {
			sc = r.Sharpe*r.PnL - 0.35*r.MaxDrawdown
		}
		if sc > bestScore {
			bestScore = sc
			best = r
		}
	}
	return best
}

func labVsProdMD(prod *StrategyLabRunRow, sel StrategyLabRunRow, lang string) string {
	if prod == nil {
		return warnLine(lang, "Текущий прод в матрице не передан.", "No production baseline in matrix.")
	}
	delta := sel.PnL - prod.PnL
	if lang == "en" {
		return fmt.Sprintf("Production: **%s** PnL **%.2f** (%d trades). Selected vs prod: **%+.2f** pts.",
			prod.Strategy, prod.PnL, prod.Trades, delta)
	}
	return fmt.Sprintf("Сейчас на проде: **%s** PnL **%.2f** (%d сделок). Выбранное vs прод: **%+.2f** пт.",
		prod.Strategy, prod.PnL, prod.Trades, delta)
}

func labApplyHint(sel StrategyLabRunRow, lang string) string {
	if !sel.LiveEligible {
		return warnLine(lang,
			"На шаге «Запуск» эту строку применить нельзя (только research или ORB). Выберите строку с «Прод? = да».",
			"Cannot deploy this row (research-only or blocked). Pick a row with Live = yes.")
	}
	params := paramsSummary(sel.Params)
	if lang == "en" {
		return fmt.Sprintf("On **Launch**: keep row **%s / %s** selected, then **Write config and start**. Params: %s",
			sel.Strategy, sel.Mode, params)
	}
	return fmt.Sprintf("На шаге **«Запуск»**: оставьте строку **%s / %s**, нажмите **«Записать в config и запустить»**. Параметры: %s",
		sel.Strategy, sel.Mode, params)
}

func paramsSummary(p map[string]string) string {
	if len(p) == 0 {
		return "—"
	}
	var parts []string
	for k, v := range p {
		if k == "interval" || k == "type" || k == "quantity" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	if len(parts) == 0 {
		return "—"
	}
	return strings.Join(parts, ", ")
}

func warnLine(lang, ru, en string) string {
	if lang == "en" {
		return en
	}
	return ru
}

// StrategyLabRunRow mirrors backtest JSON rows for interpret API.
type StrategyLabRunRow struct {
	Strategy         string            `json:"strategy"`
	Mode             string            `json:"mode,omitempty"`
	Interval         string            `json:"interval,omitempty"`
	Params           LabParamMap `json:"params"`
	PnL              float64           `json:"pnl"`
	Trades           int               `json:"trades"`
	Wins             int               `json:"wins,omitempty"`
	Losses           int               `json:"losses,omitempty"`
	WinRate          float64           `json:"win_rate,omitempty"`
	MaxDrawdown      float64           `json:"max_drawdown"`
	Sharpe           float64           `json:"sharpe"`
	ProfitFactor     float64           `json:"profit_factor,omitempty"`
	RiskScore        float64           `json:"risk_score,omitempty"`
	Quality          string            `json:"quality,omitempty"`
	LiveEligible     bool              `json:"live_eligible,omitempty"`
	LiveBlockReason  string            `json:"live_block_reason,omitempty"`
}
