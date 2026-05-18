# TASK-027: Production Trading Safety Remediation

## Статус
In Progress

## Контекст
18 мая 2026 на боевом счёте была открыта позиция без достаточного независимого защитного контура: broker position существовала, а strategy state мог считать себя flat. Software trailing stop зависел от живого runner, stream/execution sync и корректного internal state.

## Цель
Перевести live trading в режим fail-closed и вернуть боевую торговлю только после внедрения hard safety layer.

## Scope
- Остановить новые live-входы текущих ручных strategy instances.
- Запретить ручной старт disabled instance через management API.
- Не запускать live `sma_crossover` без `trailing_stop_pct > 0`.
- Запретить live `orb_breakout`, пока у него нет защитного stop-loss/trailing.
- Обновить repo и Obsidian политику боевой торговли.

## Out of Scope
- Полная реализация broker-native stop orders.
- Полная реализация flatten watchdog.
- Фьючерсный GO/margin accounting.

Эти работы должны идти следующими задачами после fail-closed фикса.

## Definition of Done
- Production config не автостартует текущие ручные стратегии.
- Management API не позволяет стартовать disabled instance.
- Enabled SMA без trailing stop не стартует.
- Документы явно говорят: software stop не считается достаточной защитой для real-money rollout.
- Есть список обязательных условий для возврата live trading.
