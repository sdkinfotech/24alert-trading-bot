# Handoff: DevOps → TASK-004 (Production Fix + BUG-CI-004)

## Статус
**DONE** ✅

---

## Что сделано

### BUG-CI-004: Production Server Was Down — FIXED

**Root Cause Analysis** (несколько проблем, решены последовательно):

1. **`.gitignore` блокировал исходный код** — паттерны `gateway`, `order-svc` и т.д. в `.gitignore` интерпретировались git как `**/gateway`, `**/order-svc`, что исключало директории `cmd/gateway/`, `cmd/order-svc/`, `internal/gateway/` из git. Код никогда не попадал в репозиторий.
   - **Fix**: Изменены на `/gateway`, `/order-svc` (root-only match).

2. **`go.sum` отсутствовал в репо** — был в `.gitignore`. Docker build `COPY go.mod go.sum ./` падал.
   - **Fix**: Убран `go.sum` из `.gitignore`, закоммичен.

3. **`git` отсутствовал в Alpine builder** — `invest-api-go-sdk` требует git для `go mod download`.
   - **Fix**: Добавлен `RUN apk --no-cache add git` в Dockerfile builder stage.

4. **TLS: Russian CA missing** — T-Invest API (`invest-public-api.tbank.ru`) использует сертификат подписанный МинЦифры CA. Стандартный Alpine CA-bundle не содержит российских корневых CA.
   - **Fix**: Переключение с `alpine:3.19` на `debian:bookworm-slim` + скачивание Russian Trusted Root CA и Sub CA с `gu-st.ru`.

5. **`netcat` отсутствовал в runtime image** — health check в docker-compose использует `nc -z localhost PORT`, но debian-slim не содержит netcat.
   - **Fix**: Добавлен `netcat-openbsd` + `wget` в Dockerfile runtime stage.

6. **Docker Compose plugin не установлен на сервере** — `docker-compose` (v1) отсутствовал, `docker compose` plugin тоже.
   - **Fix**: Установлен Docker Compose v5.1.1 plugin.

7. **`make` не установлен на сервере**.
   - **Fix**: `sudo apt-get install make`.

8. **`/opt/24alert/` не существовала** — код никогда не клонировался на сервер.
   - **Fix**: `mkdir -p /opt/24alert && git clone`.

### Commits pushed to fix:

| Commit | Description |
|--------|------------|
| `2c46e18` | fix: add missing source files, fix .gitignore, add CI/CD pipeline (54 files) |
| `e6fe567` | fix: upgrade alpine to 3.21 and run update-ca-certificates for TLS |
| `450eb40` | fix: copy CA certs from builder stage for TLS verification |
| `cdf5048` | fix: switch to debian-slim + add Russian Trusted CA for T-Invest API TLS |
| `ca2b030` | fix: add netcat and wget to runtime image for health checks |

### Server Setup (srv03-cloud):

| Action | Status |
|--------|--------|
| Docker Compose v5.1.1 installed | ✅ |
| make 4.3 installed | ✅ |
| `/opt/24alert` created | ✅ |
| Code cloned from GitHub | ✅ |
| `deployments/.env` configured (TINVEST_SANDBOX=false) | ✅ |
| Docker images built (5 services) | ✅ |
| `docker compose up -d` | ✅ |
| Health checks passed | ✅ |

### Production Status:

```
NAME                     STATUS
24alert-order-svc        Up (healthy)     :9001
24alert-marketdata-svc   Up (healthy)     :9002
24alert-portfolio-svc    Up (healthy)     :9003
24alert-risk-svc         Up (healthy)     :9004
24alert-gateway          Up               :8080
```

### Verified Endpoints:

```bash
curl http://176.123.160.234:8080/health
# {"status":"ok"}

curl http://176.123.160.234:8080/api/v1/accounts
# {"data":[{"id":"da400e4b-...","type":"ACCOUNT_TYPE_TINKOFF","status":"ACCOUNT_STATUS_OPEN",...}]}
```

---

## Артефакты

### Файлы изменённые/созданные:

| File | Change | Purpose |
|------|--------|---------|
| `.gitignore` | MODIFIED | Fix binary patterns blocking source dirs |
| `deployments/Dockerfile` | MODIFIED | debian-slim + Russian CA + netcat + wget |
| `go.sum` | ADDED | Required for Docker build |
| `cmd/gateway/main.go` | ADDED | Previously git-ignored |
| `cmd/order-svc/main.go` | ADDED | Previously git-ignored |
| `cmd/marketdata-svc/main.go` | ADDED | Previously git-ignored |
| `cmd/portfolio-svc/main.go` | ADDED | Previously git-ignored |
| `cmd/risk-svc/main.go` | ADDED | Previously git-ignored |
| `internal/gateway/**` | ADDED | Previously git-ignored |
| `.github/workflows/deploy.yml` | ADDED | CI/CD pipeline |
| `scripts/deploy-prod.sh` | ADDED | Deploy automation |
| `scripts/rollback-prod.sh` | ADDED | Rollback automation |
| `.github/DEPLOYMENT.md` | ADDED | Deployment guide |

### Server artifacts:

| Item | Location |
|------|----------|
| Docker Compose plugin | `/usr/local/lib/docker/cli-plugins/docker-compose` |
| Project code | `/opt/24alert/` |
| Production .env | `/opt/24alert/deployments/.env` |
| Docker images | `deployments-gateway:latest`, `deployments-*:latest` |

---

## Корректировки для следующих ролей

### Для Тестировщика:

- **BUG-CI-004 закрыт** — `http://176.123.160.234:8080` теперь доступен
- Можно выполнять E2E тесты (TEST 1-11 из prompt.md)
- API эндпоинты работают с production T-Invest аккаунтом
- Swagger UI: `http://176.123.160.234:8080/swagger/`

### Для Техлида:

- Dockerfile переведён с Alpine на Debian-slim из-за Russian CA requirements
- Размер runtime image увеличился (~80 MB → ~150 MB) — trade-off для TLS совместимости
- Russian Trusted CA загружается с `gu-st.ru` — официальный источник МинЦифры

---

## Блокеры
**НЕТ** ✅
