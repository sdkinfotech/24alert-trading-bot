# TASK-005: Full API E2E Testing (Sandbox)

**Priority**: HIGH
**Complexity**: L
**Phase**: 2

## Goal

Покрыть реальными E2E тестами все 22 REST endpoint'а торгового робота.
Тесты выполняются на production сервере (sandbox mode T-Invest API)
с дешёвыми инструментами (1 лот).

## Scope

- Все REST endpoints (/api/v1/*)
- Все типы ордеров: market, limit, bestprice
- Stop-orders: stop_loss, take_profit, stop_limit
- Market data: candles (все интервалы), orderbook, prices, trading-status
- Portfolio: positions, portfolio, limits, operations, margin
- Risk: status, reset
- Health + Swagger
- Negative / edge cases
- Полные сценарии: открытие → проверка → закрытие позиции

## Dependencies

- TASK-004 (CI/CD) — Done
- Production server running (sandbox mode)
