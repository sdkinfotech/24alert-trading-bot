# Регрессионный чек-лист 24alert-gateway

> Обновлять после каждой задачи, затрагивающей работу сервисов.
> Последняя ревизия: 2026-04-17 (TASK-007)

---

## 🔴 Критический путь (выполнять ПРИ КАЖДОМ деплое)

### Health & Connectivity
- [ ] `curl https://gateway.24alert.ru:8080/health` → 200 OK, `{"status":"ok"}`
- [ ] Все контейнеры healthy: `docker compose -p 24alert ps`
- [ ] Логи без panic/OOM за последние 5 минут

### REST API — Аккаунты
- [ ] `GET /api/v1/accounts` → список счётов (не пустой)
- [ ] `GET /api/v1/margin/{account_id}` → показатели маржи

### REST API — Ордера
- [ ] `GET /api/v1/orders?account_id=...` → список активных ордеров
- [ ] `POST /api/v1/orders` (market buy тестовый инструмент) → 201 Created
- [ ] `GET /api/v1/orders/{order_id}` → статус размещённого ордера
- [ ] `DELETE /api/v1/orders/{order_id}` → отмена ордера
- [ ] `PUT /api/v1/orders/{order_id}` → replace лимитного ордера

### REST API — Стоп-ордера
- [ ] `POST /api/v1/stop-orders` (stop_loss) → 201
- [ ] `GET /api/v1/stop-orders` → список активных
- [ ] `DELETE /api/v1/stop-orders/{id}` → отмена

### REST API — Маркет-дата
- [ ] `GET /api/v1/candles?instrument_uid=...&interval=1h` → массив свечей
- [ ] `GET /api/v1/orderbook/{uid}?depth=5` → стакан с bids/asks
- [ ] `GET /api/v1/prices` → список последних цен
- [ ] `GET /api/v1/trading-status/{uid}` → статус торгов

### REST API — Портфель
- [ ] `GET /api/v1/positions?account_id=...` → текущие позиции
- [ ] `GET /api/v1/portfolio?account_id=...` → портфель
- [ ] `GET /api/v1/operations?account_id=...` → история операций
- [ ] `GET /api/v1/limits?account_id=...` → лимиты вывода

### REST API — Риск
- [ ] `GET /api/v1/risk/status` → статус circuit breaker
- [ ] `POST /api/v1/risk/reset` → сброс circuit breaker

### WebSocket Stream
- [ ] `WSS /api/v1/stream/orderbook?uids=...` → handshake 101
- [ ] Получение хотя бы 1 frame типа `snapshot` за 30 секунд
- [ ] Frame `ping` получен в течение 20 секунд

---

## 🟡 По задачам (дополнять при закрытии каждой задачи)

### TASK-007: Закрытие портов
- [ ] `nmap -p 9001,9002,9003,9004,9090 176.123.160.234` → все closed
- [ ] Микросервисы доступны локально: `curl http://127.0.0.1:9001/health` и т.д.
- [ ] Prometheus доступен через SSH-туннель: `curl http://127.0.0.1:9090/api/v1/...`

---

## 📋 Процедура прогона

1. **Перед деплоем**: снять baseline (записать текущие метрики)
2. **Сразу после деплоя**: выполнить критический путь (ожидание: всё зелёное)
3. **Через 5 минут**: повторить критический путь (ловим race conditions, утечки)
4. **Через 30 минут**: проверить логи на ошибки, метрики на аномалии
5. **Следующий день**: утренняя проверка после ночного клиринга

## Где хранить результаты

- Результаты прогона: `.tasks/TASK-NNN/tester/handoff.md` → секция "Post-Deploy Verification"
- Скриншоты/логи: `.tasks/TASK-NNN/artifacts/`
- Дата и версия коммита обязательно указываются