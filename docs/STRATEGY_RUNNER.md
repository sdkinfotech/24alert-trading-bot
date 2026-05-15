# Strategy runner (`strategy-runner`)

Отдельный процесс **`cmd/strategy-runner`**: подписка на свечи T-Invest → оценка стратегии → проверка **risk** → выставление **ордеров** → обратная связь по **стриму состояний ордеров** и **стриму сделок**. Опционально: журнал в **SQLite**, **ledger** позиций, бизнес-**метрики**, **watchdog**, уведомления в **Telegram**.

## Содержание

- [Назначение](#назначение)
- [Запуск](#запуск)
  - [Один инстанс через CLI](#один-инстанс-через-cli-без-правки-yaml)
- [Конфигурация](#конфигурация)
- [Управляющий HTTP API](#управляющий-http-api)
- [Журнал и состояние стратегии](#журнал-и-состояние-стратегии)
- [Ledger, PnL, slippage](#ledger-pnl-slippage)
- [Watchdog](#watchdog)
- [Telegram](#telegram)
- [Prometheus и Grafana](#prometheus-и-grafana)
- [Типы стратегий](#типы-стратегий)
- [Ограничения](#ограничения)

## Назначение

- Один бинарник держит общий T-Invest клиент, **market data** (свечи), **order** stream (состояния + трейды для средней цены исполнения), **portfolio** (реконсиляция с брокером).
- Несколько **инстансов** (`strategies.instances[]`): у каждого свой `id`, `type`, счёт, список инструментов, параметры.

## Запуск

### Локально

```bash
go run ./cmd/strategy-runner --config config/config.yaml
```

### Один инстанс через CLI (без правки YAML)

Если заданы **`--strategy-account`** и **`--strategy-instrument-uid`**, блок `strategies.instances` из файла **полностью заменяется** одним включённым инстансом (удобно для первого боевого запуска без коммита счёта/UID в git).

```bash
# Прод-контур (боевой токен)
export TINVEST_SANDBOX=false
export TINVEST_PROD_TOKEN="t.***"

go run ./cmd/strategy-runner --config config/config.yaml \
  --strategy-account "ВАШ_СЧЁТ" \
  --strategy-instrument-uid "UID_ИНСТРУМЕНТА" \
  --strategy-interval "1h" \
  --strategy-quantity "1" \
  --strategy-instance-id "live-sma-1"
```

gRPC-стратегия:

```bash
go run ./cmd/strategy-runner --config config/config.yaml \
  --strategy-account "ВАШ_СЧЁТ" \
  --strategy-instrument-uid "UID" \
  --strategy-type grpc \
  --strategy-grpc-endpoint "127.0.0.1:9010"
```

UID инструмента возьмите из API T-Invest (`InstrumentByUid` / поиск по тикеру) или из ответов gateway (`/api/v1/prices?instrument_uid=...`). Счёт — из `GET /api/v1/accounts` (на проде часто только с хоста/VPN/SSH).

**Риск:** `sma_crossover` на реальном счёте выставляет **рыночные** заявки по сигналу. Сначала проверьте **sandbox** (`TINVEST_SANDBOX=true`, `TINVEST_SANDBOX_TOKEN`) или счёт с минимальным остатком.

Порты по умолчанию из `config/config.yaml`:

- **9020** — управляющий HTTP (`runner_port`)
- **9120** — только `/metrics` (`metrics_port`)

### Docker Compose

Сервис **`strategy-runner`** в [`deployments/docker-compose.yaml`](../deployments/docker-compose.yaml) использует тот же `deployments/.env`, что и остальные сервисы (токены T-Invest, `LOG_LEVEL`).

## Конфигурация

Раздел **`strategies:`** в [`config/config.yaml`](../config/config.yaml).

| Поле | Описание |
|------|-----------|
| `runner_port` | Порт HTTP управления (по умолчанию 9020). |
| `metrics_port` | Порт только с `/metrics` (по умолчанию 9120). |
| `evaluation_timeout_ms` | Таймаут вызова стратегии (в т.ч. gRPC `Evaluate`). |
| `journal_path` | Путь к файлу SQLite журнала, если включён `features.enable_order_journal`. Пустой → `data/strategy_journal.db`. |
| `watchdog` | Периодические проверки (см. [Watchdog](#watchdog)). |
| `notifications.telegram` | `bot_token`, `chat_id` для алертов (см. [Telegram](#telegram)). |
| `instances[]` | Список инстансов (см. ниже). |

### Инстанс (`instances[]`)

| Поле | Описание |
|------|-----------|
| `id` | Уникальный идентификатор инстанса. |
| `type` | Встроенный тип (например `sma_crossover`) или `grpc`. |
| `account_id` | Счёт T-Invest для ордеров. |
| `instruments` | Список UID инструментов. |
| `enabled` | Если `false`, инстанс не стартует при запуске процесса. |
| `params` | Строковые параметры стратегии (периоды SMA, `interval` свечей и т.д.). |
| `endpoint` | Для `type: grpc` — адрес gRPC-сервера стратегии. |

Пример фрагмента:

```yaml
strategies:
  runner_port: 9020
  metrics_port: 9120
  evaluation_timeout_ms: 5000
  journal_path: "data/strategy_journal.db"
  watchdog:
    enabled: true
    check_interval_sec: 60
    max_drawdown_percent: 15
    max_daily_loss_rub: 50000
    pause_on_drawdown: true
    stuck_order_minutes: 30
  notifications:
    telegram:
      bot_token: ""   # лучше через env, не коммитить
      chat_id: ""
  instances:
    - id: demo-sma
      type: sma_crossover
      account_id: "YOUR_ACCOUNT_ID"
      instruments: ["INSTRUMENT_UID"]
      enabled: true
      params:
        interval: "1h"
        fast_period: "5"
        slow_period: "20"
        quantity: "1"
```

### Feature flags (`features:`)

| Флаг | Значение для runner |
|------|---------------------|
| `enable_order_journal` | `true` — SQLite по `journal_path`: сигналы, ордера, исполнения, снимок состояния `StatefulStrategy`. |
| `enable_risk_validation` | `true` — перед `PostOrder` вызывается `ValidateOrderIntent` (сессия, баланс, лимит позиции, circuit breaker). |

## Управляющий HTTP API

Базовый URL: `http://127.0.0.1:<runner_port>/` (в Docker — по публикации порта контейнера).

| Метод и путь | Описание |
|--------------|----------|
| `GET /health` | `{"status":"ok"}`. |
| `GET /instances` | Список инстансов из конфига + флаг `running`. |
| `POST /instances/{id}/start` | Старт инстанса по `id` из конфига (если ещё не запущен). |
| `POST /instances/{id}/stop` | Остановка: отмена контекста свечей, снимок `StatefulStrategy` в журнал, удаление ledger инстанса. |
| `GET /instances/{id}/pnl` | Реализованный / нереализованный / суммарный PnL (RUB), только для **запущенного** инстанса. |
| `GET /instances/{id}/ledger` | Карты количеств (акции), средних цен входа, накопленный realized. |
| `GET /instances/{id}/executions?limit=N` | Последние записи исполнений из журнала (`limit` 1…5000, по умолчанию 50). |
| `GET /report/daily?date=YYYY-MM-DD` | Агрегат журнала за UTC-сутки: число сигналов, ордеров, исполнений. |

Ответы — JSON. Ошибки — текст в теле и HTTP-код (например 404, если инстанс не в `running` для `/pnl` и `/ledger`).

## Журнал и состояние стратегии

При `features.enable_order_journal: true` открывается SQLite по `strategies.journal_path`.

Таблицы (схема в `internal/journal/sqlite.go`):

- **signals** — сигналы стратегии (в т.ч. ref price).
- **orders** — связь `order_id` ↔ инстанс, направление, тип ордера.
- **executions** — события исполнения (статус, накопленное кол-во лотов, средняя цена).
- **strategy_state** — последний blob от `StatefulStrategy.Snapshot()` по `instance_id`.

Встроенный пример **`sma_crossover`** реализует `StatefulStrategy`: при остановке инстанса состояние сохраняется; при старте — загружается после `Configure`.

## Ledger, PnL, slippage

- **Ledger** (`internal/strategy/ledger`): позиция в **штуках инструмента** (акции), средняя цена входа, накопленный **realized** PnL в RUB по сделкам закрытия/частичного закрытия (best-effort от цен исполнения).
- **Unrealized** считается от последних цен (`PriceCache`) и средних входов (`internal/strategy/pnl`).
- **Slippage (bps)** — относительно опорной цены на момент сигнала (mark или лимитная цена), см. `runner_exec.go`.

Проверка риска **`position_limit`** учитывает направление: покупка увеличивает позицию, продажа уменьшает; лимит действует симметрично по модулю (не допускается превышение `±max_position_lots` в штуках позиции с брокера + дельта ордера — см. `internal/risk/checker/position_limit.go`).

## Watchdog

Включается `strategies.watchdog.enabled: true`.

| Поле | Назначение |
|------|------------|
| `check_interval_sec` | Период тика (не меньше 10 с; если меньше — принудительно 60 с). |
| `max_drawdown_percent` | Порог просадки от пика суммарного PnL (в процентах). |
| `max_daily_loss_rub` | Остановка инстанса, если суммарный PnL (realized + unrealized) строго меньше `−max_daily_loss_rub`. `0` — проверка отключена. |
| `pause_on_drawdown` | При превышении просадки — `StopInstance`. |
| `stuck_order_minutes` | Возраст «зависшего» активного ордера для предупреждения в лог и Telegram. |

На каждом тике: реконсиляция ledger с позициями портфеля по инструментам инстанса; проверка лимитов; проверка зависших ордеров по счетам запущенных инстансов; раз в сутки (UTC, окно ~15:50) — краткий daily-отчёт в Telegram при наличии клиента.

## Telegram

`pkg/notify/telegram`: если заданы непустые `bot_token` и `chat_id`, runner шлёт текстовые сообщения (дрейф ledger, остановка по лимитам, stuck orders, daily summary).

**Не коммить** токены; для продакшена предпочтительно подставлять через переменные окружения и шаблонизацию конфига (viper `AutomaticEnv` / отдельный секрет-файл вне git).

## Prometheus и Grafana

Метрики на отдельном listener (`metrics_port`), namespace **`alert24`**, подсистема **`strategy`**:

- `alert24_strategy_signals_total{instance,direction}`
- `alert24_strategy_orders_total{instance,status}`
- `alert24_strategy_evaluation_duration_seconds{instance}`
- `alert24_strategy_realized_pnl_rub{instance}`
- `alert24_strategy_unrealized_pnl_rub{instance}`
- `alert24_strategy_total_pnl_rub{instance}`
- `alert24_strategy_win_rate{instance}`
- `alert24_strategy_trades_total{instance,result}` (`win` / `loss`)
- `alert24_strategy_drawdown_percent{instance}`
- `alert24_strategy_slippage_bps{instance}`
- `alert24_strategy_position_qty_shares{instance,instrument}`
- `alert24_strategy_reconcile_mismatch_total{instance}`

Артефакты «as code»:

- Дашборд: [`deployments/grafana/dashboards/strategy-overview.json`](../deployments/grafana/dashboards/strategy-overview.json)
- Правила алертов: [`deployments/grafana/alerts/strategy-runner.rules.yml`](../deployments/grafana/alerts/strategy-runner.rules.yml) (пример: высокая просадка по `alert24_strategy_drawdown_percent`).

## Типы стратегий

### Встроенные (`internal/strategy`, регистрация в `Registry`)

| Тип | Пакет | Описание | Параметры |
|-----|-------|----------|-----------|
| `sma_crossover` | `internal/strategy/sma` | Пересечение быстрой/медленной SMA по завершённым свечам | `fast_period`, `slow_period`, `quantity`, `interval` |
| `orb_breakout` | `internal/strategy/orb` | Intraday Opening Range Breakout: вход при пробое диапазона открытия, выход до конца сессии | `range_candles`, `quantity`, `interval`, `cutoff_hour`, `cutoff_min`, `timezone` |

**`orb_breakout` — Opening Range Breakout:**
1. **Наблюдение (первые N свечей):** Запоминает High и Low первых `range_candles` свечей дня — формирует "opening range".
2. **Торговля:** При закрытии свечи выше Range High → BUY; ниже Range Low → SELL. При пробое в обратную сторону — разворот позиции (2x qty).
3. **Закрытие (EOD):** После `cutoff_hour`:`cutoff_min` все позиции закрываются рыночным ордером, новые не открываются. На новый день — полный сброс.

Контракт интерфейса: `internal/strategy/strategy.go` (`OnCandle`, `OnExecution`, …). Опционально: `internal/strategy/stateful.go` (`Snapshot` / `Restore`).

### Внешние gRPC (`type: grpc`)

Контракт: `proto/strategy/v1/strategy.proto`. Адаптер: `internal/strategy/grpcadapter`. В `instances[].endpoint` указывается хост:порт gRPC-сервера.

## Ограничения

- PnL и ledger — **оценочные**: без полного учёта комиссий, НКД, валютных позиций; цены исполнения берутся из стрима сделок + репозитория исполнений.
- **E2E** тесты в `tests/e2e` требуют доступный gateway по `API_BASE_URL`; для проверки только runner используйте `go test ./internal/... ./pkg/...`.
- Публичный nginx у продакшена может **не проксировать** большинство REST-путей gateway; управление **strategy-runner** обычно с хоста/VPN по опубликованному порту или SSH-туннелю — согласуйте с вашей схемой деплоя.

## Первый боевой запуск (пример: ВТБ на ИИС)

Полный сценарий, выполненный 2026-05-15:

1. **Счёт:** ИИС `2001673385`, баланс 2 000 RUB, маржа недоступна.
2. **Инструмент:** ВТБ (VTBR) `8e2b0325-0292-4654-8a18-4f63ed3b0e09`, ~122 RUB/лот, лот = 1 акция. (UID `962e2a95-…` в старых черновиках — это **GAZP**, не путать.)
3. **Критерии подбора:** цена лота вписывается в баланс; тысячи лотов в стакане; `api_trade_available: true`; спред 1 коп.
4. **Конфигурация:** `config/config.yaml` → `strategies.instances` с `iis-vtb-sma`, `quantity: 1`, `interval: 1h`.
5. **Watchdog:** `max_drawdown_percent: 10`, `max_daily_loss_rub: 500`, `pause_on_drawdown: true`.
6. **Деплой:** `git push` → `git pull` на сервере → `docker compose build strategy-runner` → `docker compose up -d`.
7. **Проверка:** `curl http://127.0.0.1:9020/instances` → `running: true`; логи: `strategy instance started`, `SUBSCRIPTION_STATUS_SUCCESS`.

Подробно о том, как подбирать инструменты: [`docs/INSTRUMENT_SELECTION.md`](INSTRUMENT_SELECTION.md).

## Web Dashboard

Встроенный React SPA для визуализации индикатора, сигналов и торговых событий.

**Доступ:** `http://127.0.0.1:9020/dashboard/` (management порт, через SSH-туннель).

Подробнее: [`docs/STRATEGY_MONITORING.md` → Web Dashboard](STRATEGY_MONITORING.md#web-dashboard).

## AI Assistant

### Архитектура

Система включает два AI-компонента:

| Компонент | Назначение | Провайдер | Модель |
|-----------|-----------|-----------|--------|
| **AI Scanner** (cron) | Автономный анализ рынка, бэктест, управление стратегиями | Cursor Agent CLI | `composer-2` |
| **AI Chat** (дашборд) | Интерактивный ассистент в дашборде | OpenRouter | `anthropic/claude-sonnet-4` |

### AI Chat в дашборде

Кнопка чата (синий кружок) отображается в правом нижнем углу дашборда. Ассистент видит текущее состояние стратегий, PnL и позиции. Может помочь с:
- Объяснением текущего состояния стратегий
- Рекомендациями по параметрам
- Анализом торговых результатов

API-эндпоинты (strategy-runner :9020):
- `POST /ai-chat` — отправить сообщение (JSON: `{"message": "..."}`)
- `POST /ai-chat/reset` — сбросить историю диалога
- `GET /ai-chat/status` — статус (доступность, модель, cron)

### AI Scanner (автономный)

Docker-контейнер `ai-scanner` выполняет cron-задачи (МСК):
- **Ночной скан** (02:00 вт-сб) — сканирование рынка, бэктест, отбор и настройка стратегий на следующий торговый день
- **Pre-market** (09:50 пн-пт) — проверка готовности к торговле, health-check, запуск незапущенных стратегий
- **Health** (каждые 4ч) — health-check системы

### Управление API-ключами

Все ключи хранятся в `deployments/.env` (не коммитится в git):

```bash
# === AI Scanner (Cursor Agent CLI) ===
CURSOR_API_KEY=crsr_...          # Cursor API Key
AI_SCANNER_MODEL=composer-2      # Модель для автономного агента

# === AI Chat (OpenRouter) ===
OPENROUTER_API_KEY=sk-or-v1-...  # OpenRouter API Key
AI_CHAT_MODEL=anthropic/claude-sonnet-4  # Модель для чата
```

**Как поменять ключи:**

1. **Cursor API Key** — генерируется на https://cursor.com/dashboard/integrations
   ```bash
   # На сервере:
   cd /root/24alert/deployments
   nano .env                      # изменить CURSOR_API_KEY=crsr_NEW_KEY
   docker compose --profile ai-scanner up -d ai-scanner  # перезапустить
   ```

2. **OpenRouter API Key** — генерируется на https://openrouter.ai/keys
   ```bash
   cd /root/24alert/deployments
   nano .env                      # изменить OPENROUTER_API_KEY=sk-or-v1-NEW_KEY
   docker compose up -d --no-deps strategy-runner  # перезапустить
   ```

3. **Смена модели чата:**
   ```bash
   # Изменить AI_CHAT_MODEL в .env
   # Доступные модели OpenRouter: https://openrouter.ai/models
   # Примеры: anthropic/claude-sonnet-4, openai/gpt-4.1, google/gemini-2.5-flash
   ```

4. **Смена модели Scanner:**
   ```bash
   # Изменить AI_SCANNER_MODEL в .env
   # Доступные модели Cursor: composer-2, claude-4-sonnet-thinking, gpt-5.2
   ```

## Связанные файлы

| Область | Путь |
|---------|------|
| Точка входа | `cmd/strategy-runner/main.go` |
| Оркестрация | `internal/strategy/runner.go` |
| Исполнения, метрики, watchdog-хелперы | `internal/strategy/runner_exec.go`, `runner_api.go` |
| HTTP админка | `internal/strategy/admin.go` |
| AI Chat handler | `internal/strategy/aichat.go` |
| Web Dashboard (SPA) | `web/strategy-dashboard/` |
| Dashboard embed | `internal/strategy/dashboard.go` |
| AI Scanner | `deployments/ai-scanner/` |
| Журнал | `internal/journal/` |
| Ledger | `internal/strategy/ledger/` |
| Конфиг типов | `pkg/config/config.go` |
| Подбор инструментов | [`docs/INSTRUMENT_SELECTION.md`](INSTRUMENT_SELECTION.md) |
| Мониторинг и метрики | [`docs/STRATEGY_MONITORING.md`](STRATEGY_MONITORING.md) |
