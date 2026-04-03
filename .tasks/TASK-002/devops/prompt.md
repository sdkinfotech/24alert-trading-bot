# Промпт: DevOps → TASK-002

## Контекст
Ты — **DevOps-инженер**. Задача — контейнеризация 5 Go-микросервисов и настройка локального запуска через Docker Compose.

**Исходная постановка**: `.tasks/TASK-002/task.md`
**План**: `.tasks/TASK-002/plan.md`
**Handoff бэкенда**: `.tasks/TASK-002/backend/handoff.md`

---

## Что делать

### 1. Dockerfile для каждого сервиса

Единый шаблон multi-stage build:

```dockerfile
# Stage 1: Build
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/service ./cmd/<service-name>/main.go

# Stage 2: Run
FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /bin/service /bin/service
COPY config/ /etc/24alert/
EXPOSE <port>
ENTRYPOINT ["/bin/service"]
```

Сервисы и порты:
- `Dockerfile.gateway` → порт 8080
- `Dockerfile.order-svc` → порт 9001
- `Dockerfile.marketdata-svc` → порт 9002
- `Dockerfile.portfolio-svc` → порт 9003
- `Dockerfile.risk-svc` → порт 9004

Расположение: `deployments/docker/`

### 2. docker-compose.yaml

```yaml
# Все 5 сервисов + общая сеть
# Environment: TINVEST_TOKEN из .env
# Volumes: config/ маппинг
# Depends_on: gateway зависит от 4 сервисов
# Health checks: HTTP /health для gateway, gRPC health для остальных
```

Расположение: `deployments/docker-compose.yaml`

Требования:
- `.env` файл с `TINVEST_TOKEN` (не коммитить!)
- Сеть: все сервисы видят друг друга по имени (order-svc, marketdata-svc, etc.)
- Gateway маппит порт 8080 наружу
- Все сервисы логируют в stdout (JSON)

### 3. Makefile targets

Добавить в корневой `Makefile`:
```makefile
docker-build:    # Build all images
docker-up:       # docker-compose up -d
docker-down:     # docker-compose down
docker-logs:     # docker-compose logs -f
docker-clean:    # Remove images and volumes
```

### 4. .env.example

```env
TINVEST_TOKEN=your-token-here
TINVEST_SANDBOX=true
```

### 5. README секция

Добавить в README.md секцию "Docker Quick Start":
```markdown
## Docker Quick Start
1. cp .env.example .env
2. Вставить TINVEST_TOKEN
3. make docker-build
4. make docker-up
5. Swagger: http://localhost:8080/swagger/
```

---

## Зависимости
- Начинай после Phase 1 бэкенда (scaffold + go.mod готовы)
- Можешь работать параллельно с Phase 2-3

## Handoff
Создай `.tasks/TASK-002/devops/handoff.md`:
- Список Dockerfile'ов
- docker-compose.yaml статус
- Инструкция запуска
- Корректировки для тестировщика (как поднять стек для тестов)
