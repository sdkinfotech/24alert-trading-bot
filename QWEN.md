# QWEN.md — 24Alert Trading Bot

> **Purpose**: Instructional context for the Qwen Code agent. This document describes the project's architecture, conventions, and operational details so the agent can make informed, idiomatic contributions.

---

## 1. Project Overview

**24Alert Trading Bot** is a modular Go microservices trading system built on the **T-Invest (Tinkoff Investments) API**. It supports order placement, market data streaming, portfolio management, and risk controls — all orchestrated through a gateway exposing REST + Swagger and a Cobra CLI.

### Core Technologies

| Category       | Technology                                                                 |
|----------------|---------------------------------------------------------------------------|
| Language       | Go 1.25                                                                   |
| API Framework  | go-chi/chi v5 (REST), swaggo (Swagger)                                   |
| RPC            | gRPC + Protocol Buffers (inter-service communication)                    |
| CLI            | spf13/cobra                                                               |
| Config         | spf13/viper (YAML + env vars)                                            |
| Logging        | slog (structured JSON)                                                   |
| Metrics        | Prometheus client (prometheus/client_golang)                              |
| T-Invest SDK   | russianinvestments/invest-api-go-sdk v1.40.1                              |
| Container      | Docker Compose (multi-service, with optional monitoring profile)         |
| Linting        | golangci-lint                                                             |

### Architecture

```
gateway (:8080)  ── REST/Swagger + Cobra CLI
    │
    ├── order-svc (:9001)       → T-Invest Orders API (gRPC)
    ├── marketdata-svc (:9002)  → T-Invest MarketData API + streaming
    ├── portfolio-svc (:9003)   → T-Invest Operations/Users API
    └── risk-svc (:9004)        → portfolio-svc + marketdata-svc

strategy-plugin (:9010) ─── external gRPC server (contract defined, not yet implemented)
```

### Directory Layout

```
.
├── cmd/                       # Service entrypoints (main packages)
│   ├── gateway/               # HTTP gateway + CLI
│   ├── order-svc/
│   ├── marketdata-svc/
│   ├── portfolio-svc/
│   ├── risk-svc/
│   └── trade/
├── internal/                  # Business logic
│   ├── gateway/               # REST handlers + adapter layer
│   ├── order/                 # Order CRUD, streaming, repository
│   ├── marketdata/            # Candles, orderbook, prices, stream manager
│   ├── portfolio/
│   └── risk/                  # Circuit breaker, checkers, validation
├── pkg/                       # Shared, reusable packages
│   ├── config/                # Viper-based config loader
│   ├── tinvest/               # T-Invest SDK client wrapper + rate limiter
│   ├── types/
│   ├── logging/               # Structured slog logger factory
│   ├── metrics/               # Prometheus metric definitions
│   └── idempotency/
├── proto/                     # .proto files (service contracts)
├── config/                    # YAML config files
├── deployments/               # Dockerfile, docker-compose, .env.example
├── docs/                      # Swagger source + API reference
├── monitoring/                # Grafana dashboards + alerting rules
├── scripts/                   # CI/CD and deployment scripts
├── tests/                     # E2E test suites
├── Makefile
├── go.mod / go.sum
├── BACKLOG.md
└── README.md
```

---

## 2. Building and Running

### Prerequisites

- Go 1.25+
- Docker 29+ and Docker Compose (for containerized run)
- T-Invest API token (sandbox and/or production)
- protoc compiler (optional, only for make proto-gen)

### Makefile Targets

| Target             | Description                                    |
|--------------------|------------------------------------------------|
| make install-tools | Installs protoc plugins, golangci-lint, goimports |
| make build         | Builds all 5 service binaries into ./bin/      |
| make build-all     | Runs proto-gen then build                      |
| make proto-gen     | Generates Go code from .proto files into gen/  |
| make test          | Runs all unit tests with coverage              |
| make lint          | Runs golangci-lint                              |
| make fmt           | Formats code (go fmt + goimports)              |
| make ci-check      | Lint + test + build (local CI gate)            |
| make docker-up     | Starts full stack via Docker Compose           |
| make docker-down   | Stops and removes containers                   |
| make docker-logs   | Follows container logs                         |
| make clean         | Removes bin/, gen/, vendor/, .pb.go files      |

### Docker Compose (recommended)

```bash
cp deployments/.env.example deployments/.env
# Edit deployments/.env: set TINVEST_SANDBOX, TINVEST_SANDBOX_TOKEN, TINVEST_PROD_TOKEN
make docker-up
make docker-logs
make docker-down
```

Monitoring (Prometheus + Promtail) is opt-in:
```bash
docker compose --profile monitoring up -d
```

### Local Development (no Docker)

Build all services with `make build`, then run each in a separate terminal:

```bash
./bin/order-svc -config config/config.yaml
./bin/marketdata-svc -config config/config.yaml
./bin/portfolio-svc -config config/config.yaml
./bin/risk-svc -config config/config.yaml
./bin/gateway serve -config config/config.yaml
```

### Environment Variables

| Variable                  | Required | Description                                  |
|---------------------------|----------|----------------------------------------------|
| TINVEST_SANDBOX           | Yes      | "true" = sandbox, "false" = production       |
| TINVEST_SANDBOX_TOKEN     | Yes*     | Sandbox API token (t.xxx)                    |
| TINVEST_PROD_TOKEN        | Yes*     | Production API token (t.xxx)                 |
| TINVEST_TOKEN             | No       | Fallback token if above are empty            |
| TINVEST_ENDPOINT          | No       | Override API endpoint manually               |
| LOG_LEVEL                 | No       | debug / info / warn / error                  |

The active token is auto-selected based on TINVEST_SANDBOX. Secrets must never be committed.

---

## 3. Configuration

Primary config: `config/config.yaml`

Key sections:
- `tinvest` — API endpoint, retries, timeouts (endpoint auto-set from TINVEST_SANDBOX env var)
- `services` — port numbers for each microservice (order: 9001, marketdata: 9002, portfolio: 9003, risk: 9004, gateway: 8080)
- `risk` — max position lots, circuit breaker threshold/cooldown, margin call threshold
- `logging` — level, format (json/text), output (stdout/file/both)
- `metrics` — Prometheus toggle and endpoint
- `rate_limits` — per-method requests-per-minute limits
- `market_data_stream` — max subscriptions per stream, concurrent streams, reconnect settings
- `features` — feature flags (risk_validation, order_journal, md_cache)

Secrets are always loaded from environment variables, never from config files.

---

## 4. Key Packages and Conventions

### pkg/config — Configuration Loader
- Uses Viper to read config.yaml and bind environment variables.
- `config.Load(path)` returns `*Config`. Errors if required token is missing.
- `config.IsSandbox()` checks TINVEST_SANDBOX env var.
- `config.GetTInvestToken()` resolves the correct token based on sandbox mode.
- Env binding uses `24ALERT_` prefix via `v.SetEnvPrefix`.

### pkg/tinvest — T-Invest SDK Wrapper
- `Client` wraps `investgo.Client` and exposes typed sub-clients (Orders, MarketData, Portfolio, etc.).
- Factory: `tinvest.NewTInvestClient(ctx, endpoint, token, logger)`.
- Graceful shutdown via `Client.Stop()`.
- Endpoint auto-selected: sandbox vs production based on config.

### pkg/tinvest/rate_limiter.go — Rate Limiting
- `RateLimiterManager` provides per-method token bucket rate limiters.
- Configured from `rate_limits` map in YAML.
- Usage: `rlm.Wait(ctx, "post_order")` before each API call.
- Critical bottleneck: `post_order` (900 req/min), `get_history` (30 req/min).

### pkg/metrics — Prometheus Metrics
- All metrics use namespace `alert24`.
- Key families: http_*, trading_*, tinvest_*, marketdata_*, risk_*.
- Auto-registered via promauto. Exposed at gateway :8080/metrics and per-service :91xx/metrics.

### pkg/logging — Structured Logging
- Factory: `logging.NewLogger(level, format, output, filePath)`.
- Uses Go stdlib slog with JSON encoding.
- Correlation ID middleware in gateway adds request tracing.

### internal/gateway — HTTP Gateway
- `server.go` — sets up chi router, middleware, health/metrics/swagger endpoints, and wires all handler routes.
- `adapter/` — wraps internal services to satisfy handler interfaces. This decoupling must be preserved.
- `cli/` — Cobra command definitions for the CLI.
- `handlers/` — HTTP handler implementations for each domain (orders, marketdata, portfolio, risk, etc.).

### internal/risk — Risk Management
- `CircuitBreaker` — trips after N consecutive failures, enforces cooldown.
- `checker/` — SessionChecker, BalanceChecker, PositionLimitChecker.
- `Service.ValidateOrderIntent()` runs all checks, updates circuit breaker, returns aggregated RiskResponse.

### internal/marketdata — Market Data
- `Service` — wraps T-Invest MarketData gRPC client with rate limiting and caching.
- `StreamManager` — maintains multiplexed gRPC streams (up to 300 subscriptions per stream, 4 concurrent streams).
- Caches: InstrumentCache, PriceCache.

### internal/order — Order Management
- `Repository` — in-memory order store with thread-safe operations.
- `Service` — wraps T-Invest Orders/StopOrders gRPC clients.
- `Stream` — subscribes to order execution updates via T-Invest streaming API.

### proto/ — Protocol Buffers
- Service contracts in `proto/<service>/v1/*.proto`.
- Generated Go code goes in `gen/go/` (generated by `make proto-gen`).
- Any .proto change requires regeneration.

---

## 5. Gateway REST API

Base URL: `http://localhost:8080/api/v1/`

| Method   | Endpoint                      | Description                         |
|----------|-------------------------------|-------------------------------------|
| POST     | /orders                       | Place exchange order                |
| GET      | /orders                       | List active orders                  |
| GET      | /orders/:id                   | Get order state                     |
| DELETE   | /orders/:id                   | Cancel order                        |
| PUT      | /orders/:id                   | Replace order                       |
| POST     | /stop-orders                  | Place stop order                    |
| GET      | /stop-orders                  | List active stop orders             |
| DELETE   | /stop-orders/:id              | Cancel stop order                   |
| GET      | /candles                      | Historic candlesticks               |
| GET      | /orderbook/:uid               | Order book snapshot                 |
| GET      | /prices                       | Last prices                         |
| GET      | /trading-status/:uid          | Trading status for instrument       |
| GET      | /positions                    | Current positions                   |
| GET      | /portfolio                    | Full portfolio                      |
| GET      | /limits                       | Withdraw limits                     |
| GET      | /operations                   | Operation history                   |
| GET      | /accounts                     | Brokerage accounts                  |
| GET      | /margin/:account_id           | Margin attributes                   |
| GET      | /risk/status                  | Risk/circuit breaker status         |
| POST     | /risk/reset                   | Reset circuit breaker               |
| GET      | /swagger/*                    | Swagger UI                          |
| GET      | /health                       | Health check                        |

---

## 6. CLI Usage (Cobra)

```bash
./bin/gateway order post --instrument <uid> --qty 10 --type limit --price 100 --direction buy
./bin/gateway order list
./bin/gateway order state --order-id <id>
./bin/gateway order cancel --order-id <id>
./bin/gateway market candles --instrument <uid> --interval 1h
./bin/gateway market book --instrument <uid> --depth 20
./bin/gateway portfolio positions
./bin/gateway portfolio limits
./bin/gateway risk status
./bin/gateway risk reset
```

---

## 7. Development Conventions

- **Язык общения**: русский. Все комментарии, сообщения и взаимодействие — на русском языке.
- Idiomatic Go (stdlib patterns, slog для логирования, context propagation).
- go fmt + goimports before committing.
- golangci-lint must pass.
- All new code must have unit tests.
- Every T-Invest API call goes through the rate limiter — never bypass it.
- Secrets via environment variables only — never hardcoded.
- Adapter pattern in internal/gateway/adapter/ — do not break this decoupling.
- Feature flags use the FeaturesConfig struct for toggleable functionality.

---

## 8. Project Status

Phase 2 (Stabilization) in progress. Phase 1 MVP is complete. See BACKLOG.md for the full roadmap with priorities, complexity estimates, and dependencies.

---

## 9. Available Skills (.qwen/skills/)

Проект использует систему скиллов Qwen, расположенную в `.qwen/skills/`.

### Obsidian Vault (obsidian-vault)

**Расположение скилла**: `.qwen/skills/obsidian-vault/obsidian_skill.md`
**Расположение vault**: `C:\vault\obsidian\devops\24alert\`

Этот скилл содержит:
- Полную структуру vault с описанием всех разделов
- Правила создания и именования заметок (frontmatter, теги, статусы)
- Формат ежедневников и handoff-документов
- Правила обновления MOC (Map of Content)
- Кросс-ссылки между заметками
- Интеграцию с репозиторием 24alert

**Заметки в vault**:
- `24alert/MOC.md` — карта проекта (деплой, операции, мониторинг, эндпоинты)
- `24alert/Knowledge/Knowledge MOC.md` — карта знаний T-Invest API, архитектура, производительность
- `24alert/Knowledge/T-Invest-API/T-Invest API MOC.md` — полная документация по T-Invest API (20+ сервисов, SDK, лимиты, ошибки)
- `24alert/Deployment.md` — деплой на srv03-cloud (nginx, TLS, certbot, systemd)
- `24alert/Operations.md` — рутинные операции, smoke-тесты, логи
- `24alert/Grafana.md` — мониторинг, метрики, алерты, дашборды
- `24alert/Troubleshooting.md` — типовые проблемы и их решения
- `24alert/Tokens.md` — управление токенами T-Invest
- `24alert/OrderBook Stream.md` — WebSocket стрим стакана (TASK-019)

---

## 10. Agent Roles (.cursor/rules/)

В проекте определены 9 ролей агентов в `.cursor/rules/*.mdc`. Каждая роль — контекстный промпт для выполнения задач определённого типа.

| Роль | Файл | Когда использовать |
|------|------|-------------------|
| **Backend Senior** | `role-backend-senior.mdc` | API, данные, производительность, безопасность |
| **Frontend Senior** | `role-frontend-senior.mdc` | UI/UX, доступность, производительность FE |
| **DevOps Senior** | `role-devops.mdc` | Инфра, CI/CD, деплой, мониторинг |
| **Tester (QA)** | `role-tester.mdc` | Стратегия тестирования, регресс, автоматизация |
| **Planner** | `role-planner.mdc` | Декомпозиция, бэклог, velocity |
| **Analyst** | `role-analyst.mdc` | Исследование, Grafana, Wiki, Memory graph, MCP |
| **Tech Lead** | `role-tech-lead.mdc` | Архитектура, качество, решения |
| **Workflow** | `workflow.mdc` | Конвейер ролей — протокол задач и handoff'ов |
| **Planner Backlog Guide** | `planner-backlog-guide.mdc` | Инструкция по управлению бэклогом |

### Ключевые роли в контексте 24alert

**Role: Аналитик** (`role-analyst.mdc`) предполагает подключение внешних MCP-инструментов:
- `user-grafana` — доступ к Grafana (дашборды, PromQL, метрики)
- `user-memory` — граф знаний (сущности: Service, Server, Metric, Risk, Decision + отношения)
- Obsidian Vault — долгосрочное хранилище знаний в `C:\vault\obsidian\devops\traderbook`

### Конвейер ролей

```
Пользователь → Планировщик → Бэкенд → Фронтенд → DevOps → Тестировщик → Техлид
                                                                  ↑
Аналитик ─── (параллельно, после первого handoff) ────────────────┘
```

### Tiering задач

| Тир | Критерий | Роль |
|-----|----------|------|
| **S** | 1–3 файла, 1 зона | `fullstack-agent`, только коммит |
| **M** | 2–8 файлов, FE+BE | `fullstack-agent`, 1 handoff |
| **L** | Миграция, новый сервис | Специализированные роли |
| **XL** | Новая архитектура, security | ADR + все роли |

---

## Project Summary

This is a Go 1.25 microservices trading system (T-Invest API) with:
- 5 services (gateway, order, marketdata, portfolio, risk) communicating via gRPC
- REST API + Swagger on gateway (`:8080`)
- Cobra CLI for command-line access
- Prometheus metrics on each service
- Docker Compose deployment with TLS (Let's Encrypt DNS-01)
- Live WebSocket streaming for market data (orderbook, candles)
- In-memory storage (PostgreSQL planned in TASK-014)
- Russian language for all interactions

All interaction happens in Russian. Vault, documentation, and comments are in Russian where project convention dictates.