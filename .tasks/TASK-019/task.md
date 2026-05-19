---
ticket_id: TASK-019
slug: TASK-019
title: "OrderBook WebSocket Stream (gateway + nginx TLS)"
status: done
priority: high
complexity: M
phase: 2
created: 2026-04-16
completed: 2026-04-17
consumers:
  - traderbook/services/market-data (TB-036)
---

# TASK-019 · OrderBook WebSocket Stream

## Контекст

Traderbook (TB-036 «Подсветка плотностей стакана на /charts») требует реального стакана котировок T-Invest по всем интересующим UID с задержкой ≤ 1.5 с. Прямой gRPC-доступ к T-Invest из node-сервиса нежелателен (токены, mTLS, rate-limit), поэтому gateway 24alert выступает adapter'ом: один gRPC-клиент к T-Invest, N WebSocket-потребителей.

## Требования

1. **Endpoint**: `GET /api/v1/stream/orderbook` рядом с существующим `/stream/candles`.
2. **Контракт**: JSON-фреймы `snapshot|ping|error` (см. `docs/STREAM_ORDERBOOK.md`).
3. **Параметры**: `uids` (CSV, required), `depth` (default 20).
4. **Безопасность**: ACL по IP на nginx, TLS Let's Encrypt, токены T-Invest только в `.env`.
5. **Прод-деплой**: публичный URL `wss://gateway.24alert.ru:8080/api/v1/stream/orderbook`.
6. **Тесты**: unit (контракт, валидация), внешний smoke с traderbook VPS.

## Выполнено

- [x] `internal/gateway/handlers/stream.go` — `StreamOrderBook` + типы.
- [x] `stream_orderbook_test.go` — 4 кейса.
- [x] Регистрация в `StreamHandlers.Routes`.
- [x] `deployments/docker-compose.yaml` — publish `127.0.0.1:18080:8080`.
- [x] nginx `gateway.24alert.ru` — TLS + ACL + proxy_pass + WS headers.
- [x] DNS `gateway.24alert.ru` A-record через Timeweb CLI.
- [x] Let's Encrypt через DNS-01 (manual-hooks, TXT через Timeweb CLI).
- [x] Smoke WSS handshake + snapshot frames с traderbook VPS.
- [x] `docs/STREAM_ORDERBOOK.md`.
- [x] `README.md` — раздел «Real-time streams».
- [x] `BACKLOG.md` → Done.

## Smoke-результаты (2026-04-17)

- `orderbook_updates_total` у потребителя (traderbook market-data): **12332** snapshot'ов за 11 минут по 5 UID (SBER 3881, GAZP 2828, GMKN 2925, LKOH 2373, ROSN 2325).
- `orderbook_densities_active` > 0 для SBER (bid=2, ask=1) и ROSN (ask=1) — конец-в-конец детекция работает.
- p95 `orderbook_detect_duration_ms` < 50 мс на клиенте.
- TLS-сертификат валиден до следующего auto-renew (90 дней).

## Связанные

- `docs/STREAM_ORDERBOOK.md` — спецификация endpoint'а.
- Traderbook `docs/runbooks/orderbook-density.md` — использование со стороны потребителя.
- Traderbook backlog TB-036 — зависимая задача.
