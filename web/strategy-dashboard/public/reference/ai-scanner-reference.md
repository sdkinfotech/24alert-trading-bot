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
- Read `/workspace/memory/agent-memory.md` before strategy decisions.
- Write reports to `/workspace/reports/`.

