# Production Git Sync Strategy — 24alert Trading Bot

## ✅ Правильный подход (CI/CD стандарты)

```
GitHub (canonical source)
    ↓ (webhook)
Server (/opt/24alert)
    ↓ (git pull)
Docker rebuild & restart
    ↓ (automated)
Production API live
```

---

## Архитектура синхронизации

### 1. GitHub репозиторий (Source of Truth)

**URL**: `git@github.com:sdkinfotech/24alert-trading-bot.git`  
**Branch**: `main` (production)

Содержит:
- Исходный код (Go микросервисы)
- Конфигурация (config.yaml, docker-compose.yaml, Dockerfile)
- Документация (.tasks/, README.md)
- Exclude: `.env` (в .gitignore)

### 2. Сервер (Deployment Environment)

**Location**: `/opt/24alert` на srv03-cloud (176.123.160.234)  
**User**: `adm-srv03-cloud`  
**Access**: SSH + Git

**Процесс**:
```bash
cd /opt/24alert
git pull origin main          # ← Обновить код
make docker-build            # ← Пересобрать образы
make docker-up               # ← Рестартнуть контейнеры
curl http://localhost:8080/health  # ← Проверить здоровье
```

### 3. Локальная машина (Development)

**Location**: `c:\Users\sdk\proj\24alert`  
**Role**: Разработка → Коммит → Push на GitHub  
**Процесс**:
```bash
git add .
git commit -m "feat: description"
git push origin main         # ← Автоматически триггирует деплой на сервер
```

---

## Настройка автоматического деплоя

### Option A: Git Post-Receive Hook (самый простой)

На сервере создать `.git/hooks/post-receive`:

```bash
#!/bin/bash
cd /opt/24alert
git pull origin main
make docker-build
make docker-up
echo "✓ Deployment complete"
```

**Включить**:
```bash
chmod +x /opt/24alert/.git/hooks/post-receive
```

### Option B: SSH Deploy Key (рекомендуется)

На сервере создать deploy ключ без пароля:

```bash
ssh-keygen -t ed25519 -f ~/.ssh/id_deploy -N ""
cat ~/.ssh/id_deploy.pub  # → добавить в GitHub Deploy Keys
```

Добавить в GitHub (Settings → Deploy Keys → Add deploy key):
```
paste ~/.ssh/id_deploy.pub
```

Затем сервер может делать `git pull` без ввода пароля:
```bash
ssh -i ~/.ssh/id_deploy git@github.com  # тест
cd /opt/24alert && git pull origin main
```

### Option C: GitHub Actions (полный CI/CD)

Создать `.github/workflows/deploy.yml`:

```yaml
name: Deploy to Production

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: SSH Deploy
        uses: appleboy/ssh-action@master
        with:
          host: 176.123.160.234
          username: adm-srv03-cloud
          key: ${{ secrets.DEPLOY_KEY }}
          script: |
            cd /opt/24alert
            git pull origin main
            make docker-build
            make docker-up
            curl http://localhost:8080/health
```

**Настройка**:
1. На сервере: создать deploy SSH ключ
2. В GitHub Settings → Secrets: добавить `DEPLOY_KEY`
3. Каждый `git push origin main` автоматически триггирует деплой

---

## Рекомендуемая настройка для 24alert

### Вариант 1: Simple (для разработки)

```bash
# На сервере один раз:
cd /opt/24alert
git remote add origin git@github.com:sdkinfotech/24alert-trading-bot.git

# Затем вручную:
git pull origin main
make docker-build && make docker-up
```

**Использование**: `ssh` на сервер и выполнить перед каждым деплоем.

### Вариант 2: Post-Receive Hook (для малых команд)

```bash
# На сервере один раз:
cat > /opt/24alert/.git/hooks/post-receive << 'EOF'
#!/bin/bash
cd /opt/24alert
git pull origin main
make docker-build
make docker-up
curl http://localhost:8080/health > /dev/null && echo "✓ Deploy OK" || echo "✗ Deploy failed"
EOF

chmod +x /opt/24alert/.git/hooks/post-receive
```

**Использование**: `git push origin main` с локальной машины.

### Вариант 3: GitHub Actions (для production)

**Полностью автоматизированный деплой** при каждом push.

---

## Реализация: Пошаговая инструкция

### Шаг 1: Настроить SSH на сервере

```bash
ssh adm-srv03-cloud@176.123.160.234

# Генерировать deploy ключ
ssh-keygen -t ed25519 -f ~/.ssh/id_deploy_gh -N ""

# Показать публичный ключ
cat ~/.ssh/id_deploy_gh.pub
```

### Шаг 2: Добавить Deploy Key на GitHub

На https://github.com/sdkinfotech/24alert-trading-bot/settings/keys:
- Title: "Production Server Deployment"
- Key: (вставить содержимое `id_deploy_gh.pub`)
- ✓ Allow write access (если нужны обратные push'и)

### Шаг 3: Настроить Git на сервере

```bash
ssh adm-srv03-cloud@176.123.160.234

cd /opt/24alert

# Убедиться, что репо инициализирован
ls -la .git

# Если нет:
git init
git remote add origin git@github.com:sdkinfotech/24alert-trading-bot.git

# Тест доступа
GIT_SSH_COMMAND="ssh -i ~/.ssh/id_deploy_gh" git pull origin main

# Если успешно, можно добавить в ~/.bashrc:
alias gitpull='GIT_SSH_COMMAND="ssh -i ~/.ssh/id_deploy_gh" git pull origin main'
```

### Шаг 4: Создать скрипт автоматического деплоя

```bash
cat > /opt/24alert/deploy.sh << 'EOF'
#!/bin/bash
set -e

cd /opt/24alert

echo "Pulling from GitHub..."
GIT_SSH_COMMAND="ssh -i ~/.ssh/id_deploy_gh" git pull origin main

echo "Building Docker images..."
make docker-build

echo "Restarting containers..."
make docker-up

echo "Waiting for service health..."
sleep 5

HEALTH=$(curl -s http://localhost:8080/health | jq -r '.status // "offline"')
if [ "$HEALTH" = "ok" ]; then
    echo "✓ Deployment successful"
    exit 0
else
    echo "✗ Health check failed"
    exit 1
fi
EOF

chmod +x /opt/24alert/deploy.sh
```

### Шаг 5: Тестировать

```bash
# На сервере
cd /opt/24alert
./deploy.sh

# Проверить
curl http://localhost:8080/health
docker ps
```

### Шаг 6: Использовать из локальной машины

```bash
# Локально (Windows)
cd c:\Users\sdk\proj\24alert

git add .
git commit -m "feat: new trading feature"
git push origin main

# На сервере срабатывает автоматически (если webhook) или:
ssh adm-srv03-cloud@176.123.160.234 /opt/24alert/deploy.sh
```

---

## Мониторинг деплоев

### Логирование деплоев

```bash
# На сервере
cat > /opt/24alert/deploy-with-logging.sh << 'EOF'
#!/bin/bash
LOG_FILE="/var/log/24alert-deploy.log"
{
    echo "[$(date)] Starting deployment..."
    cd /opt/24alert
    GIT_SSH_COMMAND="ssh -i ~/.ssh/id_deploy_gh" git pull origin main
    make docker-build
    make docker-up
    curl http://localhost:8080/health
    echo "[$(date)] ✓ Deployment complete"
} >> $LOG_FILE 2>&1
EOF

chmod +x /opt/24alert/deploy-with-logging.sh
```

Просмотр логов:
```bash
tail -f /var/log/24alert-deploy.log
```

### Алерты при ошибках

```bash
# Отправлять уведомления в Slack при ошибке
if ! /opt/24alert/deploy.sh; then
    curl -X POST $SLACK_WEBHOOK -d '{"text":"❌ 24alert deployment failed"}'
fi
```

---

## Summary

✅ **Правильная архитектура**:
1. **GitHub** = canonical source of truth
2. **Server** имеет SSH deploy key для доступа к GitHub
3. **Local** pushит на GitHub
4. **Server** автоматически `git pull` и деплоит

✅ **Рекомендация для 24alert**: **Вариант 2** (Post-Receive Hook)
- Простая настройка
- Автоматический деплой при push
- Не требует GitHub Actions
- Подходит для малых команд

**Следующий шаг**: Выполнить Шаги 1-5 на сервере (15 минут работы).
