# Handoff: backend | ticket_id: TASK-019 | slug: TASK-019

## Тикет
- **ticket_id:** TASK-019
- **slug:** TASK-019

## Статус
DONE

## Что сделано
- `internal/gateway/handlers/stream.go`: добавлены типы `StreamLevel`, `StreamOrderBookMsg`; реализован handler `StreamOrderBook(w, r)` с WS upgrade, парсингом `uids`/`depth`, подпиской `StreamManager.SubscribeOrderbook`, сериализацией `pb.OrderBook` → JSON snapshot, heartbeat `ping` каждые 15с, корректным закрытием по `r.Context().Done()`.
- Регистрация маршрута в `StreamHandlers.Routes`: `GET /api/v1/stream/orderbook`.
- `internal/gateway/handlers/stream_orderbook_test.go`: 4 unit-теста (пустой `uids` → 400, whitespace-only `uids` → 400, snapshot JSON shape, ping JSON shape).
- `go build ./...` и `go test ./internal/gateway/...` зелёные.

## Артефакты
- Файлы: `internal/gateway/handlers/stream.go`, `internal/gateway/handlers/stream_orderbook_test.go`.
- Коммит: ветка `main`, `feat(gateway): add /api/v1/stream/orderbook WS endpoint (TASK-019)` (см. `git log`).

## Корректировки для следующих ролей
Для роли **devops**: передеплоить gateway на srv03-cloud, поднять nginx `gateway.24alert.ru` (TLS, ACL), получить LE-сертификат через DNS-01 (80/443 заблокированы провайдером, слушать на 8080).

## Блокеры
НЕТ
