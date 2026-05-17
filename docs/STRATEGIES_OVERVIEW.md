# Обзор встроенных стратегий (`strategy-runner`)

Каноничная операционная документация живёт в Obsidian: `24alert/Strategies.md` и `24alert/Strategy Optimization Methodology.md`.

Текущий production-режим с 2026-05-16: **только фьючерсы MOEX FORTS**. Старые акционные инстансы больше не являются актуальным prod.

## Production instances

| ID | Стратегия | Фьючерс | Интервал | Назначение |
|----|-----------|---------|----------|------------|
| `fut-brent-mini-lb` | `level_bounce` | `BMM6` Brent mini | `15min` | mean reversion от S/R уровней |
| `fut-gas-mini-sma` | `sma_crossover` | `NGM6` Natural Gas mini | `1h` | тренд по пересечению SMA |
| `fut-mechel-lb` | `level_bounce` | `MCM6` Mechel futures | `15min` | mean reversion от S/R уровней |

Все текущие инстансы используют счёт `2001673385` и `quantity=1`.

## Общая цепочка исполнения

1. `CandleHub` получает свечи T-Invest по `instrument_uid + interval`.
2. `runner` считает предыдущую свечу закрытой при приходе новой свечи.
3. `Strategy.OnCandle` возвращает 0..N сигналов.
4. `TradingSchedule` блокирует отправку сигналов вне FORTS-окон `10:00–14:00`, `14:05–18:50`, `19:00–23:50 Europe/Moscow` и полностью блокирует выходные. Клиринг `14:00–14:05` не торгуется; вечерняя сессия торгуется.
5. `risk.ValidateOrderIntent` проверяет лимиты.
6. `order.PostOrder` отправляет заявку.
7. `OrdersStream` возвращает fill/reject/cancel в `Strategy.OnExecution`.

Warmup-сигналы не считаются боевыми и очищаются из dashboard markers после прогрева.
Если сигнал отменён session/risk/order guard, он пишется в application log и в Web Dashboard как `Signal cancelled` с кратким объяснением причины. Такая запись не означает реальную заявку и не должна трактоваться как исполненная торговля.

## `sma_crossover`

Пакет: `internal/strategy/sma`.

Логика:

- считает Fast/Slow SMA по закрытиям;
- golden cross -> `buy`;
- death cross -> `sell`;
- отдельного SL/TP нет, выход по обратному сигналу;
- `pendingEntry` блокирует повторные сигналы до результата risk/order/execution.

Текущий prod config: `fut-gas-mini-sma`, `NGM6`, `interval=1h`, `fast_period=9`, `slow_period=26`.

## `level_bounce`

Пакет: `internal/strategy/lb`.

Логика:

- дневной warmup строит top-3 support/resistance;
- источники уровней теперь отдаются в dashboard: для каждого `S1-S3`/`R1-R3` видны цена, дата дневной свечи и тип (`low`/`high`);
- при восстановлении state дневные бары дедуплицируются по дате, старые snapshots без дат пересобираются из fresh warmup, чтобы не накапливать дубли уровней после restart;
- ATR по дневным свечам задаёт ширину зоны около уровня;
- bounce от support -> `buy`;
- reject от resistance -> `sell`;
- стоп/тейк: `sl_mult` и `tp_mult` от ATR;
- EOD flatten перед cutoff.

Текущий prod config:

| Instance | Params |
|---|---|
| `fut-brent-mini-lb` | `atr_mult=0.3`, `sl_mult=0.3`, `tp_mult=2.0`, `cutoff=23:30`, `level_days=10` |
| `fut-mechel-lb` | `atr_mult=0.5`, `sl_mult=0.7`, `tp_mult=1.0`, `cutoff=23:30`, `level_days=10` |

Dashboard history:

- `level_bounce` прогревает до 600 intraday candles для графика;
- `sma_crossover` прогревает минимум 500 candles для графика;
- runner запрашивает расширенный calendar lookback, чтобы FORTS ночные/выходные gaps не обрезали фактическую историю.

## `orb_breakout`

Пакет: `internal/strategy/orb`.

Стратегия остаётся в коде, но не используется в текущем production config. Её можно рассматривать как экспериментальный шаблон для волатильных инструментов после отдельного backtest.

## AI Scanner

`24alert-ai-scanner` сканирует только futures endpoint, проверяет `contract_price`, затем прогоняет `sma` и `level_bounce` через оптимизационный backtest.

Правило владения:

- ручные инстансы `fut-*` не менять автоматически;
- авто-инстансы должны иметь префикс `auto-fut-*`.

## Где смотреть

- `config/config.yaml` -> `strategies.instances`
- `internal/strategy/runner.go`
- `internal/strategy/sma/sma.go`
- `internal/strategy/lb/lb.go`
- `internal/strategy/orb/orb.go`
- `docs/STRATEGY_RUNNER.md`
- `docs/STRATEGY_MONITORING.md`
