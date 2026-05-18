# Plan: TASK-027 Production Trading Safety Remediation

## Цель
Fail-closed production safety: остановить новые боевые входы до появления независимого hard safety layer.

## Роли
- Planner: создать task/plan, критерии готовности, зависимости.
- Backend: guardrails в runner/config.
- DevOps: корректный git-based rollout на production.
- Tester: локальные тесты + production smoke.
- Tech Lead: проверить, что live trading нельзя включить без явного решения.
- Analyst: зафиксировать incident lessons в Obsidian.

## Порядок
1. Planner: оформить задачу и обновить backlog.
2. Backend: внести emergency guardrails.
3. Docs/Analyst: обновить repo docs и Obsidian policy.
4. Tester: `go test` релевантных пакетов, проверка config reload/start behavior.
5. DevOps: commit -> push -> pull -> build/recreate `strategy-runner`.
6. Tech Lead: post-deploy smoke: `/instances` показывает `enabled_in_config=false`, `running=false`; broker positions flat.

## Зависимости
- Текущий фикс broker reconciliation уже выкачен в production (`89d2c1d`, `a2f44fb`).
- Следующие задачи должны добавить broker-native stops и flatten watchdog.

## Риски
- Если выключить instances без проверки broker portfolio, можно оставить позицию без менеджмента. Перед rollout обязательна проверка broker positions.
- Если оставить management API без запрета ручного старта disabled instances, оператор может случайно вернуть live trading.

## Definition of Done
- Текущие instances disabled in config.
- Disabled instance cannot be manually started.
- `sma_crossover` without `trailing_stop_pct > 0` refused.
- `orb_breakout` refused for live runner.
- Policy docs updated in repo and Obsidian.
- Production deployed and verified, unless user explicitly pauses deployment.
