# План: TASK-002 — Базовый торговый робот (Go, микросервисы)

## Цель
Go-монорепо с 5 микросервисами (order, marketdata, portfolio, risk, gateway), общими proto-контрактами, CLI и Swagger. Стратегия подключается как плагин через gRPC-интерфейс. Биржевой API — T-Invest (Тинькофф Инвестиции).

## Scope / Out of scope

### In scope
- Scaffold монорепо (Go workspace, Makefile, config)
- Proto-контракты для всех сервисов + strategy plugin
- Shared пакеты (config, tinvest SDK wrapper, rate-limiter, types, logging, idempotency)
- 5 микросервисов: order, marketdata, portfolio, risk, gateway
- CLI (cobra) с полным набором команд
- Swagger UI для REST API
- Docker Compose для локального запуска
- README с архитектурой, quick start, CLI reference

### Out of scope
- Реализация конкретной стратегии (только интерфейс + mock)
- Frontend UI (только CLI + Swagger)
- Production deployment на сервер (отдельный TASK)
- NATS event bus (опционально, Phase 2)
- Prometheus метрики (заглушка, детальная реализация — Phase 2)
- Бэктестинг

## Порядок ролей в конвейере

```
Аналитик ✅ ────── (исследование T-Invest API) ─────────────────┐
                                                                │
Планировщик (ВЫ) → Бэкенд ✅ (Phase 1→2→3) → DevOps ✅ → Тестировщик ✅ → Техлид ✅ ←──┘
```

## Фазы и зависимости

| Фаза | Роль | Что делается | Зависит от | Размер |
|------|------|-------------|-----------|--------|
| 0 | **Аналитик** | Исследование T-Invest API, SDK, rate limits, sandbox; выводы в Wiki + Memory | — | M |
| 1 | **Бэкенд** | Scaffold + proto + pkg (shared) | Фаза 0 (findings) | M |
| 2 | **Бэкенд** | 4 core сервиса: order, marketdata, portfolio, risk | Фаза 1 | XL |
| 3 | **Бэкенд** | Gateway (REST + Swagger) + CLI (cobra) | Фаза 2 | L |
| 4 | **DevOps** | Dockerfiles + docker-compose + Makefile targets | Фаза 1 (scaffold) | M |
| 5 | **Тестировщик** | Интеграционные тесты: order flow, risk validation | Фаза 2 + 3 | M |
| 6 | **Техлид** | Архитектурный review, code quality, security | Фаза 2 + 3 + 4 | M |

### Критический путь
```
Аналитик(0) → Бэкенд(1) → Бэкенд(2) → Бэкенд(3) → Тестировщик(5) → Техлид(6)
                              ↓
                          DevOps(4) [параллельно с Phase 2-3]
```

## Роли

### Бэкенд — основная работа
Реализует весь Go-код: scaffold, proto, shared пакеты, 5 сервисов, CLI.
Подробный промпт: `.tasks/TASK-002/backend/prompt.md`

### DevOps — контейнеризация
Dockerfiles для каждого сервиса, docker-compose.yaml, Makefile targets.
Может начать после Phase 1 (scaffold готов).
Подробный промпт: `.tasks/TASK-002/devops/prompt.md`

### Тестировщик — интеграционные тесты
Тестирование order flow end-to-end, risk validation, CLI smoke tests.
Начинает после Phase 3 (gateway + CLI готовы).
Подробный промпт: `.tasks/TASK-002/tester/prompt.md`

### Техлид — review
Архитектурный review, code quality, security, gRPC contract consistency.
Начинает после всех предыдущих ролей.
Подробный промпт: `.tasks/TASK-002/tech-lead/prompt.md`

### Аналитик — исследование
T-Invest API docs, SDK source, rate limits, sandbox behaviour; выводы в Wiki + Memory.
Работает параллельно, начинает первым.
Подробный промпт: `.tasks/TASK-002/analyst/prompt.md`

### Фронтенд — НЕ ТРЕБУЕТСЯ
В этой задаче фронтенд не нужен. Только CLI + Swagger.

## Риски

| Риск | Вероятность | Влияние | Митигация |
|------|------------|---------|----------|
| T-Invest API rate limits жёстче, чем ожидалось | MEDIUM | HIGH | Rate limiter per method + sandbox тестирование |
| gRPC streaming сложнее, чем unary calls | MEDIUM | MEDIUM | Начать с unary, стримы — последними |
| Большой объём кода (5 сервисов) | HIGH | MEDIUM | Фазирование: scaffold → core → gateway → CLI |
| Sandbox API отличается от production | LOW | HIGH | Аналитик исследует до начала кода |
