# Plan: TASK-004 — CI/CD Pipeline (GitHub Actions)

**Date Created**: 2026-04-03  
**Created by**: Planner  
**Status**: Ready to Execute

---

## Goal (1 Sentence)

Полностью автоматизированный GitHub Actions workflow для zero-click deployment: тесты → Docker build → production deploy → health checks → rollback on failure.

---

## Scope & Out of Scope

### In Scope
- ✅ GitHub Actions workflow (.github/workflows/deploy.yml)
- ✅ Testing pipeline (go test, golangci-lint, coverage)
- ✅ Docker build & push to registry
- ✅ SSH deployment to srv03-cloud
- ✅ Health checks & validation
- ✅ Rollback mechanism
- ✅ Slack notifications
- ✅ GitHub Secrets management
- ✅ Makefile CI targets (local simulation)

### Out of Scope
- ❌ Kubernetes (→ TASK-005)
- ❌ Multi-environment branching (v2 feature)
- ❌ Canary/Blue-Green deployment (→ TASK-005)
- ❌ Private Docker registry (use GitHub Packages)
- ❌ GitHub Actions cost optimization

---

## Role Execution Order & Dependencies

```
Backend (1 day)
    ↓ — creates Makefile targets, test config
DevOps (2 days)
    ↓ — creates workflow, deploy scripts, secrets setup
Tester (1 day)
    ↓ — validates e2e flow, rollback, notifications
Tech-Lead (0.5 day)
    ↓ — security review, production readiness
```

**Critical Path**: Backend → DevOps → Tester → Tech-Lead  
**No parallel tracks** (each depends on previous)

---

## Role Assignments & Prompts

### 1️⃣ Backend (1 day)
**File**: `.tasks/TASK-004/backend/prompt.md`

**What to deliver**:
- `go test -v -cover ./...` in GitHub Actions
- `golangci-lint` configuration (.golangci.yml)
- `Makefile` targets: `ci-test`, `ci-check`, `ci-build`
- `scripts/ci-test.sh` (local test runner)
- Code coverage threshold: 70% minimum (enforced in workflow)

**Success Criteria**:
- ✅ All unit tests pass
- ✅ Coverage >= 70%
- ✅ Linting clean (no errors/warnings)
- ✅ Makefile targets work locally
- ✅ Handoff specifies how to run tests locally

### 2️⃣ DevOps (2 days)
**File**: `.tasks/TASK-004/devops/prompt.md`

**What to deliver**:
- `.github/workflows/deploy.yml` (complete workflow YAML)
- `scripts/deploy-prod.sh` (deployment script for srv03-cloud)
- `scripts/rollback-prod.sh` (rollback script)
- GitHub Secrets setup documentation
- Timeout & retry logic
- Health check curl commands
- `.github/DEPLOYMENT.md` (runbook for troubleshooting)

**Success Criteria**:
- ✅ Workflow triggers on `push main`
- ✅ Tests run → pass before build
- ✅ Docker images build and push successfully
- ✅ SSH deploy completes < 2 minutes
- ✅ Health check validates all services
- ✅ Rollback tested manually (simulated failure → recovery)
- ✅ Slack webhook sends notifications
- ✅ No secrets in logs

### 3️⃣ Tester (1 day)
**File**: `.tasks/TASK-004/tester/prompt.md`

**What to validate**:
- E2E test: push commit → workflow runs → deploy succeeds
- Smoke tests: API endpoints respond (200 OK)
- Rollback test: simulate deploy failure → previous version restored
- Slack notification test: check message format & webhook timing
- Log validation: no tokens/secrets leaked

**Test Cases** (10+):
1. ✅ Push to main triggers workflow
2. ✅ Tests run and pass
3. ✅ Code coverage computed
4. ✅ Docker images built
5. ✅ Images tagged correctly
6. ✅ Deploy via SSH succeeds
7. ✅ All 5 services healthy after deploy
8. ✅ Rollback on simulated failure
9. ✅ Slack notification sent
10. ✅ No secrets visible in logs

**Success Criteria**:
- ✅ All 10+ tests pass
- ✅ E2E workflow executes end-to-end (push → prod)
- ✅ Handoff specifies test commands

### 4️⃣ Tech-Lead (0.5 day)
**File**: `.tasks/TASK-004/tech-lead/prompt.md`

**What to review**:
- Security: no hardcoded secrets, GitHub Secrets best practices
- Best practices: idempotency, timeout/retry logic, error handling
- Workflow structure: clarity, maintainability
- Deployment strategy: safety, rollback mechanism
- Documentation: clarity for DevOps on-call

**Sign-off criteria**:
- ✅ No security risks
- ✅ Idempotent deployment (deploy twice = same state)
- ✅ Timeout logic sound (< 10 min total)
- ✅ Rollback reliable
- ✅ Documentation complete

---

## Key Technical Decisions

| Decision | Rationale |
|----------|-----------|
| GitHub Actions (not GitLab CI, Jenkins) | Free, integrated with GitHub, good matrix support |
| Docker Hub registry | Free tier, simplicity (vs self-hosted) |
| SSH key auth (not token) | More secure for deploy key (rotatable) |
| Health check = HTTP GET /health | Simple, fast validation |
| Rollback = restart previous image | Atomic, reliable (vs complex state management) |
| Slack webhook (not email) | Real-time, visible to team |
| Makefile CI targets | Local dev → CI parity (can test locally) |

---

## Risks & Mitigations

| Risk | Probability | Impact | Mitigation |
|------|---|---|---|
| GitHub Actions quota exceeded | LOW | Deploy blocked | Monitor usage, use self-hosted runner if needed |
| SSH timeout to srv03-cloud | MEDIUM | Deploy fails, manual intervention | 5 min timeout, retry logic, Slack alert + docs |
| Deploy key compromised | MEDIUM | Security breach | Key rotation weekly, GitHub log audit monthly |
| Health check false positive | MEDIUM | Rollback on bad health | Multiple endpoints, verbose logging, retry logic |
| Previous version unreachable | LOW | Rollback fails, manual recovery | Keep 2 stable images tagged, docs for manual recovery |
| Slack webhook expires | LOW | Notifications silent | Test webhook monthly, document renewal process |
| Network partition mid-deploy | LOW | Inconsistent state | Idempotent scripts, state file validation |
| Docker image push fails | MEDIUM | Deploy skipped, undetected | Push retry logic, explicit error handling |

---

## Timeline

| Phase | Owner | Days | Deliverable | Notes |
|-------|-------|------|---|---|
| Backend | Backend | 1 | Makefile + test config | Can start immediately |
| DevOps | DevOps | 2 | Workflow + deploy scripts | Depends on Backend |
| Tester | Tester | 1 | E2E tests + validation | Depends on DevOps |
| Tech-Lead | Tech-Lead | 0.5 | Sign-off + docs | Depends on Tester |
| Buffer | — | 0.5 | Fixes, re-tests | Contingency |
| **TOTAL** | — | **5 days** | Production-ready CI/CD | Start date: 2026-04-03 |

**Estimated Completion**: 2026-04-08 (5 calendar days, assuming parallel where possible)

---

## Artifacts to Create

- `.github/workflows/deploy.yml` — GitHub Actions workflow
- `.github/DEPLOYMENT.md` — deployment runbook
- `.golangci.yml` — linting configuration
- `Makefile` — updated with ci-* targets
- `scripts/ci-test.sh` — test runner script
- `scripts/deploy-prod.sh` — deployment script
- `scripts/rollback-prod.sh` — rollback script
- `.tasks/TASK-004/backend/handoff.md` — Backend deliverables
- `.tasks/TASK-004/devops/handoff.md` — DevOps deliverables
- `.tasks/TASK-004/tester/handoff.md` — Tester results
- `.tasks/TASK-004/tech-lead/handoff.md` — Tech-Lead approval

---

## Dependencies

**External**:
- ✅ TASK-003 completed (production running)
- ✅ GitHub repo created (tinvest-api-bot)
- ✅ Docker Hub account (push permissions)
- ✅ Slack webhook URL (create in Slack workspace)

**Internal**:
- ✅ GitHub Secrets configured (DEPLOY_KEY, DEPLOY_HOST, etc.)
- ✅ srv03-cloud SSH key setup
- ✅ Makefile with test/build targets (mostly exist, may need ci-* additions)

---

## Success Criteria (DoD)

**КРИТИЧНОЕ УСЛОВИЕ**: Все критерии должны быть основаны на ФАКТИЧЕСКИХ результатах, не на предположениях!

- ✅ GitHub Actions workflow executes end-to-end without manual intervention (ФАКТИЧЕСКОЕ выполнение!)
- ✅ All tests pass (100% of test suite) — на production сервере
- ✅ Code coverage >= 70% — верифицировано в GitHub Actions logs
- ✅ Docker images build, tag, and push successfully — с логами
- ✅ Deployment to srv03-cloud succeeds via SSH — SSH команды выполнены
- ✅ Health checks pass (all 5 services respond) — запрос к реальному эндпоинту
- ✅ Rollback tested and working (failure → recovery) — реальная симуляция failure
- ✅ Slack notifications sent on success & failure — сообщения в канал
- ✅ No secrets or tokens visible in logs — проверка GitHub Actions logs
- ✅ Idempotent deployment (deploy twice = same state) — два реальных deployment'а
- ✅ E2E test passes (push → production updated) — реальный git push
- ✅ Deployment documentation complete — `.github/DEPLOYMENT.md` существует
- ✅ Tech-Lead signed off (APPROVED) — после проверки фактических результатов

---

## Communication & Handoff

- **Backend → DevOps**: Handoff includes test/linting setup, Makefile targets
- **DevOps → Tester**: Handoff includes workflow YAML, test commands, credentials
- **Tester → Tech-Lead**: Handoff includes test results, validation logs
- **Tech-Lead → Done**: APPROVED status, production readiness confirmed

---

## Post-Deployment Notes

- **Monitoring**: Log all deploy actions for audit trail
- **On-call**: Include deployment runbook in on-call playbook
- **Incident**: If deploy fails, follow rollback docs, don't manually intervene unless documented
- **Future**: Phase 2 (TASK-005) will migrate to Kubernetes, upgrading rollout strategy

---

## ⚠️ КРИТИЧНОЕ ЗАМЕЧАНИЕ ДЛЯ ВСЕХ РОЛЕЙ

**После анализа Tester обнаружено**: На production сервере (`srv03-cloud`, 176.123.160.234) **ничего не было развёрнуто**:
- ❌ Docker не установлен
- ❌ Git репозиторий не клонирован
- ❌ Порты не открыты

**Результат**: Все предыдущие отчёты о "пройденных" интеграционных/E2E тестах были **фактически невозможны**.

### Что это значит для каждой роли:

**Backend**: ✅ Ваша работа валидна (unit-тесты, code quality)
- Убедитесь, что код закоммичен и готов к push в main

**DevOps**: ⚠️ КРИТИЧНОЕ ДЕЙСТВИЕ ТРЕБУЕТСЯ
- Установить Docker & Docker Compose на srv03-cloud
- Клонировать репозиторий
- Развернуть приложение с `.env` production
- Настроить GitHub Secrets для workflow
- **ТОЛЬКО ПОСЛЕ ЭТОГО** Tester может выполнить настоящие E2E тесты

**Tester**: ⏹️ Ожидание
- Текущие интеграционные тесты невозможны
- Дождитесь, пока DevOps развернёт production
- Затем повторите ВСЕ 11 тестов на реальном сервере
- Используйте только **фактические результаты**, не предположения

**Tech-Lead**: 📋 Запросите доказательства
- Требуйте логи GitHub Actions
- Требуйте скриншоты curl requests
- Требуйте реальные Slack сообщения
- Требуйте доказательства rollback'а
- **Не принимайте отчёты на основе "анализа кода"**

### Правило №1: Не выдумываем, проверяем

✅ Фактические результаты (с логами, скриншотами, коммитами)
❌ Предположения ("это должно работать")
❌ Статический анализ вместо реального тестирования

---
