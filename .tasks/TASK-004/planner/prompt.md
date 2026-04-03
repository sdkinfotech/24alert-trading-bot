# Промпт: Планировщик → TASK-004

## Контекст

Ты — **Планировщик**. TASK-003 завершена и одобрена. Теперь нужно декомпозировать TASK-004 (CI/CD Pipeline) и создать план с промптами для ролей.

**Исходная постановка**: `.tasks/TASK-004/task.md`

---

## Что делать

### 1. Прочитай исходную задачу

```
c:\Users\sdk\proj\24alert\.tasks\TASK-004\task.md
```

Убедись, что понял:
- Цель: полностью автоматизированный GitHub Actions workflow
- Requirements: все требования функциональные и нефункциональные
- DoD: критерии готовности
- Dependencies: TASK-003 (deployment готов)

### 2. Создай plan.md

**Путь**: `.tasks/TASK-004/plan.md`

**Структура**:
- Цель (1 предложение)
- Scope / Out of scope
- Порядок ролей (тебе нужны: Backend, DevOps, Tester, Tech-lead)
- Архитектура (workflow diagram)
- Риски и mitigation
- Timeline (сколько дней на каждую фазу)
- Файлы для создания

**Рекомендуемый порядок**:
```
Backend (1 день)   → tests + linting config
    ↓
DevOps (2 дня)     → GitHub Actions workflow + secrets
    ↓
Tester (1 день)    → e2e deploy tests
    ↓
Tech-lead (0.5 дня) → security & best practices review
```

**Риски** (примеры):
- GitHub Actions quota exceeded
- SSH timeout к серверу
- Deploy key compromised
- Health check false positive
- Rollback не работает

### 3. Создай промпты для каждой роли

**Backend** (`.tasks/TASK-004/backend/prompt.md`):
- Добавить `go test -v -cover ./...`
- Добавить `golangci-lint` config
- Создать `scripts/ci-test.sh`
- Обновить `Makefile` с target `ci-check`

**DevOps** (`.tasks/TASK-004/devops/prompt.md`):
- Создать `.github/workflows/deploy.yml`
- Настроить GitHub Secrets (DEPLOY_KEY, DOCKER_TOKEN, SLACK_WEBHOOK)
- Deploy script на сервере (SSH + git pull + docker-compose)
- Health check validation
- Rollback mechanism

**Tester** (`.tasks/TASK-004/tester/prompt.md`):
- E2E тесты: push → deploy → health check
- Rollback tests (simulate failure, trigger rollback)
- Slack notification verification
- Log validation

**Tech-lead** (`.tasks/TASK-004/tech-lead/prompt.md`):
- Security review (secrets, SSH key permissions)
- Best practices (idempotency, timeout, retry logic)
- Production readiness

### 4. Обнови BACKLOG.md

Найди TASK-004 в таблице и обнови:
- Status: "In Progress" → "In Progress"
- Добавь link к `.tasks/TASK-004/plan.md`

---

## Примеры для вдохновения

### GitHub Actions Workflow Shape

```yaml
name: Deploy Trading Bot

on:
  push:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
      - run: make ci-check
  
  build:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: docker/setup-buildx-action@v2
      - run: make docker-build && docker push ...
  
  deploy:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: appleboy/ssh-action@master
        with:
          host: ${{ secrets.DEPLOY_HOST }}
          username: ${{ secrets.DEPLOY_USER }}
          key: ${{ secrets.DEPLOY_KEY }}
          script: |
            cd /opt/24alert
            git pull origin main
            make docker-build && make docker-up
      - run: curl http://localhost:8080/health
  
  notify:
    if: always()
    runs-on: ubuntu-latest
    steps:
      - uses: 8398a7/action-slack@v3
        with:
          webhook_url: ${{ secrets.SLACK_WEBHOOK }}
          status: ${{ job.status }}
```

---

## Следующие шаги (после план.md)

1. Просмотри draft всех четырёх prompt.md
2. Подумай о зависимостях между ролями
3. Обнови timeline если что-то изменилось
4. Передай Backend'у для начала работы

---

## Success Criteria

✅ plan.md содержит чёткую архитектуру  
✅ Все 4 prompt'а созданы с конкретными заданиями  
✅ Зависимости понятны каждой роли  
✅ BACKLOG.md обновлён  
✅ Backend готов начинать работу
