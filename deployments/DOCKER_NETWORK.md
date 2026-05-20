# Docker-сеть 24alert (обязательно к прочтению)

## Проблема

Если запускать compose **без единого project name**, Docker создаёт разные сети:

| Команда | Сеть |
|---------|------|
| `cd deployments && docker compose up` | `deployments_trading-bot-net` |
| `docker compose -p 24alert -f deployments/docker-compose.yaml up` | `24alert_trading-bot-net` |

Контейнеры в разных сетях **не видят** друг друга (`advisor` → `strategy-runner` = HTTP 000).

## Решение в репозитории

1. **Одна внешняя сеть** `24alert-trading-bot-net` (`external: true` в compose).
2. **DNS по `container_name`**: `24alert-strategy-runner`, `24alert-advisor-svc` (не зависит от `-p`).
3. **Только обёртка** `scripts/compose.sh` или `make docker-up` на проде.

## Правильный деплой на srv03

```bash
cd /opt/24alert
sudo bash scripts/deploy-srv03.sh
# или вручную:
sudo bash scripts/ensure-docker-network.sh
sudo bash scripts/compose.sh up -d
sudo bash scripts/verify-docker-network.sh
```

## Если снова «разъехались»

```bash
sudo bash scripts/reconcile-docker-network.sh
sudo bash scripts/verify-docker-network.sh
```

## Ошибка Conflict: container name already in use

При деплое через CI или `compose up` может появиться:

```text
Error: container name "/24alert-strategy-runner" is already in use
```

Причина: старый контейнер с тем же `container_name` остался от другого compose project.

**Исправление на srv03:**

```bash
sudo docker rm -f 24alert-strategy-runner 24alert-advisor-svc
cd /opt/24alert
sudo bash scripts/compose.sh up -d
sudo bash scripts/verify-docker-network.sh
```

`scripts/deploy-prod.sh` (GitHub Actions) выполняет `docker rm -f` для этих имён перед `compose up`.

## Неправильно (не использовать)

```bash
cd /opt/24alert/deployments
docker compose up -d   # project=deployments → другая сеть
```

## Диск VPS (30 GB): очистка Docker

Каждый `compose build` оставляет промежуточные образы (`<none>`). Без уборки `git pull` падает с `No space left on device`.

**Разово:**

```bash
cd /opt/24alert
sudo bash scripts/docker-disk-prune.sh
```

**Проверка без удаления:** `sudo bash scripts/docker-disk-prune.sh --dry-run`

**Cron (раз в неделю, вс 04:30 UTC):**

```bash
cd /opt/24alert
sudo bash scripts/install-docker-prune-cron.sh
```

Лог: `/var/log/24alert-docker-prune.log`

`scripts/deploy-prod.sh` запускает prune после деплоя и ставит cron, если его ещё нет.
