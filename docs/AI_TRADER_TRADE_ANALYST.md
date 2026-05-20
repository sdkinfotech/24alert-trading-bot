# AI Trader Trade Analyst

Post-market analyst for `level_intraday` / `armed_live` sessions: trade journal per instrument, LLM decision correlation, frequency and SL/TP review, hourly volatility, hints for the next session.

## Storage

| File | Purpose |
|------|---------|
| `data/ai_trader_trade_analyst.db` | SQLite: `trade_rounds`, `session_reports`, `instrument_stats`, `trading_hints` |
| `data/ai_trader_journal.jsonl` | Input: all AI Trader decision events (read-only) |

Env overrides: `AI_TRADER_ANALYST_DB`, `AI_TRADER_JOURNAL_PATH`.

## Live execution log (UI)

While a session is in `trading`, each **entry** and **exit** (live or paper fill, SL/TP, flatten) appends to `session.execution_log` with:

- trigger (`confluence`, `llm_signal`, `stop_loss`, `take_profit`, …)
- nearest **LLM** summary / reason / intent / bias from session events
- `trade_signal` reason and active policy summary
- SL/TP at the moment of the fill

Dashboard block: **«Входы и выходы (с обоснованием LLM)»** above the trading desk. Same events are written to `ai_trader_journal.jsonl` as `trade_entry` / `trade_exit`.

## When analysis runs

1. **Automatically** when a session stops (`POST .../stop`) or the runner goroutine ends (`finishAITraderSession`).
2. **Manual** `POST /ai-trader/analyst/sessions/{id}/postmarket`
3. **CLI** on runner host:

```bash
go run ./cmd/ai-trader-postmarket -session ai-trader-bmm6-20260520-115123
```

## API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/ai-trader/analyst/sessions/{id}/report` | Last saved post-market report |
| `POST` | `/ai-trader/analyst/sessions/{id}/postmarket` | Run analysis now |
| `GET` | `/ai-trader/analyst/instruments/{ticker}/journal` | Reports, rounds, stats, hints for ticker |

Example journal response keys: `reports`, `trade_rounds`, `stats`, `hints`.

## Report contents

- **trade_rounds** — entry/exit, hold time, outcome, SL/TP tags
- **decision_links** — nearest journal event (LLM) to each fill
- **hourly_volatility** — range/ATR by UTC hour from session chart bars
- **frequency** — `ok` / `too_often` / `too_rare`
- **targets** — TP/SL distance vs ATR, hit rates
- **strategy_fit** — alignment with level-intraday method
- **recommendations** + **summary_ru**

## Feed-forward to next session

`trading_hints` per ticker adjusts:

- `allow_new_entry` (block if too many attempts)
- `entry_min_confidence` floor
- `tp_mult_scale` / `sl_mult_scale`
- `avoid_hours_utc` — no new entries that hour

Applied in `effectivePolicy()` and `allowNewEntry()`.

## Obsidian / analyst role

Human-readable long-form notes: copy `summary_ru` + `recommendations` into  
`traderbook/Knowledge/AI Trader/{TICKER}.md` (Obsidian vault) after major sessions.

Parallel **role-analyst** workflow: `.tasks/TASK-029/analyst/prompt.md` for deep dives (Grafana, Memory MCP).
