# Мониторинг стратегии: как отслеживать работу и результаты

Инструкция для оператора `strategy-runner`: логи, HTTP API, Prometheus метрики, Grafana, Telegram.

## Содержание

- [Быстрый чеклист (5 минут)](#быстрый-чеклист)
- [Web Dashboard](#web-dashboard)
- [Логи контейнера](#логи-контейнера)
- [Management HTTP API](#management-http-api)
- [Prometheus метрики](#prometheus-метрики)
- [Grafana дашборд](#grafana-дашборд)
- [Telegram уведомления](#telegram-уведомления)
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
| PnL | `curl -s http://127.0.0.1:9020/instances/iis-vtb-sma/pnl` | JSON с realized/unrealized/total |
| Позиции | `curl -s http://127.0.0.1:9020/instances/iis-vtb-sma/ledger` | Карты quantities/avg_prices |
| Последние сделки | `curl -s 'http://127.0.0.1:9020/instances/iis-vtb-sma/executions?limit=5'` | Массив записей (или null если сделок ещё не было) |
| Дневной отчёт | `curl -s 'http://127.0.0.1:9020/report/daily?date=2026-05-15'` | Счётчики signals/orders/executions |
| Ошибки в логах | `docker logs --tail 50 24alert-strategy-runner \| grep -i error` | Пусто — всё хорошо |

## Web Dashboard

Встроенный React SPA-дашборд для визуализации работы стратегии, доступный в браузере.

**URL:** `http://127.0.0.1:9020/dashboard/` (через SSH-туннель: `http://localhost:9020/dashboard/`)

### Что показывает

- **Indicator Chart** — свечной график цены инструмента (OHLC). Для SMA-стратегий отображает линии Fast/Slow SMA; для ORB — горизонтальные пунктирные линии Range High (зелёная) / Range Low (красная). Маркеры сигналов: зелёная стрелка вверх = buy, красная стрелка вниз = sell. Счётчик сигналов с момента запуска.
- **Trade Event Log** — хронологическая лента событий (signals → orders → executions) с цветовой кодировкой и фильтрами по типу.
- **Stats Panel** — Total/Realized/Unrealized PnL, текущая позиция, дневная статистика.
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
| `GET /instances/{id}/signals?limit=N` | История сигналов из журнала |
| `GET /instances/{id}/events?limit=N` | Объединённый таймлайн (signals + orders + executions) |

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
| `risk rejected signal` | Risk-сервис отклонил сигнал (лимит позиции, баланс, сессия). |
| `PostOrder failed` | Ошибка при отправке ордера в T-Invest. |
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

# Сигналы и ордера
docker logs 24alert-strategy-runner 2>&1 | grep -E 'signal|order submitted|PostOrder'
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
    "id": "iis-vtb-sma",
    "type": "sma_crossover",
    "account_id": "2001673385",
    "enabled_in_config": true,
    "running": true
  }
]
```

### PnL (прибыль/убыток)

```bash
curl -s http://127.0.0.1:9020/instances/iis-vtb-sma/pnl | jq
```

Ответ:
```json
{
  "instance_id": "iis-vtb-sma",
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
curl -s http://127.0.0.1:9020/instances/iis-vtb-sma/ledger | jq
```

Ответ:
```json
{
  "instance_id": "iis-vtb-sma",
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
curl -s 'http://127.0.0.1:9020/instances/iis-vtb-sma/executions?limit=10' | jq
```

Ответ — массив `ExecutionRecord`:
```json
[
  {
    "InstanceID": "iis-vtb-sma",
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
curl -X POST http://127.0.0.1:9020/instances/iis-vtb-sma/stop

# Запуск (инстанс должен быть в конфиге)
curl -X POST http://127.0.0.1:9020/instances/iis-vtb-sma/start
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
alert24_strategy_total_pnl_rub{instance="iis-vtb-sma"}

# Скорость сигналов за последний час
rate(alert24_strategy_signals_total{instance="iis-vtb-sma"}[1h])

# Просадка
alert24_strategy_drawdown_percent{instance="iis-vtb-sma"}

# Процент прибыльных сделок
alert24_strategy_win_rate{instance="iis-vtb-sma"}

# Средний slippage (bps) за последние 24ч
rate(alert24_strategy_slippage_bps_sum{instance="iis-vtb-sma"}[24h])
/ rate(alert24_strategy_slippage_bps_count{instance="iis-vtb-sma"}[24h])

# Количество ордеров по статусу
alert24_strategy_orders_total{instance="iis-vtb-sma"}

# Risk rejection rate
rate(alert24_strategy_orders_total{instance="iis-vtb-sma",status="risk_rejected"}[1h])
/ rate(alert24_strategy_orders_total{instance="iis-vtb-sma"}[1h])
```

## Grafana дашборд

**URL:** `http://213.171.27.217:3535/d/24alert-strategy/24alert-strategy-runner`

Артефакт: `deployments/grafana/dashboards/strategy-overview.json`.
Дашборд провиженирован в папку **24alert** на мониторинговом сервере (`/opt/monitoring/grafana/dashboards-24alert/strategy-runner.json`), автоматически обновляется при изменении файла.

### Панели

**Row 1 — Ключевые числа (stat)**

| Панель | Метрика | Пороги |
|--------|---------|--------|
| Total PnL (RUB) | `alert24_strategy_total_pnl_rub` | красный < 0, зелёный > 0 |
| Realized PnL | `alert24_strategy_realized_pnl_rub` | красный < 0, зелёный > 0 |
| Unrealized PnL | `alert24_strategy_unrealized_pnl_rub` | красный < 0, зелёный > 0 |
| Win Rate | `alert24_strategy_win_rate` | красный < 40%, зелёный > 55% |
| Drawdown % | `alert24_strategy_drawdown_percent` | зелёный < 5%, жёлтый < 10%, красный > 10% |
| Net Position (shares) | `alert24_strategy_position_qty_shares` | нейтральный |

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

**Row 5 — Логи (Loki)**

- **Strategy Runner Logs** — `{service="strategy-runner"}`, фильтрация по уровню.

### Переменные

- `$instance` — фильтр по инстансу стратегии (label `exported_instance`), поддерживает мульти-выбор и All.

### Как обновить дашборд

Два способа:

**1. Файл-провижининг (рекомендуется):**
```bash
# Локально: отредактировать JSON
# Скопировать на мониторинг-сервер:
scp deployments/grafana/dashboards/strategy-overview.json \
  admin-cloud-srv-01@213.171.27.217:/tmp/strategy-runner.json
ssh admin-cloud-srv-01@213.171.27.217 \
  "sudo cp /tmp/strategy-runner.json /opt/monitoring/grafana/dashboards-24alert/strategy-runner.json"
# Grafana подхватит изменения за 30 секунд
```

**2. Через UI (быстрые правки):**
- Открыть http://213.171.27.217:3535/d/24alert-strategy
- Отредактировать панель
- Сохранить дашборд
- Экспортировать JSON обратно в репо

### Алерты (Alertmanager)

Файл: `deployments/grafana/alerts/strategy-runner.rules.yml`.
Развёрнут на мониторинг-сервере в `/opt/monitoring/prometheus/rules/strategy-runner.yml`.

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

Prometheus agent и Promtail запущены на сервере 24alert (`docker compose --profile monitoring up -d`):

| Контейнер | Роль |
|-----------|------|
| `24alert-prometheus-agent` | Scrape метрик со всех сервисов → remote_write на `213.171.27.217:9191` |
| `24alert-promtail` | Сбор Docker-логов → отправка в Loki на `213.171.27.217:3100` |

Scrape targets (config: `config/prometheus-agent.yaml`):
- gateway:8080
- order-svc:9101
- marketdata-svc:9102
- portfolio-svc:9103
- risk-svc:9104
- **strategy-runner:9120**

## Telegram уведомления

Если в `config/config.yaml` заданы `notifications.telegram.bot_token` и `chat_id`, runner автоматически шлёт сообщения:

| Событие | Пример сообщения |
|---------|-----------------|
| Watchdog остановил инстанс (потери) | `strategy-runner: instance iis-vtb-sma stopped (total PnL -520.00 < -500.00 RUB)` |
| Watchdog остановил инстанс (просадка) | `strategy-runner: instance iis-vtb-sma stopped (drawdown 12.3% > 10.0%)` |
| Зависший ордер | `stuck order: account=2001673385 order=abc-123 status=NEW age>30m` |
| Дрейф ledger | `ledger drift reconciled: instance=iis-vtb-sma instrument=962e... broker_qty=1.0000` |
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
curl -X POST http://127.0.0.1:9020/instances/iis-vtb-sma/start
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
  curl -s http://127.0.0.1:9020/instances/iis-vtb-sma/pnl && echo && \
  echo "=== LEDGER ===" && \
  curl -s http://127.0.0.1:9020/instances/iis-vtb-sma/ledger && echo && \
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
- `deployments/grafana/dashboards/strategy-overview.json` — дашборд Grafana.
- `deployments/grafana/alerts/strategy-runner.rules.yml` — алерт-правила.
