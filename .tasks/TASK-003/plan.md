# План: TASK-003 — Развёртывание и smoke testing

## Цель
Развернуть торгового робота на production-сервере и провести ручное тестирование через CLI, REST API и Swagger. Убедиться, что система готова для реального использования.

## Scope / Out of scope

### In scope
- SSH на srv03-cloud, git clone проекта
- Настройка .env с production T-Invest ключом
- `make docker-build && docker-compose up -d`
- Health checks всех сервисов
- Smoke tests: CLI и curl
- Документирование deployment guide

### Out of scope
- Автоматизация deployment (CI/CD) — TASK-004
- Kubernetes — TASK-005
- Мониторинг/Alerting (детально) — TASK-006
- Резервное копирование состояния

## Порядок ролей

```
DevOps → Тестировщик → Техлид
```

Только 3 роли (бэкенд уже готов).

| Роль | Промпт | Что делает | Когда |
|------|--------|-----------|-------|
| **DevOps** | `devops/prompt.md` | Развёртывание на prod, конфигурация, health checks | Первый |
| **Тестировщик** | `tester/prompt.md` | Smoke tests (CLI, curl, Swagger) | После DevOps |
| **Техлид** | `tech-lead/prompt.md` | Review процесса, sign-off | После тестов |

## Риски

| Риск | Вероятность | Влияние | Митигация |
|------|------------|---------|----------|
| T-Invest API ключ неверный/истёк | MEDIUM | HIGH | DevOps проверяет ключ до запуска |
| Недостаточно места на диске | LOW | HIGH | Проверить свободное место (24 GB есть) |
| Порт 8080 занят | LOW | MEDIUM | Использовать другой порт / проверить netstat |
| Контейнеры не стартуют | MEDIUM | HIGH | Логи + обращение к бэкенду за debuggingом |
