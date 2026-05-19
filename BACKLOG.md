# Product Backlog — 24alert

**Last Updated**: 2026-05-19  
**Status**: Phase 2 — production stabilization for futures-only trading

## Current Production Baseline

- Server: `srv03-cloud` / `176.123.160.234`.
- Compose project: `24alert`.
- Services: `gateway`, `order-svc`, `marketdata-svc`, `portfolio-svc`, `risk-svc`, `redis`, `strategy-runner`, `ai-scanner`.
- Trading: MOEX FORTS futures only (`BMM6`, `NGM6`, `MCM6`).
- Canonical docs: Obsidian `24alert/`; repo docs are concise technical references.

## Active / Planned Work

| ID | Title | Priority | Complexity | Status | Notes |
|----|-------|----------|------------|--------|-------|
| TASK-027 | Production trading safety remediation | Critical | M | In Progress | Fail-closed live trading, mandatory protective exits, policy/docs after real-money incident. |
| TASK-020 | Futures margin and exact PnL accounting | Critical | M | Planned | Integrate `GetFuturesMargin`, price-step value, commission-aware reporting. |
| TASK-021 | Futures contract rollover | High | M | Planned | Resolve/replace expiring futures UID for manual and auto instances. |
| TASK-022 | Strategy monitoring rollout | High | S | Planned | Decide shared Traderbook Prometheus vs local `monitoring` profile; scrape `strategy-runner:9120`. |
| TASK-023 | Dashboard access hardening | High | S | Planned | Public nginx routes expose dashboard/API; decide auth/ACL. |
| TASK-024 | Secret-bearing task archive cleanup | Critical | S | Blocked | `.tasks/TASK-003/devops/DEPLOYMENT.md` may contain prod token; requires rotation/history decision. |
| TASK-025 | AI Scanner production safeguards | Medium | M | Planned | Paper-mode/confirmation gate before auto strategy changes. |
| TASK-026 | Durable event storage decision | Medium | L | Backlog | SQLite journal is current; DB may be needed for reporting/history. |

## Recently Completed

| ID | Title | Status | Notes |
|----|-------|--------|-------|
| TASK-028 | AI Trader level intraday | Done | Phased collect→analyze→ready→trading; BMM6 preset; paper limits; advisor readiness. |
| TASK-019 | OrderBook WebSocket Stream | Done | Public WSS stream for Traderbook, nginx TLS/ACL, docs. |
| STRAT-001 | Strategy runner productionization | Done | Dashboard, journal, ledger, watchdog, futures-only config. |
| STRAT-002 | AI Scanner futures-only selection | Done | Futures endpoint, contract price filter, backtest-first decision. |
| OPS-001 | Production documentation refresh | Done | Obsidian runbooks updated to real prod state. |
| OPS-002 | Repository cleanup pass | In Progress | Archive old tasks/docs, remove cache/temp artifacts. |

## Historical Archive

- `.archive/tasks/` contains old completed task handoffs.
- `.tasks/TASK-003/` remains active only for a separate secret review.
- `.tasks/TASK-019/` remains because it is the current valuable handoff history.

## Planner Rules

1. Create new work as `TASK-NNN` only when it has a clear owner and definition of done.
2. Keep product/ops canon in Obsidian; keep repo docs short and code-adjacent.
3. Do not store tokens, credentials, or production secrets in `.tasks`, docs, or examples.
4. Before moving work to Done, update this backlog and the relevant Obsidian note.
