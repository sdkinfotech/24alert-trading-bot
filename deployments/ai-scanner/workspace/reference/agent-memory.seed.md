# 24alert AI Scanner Memory

## Durable Facts

- 2026-05-16: Production trading scope is futures-only on MOEX FORTS. Shares are not allowed in strategy config.
- 2026-05-16: Weekend trading remains blocked by policy because liquidity is thin and execution risk is high.
- 2026-05-16: Production FORTS guard is weekdays only: `10:00-14:00`, `14:05-18:50`, `19:00-23:50 Europe/Moscow`; clearing `14:00-14:05` is blocked.
- 2026-05-16: Manual production instances are `fut-brent-mini-lb`, `fut-gas-mini-sma`, `fut-mechel-lb`; do not modify them unless the user explicitly asks.
- 2026-05-18: Current manual params after FORTS Strategy Lab rollout: `BMM6` SMA `1h fast=4 slow=9 trailing_stop_pct=0.005`; `NGM6` SMA `1h fast=5 slow=17`; `MCM6` SMA `1h fast=4 slow=9`; all `quantity=1`. Brent/Mechel `-lb` IDs are legacy names only; always read the `type` field.
- 2026-05-16: Scanner score is only a ranking hint. Optimized guarded backtest is the decision gate.
- 2026-05-16: Before answering "what is trading", "why no trade", or "what do logs say", read the log-reading skill and strategy journal events. Do not infer schedule from `cutoff` alone.

## Append New Lessons Below

