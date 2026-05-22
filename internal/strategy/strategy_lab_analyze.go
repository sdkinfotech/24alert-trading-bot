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
	Ticker       string                   `json:"ticker"`
	UID          string                   `json:"uid"`
	Days         int                      `json:"days"`
	Verdict      string                   `json:"verdict"` // keep_prod | deploy_candidate | research_only | insufficient
	VerdictReason string                  `json:"verdict_reason"`
	SummaryMD    string                   `json:"summary_md"`
	VsProductionMD string                 `json:"vs_production_md,omitempty"`
	WarningsMD   string                   `json:"warnings_md,omitempty"`
	Candidate    *StrategyLabRunRow       `json:"candidate,omitempty"`
	Production   *StrategyLabRunRow       `json:"production,omitempty"`
	ConfigProd   *StrategyLabConfigProd   `json:"config_prod,omitempty"`
	TopRows      []StrategyLabRunRow      `json:"top_rows"`
	Rollout      StrategyLabRolloutPlan   `json:"rollout"`
	Matrix       json.RawMessage          `json:"matrix,omitempty"`
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
	top := collectLabTopRows(inst, prod, 12)

	verdict, reason := labVerdict(candidate, prod, cfgProd, lang)
	summary := labAnalysisSummaryMD(ticker, days, lang, inst, candidate, prod, cfgProd, verdict, reason)
	vsProd := labAnalysisVsProdMD(candidate, prod, cfgProd, lang)
	warnings := labStandardWarningsForRow(candidate, lang)
	warnings = strings.TrimSpace(warnings + "\n" + labAnalysisExtraWarnings(lang))

	rollout := buildRolloutPlan(verdict, candidate, cfgProd, lang)

	return &StrategyLabAnalyzeResponse{
		Ticker:         ticker,
		UID:            uid,
		Days:           days,
		Verdict:        verdict,
		VerdictReason:  reason,
		SummaryMD:      summary,
		VsProductionMD: vsProd,
		WarningsMD:     warnings,
		Candidate:      candidate,
		Production:     prod,
		ConfigProd:     cfgProd,
		TopRows:        top,
		Rollout:        rollout,
	}
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
