# Промпт: Тестировщик → TASK-003

## Контекст
Ты — **QA-инженер**. Задача — провести smoke testing развёрнутого торгового робота на prod-сервере через CLI, REST API и Swagger UI.

**Исходная постановка**: `.tasks/TASK-003/task.md`
**План**: `.tasks/TASK-003/plan.md`
**Handoff DevOps**: `.tasks/TASK-003/devops/handoff.md` (что развёрнуто)

---

## Smoke tests (базовые)

### Тест 1: CLI работает

```bash
# На prod-сервере, в контейнере gateway

# Проверить, что CLI работает
docker-compose exec gateway 24alert --help

# Должен вывести список команд
```

### Тест 2: Accounts

```bash
# Получить список счетов
docker-compose exec gateway 24alert account list

# Ожидаемый результат: таблица с account_id, name, status
```

### Тест 3: Portfolio (из localhost)

```bash
curl -s http://localhost:8080/api/v1/portfolio | jq .

# Ожидаемый результат: positions, balances, total_balance
```

### Тест 4: Market Data

```bash
# Получить цены
curl -s http://localhost:8080/api/v1/prices \
  -H "Content-Type: application/json" \
  -d '{"instruments": ["BBG000B9XRY4"]}' | jq .

# Ожидаемый результат: последние цены инструментов
```

### Тест 5: Risk Status

```bash
curl -s http://localhost:8080/api/v1/risk/status | jq .

# Ожидаемый результат: circuit_breaker_state, position_count, margin_level
```

### Тест 6: Swagger UI

Открыть в браузере:
```
http://176.123.160.234:8080/swagger/
```

Проверить:
- [ ] Страница загружается
- [ ] Видны все endpoints (/api/v1/*)
- [ ] Можно развернуть каждый метод
- [ ] Есть описания параметров и responses

### Тест 7: Структурированные логи

```bash
docker-compose logs gateway | head -20

# Ожидаемый результат: JSON каждой строки логов с полями:
# - timestamp
# - level (info, error, warn, debug)
# - msg
# - service (если есть)
```

### Тест 8: Health check всех сервисов

```bash
# Gateway
curl -s http://localhost:8080/health | jq .

# Ожидаемый результат: {"status": "ok"} или {"status": "healthy"}
```

### Тест 9: Отправить реальную заявку (осторожно!)

```bash
# Получить инструмент
curl -s http://localhost:8080/api/v1/instruments | jq '.[0]' > /tmp/instr.json

# Отправить LIMIT order (малый объём!)
curl -X POST http://localhost:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -d '{
    "instrument_uid": "инструмент_uid",
    "quantity": 1,
    "direction": "BUY",
    "order_type": "LIMIT",
    "price": 100
  }' | jq .

# Отмена заявки
curl -X DELETE http://localhost:8080/api/v1/orders/order_id | jq .
```

### Тест 10: Negative case — Rate limit

```bash
# Отправить быстро много запросов
for i in {1..50}; do
  curl -s http://localhost:8080/api/v1/portfolio &
done
wait

# Ожидаемый результат: некоторые запросы должны получить 429 (Too Many Requests)
# или graceful backoff (логи с rate limit info)
```

---

## Результаты тестирования

Для каждого теста:
- ✓ PASSED / ✗ FAILED
- Время ответа (должен быть <1s)
- Ошибки (если есть)

---

## Handoff

Создай `.tasks/TASK-003/tester/handoff.md`:
- Результаты всех 10 smoke tests
- Время ответа API
- Найденные проблемы (если есть)
- Рекомендации для техлида
