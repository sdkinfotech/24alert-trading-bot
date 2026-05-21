# AI Trader (архив)

Статус: **архивирован** с 2026-05-21. Новые live-сессии отключены (`strategies.ai_trader_archived: true`, `AI_TRADER_ARMED_LIVE=false`).

## Вместо AI Trader

- **Торговля:** классические инстансы `sma_crossover` в `config/config.yaml` (дашборд → Мониторинг).
- **Отбор и оптимизация:** контейнер `24alert-ai-scanner` (cron: post-market, pre-market, health). Профиль `ai-scanner` включён в `scripts/compose.sh`.

## Документы (история)

| Файл | Описание |
|------|----------|
| [AI_TRADER_SCALPER.md](AI_TRADER_SCALPER.md) | Продуктовая спецификация скальпера |
| [AI_TRADER_VISION.md](AI_TRADER_VISION.md) | Целевой цикл observe → plan → trade |
| [AI_TRADER_TRADE_ANALYST.md](AI_TRADER_TRADE_ANALYST.md) | Trade analyst / policy hints |

Код остаётся в `internal/strategy/ai_trader*.go` для возможного возврата; API `/ai-trader/*` возвращает ошибку при `archived`.
