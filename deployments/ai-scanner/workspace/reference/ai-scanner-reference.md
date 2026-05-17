# 24alert AI Scanner Reference

## Mission

AI Scanner maintains 24alert futures strategies. It must use project facts from this reference and `/workspace/memory/agent-memory.md` before making decisions.

## Hard Rules

- Trade only MOEX FORTS futures (`class_code=SPBFUT`, `instrument_type=future`).
- Do not add shares, ETFs, currencies, bonds, or options to strategy config.
- Production account for strategies: `2001673385`.
- Do not change manual instances unless the user explicitly asks. Manual instances currently have IDs without `auto-`.
- Auto-managed instances must use IDs like `auto-fut-<ticker>-<strategy>`.
- Maximum simultaneous instances: 5 including manual and auto.
- Never use weekend trading even if broker status says trading is available.
- Do not treat scanner score as the final decision. Backtest result is the decision gate.

## Production Schedule

Use FORTS sessions in `Europe/Moscow`, weekdays only:

- `10:00-14:00` day session before clearing.
- `14:00-14:05` clearing break, blocked.
- `14:05-18:50` day session after clearing.
- `19:00-23:50` evening session.

For `level_bounce`, evening EOD cutoff may be `23:30`. Never set cutoff at or after `23:50`.

## Current Manual Strategies

| Instance | Ticker | Type | Interval | Current Parameters |
|---|---|---|---|---|
| `fut-brent-mini-lb` | `BMM6` | `level_bounce` | `15min` | `atr_mult=0.3`, `sl_mult=0.3`, `tp_mult=2.0`, `level_days=10`, `cutoff=23:30`, `quantity=1` |
| `fut-gas-mini-sma` | `NGM6` | `sma_crossover` | `1h` | `fast_period=9`, `slow_period=26`, `quantity=1` |
| `fut-mechel-lb` | `MCM6` | `level_bounce` | `15min` | `atr_mult=0.5`, `sl_mult=0.7`, `tp_mult=1.0`, `level_days=10`, `cutoff=23:30`, `quantity=1` |

## Decision Thresholds

Candidate strategy may be added only if all are true:

- `trades >= 5`
- `pnl > 0`
- `sharpe > 1.0`
- `win_rate > 45%`
- `profit_factor > 1.3`

If two variants are close, prefer lower drawdown and more trades over a tiny Sharpe improvement.

## Tools

- Scan futures: `python3 /opt/ai-scanner/scan_market.py --gateway-url $GATEWAY_URL --top-n 10 --max-contract-price ${AI_SCANNER_MAX_CONTRACT_PRICE:-10000} --json`
- Optimize SMA: `python3 /opt/ai-scanner/backtest.py --gateway-url $GATEWAY_URL --uid <UID> --strategy sma --optimize --json`
- Optimize Level Bounce: `python3 /opt/ai-scanner/backtest.py --gateway-url $GATEWAY_URL --uid <UID> --strategy level_bounce --optimize --json`
- Health: `curl -s $GATEWAY_URL/health` and `curl -s $STRATEGY_RUNNER_URL/health`
- Instances: `curl -s $STRATEGY_RUNNER_URL/instances`
- Reload config after edits: `curl -s -X POST $STRATEGY_RUNNER_URL/config/reload`
- Strategy/dashboard smoke-check after config edits: `python3 /opt/ai-scanner/monitoring/strategy_dashboard_smoke.py --strategy-runner-url $STRATEGY_RUNNER_URL --dashboard-json /workspace/reference/24alert-strategy-runner.json`

## Strategy Dashboard Synchronization Contract

The Grafana `24alert Strategy Runner` dashboard must not hardcode strategy IDs in the top status tiles. It uses a repeated stat panel driven by Prometheus:

- variable: `strategy_instance`
- source metric: `alert24_strategy_instance_enabled{exported_instance=...}`
- tile readiness formula: `enabled * 2 + running`
- value mapping: `3=RUNNING`, `2=STOPPED`, `1=CONFIG OFF / RUNNING`, `0=DISABLED`

After adding, deleting, renaming, enabling, or disabling any strategy instance:

1. Save `/app/config/config.yaml`.
2. Run `curl -s -X POST $STRATEGY_RUNNER_URL/config/reload`.
3. Wait 20-40 seconds for metrics scrape.
4. Run the smoke-check tool above.
5. Do not report success until all enabled instances are running and the dashboard JSON still contains the repeated `strategy_instance` tile.

## Memory Protocol

- Read `/workspace/memory/agent-memory.md` before scans, optimization, config edits, and health decisions.
- Append durable lessons to memory after a meaningful decision, rejected candidate, config change, incident, or deployment.
- Keep memory concise: date, decision, reason, metrics, and whether production config changed.
- Write detailed run reports to `/workspace/reports/`.
