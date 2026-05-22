"""Level bounce — aligned with internal/strategy/lb."""
from backtestlib.schedule import MSK, is_main_session


def find_levels(daily_candles, n=10):
    if len(daily_candles) < n:
        return [], []
    highs = [c["high"] for c in daily_candles[-n:]]
    lows = [c["low"] for c in daily_candles[-n:]]
    return sorted(lows)[:3], sorted(highs, reverse=True)[:3]


def run_level_bounce(candles_15m, daily_candles, atr_mult=0.4, sl_mult=0.5,
                     tp_mult=1.5, cutoff_hour=18, cutoff_min=30, level_days=10):
    support, resistance = find_levels(daily_candles, level_days)
    if not support or not resistance:
        return []

    trs = []
    for i in range(1, len(daily_candles)):
        c = daily_candles[i]
        pc = daily_candles[i - 1]["close"]
        tr = max(c["high"] - c["low"], abs(c["high"] - pc), abs(c["low"] - pc))
        trs.append(tr)
    atr = sum(trs[-14:]) / min(14, len(trs)) if trs else 0

    trades = []
    pos = 0
    stop_loss = take_profit = 0.0
    current_day = ""
    eod_sent = False
    last_entry_day = last_entry_dir = ""
    last_entry_level = 0.0

    for c in candles_15m:
        t = c["time"].astimezone(MSK)
        tradable = is_main_session(c["time"])
        day = t.strftime("%Y-%m-%d")
        if day != current_day:
            current_day = day
            eod_sent = False

        cutoff = t.replace(hour=cutoff_hour, minute=cutoff_min, second=0, microsecond=0)
        if t >= cutoff:
            if pos != 0 and not eod_sent and tradable:
                d = "sell" if pos > 0 else "buy"
                trades.append({"time": c["time"].isoformat(), "dir": d,
                               "price": c["close"], "reason": "eod"})
                eod_sent = True
                pos = 0
            continue

        if not tradable:
            continue

        if pos > 0:
            if c["low"] <= stop_loss:
                trades.append({"time": c["time"].isoformat(), "dir": "sell",
                               "price": stop_loss, "reason": "stop"})
                pos = 0
                continue
            if c["high"] >= take_profit:
                trades.append({"time": c["time"].isoformat(), "dir": "sell",
                               "price": take_profit, "reason": "tp"})
                pos = 0
                continue
        elif pos < 0:
            if c["high"] >= stop_loss:
                trades.append({"time": c["time"].isoformat(), "dir": "buy",
                               "price": stop_loss, "reason": "stop"})
                pos = 0
                continue
            if c["low"] <= take_profit:
                trades.append({"time": c["time"].isoformat(), "dir": "buy",
                               "price": take_profit, "reason": "tp"})
                pos = 0
                continue

        if pos != 0:
            continue

        for s in support:
            dup = last_entry_day == day and last_entry_dir == "buy" and abs(last_entry_level - s) < 1e-9
            if c["low"] <= s and c["close"] > s and not dup:
                entry_price = c["close"]
                stop_loss = s - atr * sl_mult
                take_profit = entry_price + atr * tp_mult
                trades.append({"time": c["time"].isoformat(), "dir": "buy",
                               "price": entry_price, "reason": f"bounce_S={s:.1f}"})
                last_entry_day, last_entry_dir, last_entry_level = day, "buy", s
                pos = 1
                break

        if pos != 0:
            continue

        for r in resistance:
            dup = last_entry_day == day and last_entry_dir == "sell" and abs(last_entry_level - r) < 1e-9
            if c["high"] >= r and c["close"] < r and not dup:
                entry_price = c["close"]
                stop_loss = r + atr * sl_mult
                take_profit = entry_price - atr * tp_mult
                trades.append({"time": c["time"].isoformat(), "dir": "sell",
                               "price": entry_price, "reason": f"reject_R={r:.1f}"})
                last_entry_day, last_entry_dir, last_entry_level = day, "sell", r
                pos = -1
                break

    return trades
