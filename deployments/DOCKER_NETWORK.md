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

## Неправильно (не использовать)

```bash
cd /opt/24alert/deployments
docker compose up -d   # project=deployments → другая сеть
```
