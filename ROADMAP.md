# Roadmap 24alert

> Last updated: 2026-05-16  
> Canonical product/ops documentation: Obsidian `24alert/`. This file is a short repository roadmap only.

## Current Production State

- Gateway runs on `srv03-cloud` behind nginx `gateway.24alert.ru:8080`.
- Public gateway REST is intentionally closed except explicit routes; full REST is available on VPS loopback `127.0.0.1:18080`.
- `strategy-runner` is live on `127.0.0.1:9020`, dashboard is proxied by nginx at `/dashboard`.
- Trading mode is futures-only: `BMM6`, `NGM6`, `MCM6`.
- `ai-scanner` runs as a compose profile/container with cron-based futures scan/backtest.

## Completed

| Area | Status |
|------|--------|
| Gateway REST/WS facade | Done |
| Docker Compose production deployment | Done |
| OrderBook WebSocket stream for Traderbook | Done |
| Strategy runner with dashboard and journal | Done |
| Futures-only production strategy config | Done |
| AI Scanner futures-only scan/backtest flow | Done |
| Port hardening for service ports | Done |

## Active / Near-term

| Priority | Work | Why |
|----------|------|-----|
| Critical | Futures margin and exact PnL accounting | Current futures PnL/GO are approximate until `GetFuturesMargin` and price-step value are integrated. |
| High | Strategy auto-rollover for futures contracts | Current futures UID changes on expiration. |
| High | Clean secret-bearing historical task material | `.tasks/TASK-003/devops/DEPLOYMENT.md` needs separate review/rotation/history decision. |
| High | Monitoring decision | Either enable local `monitoring` profile or wire `strategy-runner:9120` into shared Traderbook monitoring. |
| Medium | Backtest engine hardening | Current scanner backtests are useful for ranking, but need production-grade futures accounting. |
| Medium | Dashboard auth/access policy | Nginx exposes dashboard/API routes; decide if additional auth/ACL is required. |

## Later

| Work | Notes |
|------|-------|
| PostgreSQL or durable event store | SQLite journal is enough for current runner, but broader history/reporting needs a DB decision. |
| Multi-account strategy management | Current production strategy account is `2001673385`. |
| External gRPC strategy plugins | Contract exists; not part of current production flow. |
| Kubernetes migration | Not needed for current single-VPS operation; revisit only after load/availability requirements change. |

## Archive

Older phase/task roadmaps were moved to `.archive/` or are preserved in historical `.tasks` handoffs. Use `BACKLOG.md` for current task tracking.
