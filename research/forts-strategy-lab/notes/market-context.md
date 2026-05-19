# FORTS Market Context

## Scope

First research iteration focuses on FORTS futures that can be traded by 24alert:

- Core energy contracts: Brent mini (`BM*`), Brent (`BR*`), Natural Gas (`NG*`).
- Current production contracts: `BMM6`, `NGM6`, `MCM6`.
- Nearby liquid MOEX futures for comparison: RTS/MOEX index futures, Gazprom, Sber, Mechel and other contracts available via gateway.

## Public Strategy Patterns

### Brent / Crude Oil

Common intraday patterns found in public systematic trading material:

- Opening range breakout: define the first 15-30 minutes after session start, then trade close/break through range high/low.
- VWAP or mean reversion: fade stretched moves back toward a rolling mean/VWAP-like proxy, especially on non-trending days.
- Volatility breakout: trade only when ATR/range expands and the breakout candle has enough body/volume.
- Failed breakout: if price breaks a prior daily high/low and closes back inside, trade reversal with tight stop.

Operational risks:

- Oil contracts can gap on external news and inventory/geopolitical headlines.
- Evening session may carry useful continuation, but liquidity often decays later in the evening.
- A strategy must distinguish trend days from mean-reverting days; simple counter-trend entries can be dangerous.

### Natural Gas

Natural gas has a different profile from oil:

- High volatility and frequent false breakouts.
- Strong sensitivity to weather, storage/inventory reports, seasonality and LNG/global gas context.
- Breakout logic is most plausible around new information or volatility expansion.
- Mean reversion may work in shoulder seasons or after sharp short-term overshoots, but stops must be strict.

Operational risks:

- Larger single-bar moves can dominate small samples.
- False breakouts are common, so volume/range filters are important.
- Position sizing should be conservative even when backtest metrics look attractive.

### MOEX/FORTS Session Context

Production trading guard for this project:

- Mon-Fri only.
- `10:00-14:00` MSK.
- `14:05-18:50` MSK.
- `19:00-23:50` MSK.
- Clearing break `14:00-14:05` is not tradable.
- Weekend trading is intentionally disabled due to low liquidity / thin market.

Research implication:

- Backtests must not count trades outside this schedule.
- Session-specific strategies should be evaluated separately for morning, post-clearing and evening session.
- For current `level_bounce`, entry must require an actual touch of support/resistance, not proximity via ATR.

## Research Criteria

Balanced selection, not maximum PnL:

- Positive PnL.
- Enough trades for a first-pass signal.
- Acceptable max drawdown.
- Sharpe/profit factor not driven by one lucky trade.
- Robustness across nearby parameters and instruments.
- Explains real market behavior rather than data artifacts.
