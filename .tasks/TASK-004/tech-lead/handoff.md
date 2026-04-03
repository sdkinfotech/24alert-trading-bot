# Handoff: Техлид → TASK-004

## Статус
**NEEDS_CORRECTION** ⚠️

---

## Executive Summary

CI/CD pipeline реализован и частично работает. Backend quality хороший (76.5% coverage, lint clean). DevOps проделал серьёзную работу: исправил `.gitignore`, починил TLS (Russian CA), поднял production. Однако при детальном review найдены **3 архитектурных бага** в deployment pipeline, которые делают rollback **нерабочим** и создают security risk.

**Решение**: NEEDS_CORRECTION — требуется исправить перед production sign-off.

### Верификация production (SSH проверка 2026-04-03)

| Компонент | Статус | Доказательство |
|-----------|--------|----------------|
| Docker 28.2.2 | ✅ Установлен | `docker --version` |
| Docker Compose v5.1.1 | ✅ Установлен | `docker compose version` |
| Git repo | ✅ `/opt/24alert` | `git log --oneline -10` |
| `.env` | ✅ Создан | `ls -la /opt/24alert/deployments/.env` |
| gateway (8080) | ✅ Running | `curl http://localhost:8080/health → 200 OK {"status":"ok"}` |
| order-svc (9001) | ✅ Running (healthy) | `docker ps` |
| marketdata-svc (9002) | ✅ Running (healthy) | `docker ps` |
| portfolio-svc (9003) | ✅ Running (healthy) | `docker ps` |
| risk-svc (9004) | ✅ Running (healthy) | `docker ps` |

**Примечание**: Gateway помечен `unhealthy` из-за бага в health check (Docker использует `HEAD`, сервер отвечает `405`). По `GET` всё OK. Требуется fix в docker-compose.yaml.

---

## 1. Security Review

### ✅ PASSED

| Проверка | Статус | Детали |
|----------|--------|--------|
| Secrets в GitHub Secrets | ✅ PASS | Только `${{ secrets.* }}` в workflow |
| SSH key cleanup | ✅ PASS | `rm -f ~/.ssh/deploy_key` в `if: always()` |
| Slack webhook guard | ✅ PASS | `if: env.SLACK_WEBHOOK != ''` |
| .env не в git | ✅ PASS | В .gitignore |
| Concurrency | ✅ PASS | `production-deploy`, `cancel-in-progress: false` |
| Key rotation docs | ✅ PASS | DEPLOYMENT.md, 90 дней |

### ❌ FAILED

#### SEC-001: `StrictHostKeyChecking=no` в deploy скриптах (MEDIUM)

**Файлы**: `scripts/deploy-prod.sh`, `scripts/rollback-prod.sh`

```bash
ssh -i "$DEPLOY_KEY" -o ConnectTimeout=10 -o StrictHostKeyChecking=no "$USER@$HOST" "$cmd"
```

**Проблема**: MITM-атака возможна. Workflow делает `ssh-keyscan` → `known_hosts`, но скрипт его игнорирует.

**Fix**: Убрать `-o StrictHostKeyChecking=no`, использовать `known_hosts`.

---

## 2. Architecture Review — CRITICAL FINDINGS

### ❌ BUG-ARCH-001: Rollback НЕ РАБОТАЕТ (CRITICAL)

**Deploy** = `git pull` → `docker-compose build` (локально на сервере)
**Rollback** = `docker pull` из registry → `docker-compose up`

Но `docker-compose.yaml` имеет `build:` директиву → при `up` пересоберёт **текущий (broken) код**, а не использует pulled image. Rollback фактически делает то же самое что deploy.

**Fix (рекомендация — git-based rollback)**:
```bash
git checkout HEAD~1
docker-compose build
docker-compose up -d
```

### ❌ BUG-ARCH-002: Только gateway image пушится в Docker Hub (MAJOR)

Из 5 сервисов только gateway тегируется и пушится. Остальные 4 — нет.

**Fix**: Пушить все 5 образов или убрать push целиком (git-based стратегия).

### ⚠️ BUG-ARCH-003: `steps.meta.outputs.version` self-reference (MINOR)

Output `image_version` ссылается на `steps.meta.outputs.version` внутри того же step → будет пустой.

### ⚠️ BUG-HEALTH-001: Gateway healthcheck использует HEAD (NEW)

Docker health check делает `HEAD /health` → сервер отвечает `405`. Контейнер помечен `unhealthy` хотя сервис работает.

**Fix**: В `docker-compose.yaml` заменить `wget` на `curl -f http://localhost:8080/health`.

---

## 3. Best Practices Review

### ✅ PASSED

| Проверка | Статус |
|----------|--------|
| `set -e` в bash скриптах | ✅ |
| Input validation | ✅ |
| Health check retry (5x, 2s) | ✅ |
| Deploy timeout (5 min) | ✅ |
| Meaningful logging | ✅ |
| Makefile CI targets | ✅ |
| Coverage threshold 70% | ✅ |

### ⚠️ WARNINGS

- **WARN-001**: docker-compose path hardcoded (проверяет `docker-compose.yaml`, реальный путь `deployments/docker-compose.yaml`)
- **WARN-002**: Build on server = slow (2 CPU, 3.8 GB RAM)
- **WARN-003**: DEPLOYMENT.md упоминает Go 1.25, workflow — Go 1.23

---

## 4. Backend Review — ✅ APPROVED

| Метрика | Значение | Статус |
|---------|----------|--------|
| Unit tests | 10 пакетов, 0 failures | ✅ |
| Coverage | 76.5% avg | ✅ |
| golangci-lint | 0 errors | ✅ |
| go build | 0 errors | ✅ |

---

## 5. DevOps Review — ✅ Отличная работа

DevOps нашёл и исправил **8 реальных проблем**:
1. `.gitignore` блокировал исходный код
2. `go.sum` отсутствовал
3. `git` отсутствовал в Alpine
4. Russian CA для TLS (debian-slim + MinTsifry CA)
5. `netcat` для health checks
6. Docker Compose plugin
7. `make` — установлен
8. `/opt/24alert` — создан и развёрнут

**5 коммитов** с осмысленными fix messages. Профессиональная работа.

Требуется доработка: BUG-ARCH-001, BUG-ARCH-002, BUG-HEALTH-001.

---

## 6. DoD — статус

| Критерий | Статус |
|----------|--------|
| All tests pass | ✅ PASS |
| Coverage >= 70% | ✅ PASS (76.5%) |
| golangci-lint clean | ✅ PASS |
| Docker images build | ✅ PASS (5 образов на сервере) |
| Images pushed to registry | ⚠️ PARTIAL (только gateway) |
| Workflow executes e2e | ❌ NOT VERIFIED |
| Deploy via SSH | ✅ PASS (ручной deploy работает) |
| Health checks pass (GET) | ✅ PASS (5/5 respond) |
| Rollback tested | ❌ BROKEN (BUG-ARCH-001) |
| Slack notifications | ❌ NOT VERIFIED |
| No secrets in logs | ✅ PASS |
| Idempotent deploy | ❌ NOT VERIFIED |
| Documentation | ✅ PASS |

**Итого**: 8/13 PASS, 3 NOT VERIFIED, 1 BROKEN, 1 PARTIAL

---

## 7. Решение

### ⚠️ NEEDS_CORRECTION

**Обязательные исправления (блокеры)**:

1. **BUG-ARCH-001 (CRITICAL)**: Исправить rollback → git-based
2. **BUG-ARCH-002 (MAJOR)**: Пушить все 5 образов или git-based стратегия целиком
3. **BUG-HEALTH-001 (MAJOR)**: Fix gateway healthcheck (HEAD → GET)

**Рекомендуемые (не блокеры)**:

4. SEC-001: Убрать `StrictHostKeyChecking=no`
5. BUG-ARCH-003: Fix self-reference в meta outputs
6. WARN-001: Fix path в pre-check
7. WARN-003: Go version в DEPLOYMENT.md

### Действия:

**DevOps**: Исправить пункты 1-3 (блокеры) + желательно 4-7
**Tester**: После fix — проверить rollback на production, e2e workflow
**Tech Lead**: Финальный sign-off после исправлений

---

**Дата**: 2026-04-03
**Решение**: NEEDS_CORRECTION → исправить rollback + image push + healthcheck
**Следующий шаг**: DevOps исправляет 3 блокера, затем повторный review
