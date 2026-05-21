# 24alert AI Integration

Canonical project knowledge lives in Obsidian: `24alert/AI Integration.md`.
This repo document is the developer-facing reference for code paths, env vars,
runtime mounts, and operational boundaries.

## Components

24alert currently has two AI surfaces:

| Component | Runtime | Model/API | Purpose | Writes config |
|-----------|---------|-----------|---------|---------------|
| Dashboard AI Chat | `strategy-runner` + React dashboard | OpenRouter, `AI_CHAT_MODEL` | Operator Q&A over live strategy state | No |
| **Technical assistant** | `strategy-runner` + dashboard tab «Ассистент» | OpenRouter, `ASSISTANT_MODEL` | Multi-TF level/mirror analysis, scenarios (no trading) | No |
| Autonomous AI Scanner | Docker service `24alert-ai-scanner` | Cursor Agent CLI, `AI_SCANNER_MODEL` | Cron scan/backtest/auto-config workflow | Only `auto-fut-*` |
| AI Trader / Scalper | archived (`AI_TRADER_ARCHIVED`) | advisor-svc + runner | Was live scalper; see `docs/archive/ai-trader/` | No |

## Technical assistant

See [TECHNICAL_ASSISTANT.md](TECHNICAL_ASSISTANT.md). Endpoints: `POST/GET /assistant/analyses`, chart `GET .../chart?tf=1h`.

## Dashboard AI Chat

Files:

- Backend: `internal/strategy/aichat.go`
- UI: `web/strategy-dashboard/src/components/AiChatPanel.tsx`
- API client: `web/strategy-dashboard/src/api/client.ts`

Endpoints:

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/ai-chat` | `POST` | Ask the assistant |
| `/ai-chat/reset` | `POST` | Clear in-memory chat history |
| `/ai-chat/status` | `GET` | Check availability/model/scanner key status |

Environment:

| Variable | Default | Purpose |
|----------|---------|---------|
| `OPENROUTER_API_KEY` | none | Enables dashboard chat |
| `AI_CHAT_MODEL` | `anthropic/claude-sonnet-4` | OpenRouter model |
| `CURSOR_API_KEY` | none | Only reflected as scanner status in chat UI |

The chat context is built by `Runner.buildFullContext()` and includes:

- broker portfolio and positions;
- broker-aware PnL;
- strategy configs and running status;
- runner ledger;
- current prices;
- indicator summaries;
- recent signals, executions, and timeline events;
- daily journal summary;
- FORTS session guard facts.

Important limits:

- It is read-only.
- It does not edit `config.yaml`.
- It does not run backtests.
- It does not read raw Docker logs directly.
- It does not read Obsidian or scanner reports.
- Chat history is in memory and is lost on `strategy-runner` restart.

## Autonomous AI Scanner

Files:

- Service definition: `deployments/docker-compose.yaml`
- Entrypoint runner: `deployments/ai-scanner/run-scan.sh`
- Main prompt: `deployments/ai-scanner/workspace/system-prompt.md`
- Reference: `deployments/ai-scanner/workspace/reference/ai-scanner-reference.md`
- Log-reading skill: `deployments/ai-scanner/workspace/reference/log-reading-skill.md`
- Memory seed: `deployments/ai-scanner/workspace/reference/agent-memory.seed.md`
- Metrics endpoint: `deployments/ai-scanner/metrics-server.py`

The service is under the Compose profile `ai-scanner` and runs as
`24alert-ai-scanner`.

Environment:

| Variable | Default | Purpose |
|----------|---------|---------|
| `CURSOR_API_KEY` | none | Required for Cursor Agent CLI |
| `AI_SCANNER_MODEL` | `composer-2` | Agent model |
| `AI_SCANNER_MAX_CONTRACT_PRICE` | `10000` | Futures contract price limit |
| `SCAN_POSTMARKET_CRON` | `0 2 * * 2-6` | Post-market scan |
| `SCAN_PREMARKET_CRON` | `50 9 * * 1-5` | Pre-market check |
| `SCAN_HEALTH_CRON` | `0 */4 * * *` | Health check |
| `STRATEGY_RUNNER_URL` | `http://strategy-runner:9020` | Runner API |
| `GATEWAY_URL` | `http://gateway:8080` | Gateway API |

Mounted data:

| Container path | Purpose |
|----------------|---------|
| `/workspace/system-prompt.md` | Main operating prompt |
| `/workspace/reference` | Read-only reference materials |
| `/workspace/memory` | Persistent scanner memory |
| `/workspace/reports` | Persistent scan reports |
| `/workspace/metrics` | Prometheus textfile metrics |
| `/opt/ai-scanner` | `scan_market.py`, `backtest.py` |
| `/usr/local/bin/strategy_dashboard_smoke.py` | Smoke-check tool |
| `/app/config/config.yaml` | Shared strategy config |

Jobs:

| Kind | Schedule | Responsibility |
|------|----------|----------------|
| `post-market` | Tue-Sat 02:00 MSK | Full scan/backtest and auto-config decisions |
| `pre-market` | Mon-Fri 09:50 MSK | Readiness checks before market |
| `health` | Every 4 hours | Health/dashboard/events check |

The agent command:

```bash
timeout 1800 agent -p --force --approve-mcps --trust \
  --workspace "$WS" \
  --model "$AI_SCANNER_MODEL" \
  --output-format text \
  "$PROMPT"
```

## Permissions and Safety Rules

The scanner may:

- scan only MOEX FORTS futures;
- run guarded backtests;
- add/remove/replace `auto-fut-*` instances;
- call `POST /config/reload`;
- run `strategy_dashboard_smoke.py`;
- write `/workspace/reports` and `/workspace/memory`.

The scanner must not:

- change manual instances without explicit operator instruction;
- add shares, ETFs, currencies, bonds, or options;
- trade weekends;
- treat scanner score as a final decision;
- report config success without a green smoke-check.

Manual instances currently have IDs without `auto-`, for example:

- `fut-brent-mini-lb`
- `fut-gas-mini-sma`
- `fut-mechel-lb`

## Monitoring

The AI scanner exposes metrics via `deployments/ai-scanner/metrics-server.py`
on `ai-scanner:9130/metrics`.

Prometheus scrape config is in `config/prometheus-agent.yaml` under
`job_name: "24alert-ai-scanner"`.

Key metrics:

- `alert24_ai_scanner_last_start_timestamp`
- `alert24_ai_scanner_last_success_timestamp`
- `alert24_ai_scanner_last_failure_timestamp`
- `alert24_ai_scanner_run_duration_seconds`
- `alert24_ai_scanner_last_exit_code`
- `alert24_ai_scanner_reports_total`
- `alert24_ai_scanner_run_in_progress`
- `alert24_ai_scanner_runs_total`

## Knowledge Storage

Use these layers for durable AI knowledge:

| Layer | Path | Purpose |
|-------|------|---------|
| Scanner memory | `/workspace/memory/agent-memory.md` | Short operational lessons |
| Scanner reports | `/workspace/reports/` | Run reports |
| Obsidian indicators | `24alert/Indicators/` | Indicator knowledge |
| Obsidian research | `24alert/Research/` | Strategy research records |
| Memory MCP | project memory graph | Cross-chat durable facts |

Rule: strategy research, indicator explanations, and grid/backtest conclusions
must be written to Obsidian. `.tmp` files and scanner reports are working
artifacts, not the canonical knowledge base.

## Future Role Split

Recommended next architecture is to split the single scanner prompt into roles:

| Role | Responsibility | Can edit config |
|------|----------------|-----------------|
| Market Researcher | Ideas, instruments, indicators, Obsidian research | No |
| Backtest Optimizer | Parameter grids and rankings | No |
| Risk Reviewer | Drawdown/trade-count/overfit review | No |
| Strategy Operator | Approved `auto-fut-*`, reload, smoke-check | Yes |
| Incident Analyst | Signals, cancellations, logs, explanations | No |

The safe rule is: the agent that researches an idea should not enable it in
real-money config without a review/approval stage.

## Planned AI Trader / Scalper

Detailed design: [`docs/AI_TRADER_SCALPER.md`](AI_TRADER_SCALPER.md).

The AI Trader is a planned third AI surface, separate from the read-only chat
and the cron scanner. The target workflow is:

1. Operator selects a futures ticker/UID in the dashboard.
2. Operator writes a short instruction, for example "trade from orderbook
   density, scale out in parts, watch adjacent instruments".
3. `advisor-svc` starts a session in `observe`, `paper`, or `armed_live`.
4. The service consumes order book, public prints, last price, runner state,
   broker position, technical levels, and correlated market context.
5. Live actions go only through deterministic `RiskGate` and `OrderControl`.

Hard boundaries:

- no direct broker API access from the LLM/agent;
- no live session without explicit UI enable for one selected instrument;
- no live action on stale orderbook/prints/broker state;
- no order outside FORTS session guard or trading status;
- all decisions, rejected actions, orders, fills, and outcomes are journaled.

## Operations

Start scanner:

```bash
cd /opt/24alert/deployments
docker compose --profile ai-scanner up -d ai-scanner
```

Read logs:

```bash
docker logs -f 24alert-ai-scanner
```

Run manually:

```bash
docker exec 24alert-ai-scanner run-scan.sh health
docker exec 24alert-ai-scanner run-scan.sh pre-market
docker exec 24alert-ai-scanner run-scan.sh post-market
```

Inspect runtime files:

```bash
docker exec 24alert-ai-scanner sed -n '1,200p' /workspace/memory/agent-memory.md
docker exec 24alert-ai-scanner ls -la /workspace/reports
```
