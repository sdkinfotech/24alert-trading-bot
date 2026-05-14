# Roadmap 24alert Trading Bot

> **Дата создания**: 2026-04-17
> **Текущая фаза**: Phase 2 — Stabilization & Hardening

---

## Фазы

### Phase 1: MVP (Завершена ✅)
- Сервер, микросервисы, деплой, smoke-тесты
- TASK-001..TASK-006

### Phase 2: Stabilization & Hardening (Текущая 🔄)
Критические проблемы production-системы.

| ID | Название | Приоритет | Сложность | Статус |
|----|----------|:---------:|:---------:|--------|
| TASK-007 | Закрытие портов микросервисов наружу | 🔴 CRITICAL | S | Planned |
| TASK-008 | Авторизация на REST API (JWT middleware) | 🔴 CRITICAL | M | Planned |
| TASK-009 | Разделение .env и конфигурации по сервисам | 🟠 HIGH | S | Planned |
| TASK-010 | Swap, мониторинг диска, OOM-защита | 🟠 HIGH | S | Planned |
| TASK-011 | Healthcheck между сервисами | 🟡 MEDIUM | S | Planned |
| TASK-012 | Автоматический деплой (GitHub Actions) | 🟡 MEDIUM | M | Planned |
| TASK-013 | Rate limiting улучшение (burst, adaptive) | 🟡 MEDIUM | M | Planned |

### Phase 3: Data & Persistence

| ID | Название | Приоритет | Сложность | Статус |
|----|----------|:---------:|:---------:|--------|
| TASK-014 | PostgreSQL миграция (история ордеров, портфель) | 🔴 CRITICAL | XL | Planned |
| TASK-015 | Redis кэш для маркет-данных | 🟡 MEDIUM | M | Planned |
| TASK-016 | Backup & Disaster Recovery | 🟠 HIGH | M | Planned |

### Phase 4: Trading Features

| ID | Название | Приоритет | Сложность | Статус |
|----|----------|:---------:|:---------:|--------|
| TASK-017 | Торговые стратегии (MA, RSI, Mean Reversion) | 🟡 MEDIUM | XL | Planned |
| TASK-018 | Strategy Plugin (внешний gRPC) | 🟡 MEDIUM | XL | Planned |
| TASK-019 | Backtesting Engine | 🟡 MEDIUM | L | Planned |

### Phase 5: Scaling & Ops

| ID | Название | Приоритет | Сложность | Статус |
|----|----------|:---------:|:---------:|--------|
| TASK-020 | Kubernetes миграция | 🟠 HIGH | XXL | Planned |
| TASK-021 | Horizontal Scaling gateway | 🟡 MEDIUM | L | Planned |
| TASK-022 | AlertManager (Telegram, Slack) | 🟡 MEDIUM | S | Planned |
| TASK-023 | Multi-account support | 🟡 MEDIUM | M | Planned |

---

## Метрики прогресса

| Фаза | Всего задач | Done | In Progress | Осталось |
|------|:-----------:|:----:|:-----------:|:--------:|
| Phase 1 (MVP) | 6 | 6 | 0 | 0 |
| Phase 2 (Hardening) | 7 | 0 | 0 | 7 |
| Phase 3 (Data) | 3 | 0 | 0 | 3 |
| Phase 4 (Trading) | 3 | 0 | 0 | 3 |
| Phase 5 (Scaling) | 4 | 0 | 0 | 4 |
| **Итого** | **23** | **6** | **0** | **17** |