# Trend Following Hypotheses

## SMA / EMA Crossover

Idea: trade sustained directional moves on 1h candles, avoiding noise from 15m bars.

Entry:

- Long when fast average crosses above slow average during allowed FORTS session.
- Short when fast average crosses below slow average during allowed FORTS session.

Exit:

- Reverse on opposite cross.
- Optional session cutoff for instruments where late evening liquidity is poor.

Risk:

- Whipsaw in flat ranges.
- Slow reaction after sharp news moves.

## Donchian / Channel Breakout

Idea: trade momentum when price closes outside a recent high/low channel.

Entry:

- Long when close breaks above prior `N` bar high.
- Short when close breaks below prior `N` bar low.
- Optional ATR filter: current ATR must be above recent ATR average.

Exit:

- Opposite channel break or ATR stop.

Risk:

- False breakouts, especially on natural gas.
- Needs volume/range confirmation.

## Intraday Momentum

Idea: trade continuation when a strong candle closes near its high/low with elevated volume/range.

Entry:

- Long on large bullish body and close near high.
- Short on large bearish body and close near low.

Exit:

- Fixed ATR target/stop or next opposite momentum bar.

Risk:

- Can chase exhaustion if not filtered by session and volatility regime.
