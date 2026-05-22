"""SMA crossover — aligned with internal/strategy/sma."""
from backtestlib.schedule import is_main_session


def swing_stop_price(recent_bars, pos, swing_bars):
    if swing_bars <= 0 or not recent_bars or pos == 0:
        return 0.0
    window = recent_bars[-swing_bars:]
    if pos < 0:
        return max(b["high"] for b in window)
    return min(b["low"] for b in window)


def run_sma(candles, fast_n, slow_n, trailing_pct=0.0, swing_bars=0,
            sl_pct=0.0, tp_pct=0.0):
    closes = []
    pos = 0
    trades = []
    trailing_best = 0.0
    structural_stop = 0.0
    stop_loss = 0.0
    take_profit = 0.0
    recent = []

    def clear_pos():
        nonlocal pos, trailing_best, structural_stop, stop_loss, take_profit
        pos = 0
        trailing_best = 0
        structural_stop = 0
        stop_loss = 0
        take_profit = 0

    def set_brackets(entry_px, direction):
        nonlocal stop_loss, take_profit, structural_stop, trailing_best
        trailing_best = entry_px
        structural_stop = swing_stop_price(recent[:-1], direction, swing_bars)
        if sl_pct > 0:
            stop_loss = entry_px * (1 - sl_pct) if direction > 0 else entry_px * (1 + sl_pct)
        if tp_pct > 0:
            take_profit = entry_px * (1 + tp_pct) if direction > 0 else entry_px * (1 - tp_pct)

    for c in candles:
        recent.append(c)
        if len(recent) > max(swing_bars, slow_n) + 10:
            recent = recent[-(max(swing_bars, slow_n) + 10):]

        closes.append(c["close"])
        if len(closes) < slow_n:
            continue
        if len(closes) > slow_n + 5:
            closes = closes[-(slow_n + 5):]

        def sma_tail(xs, n):
            return sum(xs[-n:]) / n if len(xs) >= n else 0

        fast = sma_tail(closes, fast_n)
        slow = sma_tail(closes, slow_n)
        prev = closes[:-1]
        if len(prev) < slow_n:
            continue
        prev_fast = sma_tail(prev, fast_n)
        prev_slow = sma_tail(prev, slow_n)

        if pos > 0:
            if sl_pct > 0 and c["low"] <= stop_loss:
                trades.append({"time": c["time"].isoformat(), "dir": "sell",
                               "price": stop_loss, "reason": f"sl_{sl_pct*100:.2f}%"})
                clear_pos()
            elif tp_pct > 0 and c["high"] >= take_profit:
                trades.append({"time": c["time"].isoformat(), "dir": "sell",
                               "price": take_profit, "reason": f"tp_{tp_pct*100:.2f}%"})
                clear_pos()
        elif pos < 0:
            if sl_pct > 0 and c["high"] >= stop_loss:
                trades.append({"time": c["time"].isoformat(), "dir": "buy",
                               "price": stop_loss, "reason": f"sl_{sl_pct*100:.2f}%"})
                clear_pos()
            elif tp_pct > 0 and c["low"] <= take_profit:
                trades.append({"time": c["time"].isoformat(), "dir": "buy",
                               "price": take_profit, "reason": f"tp_{tp_pct*100:.2f}%"})
                clear_pos()

        if pos != 0 and trailing_pct > 0:
            if trailing_best <= 0:
                trailing_best = c["close"]
            if pos > 0:
                trailing_best = max(trailing_best, c["high"])
                stop = trailing_best * (1 - trailing_pct)
                if c["low"] <= stop:
                    trades.append({"time": c["time"].isoformat(), "dir": "sell",
                                   "price": stop, "reason": f"trailing_{trailing_pct*100:.2f}%"})
                    clear_pos()
            else:
                trailing_best = min(trailing_best, c["low"])
                stop = trailing_best * (1 + trailing_pct)
                if c["high"] >= stop:
                    trades.append({"time": c["time"].isoformat(), "dir": "buy",
                                   "price": stop, "reason": f"trailing_{trailing_pct*100:.2f}%"})
                    clear_pos()

        if pos != 0 and swing_bars > 0 and structural_stop > 0:
            if pos > 0 and c["low"] <= structural_stop:
                trades.append({"time": c["time"].isoformat(), "dir": "sell",
                               "price": structural_stop, "reason": "swing_stop"})
                clear_pos()
            elif pos < 0 and c["high"] >= structural_stop:
                trades.append({"time": c["time"].isoformat(), "dir": "buy",
                               "price": structural_stop, "reason": "swing_stop"})
                clear_pos()

        if not is_main_session(c["time"]):
            continue

        if prev_fast <= prev_slow and fast > slow:
            if pos < 0:
                trades.append({"time": c["time"].isoformat(), "dir": "buy",
                               "price": c["close"], "reason": "reverse_cross"})
            if pos <= 0:
                trades.append({"time": c["time"].isoformat(), "dir": "buy",
                               "price": c["close"], "reason": "golden_cross"})
                pos = 1
                set_brackets(c["close"], 1)
        elif prev_fast >= prev_slow and fast < slow:
            if pos > 0:
                trades.append({"time": c["time"].isoformat(), "dir": "sell",
                               "price": c["close"], "reason": "reverse_cross"})
            if pos >= 0:
                trades.append({"time": c["time"].isoformat(), "dir": "sell",
                               "price": c["close"], "reason": "death_cross"})
                pos = -1
                set_brackets(c["close"], -1)

    return trades
