# Промпт: Тестировщик (Round 2) → TASK-002

## Контекст
Ты — **QA-инженер**. Задача — **повторный прогон** тестирования после bugfix-раунда бэкенда.

**Исходная постановка**: `.tasks/TASK-002/task.md`
**План**: `.tasks/TASK-002/plan.md`
**Handoff бэкенда (round 2)**: `.tasks/TASK-002/backend/handoff.md`
**Предыдущий handoff тестировщика**: `.tasks/TASK-002/tester/handoff.md` (13 багов)

---

## Что исправлено (из 13 багов)

| BUG | Статус | Как проверить |
|-----|--------|---------------|
| BUG-001 CRITICAL | FIXED | `go test -count=1 ./...` — 6 пакетов, 55 тестов |
| BUG-002 CRITICAL | FIXED | `internal/gateway/adapter/` wiring, cmd/gateway/main.go |
| BUG-003 MAJOR | FIXED | `deployments/.env` создан с реальными токенами |
| BUG-004 MAJOR | FIXED | Токены в `deployments/.env`, логика выбора по `TINVEST_SANDBOX` |
| BUG-006 MAJOR | FIXED | `gateway.Services.Validate()` — fail-fast при nil |
| BUG-008 MINOR | FIXED | Rate limiter отпускает мьютекс перед sleep |
| BUG-010 MAJOR | FIXED | Swagger UI на `/swagger/` (docs/swagger.go + httpSwagger) |
| BUG-011 MEDIUM | FIXED | `config/prometheus.yaml` создан |
| BUG-012 MINOR | FIXED | `Makefile` — `build` без proto-gen |
| BUG-013 MINOR | NOTED | TODO-комментарии для WaitGroup |
| BUG-005 MAJOR | DEFERRED | gRPC registration — зависит от proto-gen, не блокер для gateway |
| BUG-007 MINOR | NOT A BUG | CLI-подкоманды не вызывают runServer |
| BUG-009 MINOR | DEFERRED | printResult fallback — low priority |

---

## Настройка окружения (ОБЯЗАТЕЛЬНО перед тестами)

### Переменные окружения

Файл `deployments/.env` уже содержит реальные sandbox и prod токены. **Не коммитить!**

Структура:
```env
TINVEST_SANDBOX=true                    # true=sandbox, false=production
TINVEST_SANDBOX_TOKEN=t.<sandbox_key>   # песочница
TINVEST_PROD_TOKEN=t.<prod_key>         # боевой
LOG_LEVEL=info
```

Логика выбора токена (`pkg/config`):
- `TINVEST_SANDBOX=true` → `TINVEST_SANDBOX_TOKEN` → endpoint `sandbox-invest-public-api.tbank.ru:443`
- `TINVEST_SANDBOX=false` → `TINVEST_PROD_TOKEN` → endpoint `invest-public-api.tbank.ru:443`
- Fallback на `TINVEST_TOKEN` если специфичный пуст

### Запуск стека

```bash
# Sandbox (по умолчанию):
cd deployments
docker compose up -d

# или из корня:
make docker-up

# Проверка:
curl http://localhost:8080/health
# → {"data":{"status":"ok"}}
```

Для локального запуска без Docker:
```bash
# В PowerShell:
$env:TINVEST_SANDBOX="true"
$env:TINVEST_SANDBOX_TOKEN="t.<sandbox_token>"
$env:LOG_LEVEL="info"

# Запуск gateway:
.\bin\gateway.exe --config config\config.yaml
```

---

## Стратегия тестирования (Round 2)

### Уровень 1: Unit tests — RE-CHECK
```bash
go test -count=1 -v -cover ./...
```
Ожидание: 6 пакетов с тестами, все зелёные, coverage > 0%.

Тестовые файлы:
- `internal/order/repository_test.go` (10 тестов)
- `internal/risk/circuit_breaker_test.go` (9 тестов)
- `pkg/types/money_test.go` (9 тестов)
- `pkg/tinvest/ratelimiter_test.go` (9 тестов)
- `pkg/idempotency/generator_test.go` (3 теста)
- `pkg/logging/logger_test.go` (10 тестов)

### Уровень 2: Integration tests (ОСНОВНАЯ ЗАДАЧА)

**Теперь gateway подключен к backend-сервисам!** Все REST endpoints должны работать.

#### Тест 1: Order Flow (Happy Path)
1. `curl http://localhost:8080/health` → `{"data":{"status":"ok"}}`
2. `curl http://localhost:8080/api/v1/accounts` → список аккаунтов
3. `curl http://localhost:8080/api/v1/prices?instrument_uid=<uid>` → текущая цена
4. `curl http://localhost:8080/api/v1/risk/status` → circuit breaker OK
5. `curl -X POST http://localhost:8080/api/v1/orders -d '{"account_id":"...","instrument_uid":"...","quantity":1,"direction":"buy","order_type":"limit","price":100}'`
6. `curl http://localhost:8080/api/v1/orders?account_id=...` → заявка в списке
7. `curl -X DELETE http://localhost:8080/api/v1/orders/<id>?account_id=...` → отмена

#### Тест 2: Stop Orders
Аналогично через `/api/v1/stop-orders`

#### Тест 3: Market Data
- `GET /api/v1/candles?instrument_uid=...&interval=1h`
- `GET /api/v1/orderbook/<uid>?depth=10`
- `GET /api/v1/trading-status/<uid>`

#### Тест 4: Portfolio
- `GET /api/v1/positions?account_id=...`
- `GET /api/v1/portfolio?account_id=...`
- `GET /api/v1/limits?account_id=...`

#### Тест 5: Risk Validation
- `GET /api/v1/risk/status` → circuit breaker state
- `POST /api/v1/risk/reset` → reset

### Уровень 3: Negative / Edge Cases — RE-CHECK
- Код BUG-008 исправлен — rate limiter не блокирует goroutines
- Код BUG-006 исправлен — nil-guard на сервисах

### Уровень 4: Swagger API Tests
- Открыть `http://localhost:8080/swagger/` 
- Проверить: все 16 endpoints описаны, параметры корректны
- Попробовать выполнить запросы через Swagger UI

---

## Инструменты
- HTTP: `curl` / Swagger UI для REST endpoints
- Docker: `make docker-up` для поднятия стека
- Env: `deployments/.env` с реальными токенами

## Handoff
Обнови `.tasks/TASK-002/tester/handoff.md`:
- Результаты тестов Round 2 (passed/failed)
- Какие из 13 багов подтверждённо исправлены
- Новые баги (если есть)
- Покрытие (какие сценарии пройдены)
- Вердикт: готово для техлида или нужен ещё раунд
