"""EMA crossover 1h (research)."""
from backtestlib.schedule import is_main_session


def ema(prev, value, n):
    alpha = 2 / (n + 1)
    return value if prev is None else prev + alpha * (value - prev)


def run_ema(candles, fast_n, slow_n):
    ef = es = None
    pos = 0
    trades = []
    for c in candles:
        pf, ps = ef, es
        ef = ema(ef, c["close"], fast_n)
        es = ema(es, c["close"], slow_n)
        if pf is None or ps is None or not is_main_session(c["time"]):
            continue
        if pf <= ps and ef > es and pos <= 0:
            trades.append({"time": c["time"].isoformat(), "dir": "buy",
                           "price": c["close"], "reason": "ema_cross"})
            pos = 1
        elif pf >= ps and ef < es and pos >= 0:
            trades.append({"time": c["time"].isoformat(), "dir": "sell",
                           "price": c["close"], "reason": "ema_cross"})
            pos = -1
    return trades
