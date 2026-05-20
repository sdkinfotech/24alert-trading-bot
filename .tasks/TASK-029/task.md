# TASK-029: AI Trader Trade Analyst (постмаркет)

## Цель

Второй аналитик для AI Trader: оценка сделок, журнал по инструментам, сопоставление LLM-отбивок с исполнением, постмаркет и подсказки на следующие сессии.

## Критерии готовности

- [x] SQLite `data/ai_trader_trade_analyst.db` — сделки, отчёты сессий, stats/hints по тикеру
- [x] Постмаркет при stop/finish сессии (async)
- [x] API: report, journal по тикеру, POST postmarket
- [x] Hints влияют на `effectivePolicy` (частота, SL/TP mult, block hour)
- [x] CLI `ai-trader-postmarket -session <id>`
- [x] UI: блок «Входы и выходы (LLM)» + постмаркет-аналитик
- [ ] Cron post-market 02:00 MSK (опционально, через compose)

## Источники

- `ai_trader_journal.jsonl` — LLM/runner события
- `live_state.fills` / paper fills — сделки
- `market_context.chart_bars` — волатильность по часам
