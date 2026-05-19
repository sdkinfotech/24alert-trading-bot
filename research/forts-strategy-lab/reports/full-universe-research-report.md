# Full FORTS Universe Research Report

Generated: 2026-05-18

## What Was Rechecked

This pass expanded the first FORTS research from the core oil/gas/Mechel set to the full futures universe available through the production gateway.

Universe:

- Gateway returned `409` `SPBFUT` futures.
- The research runner saved all instrument metadata to `data/all-futures-universe.json`.
- A broad expensive-contract slice of `80` futures was selected for backtesting.
- The batch generated `1952` strategy/parameter results.
- Weighted optimizer ranked `1801` results with at least one completed trade.

Artifacts:

- Full universe metadata: `data/all-futures-universe.json`.
- Tested instruments: `data/instruments-full.json`.
- Full backtests: `reports/backtest-results-full.json`.
- Weighted rankings: `reports/weighted-rankings.json`.
- Human-readable weighted table: `reports/weighted-rankings.md`.
- Scoring methodology: `notes/weighted-scoring-methodology.md`.

No production config was changed.

## Strategy Families Compared

Three practical families were compared:

| Family | Implemented variants | Interpretation |
|---|---|---|
| `trend_following` | `sma_1h`, `ema_1h` | Catch sustained moves on hourly candles. |
| `level_reversal` | `level_bounce_15m` | Fade support/resistance touches using production-parity level logic. |
| `breakout_momentum` | `donchian_15m`, `orb_15m` | Trade channel or opening range breakouts. |

Family summary from weighted optimizer:

| Family | Runs | Watch-or-better | Avg score | Best candidate |
|---|---:|---:|---:|---|
| `breakout_momentum` | 540 | 40 | 7.92 | `BTK6 orb_15m`, score 86.62 |
| `level_reversal` | 663 | 255 | 39.68 | `MXM6 level_bounce_15m`, score 88.11 |
| `trend_following` | 598 | 124 | 23.11 | `SIM6 sma_1h`, score 97.00 |

Interpretation:

- `trend_following` produced the strongest high-liquidity top candidates.
- `level_reversal` produced many positive candidates, but some top raw results are vulnerable to overfit and low-trade-count effects.
- `breakout_momentum` is selective: fewer good candidates, but some ORB setups are worth watching.

## Best Weighted Candidates

Top by instrument under the balanced score:

| Rank | Ticker | Family | Strategy | Params | Score | PnL | Trades | Sharpe | PF | Volume 15m | Action |
|---:|---|---|---|---|---:|---:|---:|---:|---:|---:|---|
| 1 | `SIM6` | `trend_following` | `sma_1h` | `fast=4 slow=9` | 97.00 | 8666.0 | 30 | 7.89 | 4.17 | 15464.81 | `ADD_CANDIDATE` |
| 2 | `EUM6` | `trend_following` | `ema_1h` | `fast=4 slow=9` | 93.84 | 7226.0 | 24 | 6.21 | 2.91 | 1564.39 | `ADD_CANDIDATE` |
| 3 | `SIU6` | `trend_following` | `ema_1h` | `fast=4 slow=9` | 93.11 | 6672.0 | 23 | 6.38 | 3.62 | 868.85 | `ADD_CANDIDATE` |
| 4 | `PXM6` | `trend_following` | `sma_1h` | `fast=5 slow=17` | 92.03 | 6458.0 | 20 | 8.61 | 6.93 | 187.95 | `ADD_CANDIDATE` |
| 5 | `MGM6` | `trend_following` | `ema_1h` | `fast=4 slow=9` | 91.82 | 6999.0 | 32 | 5.89 | 3.73 | 90.75 | `ADD_CANDIDATE` |
| 6 | `SIZ6` | `trend_following` | `ema_1h` | `fast=4 slow=9` | 90.70 | 8779.0 | 22 | 7.15 | 4.26 | 30.49 | `ADD_CANDIDATE` |
| 7 | `MXM6` | `trend_following` | `sma_1h` | `fast=9 slow=26` | 88.48 | 14100.0 | 13 | 5.11 | 2.76 | 1514.83 | `ADD_CANDIDATE` |
| 8 | `MCM6` | `trend_following` | `sma_1h` | `fast=4 slow=9` | 88.23 | 1748.0 | 33 | 4.60 | 3.10 | 130.05 | `ADD_CANDIDATE` |
| 9 | `BTM6` | `trend_following` | `sma_1h` | `fast=5 slow=17` | 87.57 | 6886.0 | 13 | 6.62 | 3.19 | 541.40 | `ADD_CANDIDATE` |
| 10 | `NAM6` | `trend_following` | `sma_1h` | `fast=4 slow=9` | 86.90 | 3768.0 | 35 | 4.61 | 2.42 | 3742.61 | `ADD_CANDIDATE` |

Important:

- `SIM6` is the strongest broad-universe candidate because it combines high PnL, high trade count, high liquidity and strong risk metrics.
- Several expensive single-stock/index futures look attractive, but must be validated with orderbook/spread and walk-forward.
- Raw PnL rankings overemphasized `MXM6 level_bounce`; weighted score moved more liquid and robust trend-following strategies up.

## Current Production Instruments Rechecked

### `BMM6` Brent Mini

| Family | Best variant | Score | PnL | Trades | Sharpe | PF | Action |
|---|---|---:|---:|---:|---:|---:|---|
| `trend_following` | `sma_1h fast=4 slow=9` | 82.31 | 51.90 | 38 | 6.28 | 3.40 | `ADD_CANDIDATE` |
| `breakout_momentum` | `orb_15m range=60 rr=1.0` | 65.57 | 7.93 | 14 | 5.00 | 2.15 | `WATCH` |
| `level_reversal` | `level_bounce_15m sl=0.3 tp=1.0` | 60.03 | 8.39 | 15 | 3.82 | 1.81 | `WATCH` |

Conclusion:

- Production was updated after this research: `BMM6` now runs `sma_crossover` on `1h` with `fast=4`, `slow=9`.
- The `level_bounce_15m` result remains useful as an alternative/watch benchmark, not the active production baseline.

### `NGM6` Natural Gas

| Family | Best variant | Score | PnL | Trades | Sharpe | PF | Action |
|---|---|---:|---:|---:|---:|---:|---|
| `trend_following` | `sma_1h fast=5 slow=17` | 66.85 | 0.422 | 18 | 5.15 | 2.40 | `WATCH` |
| `level_reversal` | `level_bounce_15m sl=0.5 tp=1.0` | 64.66 | 0.222 | 9 | 9.56 | 5.83 | `WATCH` |
| `breakout_momentum` | `orb_15m range=30 rr=2.0` | 20.55 | 0.026 | 14 | 1.35 | 1.21 | `REJECT` |

Conclusion:

- Production was updated after this research: `NGM6` now runs `sma_crossover` on `1h` with `fast=5`, `slow=17`.
- Natural gas should remain conservative because external event risk is high and PnL scale is small in candle-point terms.

### `MCM6` Mechel

| Family | Best variant | Score | PnL | Trades | Sharpe | PF | Action |
|---|---|---:|---:|---:|---:|---:|---|
| `trend_following` | `sma_1h fast=4 slow=9` | 88.23 | 1748.00 | 33 | 4.60 | 3.10 | `ADD_CANDIDATE` |
| `level_reversal` | `level_bounce_15m sl=0.7 tp=1.0` | 75.57 | 259.00 | 7 | 8.12 | 3.85 | `ADD_CANDIDATE` |
| `breakout_momentum` | `donchian_15m lookback=8` | 0.00 | -17.80 | 1 | 0.00 | 0.00 | `LOW_SAMPLE` |

Conclusion:

- Production was updated after this research: `MCM6` now runs `sma_crossover` on `1h` with `fast=4`, `slow=9`.
- The `level_bounce_15m` result remains a valid alternative benchmark, not the active production baseline.
- Because this is a single-stock future, the next check must include spread/liquidity and real fill quality.

## Parameter Patterns

Observed strong parameter clusters:

- Hourly trend: `fast=4 slow=9` is repeatedly strong on `BMM6`, `MCM6`, `SIM6`, `MGM6`, `NAM6`.
- Hourly trend: `fast=5 slow=17` is a useful second cluster, especially `NGM6`, `PXM6`, `BTM6`, `RNM6`.
- Level bounce: `tp=1.0-1.5` is usually more stable than large `tp=2.0` on core instruments.
- ORB: only a few candidates pass; `BTK6 orb_15m range=30 rr=2.0` is the strongest broad-universe breakout result.

## Recommendations

Production update selected after manual confirmation:

- Keep the three existing manual instance IDs to avoid deleting/recreating user-managed strategies.
- Change only strategy family and parameters inside the existing instances.
- Do not add broad-universe candidates like `SIM6` yet; they need separate approval because they expand traded instruments.

Selected production candidates:

| Priority | Candidate | Why |
|---:|---|---|
| 1 | `BMM6 sma_1h fast=4 slow=9` | Directly improves current Brent mini research result. |
| 2 | `NGM6 sma_1h fast=5 slow=17` | Same family as production, better parameters. |
| 3 | `MCM6 sma_1h fast=4 slow=9` | Strong result on the current Mechel futures contract. |

Watch-only candidates for later approval:

| Candidate | Why not enabled now |
|---|---|
| `SIM6 sma_1h fast=4 slow=9` | Best full-universe weighted candidate, but it adds a new instrument class. |
| `MXM6 sma_1h fast=9 slow=26` | Strong index future result, but it expands current trading scope. |

Reject for immediate production:

- Any strategy with fewer than 5 completed trades.
- Natural gas breakout strategies from this pass.
- Raw top PnL candidates that are low-volume single-stock futures without orderbook validation.

## Required Next Validation

Before any real-money deployment:

1. Add walk-forward train/test split.
2. Convert point PnL into RUB using real contract specs and min price increment.
3. Include commission, spread and slippage.
4. Check orderbook depth for shortlist candidates.
5. Run candidates in paper/watch mode before production execution.
