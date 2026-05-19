# Weighted Scoring Methodology

The raw backtest winner is not always the best trading candidate. A strategy with huge PnL but low volume, few trades or one lucky move can be dangerous. The weighted optimizer ranks candidates by a balanced composite score.

## Balanced Profile

Weights used in `reports/weighted-rankings.json`:

| Component | Weight | Meaning |
|---|---:|---|
| PnL | 0.25 | Strategy must make money, but PnL alone is not enough. |
| Sharpe | 0.22 | Rewards stable returns per trade. |
| Profit factor | 0.18 | Rewards gross profit over gross loss. |
| Trades | 0.14 | Penalizes tiny samples. |
| Drawdown | 0.11 | Rewards smoother equity and lower drawdown relative to PnL. |
| Liquidity | 0.10 | Uses average candle volume; production core tickers get a minimum liquidity floor. |

## Penalties

The optimizer penalizes:

- Fewer than 5 completed trades.
- Non-positive PnL.
- Profit factor below 1.2.
- Max drawdown larger than PnL.

## Action Labels

| Label | Meaning |
|---|---|
| `ADD_CANDIDATE` | Strong enough for a paper/watch candidate, not automatic production trading. |
| `WATCH` | Interesting but needs more evidence. |
| `LOW_SAMPLE` | Too few trades to trust. |
| `REJECT` | Not worth further work in this pass. |

## Important Limitations

- Score is not final profitability in rubles.
- Contract multiplier, commission, slippage and spread are not yet included.
- Volume proxy is candle volume only, not orderbook depth.
- External event risk is not modeled, especially for natural gas.
