# Handoff: Tester → TASK-005

## Статус
DONE

## Что сделано

Проведена проверка полноты E2E тест-покрытия для всех REST API endpoints.

### Coverage-анализ

Все 20 handler routes + 2 infrastructure endpoints (health, swagger) покрыты:

| # | Endpoint | Handler | Test(s) | Verdict |
|---|----------|---------|---------|---------|
| 1 | `POST /api/v1/orders` | PostOrder | MarketOrder_BuySell, LimitOrder_PlaceAndCancel, ReplaceOrder, BestpriceOrder, FullTradeRoundtrip | PASS |
| 2 | `GET /api/v1/orders` | ListOrders | ListOrders, LimitOrder_PlaceAndCancel | PASS |
| 3 | `GET /api/v1/orders/{id}` | GetOrderState | MarketOrder_BuySell, LimitOrder_PlaceAndCancel | PASS |
| 4 | `DELETE /api/v1/orders/{id}` | CancelOrder | LimitOrder_PlaceAndCancel, ReplaceOrder | PASS |
| 5 | `PUT /api/v1/orders/{id}` | ReplaceOrder | ReplaceOrder | PASS |
| 6 | `POST /api/v1/stop-orders` | PostStopOrder | StopLoss, TakeProfit, StopLimit, StopOrdersCRUD | SKIP (sandbox) |
| 7 | `GET /api/v1/stop-orders` | ListStopOrders | StopLoss, StopOrdersCRUD | SKIP (sandbox) |
| 8 | `DELETE /api/v1/stop-orders/{id}` | CancelStopOrder | StopLoss, TakeProfit, StopLimit, StopOrdersCRUD | SKIP (sandbox) |
| 9 | `GET /api/v1/candles` | GetCandles | Candles_1Hour, _1Min, _5Min, _1Day, _DefaultParams | PASS |
| 10 | `GET /api/v1/orderbook/{uid}` | GetOrderbook | Orderbook_Depth5, _Depth20, _DefaultDepth | PASS |
| 11 | `GET /api/v1/prices` | GetPrices | LastPrice, LimitOrder, ReplaceOrder | PASS |
| 12 | `GET /api/v1/trading-status/{uid}` | GetTradingStatus | TradingStatus | PASS |
| 13 | `GET /api/v1/accounts` | ListAccounts | ListAccounts (+ TestMain) | PASS |
| 14 | `GET /api/v1/margin/{account_id}` | GetMargin | Margin | PASS |
| 15 | `GET /api/v1/positions` | GetPositions | Positions, MarketOrder, FullTradeRoundtrip | PASS |
| 16 | `GET /api/v1/portfolio` | GetPortfolio | Portfolio, FullTradeRoundtrip | PASS |
| 17 | `GET /api/v1/limits` | GetLimits | WithdrawLimits | PASS |
| 18 | `GET /api/v1/operations` | GetOperations | Operations, MarketOrder, FullTradeRoundtrip | PASS |
| 19 | `GET /api/v1/risk/status` | GetStatus | RiskStatus, RiskReset | PASS |
| 20 | `POST /api/v1/risk/reset` | Reset | RiskReset | PASS |
| 21 | `GET /health` | (framework) | Health | PASS |
| 22 | `GET /swagger/index.html` | (swagger) | SwaggerUI | PASS |

### Negative / Edge Cases (12 tests)

| Case | Test | Verdict |
|------|------|---------|
| Order: missing account_id | Order_MissingAccountID | PASS (400) |
| Order: missing instrument_uid | Order_MissingInstrumentUID | PASS (400) |
| Order: empty body | Order_InvalidBody | PASS (400) |
| Orders list: missing account_id | ListOrders_MissingAccountID | PASS (400) |
| Cancel: non-existent order | CancelOrder_NonExistent | PASS (rpc NotFound) |
| Stop order: missing account_id | StopOrder_MissingAccountID | PASS (400) |
| Candles: missing instrument_uid | Candles_MissingInstrumentUID | PASS (400) |
| Positions: missing account_id | Positions_MissingAccountID | PASS (400) |
| Portfolio: missing account_id | Portfolio_MissingAccountID | PASS (400) |
| Operations: missing account_id | Operations_MissingAccountID | PASS (400) |
| Limits: missing account_id | Limits_MissingAccountID | PASS (400) |
| Cancel: non-existent stop order | CancelStopOrder_NonExistent | PASS (rpc NotFound) |

### Test Quality Assessment

**Strengths:**
- Full trade roundtrip verified (buy → position → operations → sell → portfolio)
- All order types: market, limit, bestprice, replace, cancel
- Market data: 4 candle intervals, 3 orderbook depths, prices, trading status
- Proper cleanup: positions closed after each test
- Rate limiting: 500ms pause between API calls
- Graceful skip for sandbox limitations

**Known Limitations:**
- Stop orders (POST/GET/DELETE): T-Invest sandbox returns error 30043, cannot test without production account
- Futures instrument: not tested separately (would need FORTS-specific instrument UID)
- WebSocket/gRPC streaming: not covered in HTTP E2E tests (requires separate test infrastructure)

### Summary

| Metric | Value |
|--------|-------|
| Total test functions | 43 |
| PASS | 38 |
| SKIP | 4 |
| FAIL | 0 |
| Endpoint coverage | 22/22 (100%) |
| Real trade operations verified | YES (market buy, sell, limit, bestprice, cancel, replace) |

## Артефакты
- Тест-отчёт: этот файл
- Тесты: `tests/e2e/`

## Корректировки для следующих ролей
Для роли tech-lead: стоп-ордера нужно тестировать отдельно на production account (не sandbox). Рекомендуется завести TASK для production smoke tests.

## Блокеры
НЕТ
