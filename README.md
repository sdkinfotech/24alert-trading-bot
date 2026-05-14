# 24Alert Trading Bot

A modular Go trading bot built on T-Invest (Tinkoff Investments) API with microservices architecture. Place orders, monitor market data, manage portfolio, and implement trading strategies.

## Architecture

### Runtime (Docker Compose)

- **gateway** listens on **`:8080` inside the container**; on the host it is published as **`127.0.0.1:18080`** so nginx on `:8080/tcp` can TLS-terminate and proxy selected paths (see [`docs/STREAM_ORDERBOOK.md`](docs/STREAM_ORDERBOOK.md)).
- **REST** is implemented **in-process** in the gateway: it wires `order`, `marketdata`, `portfolio`, and `risk` packages with one shared T-Invest client (`cmd/gateway/main.go`). HTTP handlers do **not** call the sibling `order-svc` / `marketdata-svc` / … containers over Docker DNS.

**Decision — keep the `*-svc` containers in compose:** they still provide separate **`/metrics` on `:910x`** scrape targets used by [`config/prometheus-agent.yaml`](config/prometheus-agent.yaml) (plus optional direct gRPC/CLI use). Trade-off: extra T-Invest sessions vs the gateway until we consolidate metrics or drop those jobs.

- **gRPC** contracts in `proto/`; standalone `*-svc` binaries speak gRPC on host ports `127.0.0.1:9001–9004` when published.
- **CLI** (cobra) for command-line workflows.
- **Protocol Buffers** for service contracts.

```
strategy-plugin (:9010) ─── external gRPC server (contract defined, not implemented)
```

## Quick Start

### Prerequisites

- Go 1.25+
- Docker 29+ и Docker Compose (для контейнерного запуска)
- T-Invest API токен (sandbox и/или production)
- protoc compiler (опционально, для proto generation)

### Setup

```bash
# Install development tools
make install-tools

# Build all services (protoc не нужен)
make build

# The binaries will be in ./bin/
```

### Environment Variables (токены)

Приложение использует **два раздельных токена** — для песочницы и боевого контура.
Переключение между режимами — через `TINVEST_SANDBOX`.

```bash
# 1. Скопировать шаблон
cp deployments/.env.example deployments/.env

# 2. Вписать реальные токены в deployments/.env
```

Файл `deployments/.env`:
```env
# Режим: true = sandbox, false = production
TINVEST_SANDBOX=true

# Токен песочницы
TINVEST_SANDBOX_TOKEN=t.YOUR_SANDBOX_TOKEN

# Токен боевого контура
TINVEST_PROD_TOKEN=t.YOUR_PROD_TOKEN

# Уровень логирования
LOG_LEVEL=info
```

Логика выбора токена (`pkg/config`):
- `TINVEST_SANDBOX=true` → берётся `TINVEST_SANDBOX_TOKEN`, endpoint автоматически `sandbox-invest-public-api.tbank.ru:443`
- `TINVEST_SANDBOX=false` → берётся `TINVEST_PROD_TOKEN`, endpoint `invest-public-api.tbank.ru:443`
- Для обратной совместимости: если `TINVEST_SANDBOX_TOKEN` / `TINVEST_PROD_TOKEN` пуст — fallback на `TINVEST_TOKEN`
- Endpoint можно переопределить вручную через `TINVEST_ENDPOINT`

> **SECURITY**: `deployments/.env` и любые `.env` файлы в `.gitignore` — **не коммитить**.

### Configuration

Copy `config/config.yaml` to your deployment location and configure:

```yaml
tinvest:
  endpoint: "sandbox-invest-public-api.tbank.ru:443"  # or production endpoint
  
services:
  order_port: 9001
  marketdata_port: 9002
  # ... etc

risk:
  max_position_lots: 10
  circuit_breaker_threshold: 5
  # ... etc
```

For local development without Docker, set the env vars directly:

```bash
export TINVEST_SANDBOX=true
export TINVEST_SANDBOX_TOKEN="t.your_sandbox_token"
export TINVEST_PROD_TOKEN="t.your_prod_token"
export LOG_LEVEL="info"
```

### Running

#### Docker Compose (рекомендуемый способ)

```bash
# Убедитесь что deployments/.env заполнен токенами

# Запуск в sandbox-режиме:
make docker-up          # или: docker compose -f deployments/docker-compose.yaml up -d

# Просмотр логов:
make docker-logs

# Остановка:
make docker-down
```

Все контейнеры получают `TINVEST_SANDBOX`, `TINVEST_SANDBOX_TOKEN`, `TINVEST_PROD_TOKEN` и `LOG_LEVEL` из `deployments/.env`.

С хоста после `make docker-up` gateway опубликован на **http://127.0.0.1:18080** (loopback → контейнер `:8080`). Другие контейнеры в той же сети ходят на `http://gateway:8080`.

#### Локально (без Docker)

```bash
# В отдельных терминалах:
./bin/order-svc -config config/config.yaml
./bin/marketdata-svc -config config/config.yaml
./bin/portfolio-svc -config config/config.yaml
./bin/risk-svc -config config/config.yaml
./bin/gateway -config config/config.yaml
```

#### Переключение sandbox ↔ production

В `deployments/.env` (или env vars):
```env
TINVEST_SANDBOX=false    # ← переключает на боевой контур и TINVEST_PROD_TOKEN
```

### API Access

- **Docker Compose (с хоста)**: `http://127.0.0.1:18080/api/v1/...`, Swagger `http://127.0.0.1:18080/swagger/`, health `http://127.0.0.1:18080/health`
- **Локально без Docker** (бинарники слушают `:8080`): `http://localhost:8080/...`

### CLI Usage

```bash
# Order management
./bin/gateway order post --instrument <uid> --qty 10 --type limit --price 100 --direction buy
./bin/gateway order list
./bin/gateway order state --order-id <id>
./bin/gateway order cancel --order-id <id>

# Market data
./bin/gateway market candles --instrument <uid> --interval 1h
./bin/gateway market book --instrument <uid> --depth 20

# Portfolio
./bin/gateway portfolio positions
./bin/gateway portfolio limits

# Risk management
./bin/gateway risk status
./bin/gateway risk reset
```

## Project Structure

```
.
├── config/                    # Configuration files
│   ├── config.yaml           # Default configuration
│   └── config.sandbox.yaml   # Sandbox overrides
├── proto/                     # Protocol buffer definitions
│   ├── common/v1/            # Common types (Quotation, Signal, etc)
│   ├── order/v1/             # Order service contract
│   ├── marketdata/v1/        # MarketData service contract
│   ├── portfolio/v1/         # Portfolio service contract
│   ├── risk/v1/              # Risk service contract
│   └── strategy/v1/          # Strategy plugin interface
├── pkg/                       # Shared packages
│   ├── config/               # Configuration loader (viper)
│   ├── tinvest/              # T-Invest SDK wrapper + rate limiter
│   ├── types/                # Type helpers (Quotation, Instrument)
│   ├── logging/              # Structured logging (slog)
│   └── idempotency/          # Order ID generation
├── cmd/                       # Service entrypoints
│   ├── order-svc/
│   ├── marketdata-svc/
│   ├── portfolio-svc/
│   ├── risk-svc/
│   └── gateway/
├── internal/                  # Service implementations
│   ├── order/                # Order service logic
│   ├── marketdata/           # MarketData service logic
│   ├── portfolio/            # Portfolio service logic
│   ├── risk/                 # Risk service logic
│   └── gateway/              # REST handlers + CLI
├── gen/                       # Generated protobuf code (after make proto-gen)
├── docs/                      # Generated Swagger
├── deployments/               # Docker, K8s, etc
├── Makefile                   # Build targets
├── go.mod                     # Go module definition
└── README.md                  # This file
```

## Core Concepts

### Rate Limiting

Each T-Invest API method has rate limits. The bot implements per-method token bucket rate limiters:

- **PostOrder (SYNC)**: 15 req/sec (900 req/min) — CRITICAL BOTTLENECK
- **PostOrder (ASYNC)**: 600 req/min
- **GetHistory**: 30 req/min — BOTTLENECK
- See `rate-limits.json` for full table

Configuration:

```yaml
rate_limits:
  post_order: 900
  get_history: 30
  # ... etc
```

### Risk Management

The risk service validates orders before execution:

1. **Session Check**: Is the trading session open?
2. **Balance Check**: Are there sufficient funds/margin?
3. **Position Limit Check**: Would this exceed max position size?
4. **Circuit Breaker**: Have there been too many failures?

Configuration:

```yaml
risk:
  max_position_lots: 10
  circuit_breaker_threshold: 5
  circuit_breaker_cooldown: "5m"
  margin_call_threshold_percent: 80
```

### Market Data Streaming

Use multiplexed streams instead of polling:

```
One MarketDataStream connection → up to 300 subscriptions
- Candles (per interval: 1m, 5m, 15m, 1h, 4h, 1d, 1w, 1M)
- Order book (configurable depth)
- Trades
- Last prices
- Trading status
```

### Sandbox vs Production

Switch via configuration:

```yaml
# Sandbox
tinvest:
  endpoint: "sandbox-invest-public-api.tbank.ru:443"

# Production
tinvest:
  endpoint: "invest-public-api.tbank.ru:443"
```

## Development

### Generate Protocol Buffers

```bash
make proto-gen
```

Generates Go code in `gen/go/<service>/v1/`.

### Run Tests

```bash
make test
```

### Format Code

```bash
make fmt
```

### Linting

```bash
make lint
```

Install linter first:

```bash
make install-tools
```

### Clean Build Artifacts

```bash
make clean
```

## API Reference

### REST Endpoints (v1)

#### Orders

```
POST   /api/v1/orders                  # Place order
GET    /api/v1/orders                  # List orders
GET    /api/v1/orders/:id              # Get order state
DELETE /api/v1/orders/:id              # Cancel order
PUT    /api/v1/orders/:id              # Replace order
```

#### Stop Orders

```
POST   /api/v1/stop-orders             # Place stop order
GET    /api/v1/stop-orders             # List stop orders
DELETE /api/v1/stop-orders/:id         # Cancel stop order
```

#### Market Data

```
GET    /api/v1/candles                 # Get candlesticks
GET    /api/v1/orderbook/:uid          # Get order book
GET    /api/v1/prices                  # Get last prices
GET    /api/v1/trading-status/:uid     # Get trading status
```

#### Portfolio

```
GET    /api/v1/positions               # Get positions
GET    /api/v1/portfolio               # Get full portfolio
GET    /api/v1/limits                  # Get withdraw limits
GET    /api/v1/operations              # Get operation history
GET    /api/v1/accounts                # Get accounts
GET    /api/v1/margin/:account_id      # Get margin attributes
```

#### Risk

```
GET    /api/v1/risk/status             # Get risk status
POST   /api/v1/risk/reset              # Reset circuit breaker
```

#### Real-time streams (WebSocket)

```
GET    /api/v1/stream/candles          # WS: multiplexed candle stream
GET    /api/v1/stream/orderbook        # WS: multiplexed order book stream (since TASK-019)
```

Public production WebSocket: `wss://gateway.24alert.ru:8080/api/v1/stream/orderbook`
(IP-whitelisted; see `docs/STREAM_ORDERBOOK.md`). The same nginx vhost exposes **`/health`** and stream paths; **most REST paths are not proxied on that public URL** (they return 404 from nginx). Co-located apps such as Traderbook call the gateway on the Docker network, e.g. `ALERT_GATEWAY_URL=http://24alert-gateway:8080`.

Query parameters:

- `uids` — CSV of T-Invest instrument UIDs (required, ≤300 per connection)
- `depth` — order book depth (10/20/30/40/50; default 20)

JSON frames: `{type:"snapshot", uid, depth, bids[], asks[], ts}`, `{type:"ping", ts}`,
`{type:"error", error, ts}`. Full protocol + ops playbook: [`docs/STREAM_ORDERBOOK.md`](docs/STREAM_ORDERBOOK.md).

### gRPC Services

- `github.com.24alert.trading.order.v1.OrderService`
- `github.com.24alert.trading.marketdata.v1.MarketDataService`
- `github.com.24alert.trading.portfolio.v1.PortfolioService`
- `github.com.24alert.trading.risk.v1.RiskService`

See `proto/` directory for service definitions.

## Logging

Logs use structured JSON format by default:

```bash
{"time":"2026-04-03T10:00:00Z","level":"INFO","msg":"Order placed","order_id":"abc123","correlation_id":"xyz789"}
```

Configure in `config.yaml`:

```yaml
logging:
  level: "info"      # debug, info, warn, error
  format: "json"     # json or text
  output: "stdout"   # stdout, file, or both
  file_path: "logs/24alert.log"
```

## Contributing

1. All code must be idiomatic Go
2. Proto changes require regeneration: `make proto-gen`
3. Run `make fmt` and `make lint` before commits
4. Add unit tests for new features
5. Update README.md if API changes
6. See `.tasks/` directory for active development tasks and role-based workflow

## Development Workflow

This project uses a **role-based task pipeline** with a structured backlog:

```
Analyst (research) → Backend (implementation) → DevOps (deployment) 
    → Tester (validation) → Tech Lead (review)
```

### Task Status

See `.tasks/TASK-NNN/` for active tasks and `BACKLOG.md` for the complete roadmap:

**Phase 1: MVP (Completed ✅)**
- ✅ **TASK-001**: Server setup and resource validation
- ✅ **TASK-002**: Trading bot implementation (Go microservices)
- ✅ **TASK-003**: Deployment and smoke testing (Production Ready)

**Phase 2: Stabilization (In Progress)**
- ⏳ **TASK-004**: CI/CD pipeline (GitHub Actions) — Planned
- ⏳ **TASK-005**: Kubernetes migration & scaling — Planned
- ⏳ **TASK-006**: Monitoring, logging & alerting — Planned
- ⏳ **TASK-014**: Database migration (PostgreSQL) — Planned
- ⏳ **TASK-016**: Security audit & hardening — Planned

**Phase 3: Growth (Backlog)**
- **TASK-007**: Real trading strategies
- **TASK-008**: Strategy plugin marketplace
- **TASK-009**: Backtesting engine
- **TASK-010**: User management & multi-account

See `BACKLOG.md` for the complete roadmap with priorities, complexity estimates, and dependencies.

## Related Repositories

- **[tinvest-api-bot](https://github.com/sdkinfotech/tinvest-api-bot)** — Reference implementation and experimental strategies (github.com/sdkinfotech/tinvest-api-bot.git)
- **[invest-api-go-sdk](https://github.com/RussianInvestments/invest-api-go-sdk)** — Official T-Invest Go SDK

## References

- [T-Invest API Docs](https://developer.tbank.ru/invest/intro/intro/)
- [T-Invest Go SDK](https://github.com/RussianInvestments/invest-api-go-sdk)
- [Rate Limits](https://russianinvestments.github.io/investAPI/limits/)
- [Sandbox Guide](https://russianinvestments.github.io/investAPI/sandbox/)

## License

TBD

## Support

For issues or questions:
1. Check `.tasks/TASK-002/analyst/data/` for API reference
2. Review proto files in `proto/`
3. Check README in relevant service package
