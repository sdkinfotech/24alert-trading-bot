# 24alert Log Reading Skill

Use this skill whenever the user asks what is happening now, why there was no trade, why a signal was cancelled, whether anything is trading, or whether production services are healthy.

## First Read Runtime State

Inside `24alert-ai-scanner` use:

```bash
curl -s $STRATEGY_RUNNER_URL/health
curl -s $STRATEGY_RUNNER_URL/instances
curl -s $STRATEGY_RUNNER_URL/report/daily
```

For every enabled instance:

```bash
curl -s $STRATEGY_RUNNER_URL/instances/<id>/ledger
curl -s $STRATEGY_RUNNER_URL/instances/<id>/pnl
curl -s "$STRATEGY_RUNNER_URL/instances/<id>/events?limit=50"
curl -s "$STRATEGY_RUNNER_URL/instances/<id>/signals?limit=20"
curl -s "$STRATEGY_RUNNER_URL/instances/<id>/executions?limit=20"
```

Treat `/instances/<id>/events` as the canonical strategy journal. It includes:

- `signal` — strategy generated a signal.
- `signal_cancelled` — runner blocked a signal before order dispatch or broker order.
- `order` — order intent reached order flow.
- `execution` — broker execution/fill event.

## Interpret Session Guard Correctly

- Global order dispatch is controlled by `TradingSchedule`, not by strategy `cutoff`.
- Current FORTS sessions are Mon-Fri `10:00-14:00`, `14:05-18:50`, `19:00-23:50 Europe/Moscow`.
- Clearing `14:00-14:05` is blocked.
- Weekends are blocked intentionally.
- `level_bounce` cutoff `23:30` means the strategy stops opening new level-bounce trades near EOD and closes positions before session end.

## Raw Production Logs

If running from an operator shell on VPS `/opt/24alert`:

```bash
docker logs 24alert-strategy-runner --since 30m
docker logs 24alert-ai-scanner --since 30m
docker logs 24alert-gateway --since 30m
docker compose -f deployments/docker-compose.yaml -p 24alert --profile ai-scanner ps
```

Search for:

```bash
docker logs 24alert-strategy-runner --since 30m 2>&1 | grep -Ei 'signal cancelled|session_blocked|risk_rejected|risk_error|post_error|order|execution|panic|fatal|error'
```

The dashboard AI chat cannot directly read Docker stdout. It should use Trade Events from context and clearly say when raw Docker logs require operator-side inspection.

## Answer Template

When asked "what is happening":

1. State current session status: allowed or blocked, with next open if blocked.
2. State each instance status: running/stopped and position: flat/long/short.
3. State recent journal facts: signals, cancellations, orders, executions.
4. If no events exist, say "journal has no recent signals/orders/executions", not "the market is closed" unless schedule confirms it.
5. Mention raw Docker log limitations if needed.

