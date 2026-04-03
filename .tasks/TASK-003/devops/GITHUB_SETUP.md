# GitHub Setup для srv03-cloud

## Что было сделано:

✅ **GitHub CLI установлен и аутентифицирован** (пользователь: sdkinfotech)  
✅ **Репозиторий создан**: https://github.com/sdkinfotech/24alert-trading-bot  
✅ **Код залит** в репо (коммиты с TASK-002 и DEPLOYMENT-003)  
✅ **Deploy Key добавлена** для SSH доступа со srv03-cloud  

---

## Инструкции для srv03-cloud:

### 1. Скопировать приватный SSH ключ на сервер

На локальной машине:
```bash
# Скопировать приватный ключ на сервер
scp ~/.ssh/id_ed25519_gitverse adm-srv03-cloud@176.123.160.234:~/.ssh/

# Или вручную: скопировать содержимое id_ed25519_gitverse
cat ~/.ssh/id_ed25519_gitverse
```

На сервере srv03-cloud:
```bash
# Выставить правильные права доступа
chmod 600 ~/.ssh/id_ed25519_gitverse

# Добавить ключ в ssh-agent
ssh-add ~/.ssh/id_ed25519_gitverse
```

### 2. Клонировать репо через SSH

```bash
cd /opt/24alert
rm -rf * .git*  # Очистить, если уже есть код

git clone git@github.com:sdkinfotech/24alert-trading-bot.git .
```

Ожидаемо:
```
Cloning into '.'...
remote: Counting objects: 3, done.
remote: Compressing objects: 100% (3/3), done.
remote: Receiving objects: 100% (3/3), done.
```

### 3. Проверить структуру кода

```bash
ls -la

# Ожидаемо:
# go.mod, Makefile, cmd/, pkg/, config/, deployments/, .env (НЕТ!)
```

### 4. Продолжить развёртывание TASK-003

```bash
# Скопировать .env с токенами (должен быть в .gitignore)
# Или создать вручную:
cat > deployments/.env << 'EOF'
TINVEST_SANDBOX=false
TINVEST_PROD_TOKEN=t.gr4w_xSRuwyOBiLlGHs7Hm7MTATMWVhBDsfLJmn1uccXIvuK20sbpIp_6crH1RJ6rjAjwmLcB2I5fqmFKUGPxw
TINVEST_SANDBOX_TOKEN=t.haxQpgLAVgCxmP_cP7Zb9fLNRrjbmgdp8nmidr_an85UJpRsgrGVQ3SzxcfswYzk5b9yfNLoIzzEt-R5XwHWZQ
LOG_LEVEL=info
EOF

# Собрать и запустить
make docker-build
make docker-up
```

---

## GitHub Repository Details

| Параметр | Значение |
|----------|----------|
| **URL** | https://github.com/sdkinfotech/24alert-trading-bot |
| **SSH Clone** | git@github.com:sdkinfotech/24alert-trading-bot.git |
| **HTTPS Clone** | https://github.com/sdkinfotech/24alert-trading-bot.git |
| **Visibility** | Public |
| **Deploy Key** | ✓ Добавлена (id_ed25519_gitverse, read-only) |

---

## Deploy Key Info

- **ID**: 147513399
- **Title**: srv03-cloud deployment
- **Type**: ssh-ed25519
- **Access**: read-only (подходит для pull-only)
- **Added**: 2026-04-03

---

## Git Config на srv03-cloud

После клонирования репо оно будет готово к дальнейшему development:

```bash
# Проверить remote
git remote -v
# origin    git@github.com:sdkinfotech/24alert-trading-bot.git (fetch)
# origin    git@github.com:sdkinfotech/24alert-trading-bot.git (push)

# Просмотреть history
git log --oneline

# Если нужны обновления
git pull origin main
```

---

**Статус**: Ready for production deployment  
**Дата**: 2026-04-03  
**GitHub CLI Version**: 2.89.0
