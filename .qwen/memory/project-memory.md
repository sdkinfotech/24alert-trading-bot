# Project Memory — 24alert Trading Bot

> Last updated: 2026-04-17

## Project Overview
- **Name**: 24alert Trading Bot
- **Repo**: `c:\Users\sdk\24alert-trading-bot`
- **Language**: Go 1.25
- **Stack**: go-chi, gRPC, Protocol Buffers, spf13/cobra, viper, slog
- **SDK**: russianinvestments/invest-api-go-sdk v1.40.1
- **Purpose**: Trading facade over T-Invest (Tinkoff) API

## Infrastructure
- **Production server**: srv03-cloud, 176.123.160.234 (Timeweb Cloud)
- **OS**: Ubuntu 22.04.5 LTS, 2 vCPU, 3.8 GB RAM, 30 GB SSD
- **Domain**: gateway.24alert.ru:8080 (TLS, Let's Encrypt DNS-01)
- **Deploy**: Docker Compose, project name `24alert`, manual `git pull` + `docker compose up`
- **Reverse proxy**: nginx on same server
- **Server user**: `adm-srv03-cloud` (sudo access)
- **Repo on server**: `/opt/24alert`
- **SSH key**: `~/.ssh/id_ed25519`

## Running Services (all healthy)
1. 24alert-gateway (healthy, 3 weeks) — :8080, bind 127.0.0.1:18080
2. 24alert-order-svc (healthy, 5 weeks) — :9001, bind 0.0.0.0 ⚠️
3. 24alert-marketdata-svc (healthy, 5 weeks) — :9002, bind 0.0.0.0 ⚠️
4. 24alert-portfolio-svc (healthy, 5 weeks) — :9003, bind 0.0.0.0 ⚠️
5. 24alert-risk-svc (healthy, 5 weeks) — :9004, bind 0.0.0.0 ⚠️
6. 24alert-prometheus-agent — :9090, bind 0.0.0.0 ⚠️
7. 24alert-promtail — logs collector

## Known Issues (prioritized)
1. 🔴 Microservice ports (9001-9004, 9090) exposed on 0.0.0.0 — insecure
2. 🔴 No REST API authentication — anyone can trade
3. 🟡 No database — everything in-memory
4. 🟡 RAM 3.8 GB — tight for scaling
5. 🟡 No swap — OOM risk
6. 🟡 No inter-service healthchecks
7. 🟠 Manual deploy — no CI/CD pipeline active

## Obsidian Vault Location
- Path: `C:\vault\obsidian\devops\24alert\`
- Structure: MOC.md, Deployment.md, Operations.md, Grafana.md, Troubleshooting.md, Tokens.md, OrderBook Stream.md, Knowledge/ (20+ T-Invest API notes)

## Task Management
- Task flow: .tasks/TASK-NNN/{task.md, plan.md, handoff.md, artifacts/, vault/}
- Roles: Planner → Backend → DevOps → Tester → TechLead
- Backlog: BACKLOG.md in repo root
- Current phase: Phase 2 (Stabilization & Hardening)

## API Endpoints (public)
- REST: https://gateway.24alert.ru:8080/api/v1/...
- WebSocket: wss://gateway.24alert.ru:8080/api/v1/stream/...
- Health: /health (no ACL)
- Metrics: /metrics (each service on :91xx)
- Grafana: https://grafana.traderbook.ru (shared with Traderbook)

## T-Invest API Key Facts
- Prod: invest-public-api.tbank.ru:443
- Sandbox: sandbox-invest-public-api.tbank.ru:443
- postOrder: 15/sec (900/min) — main bottleneck
- postOrderAsync: 600/min
- getHistory: 30/min — second bottleneck
- MarketDataStream: 300 subs, 32 streams max
- Auth: Bearer token from .env

## Working Agreements
- Language: Russian for all interactions
- Vault updates: every task creates/updates vault notes
- Handoff format: mandatory for all roles (status, artifacts, corrections)
- Security: no secrets in code/repo, env vars only