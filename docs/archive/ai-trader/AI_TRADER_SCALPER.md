# AI Trader / Scalper Architecture

Canonical project knowledge lives in Obsidian:
`24alert/AI Trader Scalper.md`.

This document is the repo-facing technical design for adding a live AI trader
that can be enabled from the dashboard for a selected futures instrument.

## Goal

Add a separate `advisor-svc` / `ai-trader` service. The operator selects a
futures ticker in the UI, provides a short trading instruction, and starts a
**level intraday** session (`strategy_kind: level_intraday`).

Phased UX (no separate observe/paper buttons):

| Phase | Behavior |
|-------|----------|
| `collecting` | Poll order book + prints; ring buffers ≥60s; **no micro-LLM** |
| `analyzing` | Micro-LLM on buffered window; advisor rollups 5m/15m/1h |
| `ready` | `trading_ready` + `level_playbook`; UI enables **Start trading** |
| `trading` | Limit orders at playbook levels; live refresh of walls/POC; LLM policy ~45s |

`armed_live` uses real broker limits when `AI_TRADER_ARMED_LIVE=true` and `confirm_live`.

### What updates during `trading`

| Component | Interval | Notes |
|-----------|----------|-------|
| Order book + tape | ~2s | `observeAITraderOnce` |
| Level playbook merge | 60s (env `AI_TRADER_PLAYBOOK_REFRESH_SEC`) | `playbook_refreshed` event; advisor 5m draft lite merge |
| LLM `trading_policy` + `trade_signal` | ~45s (`AI_TRADER_LLM_INTERVAL_TRADING_SEC`) | `replace_limit`, `adjust_stops`, `flatten` |
| Session snapshot SQLite | 30s | `data/ai_trader_sessions.db`; resume via `POST .../resume` |
| Working limit replace | On signal | Rate limit `max_cancel_replace_per_minute` |

Classic strategy instances should stay `enabled: false` when AI Trader owns the instrument.

The target live trader watches order books, public prints, last price, strategy
state, broker position, technical context, and adjacent instruments. It can
scale in/out, manage limit orders, flatten, and explain every action.

## Non-goals

- Do not put this inside Dashboard AI Chat. The chat remains read-only.
- Do not put this inside the cron AI Scanner. The scanner remains research and
  auto-config automation.
- Do not allow direct broker API access from the AI agent.
- Do not let an LLM bypass deterministic risk gates.

## Target Architecture

```mermaid
flowchart LR
  OperatorUI["Operator UI"] --> TraderControl["AI Trader Control"]
  TraderControl --> AdvisorSvc["advisor-svc"]
  GatewayStreams["Orderbook Trades Prices"] --> MarketContext["Market Context Engine"]
  StrategyRunner["Runner State Events"] --> MarketContext
  MarketContext --> AdvisorSvc
  AdvisorSvc --> RiskGate["Risk Gate"]
  RiskGate --> OrderControl["Order Control Layer"]
  OrderControl --> Broker["Broker Orders"]
  Broker --> Journal["Journal Audit Metrics"]
  AdvisorSvc --> Journal
```

## Current Platform Gaps

Market data:

- `internal/gateway/handlers/stream.go` exposes:
  - `GET /api/v1/stream/candles`
  - `GET /api/v1/stream/orderbook`
  - `GET /api/v1/stream/trades`
  - `GET /api/v1/stream/last-price`
- `internal/marketdata/stream.go` has internal `SubscribeTrades` and
  `SubscribeLastPrice`; gateway now exposes them as read-only WebSocket APIs.
- `StreamManager` now uses shared fan-out/ref-count subscriptions, so multiple
  downstream consumers can read the same `(uid, stream type, depth)` without
  opening duplicate upstream T-Invest subscriptions.
- Order book stream currently limits `uids` to 50 in code and sends snapshots
  only.
- Slow consumers can cause silent snapshot drops.
- Reconnect re-subscribes upstream and keeps downstream fan-out hubs alive.

Orders:

- `internal/order/service.go` supports `PostOrder`, `CancelOrder`,
  `ReplaceOrder`, `GetOrders`, `GetOrderState`, and stop orders.
- Current strategy model returns `strategy.Signal`, which is not enough for
  cancel/replace/target-position workflows.
- Partial fills and replace chains need stronger persistent accounting.
- Risk checks need to include broker position + active orders + proposed action.

## Market Microstructure Foundation

Add or formalize these streams:

| Stream | Endpoint | Source | Purpose |
|--------|----------|--------|---------|
| Order book | `/api/v1/stream/orderbook` | existing orderbook stream | depth, spread, imbalance |
| Public trades | `/api/v1/stream/trades` | `SubscribeTrades` | prints, direction, quantity |
| Last price | `/api/v1/stream/last-price` | `SubscribeLastPrice` | price heartbeat |
| Unified market | `/api/v1/stream/market` | not implemented | future one-feed endpoint for `advisor-svc` |

All market events should use a common envelope:

```json
{
  "type": "orderbook",
  "uid": "instrument-uid",
  "seq": 123456,
  "exchange_ts": "2026-05-18T10:01:02.123Z",
  "receive_ts": "2026-05-18T10:01:02.156Z",
  "data_freshness_ms": 33,
  "dropped_before": 0,
  "payload": {}
}
```

Required derived features:

- best bid / best ask / mid;
- absolute and bps spread;
- top-N bid/ask volume;
- imbalance and depth skew;
- wall persistence and wall pull/add rate;
- print delta and volatility bursts;
- stale-data and dropped-frame counters.

## Order Control Layer

The AI trader should emit actions, not bare buy/sell signals.

| Action | Purpose |
|--------|---------|
| `place_market` | immediate entry/exit |
| `place_limit` | passive order with TTL |
| `cancel` | cancel one active order |
| `replace` | amend price/qty with a new client order id |
| `cancel_all` | cancel all orders owned by the session |
| `flatten` | close the current position |
| `reduce_only` | reduce exposure only |
| `target_position` | move broker position toward target qty |

Persist order ownership with:

- `session_id`, `account_id`, `instrument_uid`;
- client order id and broker order id;
- parent order id for replacement chains;
- side, type, requested qty, price;
- cumulative filled qty, remaining qty, avg fill price;
- status, timestamps, risk decision id, reason.

Partial fills must be handled as delta fills. `partially_filled` must not clear
the order lifecycle as if the whole order filled.

## Risk Gates

`RiskGate` is deterministic and blocks any action that violates policy.

Required gates:

- session explicitly enabled in UI;
- selected instrument only, futures only;
- FORTS session guard and trading status pass;
- fresh orderbook / prints / broker state;
- max position;
- max order size;
- max active orders;
- max trades per minute;
- max cancel/replace rate;
- max session loss and max daily loss;
- spread/slippage guard;
- futures margin / GO check;
- broker position and active-order reconciliation.

Emergency behavior:

- stale market data: cancel entry orders, block new entries;
- lost order stream: block actions and reconcile;
- loss limit: cancel all and flatten or safe-stop according to session policy;
- runaway cancel/replace: freeze session;
- broker/session mismatch: cancel all and require operator review.

## advisor-svc API

Draft API:

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/ai-trader/sessions` | create/start session |
| `GET` | `/ai-trader/sessions` | list sessions |
| `GET` | `/ai-trader/sessions/{id}` | status, position, active orders |
| `POST` | `/ai-trader/sessions/{id}/pause` | pause without flatten |
| `POST` | `/ai-trader/sessions/{id}/stop` | stop and cancel active orders |
| `POST` | `/ai-trader/sessions/{id}/flatten` | flatten position |
| `POST` | `/ai-trader/sessions/{id}/instruction` | update operator instruction |
| `GET` | `/ai-trader/sessions/{id}/events` | decisions/actions/rejections |
| `GET` | `/ai-trader/sessions/{id}/features` | current microstructure features |

Example session config:

```yaml
mode: armed_live
account_id: "2001673385"
instrument_uid: "..."
ticker: BMM6
instruction: "ищи роботов и плотности, скальпинг частями"
limits:
  max_position: 1
  max_order_size: 1
  max_active_orders: 2
  max_trades_per_minute: 3
  max_cancel_replace_per_minute: 10
  max_session_loss_rub: 300
  max_daily_loss_rub: 500
  max_spread_bps: 15
  stale_data_ms: 1500
  session_timeout_minutes: 30
```

## Trader Brain

Use a hybrid model:

- deterministic feature engine computes facts and hard gates;
- LLM/agent evaluates context, writes hypotheses, and chooses tactical intent;
- deterministic risk and order layers decide whether anything is executable.

### Dynamic trading policy (implemented)

Each session exposes `active_policy` on the runner API and dashboard **Session plan (AI)** card.

| Layer | Role |
|-------|------|
| **Safety floor** | Fixed caps: max lots, session loss, spread/stale, kill switch, `risk-svc` on live orders. LLM cannot override these. |
| **DynamicTradingPolicy** | LLM (`trading_policy` in JSON) and advisor readiness seed: `entry_min_confidence`, `confluence_min_score`, `sl_mult_atr`, `tp_mult_atr`, `allow_new_entry`, `market_bias`, `preferred_levels`. Values are clamped (e.g. entry confidence ≤ 0.55). |
| **Executor** | Places limits from `trade_signal` or confluence; applies soft SL/TP from policy; in trading phase LLM may send `adjust_stops` / `stop_loss` / `take_profit` on open positions. |

Env:

- `AI_TRADER_LLM_INTERVAL_TRADING_SEC` — LLM cadence in `trading` phase (default 45s; was 2× analyze interval).
- Analyze phase still uses `AI_TRADER_LLM_INTERVAL_SEC` (default 90s).

Metrics: `alert24_ai_trader_policy_updates_total`, `alert24_ai_trader_stop_adjustments_total`.

Initial tactics:

- passive entry near a persistent wall;
- breakout through liquidity;
- fade false/spoof walls;
- scale-in ladder;
- partial take-profit;
- reduce on adverse prints;
- flatten on orderbook risk;
- trailing based on microstructure.

## UI Panel

`AI Trader` tab in Strategy Dashboard (`web/strategy-dashboard`):

- default instrument preset **Brent mini (BMM6)** — UID `dc1ffa30-70a4-4a7b-807a-4f31c2951f7e`;
- account + instrument picker (MOEX catalog + strategy picks);
- **Start monitoring** → `POST /ai-trader/sessions` with `strategy_kind: level_intraday` (no `mode`);
- phase stepper: collecting → analyzing → ready → trading;
- report chips 5m / 15m / 1h from `phase_progress.reports_ready`;
- **Start trading** — disabled until `phase=ready` and `trading_ready`; green pulse when enabled;
- level playbook S/R list; **session plan (AI)** (`active_policy`); trading desk when `phase=trading`;
- instruction textarea;
- stop session;
- kill switch;
- flatten;
- cancel all active orders;
- orderbook ladder;
- prints tape;
- spread/mid/imbalance;
- broker/session position;
- active orders;
- risk state;
- last decision;
- rejected actions journal.

## Observability

Metrics:

- `alert24_ai_trader_sessions_active`
- `alert24_ai_trader_feed_freshness_ms`
- `alert24_ai_trader_events_total`
- `alert24_ai_trader_dropped_events_total`
- `alert24_ai_trader_decisions_total`
- `alert24_ai_trader_decision_latency_seconds`
- `alert24_llm_requests_total{service,model,result}` — `service`: `advisor`, `ai_trader`, `ai_chat`; `result`: `success`, `rate_limit`, `parse_error`, `http_error`, `fallback`
- `alert24_llm_request_duration_seconds{service,model}` — latency успешных вызовов
- `advisor_reports_total{timeframe,status}`, `advisor_llm_errors_total{model}` (legacy)
- Grafana: dashboard **24alert LLM / OpenRouter** (`deployments/grafana/dashboards/llm-observability.json`)
- `alert24_ai_trader_risk_rejections_total`
- `alert24_ai_trader_orders_total`
- `alert24_ai_trader_cancel_replace_total`
- `alert24_ai_trader_partial_fills_total`
- `alert24_ai_trader_slippage_rub`
- `alert24_ai_trader_pnl_rub`
- `alert24_ai_trader_kill_switch_total`

Persist every decision with:

- session id;
- operator instruction;
- model and prompt version;
- feature snapshot hash;
- tactical intent and confidence;
- chosen action;
- risk result;
- order ids;
- broker execution outcome;
- resulting position and PnL.

## Minimal Delivery Sequence

1. Add the docs and safety spec.
2. Add trades/last-price/unified gateway streams.
3. Add explicit drop counters/freshness metrics to existing orderbook/trade/last-price hubs.
4. Add persistent order-control schema and action model.
5. Add `advisor-svc` in `observe` and `paper`.
6. Add dashboard AI Trader panel without live execution.
7. Add metrics, audit journal, and session reports.
8. Enable `armed_live` only for one selected futures instrument with small
   limits, short timeout, and visible kill switch.

## Implemented Slice: level intraday (TASK-028)

Status: implemented in `strategy-runner` + `advisor-svc`; live broker execution
not enabled.

Runtime API:

- `GET /ai-trader/sessions`
- `POST /ai-trader/sessions` — `strategy_kind: level_intraday` only; legacy `mode` (`observe`/`paper`) → **400**
- `GET /ai-trader/sessions/{id}` — `phase`, `phase_progress`, `level_playbook`, `paper_state`
- `POST /ai-trader/sessions/{id}/start-trading` — only from `phase=ready`
- `POST /ai-trader/sessions/{id}/stop`
- `GET /advisor/sessions/{id}/readiness` — `trading_ready`, playbook draft

Env:

| Variable | Default | Purpose |
|----------|---------|---------|
| `AI_TRADER_COLLECT_MIN_SEC` | `60` | Minimum observation before micro-LLM |
| `AI_TRADER_TRADING_MIN_REPORT_TF` | `15m` | Require advisor report before `trading_ready` |
| `AI_TRADER_LLM_INTERVAL_SEC` | `90` | Micro-LLM cadence in `analyzing`/`trading` |

 Decision brain:

 - 60s+ ring buffers: order book snapshots, prints, minute aggregates;
 - microstructure features from order book + tape;
 - hard safety gates (stale feed, wide spread) stay rule-based;
 - micro-LLM blocked during `collecting`; OpenRouter (`OPENROUTER_API_KEY`), default ~90s interval;
 - daily S/R from LB logic + intraday walls/POC → `level_playbook`;
 - advisor 15m rollup + synthesis → `trading_ready` gate;
 - paper limit engine in `trading` phase (`alert24_ai_trader_orders_total`).

 Safety properties:

 - `observe` / `paper` request modes are **deprecated** (400 with hint);
- `armed_live` is rejected server-side;
- a session is created from `account_id + instrument_uid + operator prompt`;
- `instance_id` is only a backward-compatible fallback for prefilling
  account/instrument and must not be treated as ownership by a standard
  strategy;
- no `PostOrder`, `CancelOrder`, `ReplaceOrder`, or stop-order call is reachable
  from the AI Trader code path;
- one running AI Trader session per account/instrument;
- session timeout and conservative default limits are applied;
- decisions are appended to JSONL audit at `AI_TRADER_JOURNAL_PATH`, default
  `data/ai_trader_journal.jsonl`.

Current market source:

- polls `marketdata.Service.GetOrderbook` every `observation_interval_ms`
  (default 2000 ms);
- computes best bid/ask, mid, spread, top-5 bid/ask volume, imbalance, depth
  skew, largest bid/ask wall, freshness/stale flag, and top-of-book snapshot.

Dashboard:

- `AI Trader` tab: **Start monitoring** / **Start trading** (gated);
- thought stream + advisor timeframe analysis tabs;
- live broker orders remain disabled (paper only in `trading` phase).

## advisor-svc (hierarchical analysis)

Separate service `cmd/advisor-svc` on port **9030** (`ADVISOR_PORT`), metrics **9130**.

| Component | Role |
|-----------|------|
| `strategy-runner` | Micro observe + thought stream; calls `POST /advisor/sessions/register` and `finalize` on stop |
| `advisor-svc` | Ingest runner session every ~12s, snapshot every ≥5m to SQLite (`ADVISOR_DB_PATH`), rollup agents |
| Dashboard | `GET /advisor/sessions/{id}/analyses?tf=5m` (poll 5s), strategy tab with drafts |

Timeframes (MSK calendar buckets): `5m` → `15m` → `30m` → `1h` → `4h` → `1d` → `strategy`.

Env (OpenRouter):

| Variable | Default | Purpose |
|----------|---------|---------|
| `ADVISOR_MODEL` | Nemotron free | Primary free model |
| `ADVISOR_MODEL_FALLBACKS` | Gemma free | Comma-separated free fallbacks |
| `ADVISOR_PAID_MODEL` | `google/gemini-2.5-flash` | Last resort after free tier fails |
| `ADVISOR_LLM_RETRIES` | `2` | Attempts per model (429/5xx/parse retry) |
| `ADVISOR_LLM_MAX_TOKENS` | `4096` | Cap for JSON reports |
| `ADVISOR_FACTS_FALLBACK` | `true` | Deterministic digest if all LLM fail |
| `OPENROUTER_API_KEY` | — | Required |
| `STRATEGY_RUNNER_URL`, `ADVISOR_URL` | — | Runner hook |

If `ADVISOR_MODEL` is unset and fallbacks empty, inherits `AI_TRADER_MODEL` / `AI_TRADER_MODEL_FALLBACKS`.

LLM requests use `response_format: json_object`. Scheduler advances `last_period_end` only after a successful report; failed periods retry every ~30s **only for advisor sessions with `status=running`**, with ≥60s backoff between retries (`internal/advisor/retry.go`). Paid model is never used until free models and retries are exhausted.

15m/1h structured reports may include `key_levels`, `regime`, `confidence`. Readiness endpoint merges synthesis + playbook for the runner poll (every ~30s in `analyzing`).

Prometheus: см. метрики LLM выше; `advisor_ingest_snapshots_total`. API отчётов advisor: поле `model` (какая модель или `facts-fallback`). AI Trader events: `llm_model`.

Nginx: `deployments/nginx-advisor-location.snippet` → `location /advisor` → `127.0.0.1:9030`.

Next steps before live:

1. Move audit from JSONL to queryable SQLite session/event tables.
2. Add trades and last-price consumption.
3. Add paper position/fill simulator.
4. Add Prometheus metrics for sessions, feed freshness, decisions, and blocked
   reasons.
5. Implement deterministic OrderControl and live RiskGate before enabling
   `armed_live`.
