# Промпт: Аналитик → TASK-002

## Контекст
Ты — **Аналитик**. Задача — исследовать T-Invest (Тинькофф Инвестиции) API, SDK, rate limits, sandbox-режим и задокументировать всё для бэкенд-разработчика. Твои выводы определяют архитектурные решения.

**Исходная постановка**: `.tasks/TASK-002/task.md`
**План**: `.tasks/TASK-002/plan.md`

---

## 1. Исследовать T-Invest API

### SDK
- Репозиторий: `github.com/russianinvestments/invest-api-go-sdk`
- Получить актуальную версию, API surface, примеры
- Как создаётся клиент? Как переключаться sandbox ↔ production?
- Какие сервисы доступны: Orders, MarketData, Operations, Users, Instruments, StopOrders?

### gRPC API
- Endpoint: `invest-public-api.tbank.ru:443`
- Аутентификация: токен в metadata (`Authorization: Bearer <token>`)
- Proto-файлы: где взять? Есть ли в SDK или отдельно?
- Стриминговые методы: OrderStateStream, TradesStream, MarketDataStream, PortfolioStream, PositionsStream

### Rate Limits (критично!)
Для каждого метода собрать:
- Лимит запросов в минуту / секунду
- Лимит на стримы (количество одновременных подключений)
- Лимит на подписки в стриме
- Что происходит при превышении? (код ошибки, backoff)

Источник: https://russianinvestments.github.io/investAPI/limits/

### Sandbox
- Как создать sandbox account?
- Какие методы доступны в sandbox?
- Чем отличается от production? (фейковые данные? задержки?)
- Как пополнить sandbox баланс?

### Типы заявок
- Market, Limit, BestPrice — параметры, ограничения
- Stop orders: stop_loss, take_profit — как работают?
- Replace order — ограничения, что можно менять?
- Iceberg (если поддерживается)
- `order_id` — идемпотентность, формат (UUID? max 36 символов?)

---

## 2. Структурирование выводов

### Wiki (Obsidian) — обязательно
Создать в `traderbook/Knowledge/Architecture/`:
- `T-Invest API Reference.md` — полный справочник методов, rate limits, auth
- `T-Invest SDK Go.md` — как использовать SDK, примеры кода, gotchas

### Memory graph (MCP) — обязательно
Создать entities:
- `tinvest-api` (entityType: Service) — observations: endpoint, auth, rate limits summary
- `tinvest-go-sdk` (entityType: Service) — observations: version, repo, key classes
- `tinvest-sandbox` (entityType: Decision) — observations: как работает, ограничения

Relations:
- `24alert` --`uses`--> `tinvest-api`
- `24alert` --`uses`--> `tinvest-go-sdk`

### Data files
- `.tasks/TASK-002/analyst/data/rate-limits.json` — таблица rate limits по методам
- `.tasks/TASK-002/analyst/data/api-methods.json` — полный список методов с параметрами

---

## 3. Handoff

Создай `.tasks/TASK-002/analyst/handoff.md` с:
- Полная таблица rate limits по методам
- SDK API surface (ключевые классы/функции)
- Sandbox особенности
- Рекомендации для бэкенда (что использовать из SDK, что писать руками)
- Risks/gotchas (что может пойти не так)

**Критерий успеха**: бэкенд-разработчик может начать писать код, не читая документацию T-Invest самостоятельно.
