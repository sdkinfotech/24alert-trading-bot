# Промпт: DevOps → TASK-003

## Контекст
Ты — **DevOps-инженер**. Задача — развернуть торгового робота на production-сервере srv03-cloud.

**Исходная постановка**: `.tasks/TASK-003/task.md`
**План**: `.tasks/TASK-003/plan.md`
**Сервер**: 176.123.160.234 (TASK-001, диагностирован как ready)
**Исходный код**: github.com/24alert/trading-bot (или локальный путь)

---

## Что делать

### 1. SSH на сервер и подготовка

```bash
ssh adm-srv03-cloud@176.123.160.234
cd /opt/24alert
```

Проверки:
- Docker запущен: `docker ps` (должен быть пустой список)
- Место на диске: `df -h` (должно быть >24 GB свободно)
- Порт 8080 свободен: `sudo netstat -tuln | grep 8080` (пусто = OK)

### 2. Git clone проекта

```bash
git clone https://github.com/24alert/trading-bot.git .
# или скопировать локально собранный код
```

Убедиться:
- `go.mod` есть
- `docker-compose.yaml` есть
- `config/` есть

### 3. Конфигурация .env

Создать `.env` файл (в корне репо):

```env
TINVEST_TOKEN=<production-token-от-пользователя>
TINVEST_SANDBOX=false
TINVEST_ACCOUNT_ID=
LOGGING_LEVEL=info
LOGGING_FORMAT=json
```

**Требования**:
- `.env` в `.gitignore` (не коммитить!)
- `TINVEST_TOKEN` не логировать
- `TINVEST_SANDBOX=false` для production

### 4. Проверка конфигурации

```bash
# Убедиться, что config.yaml и config.yaml.example согласованы
diff config/config.yaml config/config.yaml.example
```

### 5. Build Docker образов

```bash
make docker-build
# или
docker-compose build
```

Проверить статус:
```bash
docker images | grep 24alert
# Должны быть: gateway, order-svc, marketdata-svc, portfolio-svc, risk-svc
```

### 6. Запуск docker-compose

```bash
docker-compose up -d
```

Проверить статус:
```bash
docker-compose ps
# Все контейнеры должны быть в статусе "Up"
```

### 7. Health checks

```bash
# Gateway health check
curl http://localhost:8080/health

# Логи каждого сервиса
docker-compose logs gateway
docker-compose logs order-svc
docker-compose logs marketdata-svc
docker-compose logs portfolio-svc
docker-compose logs risk-svc
```

**Ожидаемые логи**: структурированные JSON с `level`, `msg`, `timestamp`, `service`.

### 8. Swagger доступность

```bash
curl http://localhost:8080/swagger/
# Должен вернуть HTML
```

Доступ извне:
```
http://176.123.160.234:8080/swagger/
```

### 9. Базовые REST API тесты

```bash
# Получить список счетов
curl -s http://localhost:8080/api/v1/accounts | jq .

# Получить портфель
curl -s http://localhost:8080/api/v1/portfolio | jq .

# Получить позиции
curl -s http://localhost:8080/api/v1/positions | jq .
```

### 10. Документирование

Создать файл `DEPLOYMENT.md` в корне проекта:

```markdown
# Deployment Guide

## Production Deployment

### Prerequisites
- Ubuntu 22.04, Docker 28.2.2+
- T-Invest API ключ (production account)
- SSH доступ к prod-серверу

### Steps
1. SSH на prod-сервер
2. Git clone репо в /opt/24alert
3. Создать .env с TINVEST_TOKEN
4. make docker-build
5. docker-compose up -d
6. curl http://localhost:8080/health

### Health Checks
- Gateway: http://localhost:8080/health
- Swagger: http://localhost:8080/swagger/

### Logs
docker-compose logs -f gateway

### Shutdown
docker-compose down
```

---

## Handoff

Создай `.tasks/TASK-003/devops/handoff.md`:
- Все контейнеры запущены и в статусе Up
- Health checks пройдены
- Swagger доступен
- Логи структурированы
- Документирован процесс
- Корректировки для тестировщика (порты, endpoints)
