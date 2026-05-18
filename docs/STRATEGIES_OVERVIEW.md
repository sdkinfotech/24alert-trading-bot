# Обзор встроенных стратегий (`strategy-runner`)

Каноничная операционная документация живёт в Obsidian: `24alert/Strategies.md` и `24alert/Strategy Optimization Methodology.md`.

Текущий production-режим с 2026-05-16: **только фьючерсы MOEX FORTS**. Старые акционные инстансы больше не являются актуальным prod.

> **Protected live 2026-05-18:** после incident с real-money позицией live-входы разрешены только с обязательным `trailing_stop_pct`, broker-side `STOP_LOSS` после fill и watchdog flatten по broker truth. Политика: [`docs/PRODUCTION_TRADING_POLICY.md`](PRODUCTION_TRADING_POLICY.md).

## Production instances

| ID | Стратегия | Фьючерс | Интервал | Назначение |
|----|-----------|---------|----------|------------|
| `fut-brent-mini-lb` | `sma_crossover` | `BMM6` Brent mini | `1h` | SMA `4/9` + trailing `0.5%` + broker-side stop after fill |
| `fut-gas-mini-sma` | `sma_crossover` | `NGM6` Natural Gas mini | `1h` | SMA `5/17` + trailing `0.5%` + broker-side stop after fill |
| `fut-mechel-lb` | `sma_crossover` | `MCM6` Mechel futures | `1h` | SMA `4/9` + trailing `0.5%` + broker-side stop after fill |

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

Текущий prod config:

| Instance | Ticker | Params |
|---|---|---|
| `fut-brent-mini-lb` | `BMM6` | `interval=1h`, `fast_period=4`, `slow_period=9`, `trailing_stop_pct=0.005` |
| `fut-gas-mini-sma` | `NGM6` | `interval=1h`, `fast_period=5`, `slow_period=17`, `trailing_stop_pct=0.005` |
| `fut-mechel-lb` | `MCM6` | `interval=1h`, `fast_period=4`, `slow_period=9`, `trailing_stop_pct=0.005` |

`fut-brent-mini-lb` and `fut-mechel-lb` are legacy IDs; their current type is `sma_crossover`.

`trailing_stop_pct` is an optional protective exit for SMA positions. For `BMM6`,
`0.005` means a 0.5% trailing stop from the best broker-synced/following price.
If it triggers, SMA sends a market exit and flattens the position instead of reversing.

После safety hold `trailing_stop_pct` является обязательным для `sma_crossover`, но сам по себе не считается достаточным real-money guard: runner должен быть жив, а broker-side stop / flatten watchdog ещё должны быть реализованы.

## `level_bounce`

Пакет: `internal/strategy/lb`.

Логика:

- дневной warmup строит top-3 support/resistance;
- источники уровней теперь отдаются в dashboard: для каждого `S1-S3`/`R1-R3` видны цена, дата дневной свечи и тип (`low`/`high`);
- при восстановлении state дневные бары дедуплицируются по дате, старые snapshots без дат пересобираются из fresh warmup, чтобы не накапливать дубли уровней после restart;
- ATR по дневным свечам задаёт stop/take-profit, но больше не расширяет entry-зону;
- bounce от support -> `buy` только при фактическом касании `low <= support` и закрытии выше уровня;
- reject от resistance -> `sell` только при фактическом касании `high >= resistance` и закрытии ниже уровня;
- стоп/тейк: `sl_mult` и `tp_mult` от ATR;
- EOD flatten перед cutoff.

Текущий prod config не использует `level_bounce`; стратегия оставлена в коде и backtest-инструментах для новых кандидатов и research.

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

## Research и альтернативные стратегии

Материалы по другим стратегиям не удаляются и не считаются устаревшими только из-за отсутствия в production:

- `level_bounce` остаётся benchmark для mean-reversion от дневных S/R уровней.
- `orb_breakout` остаётся экспериментальной Go-стратегией opening range breakout.
- `research/forts-strategy-lab` хранит результаты по `sma_1h`, `ema_1h`, `donchian_15m`, `level_bounce_15m`, `orb_15m` и weighted optimizer.
- `docs/STRATEGY_RUNNER.md` содержит технические детали встроенных Go-стратегий и guardrails.

## Где смотреть

- `config/config.yaml` -> `strategies.instances`
- `internal/strategy/runner.go`
- `internal/strategy/sma/sma.go`
- `internal/strategy/lb/lb.go`
- `internal/strategy/orb/orb.go`
- `docs/STRATEGY_RUNNER.md`
- `docs/STRATEGY_MONITORING.md`
