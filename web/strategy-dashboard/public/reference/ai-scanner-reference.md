# 24alert AI Scanner Reference

Canonical runtime copy: `/workspace/reference/ai-scanner-reference.md` inside `24alert-ai-scanner`.

This public copy exists so the AI agent and operator can fetch the reference from the strategy dashboard:

`/dashboard/reference/ai-scanner-reference.md`

Key production rules:

- Trade only MOEX FORTS futures.
- Weekdays only.
- FORTS sessions: `10:00-14:00`, `14:05-18:50`, `19:00-23:50 Europe/Moscow`.
- Clearing `14:00-14:05` is blocked.
- Weekend trading is blocked intentionally.
- Manual instances: `fut-brent-mini-lb`, `fut-gas-mini-sma`, `fut-mechel-lb`.
- Current manual strategy baseline: `BMM6` `sma_crossover` `1h` `fast=4 slow=9 trailing_stop_pct=0.005`; `NGM6` `sma_crossover` `1h` `fast=5 slow=17`; `MCM6` `sma_crossover` `1h` `fast=4 slow=9`; all `quantity=1`.
- Brent/Mechel `-lb` suffixes are legacy IDs only. Always read the current `type` field from config or `/instances`.
- Read `/workspace/memory/agent-memory.md` before strategy decisions.
- Write reports to `/workspace/reports/`.

