# Промпт: Backend → TASK-004

## Контекст
Ты — **Senior Backend-разработчик**. В этой задаче твоя роль — подготовить код к CI/CD: добавить тесты, настроить linting, убедиться, что всё компилируется и проходит проверки.

**Исходная постановка**: `.tasks/TASK-004/task.md`
**План**: `.tasks/TASK-004/plan.md`

---

## Что делать

### 1. Убедиться, что код компилируется

```bash
cd c:\Users\sdk\proj\24alert
go build ./cmd/gateway
go build ./cmd/order-svc
go build ./cmd/marketdata-svc
go build ./cmd/portfolio-svc
go build ./cmd/risk-svc
```

Все должны собраться без ошибок.

### 2. Добавить тесты (если их нет)

В каждом пакете должны быть `*_test.go` файлы.

**Минимум**: Unit tests для `service.go` каждого сервиса.

```bash
go test ./...
go test -v ./...
go test -cover ./...  # показать покрытие
```

**DoD**: Все тесты пройдены, покрытие >70%.

### 3. Настроить linting

Установить `golangci-lint`:

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

Создать `.golangci.yml`:

```yaml
linters:
  enable:
    - vet
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - unused
    - misspell
    - gofmt
    - goimports

issues:
  exclude-dirs:
    - vendor
    - gen
```

Запустить:

```bash
golangci-lint run ./...
```

**DoD**: Нет лinting errors.

### 4. Создать build script

Файл `scripts/ci-test.sh`:

```bash
#!/bin/bash
set -e

echo "Running tests..."
go test -v -cover ./...

echo "Running linter..."
golangci-lint run ./...

echo "Building binaries..."
go build -o bin/gateway ./cmd/gateway
go build -o bin/order-svc ./cmd/order-svc
go build -o bin/marketdata-svc ./cmd/marketdata-svc
go build -o bin/portfolio-svc ./cmd/portfolio-svc
go build -o bin/risk-svc ./cmd/risk-svc

echo "✓ All checks passed"
```

Сделать исполняемым:

```bash
chmod +x scripts/ci-test.sh
```

### 5. Обновить Makefile

Добавить targets:

```makefile
.PHONY: test lint build-all

test:
	go test -v -cover ./...

lint:
	golangci-lint run ./...

build-all:
	go build -o bin/gateway ./cmd/gateway
	go build -o bin/order-svc ./cmd/order-svc
	go build -o bin/marketdata-svc ./cmd/marketdata-svc
	go build -o bin/portfolio-svc ./cmd/portfolio-svc
	go build -o bin/risk-svc ./cmd/risk-svc

ci-check: lint test build-all
	@echo "✓ CI checks passed"
```

### 6. Handoff

Создай `.tasks/TASK-004/backend/handoff.md`:

```markdown
# Handoff: Backend → TASK-004

## Статус
DONE

## Что сделано
- ✅ Все тесты пройдены (go test -v -cover ./...)
- ✅ Linting пройден (golangci-lint run ./..., 0 errors)
- ✅ Код компилируется все 5 сервисов (gateway, order-svc, marketdata-svc, portfolio-svc, risk-svc)
- ✅ Build script создан (scripts/ci-test.sh)
- ✅ Makefile targets добавлены (test, lint, build-all, ci-check)
- ✅ Coverage >= 70%

## Артефакты
- Файлы: .golangci.yml, scripts/ci-test.sh, updated Makefile
- Коммиты: [commit hashes for test additions/linting setup]

## Корректировки для следующей роли (DevOps)
`make ci-check` должна быть первым шагом в GitHub Actions workflow. Команда успешно выполняется за < 2 минуты на Ubuntu.

## Блокеры
НЕТ
```

---

## ✅ Успешное завершение

Когда handoff готов → **передаёшь задачу DevOps'у**:

**Следующий шаг**:
- Backend done ✅
- **→ DevOps начинает** (`.tasks/TASK-004/devops/prompt.md`)

**DevOps будет**:
- Создавать GitHub Actions workflow
- Настраивать секреты
- Писать deploy скрипты
- Использовать твой `make ci-check` как first step

---

## 📋 Чек-лист перед handoff

- [ ] `go test -v -cover ./...` проходит с coverage >= 70%
- [ ] `golangci-lint run ./...` возвращает 0 ошибок
- [ ] `make ci-check` выполняется полностью без ошибок
- [ ] `scripts/ci-test.sh` исполняемый и работает
- [ ] Все файлы закоммичены (`.golangci.yml`, script, Makefile обновлён)
- [ ] handoff.md написан с указанием корректировок для DevOps
- [ ] Git push на origin main
