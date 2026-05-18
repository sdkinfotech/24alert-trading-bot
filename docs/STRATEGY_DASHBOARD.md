# Strategy Dashboard

The strategy dashboard is the operator UI for live strategy control and post-trade inspection. It is served by `strategy-runner` at `/dashboard/` and reads persisted data from runner journal/API endpoints, not browser-only state.

## UI structure

- `Overview`: strategy status, PnL, broker positions, runner ledger reconciliation, protective stop coverage, daily counts.
- `Chart`: candles, SMA/range/level lines, broker average price, trailing stop line, signal/fill markers, hover OHLC tooltip.
- `Portfolio`: account totals, broker truth positions, runner ledger mismatch status, active broker stop orders.
- `History`: unified event timeline plus persisted orders, executions, and broker stop orders.
- `AI Trader`: separate prompt-driven observe/paper strategy for a selected account and futures instrument; it is not part of SMA/Level/ORB instances and live orders are disabled.
- `Guide`: RU/EN explanations for broker truth, runner ledger, strategy state, expected yield, trailing stop, protective stop, cancelled signals, and watchdog flatten.

## Themes and localization

The dashboard has local RU/EN and light/dark switches. Theme colors are defined as CSS variables in `web/strategy-dashboard/src/index.css`; chart colors are mapped in `web/strategy-dashboard/src/theme.tsx` and passed into `IndicatorChart`.

Both theme and language are stored in `localStorage`:

- `24alert.theme`: `light` or `dark`
- `24alert.lang`: `ru` or `en`

## Data endpoints

The dashboard uses these runner endpoints:

- `GET /instances` for strategy metadata, account, instrument UIDs, params, enabled/running state.
- `GET /instances/{id}/portfolio` for broker truth and account totals.
- `GET /instances/{id}/ledger` for runner ledger reconciliation.
- `GET /instances/{id}/pnl` for broker-aware PnL.
- `GET /instances/{id}/indicator` for chart series and strategy state.
- `GET /instances/{id}/events?limit=1000` for the unified timeline.
- `GET /instances/{id}/orders?limit=1000` for persisted journal orders.
- `GET /instances/{id}/executions?limit=1000` for persisted fills.
- `GET /instances/{id}/stop-orders` for active broker stop-loss/take-profit orders matching the instance account/instruments.
- `GET /report/daily` for daily aggregate counts.
- `GET /ai-trader/sessions`, `POST /ai-trader/sessions`, `GET /ai-trader/sessions/{instance_id}`, `POST /ai-trader/sessions/{instance_id}/stop` for observe/paper AI Trader sessions.

## Operational rule

For live trading, the UI must treat `Broker truth` as the position source of record. `Runner ledger` and `Strategy state` are displayed for reconciliation. Any open broker position without a matching broker-side protective stop is highlighted as unsafe.

AI Trader is currently an observation/paper-planning tool only. `armed_live` is rejected by the backend until deterministic OrderControl and live RiskGate are implemented.
