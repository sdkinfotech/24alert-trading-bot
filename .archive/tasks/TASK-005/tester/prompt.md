# Prompt: Tester — TASK-005

## Задача
Проверить полноту E2E тест-покрытия для всех REST API endpoints торгового робота.

## Что проверить

1. Все 22 REST endpoints покрыты тестами
2. Положительные сценарии: market/limit/bestprice orders, candles (все интервалы), orderbook (depth 5/20), prices, trading-status, portfolio, positions, operations, limits, margin, risk
3. Полный trade roundtrip: buy → verify position → verify operations → sell → verify closed
4. Negative/edge cases: missing params, invalid body, non-existent resources
5. Stop orders — known limitation (sandbox не поддерживает), graceful skip
6. Cleanup: каждый тест закрывает позиции

## Артефакты для проверки
- `tests/e2e/*.go`
- Backend handoff: `.tasks/TASK-005/backend/handoff.md`

## Результат
Напиши `.tasks/TASK-005/tester/handoff.md` с оценкой coverage и edge cases.
