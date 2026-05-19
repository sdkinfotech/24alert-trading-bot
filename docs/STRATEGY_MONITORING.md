# Мониторинг стратегии: как отслеживать работу и результаты

Инструкция для оператора `strategy-runner`: логи, HTTP API, Prometheus метрики, dashboard и текущие production futures-инстансы.

## Содержание

- [Быстрый чеклист (5 минут)](#быстрый-чеклист)
- [Web Dashboard](#web-dashboard)
- [Логи контейнера](#логи-контейнера)
- [Management HTTP API](#management-http-api)
- [Prometheus метрики](#prometheus-метрики)
- [Prometheus / Grafana](#prometheus--grafana)
- [Типичные сценарии](#типичные-сценарии)

## Быстрый чеклист

Минимальный набор проверок для оценки состояния стратегии (все команды выполняются на сервере через SSH):

```bash
ssh adm-srv03-cloud@176.123.160.234
```

| Что проверяем | Команда | Ожидаемый результат |
|---------------|---------|---------------------|
| Контейнер жив | `docker ps \| grep strategy` | `Up X minutes` |
| Healthcheck | `curl -s http://127.0.0.1:9020/health` | `{"status":"ok"}` |
| Инстанс работает | `curl -s http://127.0.0.1:9020/instances` | `"running": true` |
| PnL | `curl -s http://127.0.0.1:9020/instances/fut-gas-mini-sma/pnl` | JSON с realized/unrealized/total |
| Broker truth по позициям | `curl -s http://127.0.0.1:9020/instances/fut-gas-mini-sma/portfolio` | Позиции счёта T-Invest, filtered by strategy instrument |
| Runner ledger | `curl -s http://127.0.0.1:9020/instances/fut-gas-mini-sma/ledger` | Локальная книга quantities/avg_prices |
| Последние сделки | `curl -s 'http://127.0.0.1:9020/instances/fut-gas-mini-sma/executions?limit=5'` | Массив записей (или null если сделок ещё не было) |
| Дневной отчёт | `curl -s 'http://127.0.0.1:9020/report/daily?date=2026-05-15'` | Счётчики signals/orders/executions |
| Ошибки в логах | `docker logs --tail 50 24alert-strategy-runner \| grep -i error` | Пусто — всё хорошо |

## Web Dashboard

Встроенный React SPA-дашборд для визуализации работы стратегии, доступный в браузере.

**URL:** `http://127.0.0.1:9020/dashboard/` (через SSH-туннель: `http://localhost:9020/dashboard/`)

### Что показывает

- **Indicator Chart** — свечной график цены инструмента (OHLC). Для SMA-стратегий отображает линии Fast/Slow SMA; для ORB — горизонтальные пунктирные линии Range High (зелёная) / Range Low (красная). Маркеры сигналов: зелёная стрелка вверх = buy, красная стрелка вниз = sell. Фиолетовые точки `FILL` строятся из journal executions, поэтому остаются видимыми после restart, если исполнение было записано runner.
- **Protective trailing** — для SMA-стратегий с `trailing_stop_pct` dashboard показывает оранжевую пунктирную линию trailing stop. Срабатывание trailing отправляет market exit и закрывает позицию в `FLAT`, не переворачивая её.
- **Позиции и сверка источников** — отдельная панель broker truth / runner ledger / strategy state. Broker truth берётся напрямую из портфеля T-Invest и должен считаться главным источником активной позиции после restart.
- **Trade Event Log** — хронологическая лента событий (signals → signal cancelled → orders → executions) с цветовой кодировкой и фильтрами по типу. `Signal cancelled` означает, что стратегия сформировала торговую идею, но runner остановил её до заявки: например, weekend/session guard, risk rejection или ошибка отправки.
- **Stats Panel** — Total/Realized/Unrealized PnL, runner ledger, broker truth, дневная статистика.
- **Instance Selector** — выбор инстанса, статус (running/stopped).

Автообновление каждые 30 секунд.

### Доступ

**Через SSH-туннель (рекомендуется):**
```bash
ssh -L 9020:127.0.0.1:9020 adm-srv03-cloud@176.123.160.234
# Затем открыть http://localhost:9020/dashboard/
```

**Dev-режим (локальная разработка):**
```bash
cd web/strategy-dashboard
npm run dev
# Vite проксирует API-запросы на http://127.0.0.1:9020
```

### API-эндпоинты для дашборда

Помимо существующих (`/instances`, `/instances/{id}/pnl`, `/instances/{id}/ledger`, `/instances/{id}/executions`, `/report/daily`), добавлены:

| Эндпоинт | Описание |
|----------|----------|
| `GET /instances/{id}/indicator` | Данные индикатора: свечи с OHLC + Fast/Slow SMA, история сигналов |
| `GET /instances/{id}/portfolio` | Broker-side portfolio snapshot: активные позиции по счёту, отметка `in_instance`, expected yield, sync time |
| `GET /instances/{id}/signals?limit=N` | История сигналов из журнала |
| `GET /instances/{id}/events?limit=N` | Объединённый таймлайн (signals + signal_cancelled + orders + executions) |

Важно: при startup runner до подписки на новые свечи синхронизирует broker positions в runner ledger и strategy state. `/portfolio` остаётся главным внешним источником истины, а расхождение `/portfolio`, `/ledger` и `strategy state` после startup sync считается опасным состоянием, а не нормой.

### Сборка

```bash
make dashboard-build   # npm ci + npm run build + copy to go:embed
go build ./cmd/strategy-runner  # пересобрать бинарник с новым SPA
```

### Технологии

React 19 + TypeScript + Vite + Tailwind CSS + TradingView Lightweight Charts v5. SPA встроен в Go-бинарник через `go:embed`.

## Логи контейнера

### Просмотр в реальном времени

```bash
docker logs -f 24alert-strategy-runner
```

### Последние N строк

```bash
docker logs --tail 100 24alert-strategy-runner
```

### Что искать в логах

| Сообщение | Значение |
|-----------|----------|
| `strategy instance started` | Инстанс успешно стартовал, подписка на свечи активна. |
| `SUBSCRIPTION_STATUS_SUCCESS` | T-Invest подтвердил подписку на рыночные данные. |
| `order submitted from strategy` | Стратегия сгенерировала сигнал → risk пропустил → ордер отправлен. |
| `signal cancelled before order dispatch` | Сигнал отменён до отправки ордера; смотрите поля `stage`, `reason`, `message`, `candle_time`. |
| `signal cancelled before broker order` | Risk пропустил, но `PostOrder` не прошёл; ордер брокеру не ушёл. |
| `order_state=FILLED` | Ордер полностью исполнен. |
| `order_state=PARTIALLY_FILLED` | Ордер частично исполнен. |
| `order_state=CANCELLED` / `REJECTED` | Ордер отменён / отклонён биржей. |
| `watchdog: max loss exceeded, stopping instance` | Watchdog остановил инстанс из-за потерь. |
| `watchdog: drawdown exceeded, stopping instance` | Watchdog остановил инстанс из-за просадки. |
| `watchdog: stuck order` | Ордер висит дольше `stuck_order_minutes`. |
| `ledger reconciled from broker (drift)` | Позиция в ledger разошлась с данными брокера, скорректирована. |
| `GetPositions` (каждую минуту) | Watchdog тикает — жив, работает. |
| `market stream manager stopped` / `trades stream stopped` | Стрим оборвался, возможна потеря данных. |

### Фильтрация по уровню

```bash
# Только ошибки
docker logs 24alert-strategy-runner 2>&1 | grep '"level":"ERROR"'

# Предупреждения
docker logs 24alert-strategy-runner 2>&1 | grep '"level":"WARN"'

# Сигналы, отмены и ордера
docker logs 24alert-strategy-runner 2>&1 | grep -E 'signal|cancelled|order submitted|PostOrder'
```

### Логи с таймстемпами Docker

```bash
docker logs --timestamps --tail 50 24alert-strategy-runner
```

## Management HTTP API

Порт **9020** (только с хоста/VPN, не публикуется через nginx).

### Статус инстансов

```bash
curl -s http://127.0.0.1:9020/instances | jq
```

Ответ:
```json
[
  {
    "id": "fut-gas-mini-sma",
    "type": "sma_crossover",
    "account_id": "2001673385",
    "enabled_in_config": true,
    "running": true
  }
]
```

### PnL (прибыль/убыток)

```bash
curl -s http://127.0.0.1:9020/instances/fut-gas-mini-sma/pnl | jq
```

Ответ:
```json
{
  "instance_id": "fut-gas-mini-sma",
  "realized_rub": 0,
  "unrealized_rub": 0,
  "total_rub": 0
}
```

- **realized_rub** — зафиксированный PnL от закрытых позиций.
- **unrealized_rub** — бумажный PnL от открытых позиций (mark-to-market).
- **total_rub** — сумма realized + unrealized.

### Позиции (ledger)

```bash
curl -s http://127.0.0.1:9020/instances/fut-gas-mini-sma/ledger | jq
```

Ответ:
```json
{
  "instance_id": "fut-gas-mini-sma",
  "quantities": {},
  "avg_prices": {},
  "realized_rub": 0
}
```

Когда стратегия откроет позицию:
```json
{
  "quantities": {"962e2a95-...": 1},
  "avg_prices": {"962e2a95-...": 122.50},
  "realized_rub": 0
}
```

### Последние исполнения

```bash
curl -s 'http://127.0.0.1:9020/instances/fut-gas-mini-sma/executions?limit=10' | jq
```

Ответ — массив `ExecutionRecord`:
```json
[
  {
    "InstanceID": "fut-gas-mini-sma",
    "OrderID": "abc-123",
    "InstrumentUID": "962e2a95-...",
    "Status": "filled",
    "FilledQty": 1,
    "AvgPrice": 122.50,
    "Message": "order_state=FILLED cumulative_filled_lots=1",
    "CreatedAt": "2026-05-15T12:00:01Z"
  }
]
```

`null` — сделок ещё не было.

### Дневной отчёт

```bash
curl -s 'http://127.0.0.1:9020/report/daily?date=2026-05-15' | jq
```

Ответ:
```json
{
  "DayUTC": "2026-05-15T00:00:00Z",
  "SignalsCount": 0,
  "OrdersCount": 0,
  "ExecutionsCount": 0
}
```

За предыдущий день:
```bash
curl -s 'http://127.0.0.1:9020/report/daily?date=2026-05-14' | jq
```

### Остановка / запуск инстанса

```bash
# Остановка
curl -X POST http://127.0.0.1:9020/instances/fut-gas-mini-sma/stop

# Запуск (инстанс должен быть в конфиге)
curl -X POST http://127.0.0.1:9020/instances/fut-gas-mini-sma/start
```

## Prometheus метрики

Порт **9120**, эндпоинт `/metrics`. Namespace `alert24`, подсистема `strategy`.

### Просмотр из терминала

```bash
# Все метрики стратегии
curl -s http://127.0.0.1:9120/metrics | grep alert24_strategy

# Конкретная метрика
curl -s http://127.0.0.1:9120/metrics | grep total_pnl_rub
```

### Справочник метрик

| Метрика | Тип | Описание |
|---------|-----|----------|
| `alert24_strategy_signals_total{instance,direction}` | counter | Сигналы стратегии (buy/sell). |
| `alert24_strategy_orders_total{instance,status}` | counter | Ордера: `submitted`, `risk_rejected`, `risk_error`, `post_error`. |
| `alert24_strategy_evaluation_duration_seconds{instance}` | histogram | Время выполнения `OnCandle`. |
| `alert24_strategy_realized_pnl_rub{instance}` | gauge | Зафиксированный PnL (RUB). |
| `alert24_strategy_unrealized_pnl_rub{instance}` | gauge | Бумажный PnL (RUB). |
| `alert24_strategy_total_pnl_rub{instance}` | gauge | Суммарный PnL. |
| `alert24_strategy_win_rate{instance}` | gauge | Доля прибыльных сделок (0..1). |
| `alert24_strategy_trades_total{instance,result}` | counter | Количество win/loss исполнений. |
| `alert24_strategy_drawdown_percent{instance}` | gauge | Просадка от пика PnL (%). |
| `alert24_strategy_slippage_bps{instance}` | histogram | Проскальзывание vs ref-цена (bps). |
| `alert24_strategy_position_qty_shares{instance,instrument}` | gauge | Размер позиции в штуках. |
| `alert24_strategy_reconcile_mismatch_total{instance}` | counter | Дрейфы ledger vs брокер. |

### Полезные PromQL-запросы

```promql
# Текущий PnL
alert24_strategy_total_pnl_rub{instance="fut-gas-mini-sma"}

# Скорость сигналов за последний час
rate(alert24_strategy_signals_total{instance="fut-gas-mini-sma"}[1h])

# Просадка
alert24_strategy_drawdown_percent{instance="fut-gas-mini-sma"}

# Процент прибыльных сделок
alert24_strategy_win_rate{instance="fut-gas-mini-sma"}

# Средний slippage (bps) за последние 24ч
rate(alert24_strategy_slippage_bps_sum{instance="fut-gas-mini-sma"}[24h])
/ rate(alert24_strategy_slippage_bps_count{instance="fut-gas-mini-sma"}[24h])

# Количество ордеров по статусу
alert24_strategy_orders_total{instance="fut-gas-mini-sma"}

# Risk rejection rate
rate(alert24_strategy_orders_total{instance="fut-gas-mini-sma",status="risk_rejected"}[1h])
/ rate(alert24_strategy_orders_total{instance="fut-gas-mini-sma"}[1h])
```

## Prometheus / Grafana

**Текущий статус:** dedicated strategy Grafana dashboard развёрнут как `24alert Strategy Runner` (`uid: 24alert-strategy`). Strategy metrics доступны на `127.0.0.1:9120` на `srv03-cloud` и уходят в central Prometheus через `24alert-prometheus-agent`; общий monitoring-контур описан в Obsidian `24alert/Grafana`.

Артефакты:

- `monitoring/dashboards/24alert-strategy-runner.json`
- `monitoring/dashboards/24alert-gateway-api.json`
- `monitoring/dashboards/24alert-ai-scanner.json`
- `monitoring/dashboards/24alert-infrastructure.json`
- `monitoring/rules/24alert.yml`

### Workflow: strategy changes -> dashboard tiles

Цель: добавление/удаление strategy instance не должно требовать ручного редактирования Grafana.

Контракт:

- `24alert Strategy Runner` использует переменную Grafana `$strategy_instance`, источник `label_values(alert24_strategy_instance_enabled, exported_instance)`.
- Верхняя strategy-плитка является repeated panel по `$strategy_instance`; каждая новая метрика `exported_instance` автоматически создаёт отдельную плитку.
- Плитка считает readiness: `enabled * 2 + running`, где `3=RUNNING`, `2=STOPPED`, `1=CONFIG OFF / RUNNING`, `0=DISABLED`.
- После правки `config.yaml` обязательны `POST /config/reload`, ожидание scrape 20-40 секунд и smoke-check.

Инструмент проверки:

```bash
python3 scripts/monitoring/strategy_dashboard_smoke.py \
  --strategy-runner-url http://127.0.0.1:9020 \
  --prometheus-url http://213.171.27.217:9191 \
  --prometheus-user 24alert \
  --prometheus-password '<password>' \
  --grafana-url http://213.171.27.217:3535 \
  --grafana-user admin \
  --grafana-password '<password>'
```

AI scanner получает тот же инструмент внутри контейнера как `/usr/local/bin/strategy_dashboard_smoke.py` и dashboard JSON как `/workspace/24alert-strategy-runner.json`. Любой cron/agent run, который добавил или удалил strategy instance, обязан включить результат smoke-check в отчёт.

### Панели Strategy Runner

**Row 1 — Ключевые числа (stat)**

| Панель | Метрика | Пороги |
|--------|---------|--------|
| Total PnL (RUB) | `alert24_strategy_total_pnl_rub` | красный < 0, зелёный > 0 |
| Realized PnL | `alert24_strategy_realized_pnl_rub` | красный < 0, зелёный > 0 |
| Unrealized PnL | `alert24_strategy_unrealized_pnl_rub` | красный < 0, зелёный > 0 |
| Win Rate | `alert24_strategy_win_rate` | красный < 40%, зелёный > 55% |
| Drawdown % | `alert24_strategy_drawdown_percent` | зелёный < 5%, жёлтый < 10%, красный > 10% |
| Net Position (lots/contracts) | `alert24_strategy_position_qty_shares` | нейтральный; историческое имя метрики сохраняется |

**Row 2 — PnL и позиции (timeseries)**

- **Total PnL over time** — total/realized/unrealized на одном графике.
- **Position Qty over time** — чистая позиция по инструменту.

**Row 3 — Торговая активность**

- **Signals rate (buy/sell)** — столбчатая диаграмма, зелёный = buy, красный = sell.
- **Orders rate by status** — submitted / risk_rejected / post_error.
- **Wins** / **Losses** — stat-панели с абсолютными числами.
- **Total Signals** / **Total Orders** — stat-панели.

**Row 4 — Качество исполнения**

- **Slippage (bps)** — p50/p90/p99 проскальзывания.
- **Evaluation Duration (p50/p99)** — время выполнения OnCandle/gRPC Evaluate.
- **Reconcile Mismatches** — дрейфы ledger vs брокер (зелёный = 0, красный > 5).
- **Drawdown % over time** — с пунктирными линиями порогов.

**Row 5 — Логи**

- Production проверяется через dashboard logs panels и Loki query `{project="24alert",service="strategy-runner"}`; локально на VPS можно дополнительно использовать `docker logs 24alert-strategy-runner`.

### Переменные

- `$strategy_instance` — источник автоматических strategy tiles; обновляется из `alert24_strategy_instance_enabled`.
- `$instance` — фильтр графиков по инстансу стратегии (label `exported_instance`), поддерживает мульти-выбор и All.

### Как обновить дашборд

Strategy dashboard импортируется из `monitoring/dashboards/24alert-strategy-runner.json` в central Grafana. В центральном Prometheus strategy label называется `exported_instance`, потому что исходный `instance` конфликтует с scrape target label.

### Алерты (Alertmanager)

Файл: `monitoring/rules/24alert.yml`.
Правила включают schedule-aware `24alert_MarketDataStale`, `24alert_EnabledStrategyStopped`, cancelled signal alerts, AI scanner alerts и infrastructure alerts (`24alert_DiskAlmostFull`, `24alert_DiskCritical`, `24alert_ContainerStopped`, `24alert_DockerImagesBloat`).

Текущие правила:
```yaml
- alert: StrategyRunnerHighDrawdown
  expr: alert24_strategy_drawdown_percent > 15
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Strategy drawdown high (instance {{ $labels.instance }})"
```

### Мониторинговый стек

Monitoring profile на production должен держать три контейнера:

| Контейнер | Роль |
|-----------|------|
| `24alert-prometheus-agent` | Scrape метрик со всех сервисов → remote_write при настройке |
| `24alert-promtail` | Сбор Docker-логов → Loki при настройке |
| `24alert-ops-exporter` | Disk/Docker/container/version metrics для `24alert Infrastructure` |

Scrape targets (config: `config/prometheus-agent.yaml`):
- gateway:8080
- order-svc:9101
- marketdata-svc:9102
- portfolio-svc:9103
- risk-svc:9104
- **strategy-runner:9120**

## Telegram уведомления (план)

Если в `config/config.yaml` заданы `notifications.telegram.bot_token` и `chat_id`, runner автоматически шлёт сообщения:

| Событие | Пример сообщения |
|---------|-----------------|
| Watchdog остановил инстанс (потери) | `strategy-runner: instance fut-gas-mini-sma stopped (total PnL -520.00 < -500.00 RUB)` |
| Watchdog остановил инстанс (просадка) | `strategy-runner: instance fut-gas-mini-sma stopped (drawdown 12.3% > 10.0%)` |
| Зависший ордер | `stuck order: account=2001673385 order=abc-123 status=NEW age>30m` |
| Дрейф ledger | `ledger drift reconciled: instance=fut-gas-mini-sma instrument=962e... broker_qty=1.0000` |
| Ежедневный отчёт (~18:50 MSK) | `strategy-runner daily (UTC 2026-05-15): signals=3 orders=2 executions=2` |

## Типичные сценарии

### «Стратегия молчит, сигналов нет»

Это **нормально** для SMA crossover в первые ~20 часов: стратегия накапливает историю из `slow_period` (20) свечей перед первым сигналом. Для ORB сигналы возможны уже после формирования opening range (2-й свече), но только при пробое уровня, что происходит не каждый день.

Проверка:
```bash
# Сколько свечей получено (по логам):
docker logs 24alert-strategy-runner 2>&1 | grep -c "candle"

# Дневной счётчик сигналов:
curl -s 'http://127.0.0.1:9020/report/daily?date=2026-05-15' | jq .SignalsCount
```

### «Инстанс остановился»

```bash
# Проверить running
curl -s http://127.0.0.1:9020/instances | jq '.[].running'

# Посмотреть причину в логах
docker logs --tail 100 24alert-strategy-runner | grep -i 'stop\|error\|watchdog'

# Перезапустить
curl -X POST http://127.0.0.1:9020/instances/fut-gas-mini-sma/start
```

Типичные причины:
- Watchdog сработал (`max_daily_loss_rub`, `max_drawdown_percent`).
- Ошибка подписки на свечи (сеть, rate limit).
- Контейнер перезапустился (Docker restart policy).

### «Контейнер крашится»

```bash
# Статус и рестарты
docker inspect 24alert-strategy-runner --format '{{.RestartCount}} restarts, status: {{.State.Status}}'

# Полные логи (в т.ч. паника)
docker logs 24alert-strategy-runner 2>&1 | tail -100
```

### «Хочу посмотреть историю за неделю»

```bash
for d in $(seq 8 15); do
  echo "=== 2026-05-$d ==="
  curl -s "http://127.0.0.1:9020/report/daily?date=2026-05-$(printf '%02d' $d)"
  echo
done
```

### «Хочу проверить баланс счёта»

```bash
curl -s 'http://127.0.0.1:18080/api/v1/portfolio?account_id=2001673385' | jq
```

### Полный скрипт мониторинга (one-liner)

```bash
ssh adm-srv03-cloud@176.123.160.234 'echo "=== STATUS ===" && \
  docker ps --format "{{.Names}} {{.Status}}" | grep strategy && \
  echo "=== HEALTH ===" && \
  curl -s http://127.0.0.1:9020/health && echo && \
  echo "=== INSTANCES ===" && \
  curl -s http://127.0.0.1:9020/instances && echo && \
  echo "=== PNL ===" && \
  curl -s http://127.0.0.1:9020/instances/fut-gas-mini-sma/pnl && echo && \
  echo "=== LEDGER ===" && \
  curl -s http://127.0.0.1:9020/instances/fut-gas-mini-sma/ledger && echo && \
  echo "=== DAILY ===" && \
  curl -s "http://127.0.0.1:9020/report/daily?date=$(date -u +%Y-%m-%d)" && echo && \
  echo "=== LAST ERRORS ===" && \
  docker logs --tail 200 24alert-strategy-runner 2>&1 | grep -i error | tail -5'
```

## SSH-туннель для удобства

Чтобы не писать `ssh ...` перед каждой командой, можно открыть туннель:

```bash
ssh -L 9020:127.0.0.1:9020 -L 9120:127.0.0.1:9120 adm-srv03-cloud@176.123.160.234
```

Теперь с локальной машины:
```bash
curl -s http://localhost:9020/instances | jq
curl -s http://localhost:9120/metrics | grep alert24_strategy
```

## См. также

- [`docs/STRATEGY_RUNNER.md`](STRATEGY_RUNNER.md) — конфигурация, watchdog, типы стратегий.
- [`docs/INSTRUMENT_SELECTION.md`](INSTRUMENT_SELECTION.md) — подбор инструментов.
- [`monitoring/dashboards/24alert-strategy-runner.json`](../monitoring/dashboards/24alert-strategy-runner.json) — strategy dashboard.
- [`monitoring/rules/24alert.yml`](../monitoring/rules/24alert.yml) — production alert rules.
