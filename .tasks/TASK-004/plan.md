# План: TASK-004 — CI/CD Пайплайн (GitHub Actions)

## Цель
Настроить полностью автоматизированный GitHub Actions workflow, который деплоит изменения на production-сервер при каждом push в main branch.

## Scope / Out of scope

### In scope
- GitHub Actions workflow (`.github/workflows/deploy.yml`)
- Docker build и push на Docker Hub
- SSH деплой на srv03-cloud с deploy key
- Pre-deployment: тесты, linting, compile check
- Post-deployment: health checks, smoke tests
- Уведомления в Slack при успехе/ошибке
- Логирование деплоев на сервере (`/var/log/24alert-deploy.log`)
- Rollback скрипт при критических ошибках

### Out of scope
- Kubernetes / Helm
- Multi-environment (staging, dev) — будущее
- Кэширование Docker слоёв (базовая версия)
- Метрики и детальный мониторинг

## Порядок ролей

```
Планировщик → Backend (CI config) → DevOps (GitHub Actions) → Tester (e2e) → Tech-lead (review)
```

| Роль | Промпт | Что делает | Когда |
|------|--------|-----------|-------|
| **Планировщик** | plan.md (этот файл) | Декомпозиция, архитектура, зависимости | Первый |
| **Backend** | backend/prompt.md | Добавить тесты, lint config, build script | День 1 |
| **DevOps** | devops/prompt.md | GitHub Actions workflow, Deploy key, scripts | День 2 |
| **Tester** | tester/prompt.md | End-to-end тесты деплоя, rollback tests | День 3 |
| **Tech-lead** | tech-lead/prompt.md | Review workflow, security, best practices | День 4 |

## Архитектура CI/CD

```
git push origin main
         ↓
GitHub (webhook trigger)
         ↓
GitHub Actions (4 job'а):
  1. [test] Go build, lint, tests
  2. [build] Docker build & push
  3. [deploy] SSH to server, git pull, docker-compose up
  4. [validate] Health checks, smoke tests
         ↓
✓ Success: Slack notification
✗ Failure: Slack + auto-rollback
         ↓
Production live
```

## Риски

| Риск | Вероятность | Влияние | Митигация |
|------|------------|---------|----------|
| Deploy key скомпрометирован | LOW | CRITICAL | Rotate регулярно, restrict permissions |
| Docker push fails (Docker Hub down) | MEDIUM | MEDIUM | Fallback to local build |
| SSH timeout к серверу | MEDIUM | HIGH | Retry logic + timeout settings |
| Health check false positive | MEDIUM | MEDIUM | Multi-check validation |
| Production goes down (bug) | MEDIUM | CRITICAL | Automated rollback to previous version |

## Timeline

| Фаза | Роль | Время | Задача |
|------|------|-------|--------|
| 1 | Backend | 1 день | Добавить `go test`, `golangci-lint`, build script |
| 2 | DevOps | 1 день | Создать GitHub Actions workflow |
| 3 | DevOps | 1 день | Настроить GitHub Secrets, Deploy key |
| 4 | Tester | 1 день | Тесты деплоя, rollback, e2e |
| 5 | Tech-lead | 0.5 дня | Review, security checklist, sign-off |
| **Total** | | **4-5 дней** | |

## Файлы для создания

```
.github/
  workflows/
    deploy.yml            # Main CI/CD workflow
    test.yml              # Testing matrix (go versions, os)

config/
  golangci.yml           # Linting config
  
scripts/
  ci-test.sh             # Pre-push test script
  ci-rollback.sh         # Rollback on server
```

## Success Criteria

✅ Push → GitHub Actions → Deployment в production за <5 минут  
✅ Все smoke tests пройдены  
✅ Slack notification получена  
✅ Логи деплоя на сервере сохранены  
✅ Rollback работает и протестирован
