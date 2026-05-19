# Промпт: DevOps → TASK-004

## Контекст

Ты — **DevOps**. Backend завершил настройку тестов и Makefile. Теперь нужно создать GitHub Actions workflow и deployment скрипты для production.

**Исходные данные**:
- `TASK-004/task.md` — requirements
- `TASK-004/plan.md` — план и зависимости
- `TASK-004/backend/handoff.md` — что сделал Backend (Makefile targets, test config)

---

## Задача

### 1. Создай GitHub Actions Workflow

**Путь**: `.github/workflows/deploy.yml`

**Структура**:
```yaml
name: Deploy Trading Bot
on:
  push:
    branches: [main]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - checkout
      - setup-go
      - run: make ci-check
  
  build:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - checkout
      - docker build & push
      - tag: latest, main, v1.0.0-<commit>
  
  deploy:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - SSH to srv03-cloud
      - git pull origin main
      - docker-compose up
      - curl health check
      - on failure: rollback
  
  notify:
    if: always()
    steps:
      - Slack webhook
      - status: ✅ success | ❌ failed
```

**Requirements**:
- ✅ Test job: runs `make ci-check` (from Backend)
- ✅ Build job: Docker multi-stage, push to Docker Hub
- ✅ Deploy job: SSH key from GitHub Secrets
- ✅ Health check: `curl http://localhost:8080/health` (retry 3 times)
- ✅ Timeout: Total < 10 minutes, deploy step < 2 minutes
- ✅ Rollback: On deploy failure, revert to previous image
- ✅ Notifications: Slack webhook for all statuses

**GitHub Secrets to configure**:
- `DEPLOY_HOST`: srv03-cloud IP (176.123.160.234)
- `DEPLOY_USER`: SSH username (e.g., "ubuntu")
- `DEPLOY_KEY`: Private SSH key (for auth)
- `DOCKER_TOKEN`: Docker Hub token (for push)
- `SLACK_WEBHOOK`: Slack incoming webhook URL

### 2. Создай Deployment Скрипты

**Путь**: `scripts/deploy-prod.sh`

```bash
#!/bin/bash
set -e

HOST=$1
USER=$2
COMMIT=$3

echo "[DEPLOY] Connecting to $HOST as $USER..."
ssh -i ~/.ssh/deploy_key $USER@$HOST << 'EOF'
  set -e
  cd /opt/24alert
  
  echo "[DEPLOY] Pulling latest code..."
  git pull origin main
  
  echo "[DEPLOY] Building Docker images..."
  docker-compose build
  
  echo "[DEPLOY] Starting containers..."
  docker-compose up -d
  
  echo "[DEPLOY] Waiting for services..."
  sleep 3
  
  echo "[DEPLOY] Health check..."
  for i in {1..5}; do
    if curl -f http://localhost:8080/health; then
      echo "[DEPLOY] ✅ Health check passed"
      exit 0
    fi
    echo "[DEPLOY] Retry $i/5..."
    sleep 2
  done
  
  echo "[DEPLOY] ❌ Health check failed"
  exit 1
EOF

echo "[DEPLOY] ✅ Deployment successful"
```

**Путь**: `scripts/rollback-prod.sh`

```bash
#!/bin/bash
set -e

HOST=$1
USER=$2
PREVIOUS_IMAGE=$3

echo "[ROLLBACK] Reverting to $PREVIOUS_IMAGE..."
ssh -i ~/.ssh/deploy_key $USER@$HOST << EOF
  set -e
  cd /opt/24alert
  
  echo "[ROLLBACK] Pulling $PREVIOUS_IMAGE..."
  docker pull $PREVIOUS_IMAGE
  
  echo "[ROLLBACK] Restarting containers..."
  docker-compose up -d
  
  sleep 2
  
  echo "[ROLLBACK] Health check..."
  curl -f http://localhost:8080/health
  
  echo "[ROLLBACK] ✅ Rollback complete"
EOF
```

### 3. Создай Deployment Documentation

**Путь**: `.github/DEPLOYMENT.md`

**Содержание**:
- How to trigger manual deployment
- GitHub Secrets setup instructions
- SSH key rotation procedure
- Health check endpoints & expected responses
- Rollback procedure (manual)
- Slack webhook configuration
- Troubleshooting guide (common issues & fixes)
- On-call runbook

---

## Чек-лист

- ✅ `.github/workflows/deploy.yml` создана
- ✅ Все GitHub Secrets задокументированы
- ✅ `scripts/deploy-prod.sh` готов и работает локально
- ✅ `scripts/rollback-prod.sh` готов
- ✅ `.github/DEPLOYMENT.md` написана
- ✅ Workflow file валиден (GitHub Actions syntax check)
- ✅ Deploy scripts имеют execute permission (chmod +x)
- ✅ No secrets hardcoded (все из GitHub Secrets)

---

## Что включить в handoff.md

```markdown
# Handoff: DevOps → TASK-004

## Статус
DONE

## Что сделано
- ✓ GitHub Actions workflow (.github/workflows/deploy.yml)
- ✓ Deploy script (scripts/deploy-prod.sh)
- ✓ Rollback script (scripts/rollback-prod.sh)
- ✓ GitHub Secrets configured
- ✓ Deployment documentation (.github/DEPLOYMENT.md)

## Артефакты
- Файлы: .github/workflows/deploy.yml, scripts/deploy-prod.sh, scripts/rollback-prod.sh, .github/DEPLOYMENT.md

## Корректировки для следующих ролей
[Describe if any changes to workflow or scripts needed for Tester]

## Блокеры
НЕТ
```

---

## Success Criteria

✅ Workflow file is valid (GitHub checks it)  
✅ Deploy script tested manually (git pull → docker-compose → health check)  
✅ Rollback script tested (previous image restored)  
✅ No secrets in logs  
✅ Documentation complete & clear  
✅ Ready for Tester to validate end-to-end
