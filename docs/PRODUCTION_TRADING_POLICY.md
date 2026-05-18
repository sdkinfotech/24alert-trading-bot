# Production Trading Safety Policy

Дата: 2026-05-18  
Статус: active, mandatory

## Статус live trading

После incident 2026-05-18 live strategy trading работает по принципу **fail closed unless protected**:

- live `sma_crossover` разрешён только с `trailing_stop_pct > 0`;
- после входа runner обязан поставить broker-side `STOP_LOSS`;
- watchdog при превышении loss/drawdown должен попытаться отменить активные ордера и закрыть broker position;
- `orb_breakout` запрещён для live runner до реализации защитного стопа.

## Что считается недостаточной защитой

Software stop внутри стратегии **не является достаточной защитой** для real-money trading, если нет независимого broker-truth контура.

Недостаточно:

- trailing stop, который зависит только от живого `strategy-runner`;
- выход по обратному сигналу стратегии;
- watchdog, который только останавливает instance;
- ledger-only PnL без сверки с broker portfolio;
- ручная проверка dashboard вместо автоматического hard guard.

## Минимальные условия для live trading

Перед включением любого instance на боевом счёте должны быть выполнены все пункты:

1. Broker positions перед стартом проверены и соответствуют ожидаемому состоянию.
2. Для стратегии есть защитный выход:
   - для `sma_crossover`: `trailing_stop_pct > 0`;
   - для `level_bounce`: заданы и проверены SL/TP от ATR;
   - `orb_breakout` запрещён для live runner до реализации stop-loss/trailing.
3. Runner синхронизирует broker positions до live candles и периодически во время работы.
4. После `PostOrder` runner poll-ит `GetOrderState`, а не зависит только от stream events.
5. Watchdog при превышении loss limit должен закрывать риск: отменить активные ордера и либо flatten broker position, либо подтвердить broker-side protective stop.
6. Для открытой broker position должен существовать один из вариантов:
   - broker-native stop order;
   - активный hard watchdog, который сам закрывает позицию по broker truth.
7. Production rollout прошёл через git commit -> push -> pull on VPS -> Docker build/up -> smoke.

## Запрещено

- Включать live instance без protective exit.
- Включать `orb_breakout` на боевом счёте.
- Считать `StopInstance()` защитой позиции.
- Деплоить live trading изменения через прямое копирование файлов на VPS.
- Убирать docs/runbook updates из task, если изменение влияет на реальные деньги.

## Обязательные проверки после deploy

```bash
curl -fsS http://127.0.0.1:9020/instances
curl -fsS http://127.0.0.1:9020/instances/<id>/portfolio
curl -fsS http://127.0.0.1:9020/instances/<id>/events?limit=20
docker compose -f deployments/docker-compose.yaml -p 24alert logs --tail=100 strategy-runner
```

Успех для protected live:

- configured live instances имеют `enabled_in_config=true`, `running=true`;
- при flat-состоянии `instance_position_count=0`;
- после входа в broker position в events появляется `protective_stop submitted`;
- ручной/аварийный watchdog flatten пишет `watchdog_flatten`.

## Следующие обязательные работы

1. Persist broker stop IDs across restart and reconcile stale manual/bot stops.
2. Futures-aware PnL/margin/GO risk accounting.
3. Active stuck order cancel/requery.
4. Risk validation inside the mandatory order path, including gateway/manual orders.
