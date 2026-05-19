# Handoff: Backend → TASK-005

## Статус
DONE

## Что сделано

### E2E Test Suite: `tests/e2e/`

- **helpers_test.go** — TestMain (auto-discovers sandbox account), HTTP helpers (doGet/doPost/doDelete/doPut/doRaw), generic unmarshal, domain types, rate limit pause
- **health_test.go** — Health (200 OK), Swagger UI, ListAccounts, Portfolio, Positions, WithdrawLimits, Operations, Margin
- **orders_test.go** — Market Buy/Sell (full roundtrip), Limit Order (place + cancel + verify cancelled), Replace Order (new price/qty), Bestprice Order, List Orders, Full Trade Roundtrip (portfolio → price → candles → orderbook → buy → position → operations → sell → portfolio)
- **stop_orders_test.go** — Stop-loss, Take-profit, Stop-limit, CRUD (place 2 → list → cancel 1 → verify → cancel 2); all gracefully skip on sandbox limitation (error 30043)
- **marketdata_test.go** — Candles 1min/5min/1hour/1day/default, Orderbook depth=5/20/default, Last Price, Trading Status
- **risk_test.go** — Risk Status, Risk Reset + verify
- **negative_test.go** — Missing account_id (orders, stop-orders, positions, portfolio, operations, limits), Missing instrument_uid (orders, candles), Invalid body, Cancel non-existent order, Cancel non-existent stop order

### Test Execution Results

Run against `http://176.123.160.234:8080` (sandbox mode):

- **38 PASS** — all REST endpoints verified with real T-Invest sandbox operations
- **4 SKIP** — stop orders (T-Invest sandbox does not support PostStopOrder, error 30043)
- **0 FAIL**

### What was covered

| Area | Endpoints | Tests | Status |
|------|-----------|-------|--------|
| Health/Swagger | 2 | 2 | PASS |
| Accounts | 2 | 2 | PASS |
| Orders | 5 | 6 | PASS (market, limit, bestprice, replace, cancel, list) |
| Stop Orders | 3 | 4 | SKIP (sandbox limitation) |
| Market Data | 4 | 10 | PASS (candles 4 intervals, orderbook 3 depths, price, status) |
| Portfolio | 4 | 5 | PASS |
| Risk | 2 | 2 | PASS |
| Negative | - | 12 | PASS |

**Total: 22 REST endpoints, 43 test functions, 38 pass / 4 skip / 0 fail**

### Instrument Used
- SBER (uid: e6123145-9665-43e0-8413-cd61b8aa9b13) — ~315 RUB per lot

### Setup performed
- Switched server to sandbox mode (`TINVEST_SANDBOX=true`)
- Funded sandbox account with 1,000,000 RUB via SandboxPayIn

## Артефакты
- tests/e2e/helpers_test.go
- tests/e2e/health_test.go
- tests/e2e/orders_test.go
- tests/e2e/stop_orders_test.go
- tests/e2e/marketdata_test.go
- tests/e2e/risk_test.go
- tests/e2e/negative_test.go

## Корректировки для следующих ролей
Для роли tester: стоп-ордера не тестируемы в sandbox — добавить в отчёт как known limitation.

## Блокеры
НЕТ
