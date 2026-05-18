# 24alert Log Reading Skill

Canonical runtime copy: `/workspace/reference/log-reading-skill.md` inside `24alert-ai-scanner`.

Public copy: `/dashboard/reference/log-reading-skill.md`.

Core rule: use Trade Events as the strategy journal before answering why something traded or did not trade.

Current FORTS sessions:

- Mon-Fri `10:00-14:00`
- Mon-Fri `14:05-18:50`
- Mon-Fri `19:00-23:50`
- `14:00-14:05` clearing is blocked
- weekends are blocked intentionally

Important distinction:

- Global session guard controls whether orders may be sent.
- `level_bounce` cutoff controls strategy EOD behavior, not the whole market schedule.

Useful endpoints:

- `/instances`
- `/report/daily`
- `/instances/<id>/events?limit=50`
- `/instances/<id>/signals?limit=20`
- `/instances/<id>/executions?limit=20`
- `/instances/<id>/ledger`
- `/instances/<id>/pnl`

