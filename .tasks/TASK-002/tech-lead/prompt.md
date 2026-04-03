# Промпт: Техлид → TASK-002

## Контекст
Ты — **Техлид (Senior Architect)**. Задача — провести архитектурный review всего торгового робота: код, контракты, безопасность, production-readiness.

**Исходная постановка**: `.tasks/TASK-002/task.md`
**План**: `.tasks/TASK-002/plan.md`
**Handoff бэкенда**: `.tasks/TASK-002/backend/handoff.md`
**Handoff DevOps**: `.tasks/TASK-002/devops/handoff.md`
**Handoff тестировщика**: `.tasks/TASK-002/tester/handoff.md`
**Handoff аналитика**: `.tasks/TASK-002/analyst/handoff.md`

---

## Что проверять

### 1. Архитектура (gRPC microservices)
- [ ] Разделение ответственности: каждый сервис делает одно дело
- [ ] gRPC контракты: proto-файлы консистентны, backward-compatible
- [ ] Нет циклических зависимостей между сервисами
- [ ] Strategy plugin: интерфейс достаточно гибкий для реальных стратегий
- [ ] Risk-service не вызывает T-Invest напрямую (только через portfolio/marketdata)

### 2. Код (Go quality)
- [ ] Idiomatic Go: error handling (wrap + context), нет panic в production code
- [ ] Context propagation: ctx передаётся во все вызовы
- [ ] Graceful shutdown: все сервисы корректно завершаются по SIGTERM
- [ ] Race conditions: goroutines для стримов защищены (mutex / channels)
- [ ] No hardcoded values: всё через config.yaml + env vars

### 3. Rate Limiting
- [ ] Каждый вызов T-Invest API проходит через rate limiter
- [ ] Лимиты соответствуют документации (из analyst handoff)
- [ ] При RESOURCE_EXHAUSTED — graceful backoff, не crash
- [ ] Стримы: не превышают лимит подключений (16 streams, 300 subs)

### 4. Безопасность
- [ ] TINVEST_TOKEN не логируется, не коммитится, не попадает в swagger
- [ ] .env в .gitignore
- [ ] gRPC между сервисами: пока без TLS (internal network), но готово к добавлению
- [ ] Идемпотентность: order_id (UUID) предотвращает дублирование заявок
- [ ] Input validation: все входные параметры валидируются

### 5. Observability
- [ ] Structured logging (slog, JSON format)
- [ ] Correlation ID проходит через все сервисы
- [ ] Health check endpoints для каждого сервиса
- [ ] Order journal: все заявки и исполнения записываются

### 6. Docker & Deployment
- [ ] Multi-stage build (маленькие образы)
- [ ] docker-compose: depends_on, health checks, volumes
- [ ] Конфигурация через env vars (12-factor)

### 7. Тесты
- [ ] Unit test coverage для service.go каждого сервиса
- [ ] Integration tests покрывают happy path + negative cases
- [ ] Sandbox mode работает корректно

---

## Красные флаги (блокеры)
- Token leakage (логирование, swagger, git history)
- Отсутствие rate limiter на любом T-Invest вызове
- Panic в production code
- Отсутствие graceful shutdown
- Циклические зависимости между сервисами

## Handoff
Создай `.tasks/TASK-002/tech-lead/handoff.md`:
- APPROVED / NEEDS_CORRECTION
- Список замечаний (critical / warning / recommendation)
- Рекомендации для Phase 2 (что улучшить)
