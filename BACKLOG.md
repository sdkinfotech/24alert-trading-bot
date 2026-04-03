# Product Backlog — 24alert Trading Bot

**Last Updated**: 2026-04-03  
**Version**: 1.0

---

## Backlog Status

| ID | Title | Priority | Complexity | Status | Phase |
|---|-------|----------|-----------|--------|-------|
| TASK-004 | CI/CD Pipeline (GitHub Actions) | HIGH | M | Done | Phase 2 |
| TASK-005 | Kubernetes Migration & Scaling | HIGH | XL | Planned | Phase 2 |
| TASK-006 | Monitoring, Logging & Alerting | HIGH | L | Planned | Phase 2 |
| TASK-007 | Real Trading Strategies (Alpha) | MEDIUM | XL | Backlog | Phase 3 |
| TASK-008 | Strategy Plugin Marketplace | MEDIUM | XXL | Backlog | Phase 3 |
| TASK-009 | Backtesting Engine | MEDIUM | L | Backlog | Phase 3 |
| TASK-010 | User Management & Multi-Account | LOW | L | Backlog | Phase 3 |
| TASK-011 | Web Dashboard (React) | LOW | L | Backlog | Phase 3 |
| TASK-012 | Mobile App (React Native) | LOW | XXL | Backlog | Phase 4 |
| TASK-013 | Advanced Risk Management | MEDIUM | M | Backlog | Phase 3 |
| TASK-014 | Database Migration (PostgreSQL) | HIGH | M | Backlog | Phase 2 |
| TASK-015 | Performance Optimization | MEDIUM | M | Backlog | Phase 2 |
| TASK-016 | Security Audit & Hardening | HIGH | M | Backlog | Phase 2 |
| TASK-017 | Disaster Recovery & Backup | HIGH | L | Backlog | Phase 2 |

---

## Phase 2 (Current: Stabilization & Scale)

### TASK-004: CI/CD Pipeline (GitHub Actions)
**Priority**: HIGH | **Complexity**: M | **Effort**: 3-5 дней

**Description**: Полностью автоматизированный CI/CD пайплайн. При `git push origin main`:
1. Tests + linting (Go)
2. Docker build & push
3. SSH deploy на prod-сервер
4. Health checks + smoke tests
5. Slack notifications

**Dependencies**: TASK-003 (deployment готов)

**Roles**: Backend (tests), DevOps (workflow), Tester (e2e), Tech-lead (review)

---

### TASK-005: Kubernetes Migration & Scaling
**Priority**: HIGH | **Complexity**: XL | **Effort**: 2 недели

**Description**: Миграция с docker-compose на Kubernetes для масштабирования и надёжности.

**Scope**:
- K8s manifests (Deployments, Services, ConfigMaps, Secrets)
- Helm charts для версионирования
- Auto-scaling (HPA) для микросервисов
- Persistent volumes для данных
- Ingress для маршрутизации

**Dependencies**: TASK-004 (CI/CD)

**Roles**: DevOps (K8s), Backend (optimizations), Tech-lead (architecture)

---

### TASK-006: Monitoring, Logging & Alerting
**Priority**: HIGH | **Complexity**: L | **Effort**: 1 неделя

**Description**: Production-ready мониторинг и алертинг.

**Scope**:
- Prometheus metrics (для всех 5 сервисов)
- Grafana dashboards
- ELK Stack (Elasticsearch + Kibana) для логов
- AlertManager для уведомлений
- SLA tracking

**Dependencies**: TASK-003 (production running)

**Roles**: DevOps (infrastructure), Backend (metrics), Tech-lead (alerts)

---

### TASK-014: Database Migration (PostgreSQL)
**Priority**: HIGH | **Complexity**: M | **Effort**: 5-7 дней

**Description**: Добавить PostgreSQL для хранения истории заявок, портфеля, логов вместо in-memory.

**Scope**:
- Database schema (orders, positions, operations, logs)
- Migrations (schema versioning)
- Connection pooling (pgBouncer)
- Backup & restore strategy
- Performance tuning

**Dependencies**: TASK-003

**Roles**: Backend (schema), DevOps (infra), Tech-lead (design)

---

### TASK-016: Security Audit & Hardening
**Priority**: HIGH | **Complexity**: M | **Effort**: 1 неделя

**Description**: Security audit и hardening всей системы.

**Scope**:
- OWASP Top 10 review
- API authentication (JWT)
- Rate limiting enhancement
- Secret rotation
- Network segmentation
- Penetration testing

**Dependencies**: TASK-003

**Roles**: Tech-lead (audit), Backend (implementation), DevOps (infra)

---

### TASK-017: Disaster Recovery & Backup
**Priority**: HIGH | **Complexity**: L | **Effort**: 3-5 дней

**Description**: Disaster recovery plan и автоматические бэкапы.

**Scope**:
- Database backups (daily + continuous)
- State snapshots
- Failover mechanism
- RTO/RPO targets
- Restore testing

**Dependencies**: TASK-014 (database)

**Roles**: DevOps (backups), Tech-lead (planning)

---

### TASK-015: Performance Optimization
**Priority**: MEDIUM | **Complexity**: M | **Effort**: 1 неделя

**Description**: Оптимизация производительности на основе метрик из TASK-006.

**Scope**:
- Go code profiling (CPU, memory)
- Database query optimization
- Caching strategy (Redis)
- Connection pooling
- API response time targets

**Dependencies**: TASK-006 (metrics)

**Roles**: Backend (optimization), DevOps (profiling)

---

## Phase 3 (Growth: Features & Expansion)

### TASK-007: Real Trading Strategies (Alpha)
**Priority**: MEDIUM | **Complexity**: XL | **Effort**: 3-4 недели

**Description**: Разработка первых реальных торговых стратегий.

**Scope**:
- Technical Analysis indicators (MA, RSI, MACD, etc.)
- Signal generation logic
- Risk-adjusted position sizing
- Strategy backtesting
- Live paper trading

**Dependencies**: TASK-005 (K8s for scaling)

**Roles**: Backend (algorithm), Analyst (research), Tester (validation)

---

### TASK-008: Strategy Plugin Marketplace
**Priority**: MEDIUM | **Complexity**: XXL | **Effort**: 4-6 недель

**Description**: Marketplace для публикации и использования пользовательских стратегий.

**Scope**:
- Strategy versioning & management
- Community rating & reviews
- Strategy execution sandbox
- Revenue sharing model
- Plugin store UI

**Dependencies**: TASK-007 (strategies), TASK-011 (dashboard)

**Roles**: Full-stack (everyone)

---

### TASK-009: Backtesting Engine
**Priority**: MEDIUM | **Complexity**: L | **Effort**: 2-3 недели

**Description**: Встроенный бэктестинг для валидации стратегий на исторических данных.

**Scope**:
- Historical data ingestion (T-Invest API)
- Backtesting simulation engine
- Performance metrics (Sharpe, Sortino, DD, etc.)
- Optimization framework
- Report generation

**Dependencies**: TASK-007 (strategies)

**Roles**: Backend (engine), Analyst (metrics)

---

### TASK-013: Advanced Risk Management
**Priority**: MEDIUM | **Complexity**: M | **Effort**: 1-2 недели

**Description**: Продвинутый риск-менеджмент.

**Scope**:
- Portfolio-level risk controls
- Correlation matrix
- VaR (Value at Risk)
- Stress testing
- Scenario analysis

**Dependencies**: TASK-006 (metrics)

**Roles**: Backend (logic), Analyst (models)

---

### TASK-010: User Management & Multi-Account
**Priority**: LOW | **Complexity**: L | **Effort**: 1 неделя

**Description**: Поддержка нескольких пользователей и счётов.

**Scope**:
- User registration & authentication (JWT)
- Multi-account support per user
- Permission model (read/write/admin)
- Audit logging
- Account switching

**Dependencies**: TASK-016 (security)

**Roles**: Backend (auth), Frontend (UI)

---

### TASK-011: Web Dashboard (React)
**Priority**: LOW | **Complexity**: L | **Effort**: 2-3 недели

**Description**: Web UI для monitoring и control trading bot.

**Scope**:
- Real-time portfolio view
- Order history & management
- Strategy management
- Performance charts
- Alerts & notifications

**Dependencies**: TASK-006 (metrics), TASK-010 (auth)

**Roles**: Frontend (React), Backend (API)

---

## Phase 4 (Long-term: Enterprise)

### TASK-012: Mobile App (React Native)
**Priority**: LOW | **Complexity**: XXL | **Effort**: 2-3 месяца

**Description**: Native iOS/Android приложение.

**Dependencies**: TASK-011 (web dashboard), TASK-010 (auth)

**Roles**: Mobile (React Native), Backend (API enhancements)

---

## Adding Tasks to Backlog

**Процесс для планировщика**:

1. **Create new task**
   ```
   TASK-NNN/task.md (1-2 страницы)
   - Description
   - Requirements
   - DoD
   - Dependencies
   - Priority
   ```

2. **Update backlog** (этот файл)
   - Добавить строку в таблицу
   - Обновить фазу

3. **When ready to start**
   - Изменить Status на "In Progress"
   - Запустить планировщика (create plan.md, prompts)

4. **After completion**
   - Изменить Status на "Done"
   - Link handoff'ы

---

## Prioritization Matrix

```
HIGH:   Critical for MVP, platform stability, security, performance
MEDIUM: Nice-to-have features, enhancements
LOW:    Future, nice ideas, community features
```

**Complexity**:
- S (Small): 1-2 дней
- M (Medium): 3-7 дней
- L (Large): 1-2 недели
- XL (XL): 2-4 недели
- XXL (Huge): 1+ месяца

---

## Velocity & Timeline

**Current**: TASK-001–003 completed (3 tasks, ~4 недели)

**Next 3 months**:
- TASK-004 (CI/CD): 1 неделя
- TASK-005 (K8s): 2 недели
- TASK-006 (Monitoring): 1 неделя
- TASK-014 (Database): 1 неделя
- TASK-016 (Security): 1 неделя

**Estimated: Stabilization phase = 6-8 недель**

---

## Notes

- Backlog может меняться в зависимости от приоритетов
- Новые задачи добавляются по мере появления требований
- Dependencies должны быть проверены перед стартом задачи
- Все задачи следуют конвейеру ролей (Planner → Backend/DevOps → Tester → Tech-lead)
