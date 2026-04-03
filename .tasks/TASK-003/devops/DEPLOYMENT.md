# Deployment Guide — TASK-003

## Production Deployment: Trading Bot на srv03-cloud

### Prerequisites

- **Server**: Ubuntu 22.04 LTS
- **Docker**: 28.2.2+ (установлен и работает)
- **Docker Compose**: v2.26.0+ (встроен в docker-ce-cli)
- **SSH доступ**: `adm-srv03-cloud@176.123.160.234`
- **GitHub доступ**: SSH ключ скопирован на сервер (`~/.ssh/id_ed25519` или аналог)
- **T-Invest Tokens**:
  - `TINVEST_PROD_TOKEN` — production T-Invest API ключ (основной аккаунт)
  - `TINVEST_SANDBOX_TOKEN` — для тестирования (опционально)

### Pre-Deployment Checks

Выполнить на сервере перед развёртыванием:

```bash
# 1. Проверить Docker
docker ps
# Ожидаемо: пустой список или уже running контейнеры

# 2. Место на диске
df -h
# Ожидаемо: >=24 GB свободно в /opt/24alert или / 

# 3. Порты свободны
sudo netstat -tuln | grep -E ':8080|:9001|:9002|:9003|:9004'
# Ожидаемо: пусто (никакие сервисы не слушают)

# 4. Docker демон работает
sudo systemctl status docker
# Ожидаемо: active (running)
```

---

## Step 1: SSH и подготовка окружения

```bash
ssh adm-srv03-cloud@176.123.160.234
cd /opt/24alert
pwd  # /opt/24alert
```

### Очистить директорию (если она существует):

```bash
# Если папка пуста или старые контейнеры нужно остановить:
docker-compose down 2>/dev/null || true
rm -rf /opt/24alert/*
```

---

## Step 2: Git clone и подготовка кода

```bash
# Клонировать репозиторий
git clone https://github.com/24alert/trading-bot.git .

# Проверить структуру
ls -la
# Ожидаемо: go.mod, Makefile, deployments/, cmd/, pkg/, config/, proto/

# Проверить критические файлы
test -f go.mod && echo "✓ go.mod" || echo "✗ go.mod missing"
test -f Makefile && echo "✓ Makefile" || echo "✗ Makefile missing"
test -f deployments/docker-compose.yaml && echo "✓ docker-compose.yaml" || echo "✗ docker-compose.yaml missing"
test -f deployments/Dockerfile && echo "✓ Dockerfile" || echo "✗ Dockerfile missing"
test -d config && echo "✓ config/" || echo "✗ config/ missing"
```

---

## Step 3: Конфигурация .env

### Скопировать template:

```bash
cd deployments
cp .env.example .env
```

### Отредактировать `.env` с production параметрами:

```bash
cat > deployments/.env << 'EOF'
# =====================================================================
#  24Alert Trading Bot — Production Environment
# =====================================================================

# Режим: ВАЖНО — false для production!
TINVEST_SANDBOX=false

# Production T-Invest Token (основной аккаунт)
TINVEST_PROD_TOKEN=t.gr4w_xSRuwyOBiLlGHs7Hm7MTATMWVhBDsfLJmn1uccXIvuK20sbpIp_6crH1RJ6rjAjwmLcB2I5fqmFKUGPxw

# Sandbox Token (для тестирования — используется если TINVEST_SANDBOX=true)
TINVEST_SANDBOX_TOKEN=t.haxQpgLAVgCxmP_cP7Zb9fLNRrjbmgdp8nmidr_an85UJpRsgrGVQ3SzxcfswYzk5b9yfNLoIzzEt-R5XwHWZQ

# Логирование
LOG_LEVEL=info

# (опционально) Переопределить endpoint вручную
# TINVEST_ENDPOINT=invest-public-api.tbank.ru:443
EOF
```

### Проверить переменные:

```bash
# Убедиться, что TINVEST_SANDBOX=false
grep "TINVEST_SANDBOX=" deployments/.env

# Убедиться, что токены заполнены (не пусто)
grep "^TINVEST_PROD_TOKEN=t\." deployments/.env && echo "✓ Prod token loaded"
grep "^TINVEST_SANDBOX_TOKEN=t\." deployments/.env && echo "✓ Sandbox token loaded"
```

---

## Step 4: Проверка конфигурации

```bash
# Убедиться, что config.yaml согласован с example
diff config/config.yaml config/config.yaml.example || true
# Замечание: небольшие различия OK, главное — структура совпадает

# Убедиться, что config.yaml и docker-compose ожидают переменные
grep -i "TINVEST" config/config.yaml || echo "Note: config uses ENV vars via pkg/config"
```

---

## Step 5: Docker Build

```bash
# Перейти в корень репозитория
cd /opt/24alert

# Собрать все Docker images
make docker-build

# Или эквивалентно:
# docker compose -f deployments/docker-compose.yaml build
```

### Проверить собранные образы:

```bash
docker images | grep 24alert

# Ожидаемо:
# 24alert-trading-bot-order-svc         latest    <hash>  <size>  <created>
# 24alert-trading-bot-marketdata-svc    latest    <hash>  <size>  <created>
# 24alert-trading-bot-portfolio-svc     latest    <hash>  <size>  <created>
# 24alert-trading-bot-risk-svc          latest    <hash>  <size>  <created>
# 24alert-trading-bot-gateway           latest    <hash>  <size>  <created>
```

---

## Step 6: Docker Compose Up

```bash
# Запустить все сервисы в фоне
make docker-up

# Или:
# docker compose -f deployments/docker-compose.yaml up -d
```

### Проверить статус контейнеров:

```bash
docker-compose -f deployments/docker-compose.yaml ps

# Ожидаемо (примерно через 15-30 сек, когда health checks пройдут):
# NAME                           STATUS
# 24alert-order-svc             Up X seconds (healthy)
# 24alert-marketdata-svc        Up X seconds (healthy)
# 24alert-portfolio-svc         Up X seconds (healthy)
# 24alert-risk-svc              Up X seconds (healthy)
# 24alert-gateway               Up X seconds (healthy)
```

**Если контейнер в status `Unhealthy`** → смотри логи (Step 7 ниже).

---

## Step 7: Health Checks

### Gateway Health Check (главный):

```bash
curl -v http://localhost:8080/health

# Ожидаемо:
# HTTP/1.1 200 OK
# Content-Type: application/json
# {
#   "status": "ok",
#   "timestamp": "2026-04-03T12:34:56Z"
# }
```

### Отдельные health checks по сервисам (optional):

```bash
# Order Service
curl -s http://localhost:9001/health || echo "Order health check failed"

# Market Data Service
curl -s http://localhost:9002/health || echo "MarketData health check failed"

# Portfolio Service
curl -s http://localhost:9003/health || echo "Portfolio health check failed"

# Risk Service
curl -s http://localhost:9004/health || echo "Risk health check failed"
```

### Логи (если health check не прошёл):

```bash
# Логи gateway
docker logs 24alert-gateway

# Логи всех сервисов
docker-compose -f deployments/docker-compose.yaml logs

# Логи с фильтром на ошибки
docker-compose -f deployments/docker-compose.yaml logs | grep -i "error\|failed\|panic"

# Следить за логами в реальном времени
make docker-logs  # или: docker-compose logs -f
```

---

## Step 8: Swagger UI

### Доступность локально (на сервере):

```bash
curl -s http://localhost:8080/swagger/ | head -20
# Ожидаемо: HTML с Swagger UI
```

### Доступность извне:

Открыть в браузере:

```
http://176.123.160.234:8080/swagger/
```

Должен загрузиться интерактивный Swagger с документацией API.

---

## Step 9: Базовые REST API Smoke Tests

### Получить список счетов:

```bash
curl -s http://localhost:8080/api/v1/accounts | jq .

# Ожидаемо:
# {
#   "accounts": [
#     {
#       "accountId": "...",
#       "name": "Основной счёт",
#       "status": "ACTIVE",
#       ...
#     }
#   ]
# }
```

### Получить портфель:

```bash
curl -s http://localhost:8080/api/v1/portfolio | jq .

# Ожидаемо:
# {
#   "portfolio": {
#     "cash": {...},
#     "positions": [...]
#   }
# }
```

### Получить позиции:

```bash
curl -s http://localhost:8080/api/v1/positions | jq .

# Ожидаемо: список позиций с инструментами
```

---

## Step 10: Логирование

### Проверить формат логов:

```bash
docker logs 24alert-gateway | head -5

# Ожидаемо: структурированные JSON логи с полями:
# {
#   "level": "info",
#   "msg": "server started",
#   "timestamp": "2026-04-03T12:34:56Z",
#   "service": "gateway"
# }
```

### Сохранить логи в файл (для отладки):

```bash
docker-compose -f deployments/docker-compose.yaml logs > /tmp/trading-bot-logs.txt
tar czf /tmp/trading-bot-logs.tar.gz /tmp/trading-bot-logs.txt
```

---

## Управление (Day 2 Operations)

### Остановить все сервисы:

```bash
make docker-down
# или: docker-compose -f deployments/docker-compose.yaml down
```

### Перезапустить сервисы:

```bash
make docker-down
make docker-up
```

### Обновить код и redeploy:

```bash
cd /opt/24alert
git pull origin main
make docker-build
make docker-up
```

### Проверить версию образов:

```bash
docker inspect 24alert-gateway | grep -i "image\|version"
```

### Масштабировать сервисы (опционально для Compose):

Редактировать `deployments/docker-compose.yaml` и добавить `deploy.replicas` (requires Docker Swarm или Kubernetes).

---

## Troubleshooting

| Проблема | Решение |
|----------|---------|
| **Port already in use** | Изменить порты в docker-compose.yaml или остановить другие сервисы: `sudo lsof -i :8080` |
| **Token invalid** | Проверить `TINVEST_PROD_TOKEN` в deployments/.env, убедиться что `TINVEST_SANDBOX=false` |
| **Container exits immediately** | `docker logs <container-name>` для деталей; часто — config.yaml path issue |
| **DNS resolution failed** | Проверить сеть: `docker network ls` и `docker network inspect trading-bot-net` |
| **Out of disk space** | Очистить старые образы: `docker image prune -a`, логи: `docker system prune` |
| **Health check timeout** | Увеличить `start_period` в docker-compose.yaml; может быть slow build |

---

## Rollback

Если что-то сломалось:

```bash
# 1. Остановить текущий стек
make docker-down

# 2. Вернуться к предыдущему коммиту (если нужно)
git log --oneline | head -5
git checkout <commit-hash>

# 3. Пересобрать и заново запустить
make docker-build
make docker-up

# 4. Проверить health
curl http://localhost:8080/health
```

---

## Security Notes

- **Never commit .env** — файл уже в `.gitignore`
- **Token rotation** — TINVEST_PROD_TOKEN должна ротироваться регулярно (см. T-Invest docs)
- **Logs may contain PII** — сохранение логов требует осторожности
- **Network isolation** — trading-bot-net — bridge network, изолирована от хоста

---

## Monitoring & Observability (Future)

- **Prometheus**: доступна в профиле `monitoring` (включить: `docker-compose --profile monitoring up -d`)
- **Alerting**: настроить через Prometheus Alert Manager
- **Logging**: можно интегрировать с ELK / Grafana Loki

---

**Deployment Date**: 2026-04-03  
**Version**: trading-bot (main branch)  
**Deployed by**: DevOps role (TASK-003)  
**Next Steps**: Передача на Tester для smoke testing
