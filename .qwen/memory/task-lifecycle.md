# Task Lifecycle — Флоу прохождения задачи

## Жизненный цикл задачи

```
 ┌──────────┐
 │ CREATED  │  ← Задача создана в BACKLOG.md
 └────┬─────┘
      │ Планировщик выбирает задачу
      ▼
 ┌──────────┐
 │ PLANNED  │  ← Создан plan.md, prompts для ролей
 └────┬─────┘
      │ Backend/DevOps начинает работу
      ▼
 ┌──────────────┐
 │ IN PROGRESS  │  ← Работа ведётся, handoff'ы пишутся по ролям
 └────┬─────────┘
      │ Все роли завершили handoff
      ▼
 ┌──────────┐
 │ REVIEW   │  ← Tech-lead проверяет код и артефакты
 └────┬─────┘
      │
  ┌───┴───┐
  │       │
  ▼       ▼
┌─────┐ ┌───────┐
│DONE │ │NEEDS  │  ← Tech-lead отклонил → корректировки
│     │ │FIX    │
└──┬──┘ └───┬───┘
   │        │
   │        │ Повторный REVIEW → DEPLOY VERIFY
   │        ▼
   │    ┌──────────────┐
   │    │ DEPLOY VERIFY│  ← Post-deploy проверка на проде
   │    └──────┬───────┘
   │           │
   ▼           ▼
 ┌──────────────────┐
 │ DEPLOY VERIFY    │  ← ОБЯЗАТЕЛЬНЫЙ этап после каждого деплоя
 │ (Post-Deploy)    │
 └────┬─────────────┘
      │
      ▼
 ┌──────────┐
 │ CLOSED   │  ← Финальная запись в BACKLOG.md
 └──────────┘
```

---

## DEPLOY VERIFY — Обязательный post-deploy этап

**Для КАЖДОЙ задачи**, которая затрагивает работающие сервисы, после деплоя ОБЯЗАТЕЛЬНО проходит проверка на продакшене.

### 1. Проверка логов (Logs Check)

```bash
# Все контейнеры должны быть healthy
docker compose -p 24alert ps

# Логи gateway и сервисов — нет ошибок, OOM, panic
docker logs --since 5m 24alert-gateway 2>&1 | grep -iE 'error|panic|fatal|unhealthy'
docker logs --since 5m 24alert-order-svc 2>&1 | grep -iE 'error|panic|fatal|unhealthy'
docker logs --since 5m 24alert-marketdata-svc 2>&1 | grep -iE 'error|panic|fatal|unhealthy'
docker logs --since 5m 24alert-portfolio-svc 2>&1 | grep -iE 'error|panic|fatal|unhealthy'
docker logs --since 5m 24alert-risk-svc 2>&1 | grep -iE 'error|panic|fatal|unhealthy'
```

**Цель**: убедиться что все сервисы стартанули, нет ошибок подключения к T-Invest, нет падений.

### 2. Smoke-тесты (Smoke Check)

```bash
# Health check
curl -fsS https://gateway.24alert.ru:8080/health

# REST API — базовые запросы (зависит от задачи)
curl -fsS https://gateway.24alert.ru:8080/api/v1/accounts
curl -fsS https://gateway.24alert.ru:8080/api/v1/instruments/shares
curl -fsS https://gateway.24alert.ru:8080/api/v1/stream/orderbook?uids=...

# WebSocket стрим (если затрагивает)
python3 .tmp/wss_smoke.py
```

**Цель**: критические пути работают после изменений.

### 3. Логика-тесты (Logic Check)

Проверка конкретной бизнес-логики, которую затронула задача. Примеры:

```bash
# Пример: проверка что ордера проходят
curl -X POST https://gateway.24alert.ru:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -d '{...}'

# Проверка что стакан обновляется
python3 scripts/check_orderbook_updates.py --uid X --timeout 30

# Проверка что risk-сервис блокирует плохие ордера
curl -X POST ... # с негативными данными
```

**Цель**: бизнес-логика работает корректно, не только «сервер отвечает 200».

### 4. Регрессионные тесты (Regression)

Каждая задача ОБЯЗАТЕЛЬНО прогоняет существующий регрессионный набор. После завершения задачи — обновляет его.

**Текущий регрессионный чек-лист** (хранится в `REGRESSION.md`):

```
## Регрессионный чек-лист 24alert-gateway

### Критический путь (выполнять ВСЕГДА)
- [ ] GET /health → 200 OK
- [ ] GET /api/v1/accounts → список счётов
- [ ] POST /api/v1/orders (market buy SBER) → 201
- [ ] GET /api/v1/orders → список ордеров
- [ ] GET /api/v1/orders/{id} → статус ордера
- [ ] DELETE /api/v1/orders/{id} → отмена ордера
- [ ] GET /api/v1/candles?interval=1h → свечи
- [ ] GET /api/v1/orderbook/{uid}?depth=5 → стакан
- [ ] GET /api/v1/prices → последние цены
- [ ] GET /api/v1/positions → позиции
- [ ] GET /api/v1/portfolio → портфель
- [ ] GET /api/v1/risk/status → статус рисков
- [ ] WSS /stream/orderbook?uids=... → подключение + snapshot

### По задаче (дополнять при необходимости)
- [ ] <задача-специфичные проверки>
```

При обнаружении багов — новые проверки добавляются в этот список и остаются навсегда.

---

## Правила добавления в Regression

1. **Любой баг, найденный при ручочном тестировании** → добавляется regression check
2. **Любой новый эндпоинт** → добавляется в критический путь
3. **Любое изменение логики** → добавляется проверка конкретного поведения
4. **Regression живёт в репо** — `REGRESSION.md` обновляется при закрытии каждой задачы

---

## Формат handoff — дополнение

Каждый handoff ДОЛЖЕН содержать секцию:

```markdown
## Post-Deploy Verification

- [ ] Логи проверены (нет error/panic)
- [ ] Smoke-тесты пройдены
- [ ] Логика-тесты пройдены
- [ ] Регрессионные тесты пройдены
- [ ] Промежуток наблюдения: X минут после деплоя — без аномалий
```