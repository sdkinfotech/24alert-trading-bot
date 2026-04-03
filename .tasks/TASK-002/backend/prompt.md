# Промпт: Бэкенд → TASK-002

## Контекст
Ты — **Senior Backend-разработчик (Go)**. Задача — реализовать торгового робота как набор Go-микросервисов. Стратегия подключается как плагин, базовый робот — это инфраструктура для исполнения заявок.

**Исходная постановка**: `.tasks/TASK-002/task.md`
**План**: `.tasks/TASK-002/plan.md`
**Данные аналитика**: `.tasks/TASK-002/analyst/handoff.md` (rate limits, SDK reference)

---

## Архитектура (из плана)

```
gateway (:8080) ─── REST/Swagger + CLI
    ├── order-svc (:9001)      → T-Invest Orders API
    ├── marketdata-svc (:9002)  → T-Invest MarketData API
    ├── portfolio-svc (:9003)   → T-Invest Operations/Users API
    └── risk-svc (:9004)        → portfolio-svc + marketdata-svc (no direct T-Invest calls)

strategy-plugin (:9010) ─── external gRPC server (не реализуется, только proto)
```

Между сервисами: **gRPC**. Наружу: **REST** (gateway). CLI: **cobra**.

---

## Phase 1: Scaffold + Proto + Shared (размер M)

### 1.1 Scaffold монорепо

```
c:\Users\sdk\proj\24alert\
  go.mod                    # module github.com/24alert/trading-bot (или 24alert.ru/bot)
  go.sum
  Makefile                  # targets: proto-gen, build, test, lint, run-all
  .gitignore                # Go standard + proto gen + binaries
  README.md                 # Заполнить в конце
  config/
    config.yaml             # Default config (см. план)
    config.sandbox.yaml     # Sandbox overrides
  proto/                    # Shared proto definitions
  pkg/                      # Shared Go packages
  cmd/                      # Service entrypoints (main.go)
  internal/                 # Service implementations
  deployments/              # Docker (DevOps заполнит)
  docs/                     # Swagger (auto-generated)
```

Инициализация:
```bash
go mod init github.com/24alert/trading-bot
```

### 1.2 Proto-файлы

Все proto в `proto/` с package path `<domain>/v1`:

**`proto/common/v1/types.proto`**:
- `Quotation` (units + nano), `MoneyValue` (currency + Quotation)
- `OrderDirection` enum (BUY, SELL)
- `OrderType` enum (MARKET, LIMIT, BESTPRICE)
- `OrderExecutionType` enum (SYNC, ASYNC)
- `StopOrderType` enum (STOP_LOSS, TAKE_PROFIT)
- `Signal` (instrument_uid, direction, order_type, quantity, price, stop_price, reason)
- `OrderIntent` (signal + risk_verdict + account_id)
- `ExecutionEvent` (order_id, status, filled_qty, avg_price, timestamp)
- `InstrumentShort` (uid, figi, ticker, class_code, name, lot_size)

**`proto/order/v1/order.proto`** — `OrderService`:
- `PostOrder(PostOrderRequest) returns (PostOrderResponse)`
- `CancelOrder(CancelOrderRequest) returns (CancelOrderResponse)`
- `ReplaceOrder(ReplaceOrderRequest) returns (PostOrderResponse)`
- `GetOrders(GetOrdersRequest) returns (GetOrdersResponse)`
- `GetOrderState(GetOrderStateRequest) returns (OrderState)`
- `PostStopOrder(PostStopOrderRequest) returns (PostStopOrderResponse)`
- `CancelStopOrder(CancelStopOrderRequest) returns (CancelStopOrderResponse)`
- `GetStopOrders(GetStopOrdersRequest) returns (GetStopOrdersResponse)`
- `StreamOrderStates(StreamOrderStatesRequest) returns (stream OrderStateEvent)`
- `StreamTrades(StreamTradesRequest) returns (stream TradeEvent)`

**`proto/marketdata/v1/marketdata.proto`** — `MarketDataService`:
- `GetCandles`, `GetOrderbook`, `GetLastPrices`, `GetClosePrices`
- `GetHistory`, `GetTradingStatus`
- `Subscribe(SubscribeRequest) returns (stream MarketDataEvent)`

**`proto/portfolio/v1/portfolio.proto`** — `PortfolioService`:
- `GetPositions`, `GetPortfolio`, `GetWithdrawLimits`
- `GetOperations`, `GetAccounts`, `GetMarginAttributes`
- `StreamPortfolio`, `StreamPositions`

**`proto/risk/v1/risk.proto`** — `RiskService`:
- `ValidateOrderIntent(OrderIntent) returns (RiskVerdict)` — APPROVED/REJECTED + reason
- `GetRiskStatus(Empty) returns (RiskStatus)` — circuit breaker state, position count, margin
- `ResetCircuitBreaker(Empty) returns (Empty)`

**`proto/strategy/v1/strategy.proto`** — `StrategyService` (только интерфейс):
- `Evaluate(MarketState) returns (Signal)`
- `Configure(StrategyConfig) returns (ConfigResponse)`
- `GetInfo(Empty) returns (StrategyInfo)`

**Makefile target `proto-gen`**:
```makefile
proto-gen:
	protoc --go_out=. --go-grpc_out=. proto/**/**/*.proto
```
Сгенерированный код: `gen/go/<package>/v1/*.pb.go`

### 1.3 Shared пакеты (`pkg/`)

**`pkg/config/config.go`**:
- Viper-based loader: читает `config/config.yaml` + env vars (`TINVEST_TOKEN`, etc.)
- Struct `Config` с секциями: TInvest, Services, Risk, Logging, Metrics

**`pkg/tinvest/client.go`**:
- Обёртка над `invest-api-go-sdk`: создание клиента, sandbox/live switch
- `NewTInvestClient(cfg Config) (*TInvestClient, error)`
- Методы: `OrdersService()`, `MarketDataService()`, `OperationsService()`, `UsersService()`, `StopOrdersService()`, `MarketDataStreamService()`

**`pkg/tinvest/ratelimiter.go`**:
- Per-method rate limiter (token bucket / leaky bucket)
- Конфигурация лимитов из handoff аналитика (rate-limits.json)
- `NewRateLimiter(method string, rpm int) *RateLimiter`
- `Wait(ctx context.Context) error` — блокирует до получения токена

**`pkg/types/money.go`**:
- `QuotationToFloat64(q *Quotation) float64`
- `Float64ToQuotation(f float64) *Quotation`
- `MoneyValueToFloat64(m *MoneyValue) float64`

**`pkg/types/instrument.go`**:
- Helper для работы с UID, FIGI, ticker, class_code

**`pkg/logging/logger.go`**:
- `log/slog` setup: JSON format, configurable level
- `WithCorrelationID(ctx context.Context, id string) context.Context`
- `FromContext(ctx context.Context) *slog.Logger`

**`pkg/idempotency/generator.go`**:
- `NewOrderID() string` — UUID v4, max 36 chars (требование T-Invest API)

---

## Phase 2: Core сервисы (размер XL)

### 2.1 order-service (`cmd/order-svc/main.go`, `internal/order/`)

**`internal/order/server.go`**: gRPC server, регистрация `OrderService`.

**`internal/order/service.go`**: бизнес-логика:
- `PostOrder`: вызов `tinvest.OrdersService().PostOrder()` с rate limiting. Поддержка: MARKET, LIMIT, BESTPRICE. Idempotent order_id (UUID).
- `CancelOrder`: вызов `CancelOrder` с rate limiting.
- `ReplaceOrder`: вызов `ReplaceOrder`.
- `GetOrders`: список активных заявок.
- `GetOrderState`: статус конкретной заявки.
- `PostStopOrder`, `CancelStopOrder`, `GetStopOrders`: стоп-заявки.

**`internal/order/stream.go`**: стриминг:
- `StreamOrderStates`: подписка на `OrderStateStream`, fan-out клиентам.
- `StreamTrades`: подписка на `TradesStream`.

**`internal/order/repository.go`**: in-memory хранилище:
- Активные заявки (map по order_id)
- Журнал исполнений (fills): order_id, timestamp, qty, price, status
- `GetActiveOrders()`, `GetOrderJournal()`, `UpdateOrderState()`

Rate limits (из аналитика handoff):
- postOrder: 15 req/s (sync), 600 req/min (async)
- cancelOrder: 100 req/min
- replaceOrder: 100 req/min
- getOrders: 200 req/min
- getOrderState: 100 req/min
- PostStopOrder: 50 req/min
- CancelStopOrder: 50 req/min
- GetStopOrders: 60 req/min

### 2.2 marketdata-service (`cmd/marketdata-svc/main.go`, `internal/marketdata/`)

**`internal/marketdata/service.go`**: unary методы:
- `GetCandles`: свечи за период (interval: 1min, 5min, 15min, 1h, 4h, 1d, 1w, 1m)
- `GetOrderbook`: стакан (depth: 1-50)
- `GetLastPrices`: последние цены по списку инструментов
- `GetClosePrices`: цены закрытия
- `GetHistory`: ZIP-архив исторических данных (30 req/min)
- `GetTradingStatus`: статус торговой сессии инструмента

**`internal/marketdata/stream.go`**: stream manager:
- Один gRPC stream к T-Invest, мультиплексирование подписок
- `Subscribe(subscription_type, instrument_uid)` → добавить в стрим
- `Unsubscribe(subscription_type, instrument_uid)` → убрать
- Типы подписок: candles, orderbook, trades, last_price, info
- Лимит: 300 подписок / 32 стрима

**`internal/marketdata/cache.go`**:
- Кэш инструментов (uid → InstrumentShort)
- Кэш последних цен (обновляется из стрима)

### 2.3 portfolio-service (`cmd/portfolio-svc/main.go`, `internal/portfolio/`)

**`internal/portfolio/service.go`**:
- `GetPositions`: текущие позиции (с PnL)
- `GetPortfolio`: полный портфель (позиции + балансы + expected yield)
- `GetWithdrawLimits`: лимиты на вывод (доступные средства)
- `GetOperations`: история операций по инструменту (cursor-based pagination)
- `GetAccounts`: список счетов
- `GetMarginAttributes`: маржинальные параметры

**`internal/portfolio/stream.go`**:
- `StreamPortfolio`: подписка на изменения портфеля
- `StreamPositions`: подписка на изменения позиций

### 2.4 risk-service (`cmd/risk-svc/main.go`, `internal/risk/`)

**Не вызывает T-Invest API напрямую** — обращается к portfolio-svc и marketdata-svc через gRPC.

**`internal/risk/service.go`**:
- `ValidateOrderIntent(intent)` → `RiskVerdict {approved bool, reason string}`
  1. Вызывает `checker/session.go` → marketdata-svc.GetTradingStatus
  2. Вызывает `checker/balance.go` → portfolio-svc.GetWithdrawLimits + GetMarginAttributes
  3. Вызывает `checker/position_limit.go` → portfolio-svc.GetPositions
  4. Проверяет circuit breaker state
  5. Возвращает APPROVED или REJECTED + reason

**`internal/risk/checker/session.go`**: торговая сессия открыта?
**`internal/risk/checker/balance.go`**: достаточно средств/маржи?
**`internal/risk/checker/position_limit.go`**: не превышен лимит позиции?
**`internal/risk/circuit_breaker.go`**: N подряд ошибок → halt; cooldown → auto-reset.

**`internal/risk/service.go`**:
- `GetRiskStatus()`: circuit breaker state, active positions count, margin level
- `ResetCircuitBreaker()`: ручной сброс

Config (из config.yaml):
```yaml
risk:
  max_position_lots: 10
  circuit_breaker_threshold: 5
  circuit_breaker_cooldown: "5m"
```

---

## Phase 3: Gateway + CLI (размер L)

### 3.1 gateway-service (`cmd/gateway/main.go`, `internal/gateway/`)

**`internal/gateway/server.go`**:
- HTTP server (chi или echo)
- Swagger UI на `/swagger/`
- Health check на `/health`
- gRPC клиенты к 4 сервисам

**`internal/gateway/handlers/`**: REST handlers, каждый проксирует в gRPC:
- `orders.go`: POST /api/v1/orders, DELETE /api/v1/orders/:id, PUT /api/v1/orders/:id, GET /api/v1/orders, GET /api/v1/orders/:id
- `stop_orders.go`: POST /api/v1/stop-orders, DELETE /api/v1/stop-orders/:id, GET /api/v1/stop-orders
- `marketdata.go`: GET /api/v1/candles, GET /api/v1/orderbook/:uid, GET /api/v1/prices, GET /api/v1/trading-status/:uid
- `portfolio.go`: GET /api/v1/positions, GET /api/v1/portfolio, GET /api/v1/limits, GET /api/v1/operations
- `accounts.go`: GET /api/v1/accounts, GET /api/v1/margin/:account_id
- `risk.go`: GET /api/v1/risk/status, POST /api/v1/risk/reset

Каждый handler: swagger-аннотации (`swaggo/swag`), structured logging, error mapping (gRPC → HTTP status).

### 3.2 CLI (`internal/gateway/cli/`)

Cobra root command: `24alert`

Команды (все проксируют в gateway REST API):
```
24alert order post     --instrument <uid> --qty <n> --type <limit|market|bestprice> [--price <p>] --direction <buy|sell>
24alert order cancel   --order-id <id>
24alert order replace  --order-id <id> --qty <n> --price <p>
24alert order list     [--account-id <id>]
24alert order state    --order-id <id>
24alert stop post      --instrument <uid> --qty <n> --stop-price <p> --type <stop_loss|take_profit> --direction <buy|sell>
24alert stop cancel    --stop-order-id <id>
24alert stop list
24alert market candles --instrument <uid> --from <ts> --to <ts> --interval <1min|5min|1h|1d>
24alert market book    --instrument <uid> [--depth 20]
24alert market price   --instrument <uid>
24alert market status  --instrument <uid>
24alert portfolio positions [--account-id <id>]
24alert portfolio info      [--account-id <id>]
24alert portfolio limits    [--account-id <id>]
24alert portfolio ops       --instrument <uid> --from <ts> --to <ts>
24alert account list
24alert account margin [--account-id <id>]
24alert risk status
24alert risk reset
```

Вывод: таблица (tablewriter) или JSON (`--output json`).

---

## Общие требования ко всему коду

1. **Error handling**: gRPC status codes (NOT_FOUND, INVALID_ARGUMENT, RESOURCE_EXHAUSTED, UNAVAILABLE); wrap errors с контекстом.
2. **Logging**: `slog` с JSON format, correlation ID в каждом запросе.
3. **Config**: viper, YAML + env vars. `TINVEST_TOKEN` обязателен.
4. **Context**: передавать ctx везде; graceful shutdown через context cancellation.
5. **Rate limiting**: обязателен для КАЖДОГО вызова T-Invest API.
6. **Tests**: unit tests для service.go каждого сервиса (mock T-Invest client).
7. **Код**: простой, идиоматичный Go. Без фреймворков кроме указанных.

---

## Handoff

По завершении каждой фазы создай `.tasks/TASK-002/backend/handoff.md`:
- Что реализовано (список файлов, методов)
- Что не реализовано / отложено
- Корректировки для DevOps (если порты, зависимости изменились)
- Корректировки для тестировщика (какие endpoints тестировать)

## Дополнительные ресурсы

См. `.tasks/TASK-002/REFERENCES.md`:
- Reference implementation: [tinvest-api-bot](https://github.com/sdkinfotech/tinvest-api-bot.git)
- Official SDK docs и API reference
- Rate limits и Sandbox guide
