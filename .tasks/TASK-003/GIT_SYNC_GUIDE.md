# Git Synchronization Guide — 24alert Trading Bot

## Status

✅ **Локально** (c:\Users\sdk\proj\24alert):
- Git инициализирован (`.git/`)
- 94 файла закоммичены
- Commit: `cc1fa5a - Initial commit: 24alert trading bot - 5 microservices, CLI, Swagger, Docker, complete implementation`
- Remote: `git@github.com:sdkinfotech/24alert-trading-bot.git`
- Branch: `main`

⏳ **На GitHub**:
- Ожидает push (нужны права)

📍 **На сервере** (srv03-cloud:/opt/24alert):
- Клонирован из GitHub (может быть старой версией)
- Требуется sync

---

## Шаг 1: Push локального кода на GitHub

### Вариант A: SSH (если настроена авторизация)

```bash
cd c:\Users\sdk\proj\24alert
git push -u origin main
```

**Или полный путь с проверкой**:
```bash
git push -u origin main --force  # --force только если первый push
```

### Вариант B: HTTPS (с токеном GitHub)

```bash
git remote set-url origin https://github.com/sdkinfotech/24alert-trading-bot.git
git push -u origin main
```

Введите GitHub token вместо пароля.

---

## Шаг 2: Синхронизировать сервер

SSH на сервер и обновить код:

```bash
ssh adm-srv03-cloud@176.123.160.234

# На сервере:
cd /opt/24alert

# Если репо уже клонирован:
git pull origin main

# Если нет репо:
cd /opt
rm -rf 24alert
git clone git@github.com:sdkinfotech/24alert-trading-bot.git 24alert
cd 24alert
```

### Обновить Docker образы:

```bash
make docker-build
docker-compose -f deployments/docker-compose.yaml down
docker-compose -f deployments/docker-compose.yaml up -d
```

### Проверить здоровье:

```bash
curl http://localhost:8080/health
docker-compose -f deployments/docker-compose.yaml ps
```

---

## Шаг 3: Проверить синхронизацию

### Локально:

```bash
cd c:\Users\sdk\proj\24alert
git log --oneline
git remote -v
git status
```

**Ожидаемый результат**:
```
branch main, nothing to commit, working tree clean
origin  git@github.com:sdkinfotech/24alert-trading-bot.git
```

### На сервере:

```bash
cd /opt/24alert
git log --oneline | head -5
git status
```

**Ожидаемый результат**:
```
Branch main, nothing to commit, working tree clean
cc1fa5a Initial commit: 24alert trading bot ...
```

---

## Шаг 4: Документировать в DEPLOYMENT.md

Добавить в `DEPLOYMENT.md`:

```markdown
## Git Synchronization

Code is synchronized across three locations:

1. **Local**: `c:\Users\sdk\proj\24alert` (Windows development machine)
2. **GitHub**: `git@github.com:sdkinfotech/24alert-trading-bot.git` (canonical source)
3. **Server**: `/opt/24alert` (production deployment)

### Update Workflow

```bash
# Local → GitHub
cd c:\Users\sdk\proj\24alert
git add .
git commit -m "description"
git push origin main

# GitHub → Server
ssh adm-srv03-cloud@176.123.160.234
cd /opt/24alert
git pull origin main
make docker-build && make docker-up
```
```

---

## Commands for Daily Use

### Local (Windows)

```bash
# Check status
cd c:\Users\sdk\proj\24alert
git status

# Commit changes
git add .
git commit -m "feat: description"

# Push to GitHub
git push origin main

# Pull from GitHub
git pull origin main
```

### Server (srv03-cloud)

```bash
# Check status
cd /opt/24alert
git status

# Update from GitHub
git pull origin main

# Rebuild and redeploy
make docker-build
make docker-up

# Check health
curl http://localhost:8080/health
docker-compose ps
```

---

## Troubleshooting

### "fatal: not a git repository"

```bash
cd /opt/24alert
git init
git remote add origin git@github.com:sdkinfotech/24alert-trading-bot.git
git pull origin main
```

### "Permission denied (publickey)"

Setup SSH key forwarding or use HTTPS:
```bash
git remote set-url origin https://github.com/sdkinfotech/24alert-trading-bot.git
```

### "Your branch is ahead of origin/main"

Push changes to GitHub:
```bash
git push origin main
```

### Server stuck at old version

Force update:
```bash
cd /opt/24alert
git fetch origin
git reset --hard origin/main
make docker-build && make docker-up
```

---

## Summary

- ✅ Local: Committed to git
- ⏳ GitHub: Ready for push (awaiting SSH/HTTPS auth)
- 📍 Server: Ready for `git pull origin main`

**Next**: Execute `git push -u origin main` from local machine.
