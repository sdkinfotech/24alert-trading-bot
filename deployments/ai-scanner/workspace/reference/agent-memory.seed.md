# 24alert AI Scanner Memory

## Durable Facts

- 2026-05-16: Production trading scope is futures-only on MOEX FORTS. Shares are not allowed in strategy config.
- 2026-05-16: Weekend trading remains blocked by policy because liquidity is thin and execution risk is high.
- 2026-05-16: Production FORTS guard is weekdays only: `10:00-14:00`, `14:05-18:50`, `19:00-23:50 Europe/Moscow`; clearing `14:00-14:05` is blocked.
- 2026-05-16: Manual production instances are `fut-brent-mini-lb`, `fut-gas-mini-sma`, `fut-mechel-lb`; do not modify them unless the user explicitly asks.
- 2026-05-16: Current manual params: Brent LB `atr=0.3 sl=0.3 tp=2.0 cutoff=23:30`; Gas SMA `fast=9 slow=26`; Mechel LB `atr=0.5 sl=0.7 tp=1.0 cutoff=23:30`.
- 2026-05-16: Scanner score is only a ranking hint. Optimized guarded backtest is the decision gate.
- 2026-05-16: Before answering "what is trading", "why no trade", or "what do logs say", read the log-reading skill and strategy journal events. Do not infer schedule from `cutoff` alone.

## Append New Lessons Below

