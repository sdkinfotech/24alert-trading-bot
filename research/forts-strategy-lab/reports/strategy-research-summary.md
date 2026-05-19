# FORTS Strategy Lab Summary

Generated: 2026-05-18

## Scope

Research covered 16 FORTS candidates from gateway data:

`BMM6`, `MCM6`, `NGM6`, `BMN6`, `BRM6`, `BRN6`, `GZM6`, `GZU6`, `MCU6`, `MXM6`, `MXU6`, `NGK6`, `RIM6`, `RIU6`, `SRM6`, `SRU6`.

Core focus remains current production futures:

- Brent mini: `BMM6`.
- Natural Gas mini/current gas contracts: `NGM6`, nearby `NGK6`.
- Mechel futures: `MCM6`, nearby `MCU6`.

No production config was changed. No reload was executed.

## Method

Data sources:

- Production strategy-runner baseline: `data/prod-baseline.json`.
- Gateway futures metadata: `data/instruments.json`.
- Batch backtest and ranking: `reports/backtest-results.json`.
- Public strategy context: `notes/market-context.md`.
- Strategy hypotheses: `hypotheses/trend-following.md`, `hypotheses/reversal-and-session.md`.

Backtest constraints:

- FORTS schedule: Mon-Fri, `10:00-14:00`, `14:05-18:50`, `19:00-23:50` MSK.
- `level_bounce` research logic was aligned with production: entry requires actual level touch.
- Weekend signals are not considered tradable.
- PnL is measured in price points, not final RUB after contract multiplier, commissions and slippage.

## Best Per Instrument

| Ticker | Best strategy | Params | PnL | Trades | Sharpe | PF | Quality |
|---|---|---|---:|---:|---:|---:|---|
| `BMM6` | `sma_1h` | `fast=4 slow=9` | 48.13 | 37 | 5.94 | 3.22 | PASS |
| `BMN6` | `ema_1h` | `fast=5 slow=17` | 29.74 | 18 | 8.48 | 5.04 | PASS |
| `BRM6` | `sma_1h` | `fast=12 slow=27` | 40.37 | 11 | 7.74 | 4.30 | PASS |
| `BRN6` | `sma_1h` | `fast=4 slow=9` | 48.49 | 36 | 5.79 | 3.31 | PASS |
| `NGM6` | `sma_1h` | `fast=5 slow=17` | 0.422 | 18 | 5.15 | 2.40 | PASS |
| `NGK6` | `level_bounce_15m` | `sl=0.3 tp=1.0` | 0.1478 | 14 | 3.39 | 1.87 | PASS |
| `MCM6` | `sma_1h` | `fast=4 slow=9` | 1748.0 | 33 | 4.60 | 3.10 | PASS |
| `MCU6` | `sma_1h` | `fast=4 slow=9` | 320.0 | 17 | 1.92 | 1.52 | PASS |
| `GZM6` | `ema_1h` | `fast=5 slow=17` | 1160.0 | 20 | 5.13 | 2.48 | PASS |
| `GZU6` | `sma_1h` | `fast=12 slow=27` | 1139.0 | 14 | 7.29 | 3.71 | PASS |
| `RIM6` | `level_bounce_15m` | `sl=1.0 tp=2.0` | 5140.0 | 10 | 8.02 | 5.02 | PASS |
| `RIU6` | `ema_1h` | `fast=9 slow=26` | 4330.0 | 8 | 4.89 | 2.30 | PASS |
| `MXM6` | `ema_1h` | `fast=4 slow=9` | 14900.0 | 26 | 3.53 | 1.86 | PASS |
| `MXU6` | `sma_1h` | `fast=9 slow=26` | 13650.0 | 16 | 4.06 | 1.98 | PASS |
| `SRM6` | `level_bounce_15m` | `sl=0.3 tp=1.5` | 1191.77 | 11 | 8.15 | 4.44 | PASS |
| `SRU6` | `level_bounce_15m` | `sl=0.3 tp=1.5` | 1442.29 | 8 | 17.12 | 30.43 | PASS |

## Core Findings

### Brent / Oil

The strongest family for Brent-like contracts in this first pass is trend-following on 1h candles, not counter-trend level bounce.

Initial research found that `BMM6` should move from `level_bounce` to `sma_1h fast=4 slow=9`; production was later updated to that SMA setup. Nearby contracts also support this direction:

- `BMN6`: best `ema_1h fast=5 slow=17`.
- `BRM6`: best `sma_1h fast=12 slow=27`.
- `BRN6`: best `sma_1h fast=4 slow=9`.

Interpretation: oil contracts are currently behaving more like momentum/trend instruments than level-fade instruments.

Recommendation: `ADD_CANDIDATE / REPARAM`, but do not replace production immediately. First run a walk-forward check and add a paper/watch candidate for Brent trend-following.

### Natural Gas

`NGM6` current production uses `sma_1h fast=9 slow=26`; research again prefers a faster trend-following pair: `fast=5 slow=17`.

Nearby `NGK6` showed a viable `level_bounce` result, but PnL scale is small and gas is prone to false breakouts and sudden volatility spikes.

Recommendation: `REPARAM / WATCH` for `NGM6` from `9/26` toward `5/17` after an additional walk-forward check. Do not add aggressive gas breakout logic without stricter volatility/news filters.

### Mechel

`MCM6` showed strong `sma_1h fast=4 slow=9` results; production was later updated to that SMA setup. The previous `level_bounce` setup remains an alternative benchmark only.

This is promising but risky:

- Mechel futures liquidity can be thinner than index/energy contracts.
- Single-stock futures may have gaps and sparse 15m candles.
- Backtest PnL may be overstated without slippage and orderbook checks.

Recommendation: `WATCH / ADD_CANDIDATE`. Keep current production guarded; create a candidate trend strategy only after checking spreads/liquidity and production order fill quality.

### Other FORTS Candidates

`MX*`, `RI*`, `SR*`, `GZ*` produced many PASS results. This is useful, but they are outside the first user's core oil/gas/Mechel focus.

Recommendation: `WATCH`. They should become a second research phase after validating energy strategies.

## Risks And Limitations

- Many high scores come from short samples, especially 5-10 completed trades.
- PnL is in price points, not RUB net of contract details, commission, slippage and spread.
- Level-bounce results are sensitive to daily level calculation.
- Trend-following results can degrade sharply in flat/choppy markets.
- Natural gas is event-driven; inventory/weather shocks are not explicitly modeled.
- Current research uses candles only; it does not inspect orderbook liquidity or actual fill probability.

## Shortlist

| Action | Candidate | Reason |
|---|---|---|
| `REPARAM` | `NGM6 sma_1h 5/17` | Better than current `9/26` in first-pass research with enough trades. |
| `ADD_CANDIDATE` | `BMM6 sma_1h 4/9` | Strong trend result on current Brent mini. Needs walk-forward before production. |
| `ADD_CANDIDATE` | `BRM6 sma_1h 12/27` | Strong result on full Brent contract; useful benchmark against mini. |
| `WATCH` | `MCM6 sma_1h 4/9` | Strong result, but single-stock futures liquidity/slippage risk is material. |
| `WATCH` | `NGK6 level_bounce sl=0.3 tp=1.0` | Interesting nearby gas contract result, but gas event risk is high. |
| `REJECT_FOR_NOW` | Immediate production replacement | More validation needed: walk-forward, slippage, contract multiplier and fill quality. |

## Next Implementation Step

Before applying anything to production:

1. Add a walk-forward validator that splits history into train/test windows.
2. Add contract multiplier / min price increment / commission model to PnL.
3. Add a paper/watch mode for candidate instances.
4. Compare candidate signals with production events for at least several sessions.
