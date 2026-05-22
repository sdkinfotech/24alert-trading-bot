"""Donchian breakout 15m + ATR stop (research)."""
from backtestlib.schedule import is_main_session


def average_true_range(candles, n=14):
    trs = []
    for i in range(1, len(candles)):
        c = candles[i]
        pc = candles[i - 1]["close"]
        trs.append(max(c["high"] - c["low"], abs(c["high"] - pc), abs(c["low"] - pc)))
    return sum(trs[-n:]) / min(n, len(trs)) if trs else 0.0


def run_donchian(candles, lookback, atr_stop=1.0):
    pos = 0
    stop = 0.0
    trades = []
    for idx, c in enumerate(candles):
        if idx < lookback + 1 or not is_main_session(c["time"]):
            continue
        prev = candles[idx - lookback:idx]
        high = max(x["high"] for x in prev)
        low = min(x["low"] for x in prev)
        atr = average_true_range(candles[max(0, idx - 30):idx + 1], 14)
        if pos > 0 and c["low"] <= stop:
            trades.append({"time": c["time"].isoformat(), "dir": "sell",
                           "price": stop, "reason": "atr_stop"})
            pos = 0
        elif pos < 0 and c["high"] >= stop:
            trades.append({"time": c["time"].isoformat(), "dir": "buy",
                           "price": stop, "reason": "atr_stop"})
            pos = 0
        if pos == 0 and c["close"] > high:
            entry = c["close"]
            stop = entry - atr * atr_stop
            trades.append({"time": c["time"].isoformat(), "dir": "buy",
                           "price": entry, "reason": "donchian_breakout"})
            pos = 1
        elif pos == 0 and c["close"] < low:
            entry = c["close"]
            stop = entry + atr * atr_stop
            trades.append({"time": c["time"].isoformat(), "dir": "sell",
                           "price": entry, "reason": "donchian_breakout"})
            pos = -1
    return trades
