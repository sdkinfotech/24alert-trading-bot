"""ORB breakout on 15m — aligned with internal/strategy/orb (range_candles, EOD cutoff)."""
from backtestlib.schedule import MSK, is_main_session


def run_orb(candles_15m, range_candles=2, cutoff_hour=18, cutoff_min=30):
    """First N session candles per day form range; trade close breakouts; EOD flatten."""
    by_day = {}
    for c in candles_15m:
        if not is_main_session(c["time"]):
            continue
        day = c["time"].astimezone(MSK).strftime("%Y-%m-%d")
        by_day.setdefault(day, []).append(c)

    trades = []
    for day, rows in sorted(by_day.items()):
        range_high = 0.0
        range_low = float("inf")
        candle_count = 0
        range_formed = False
        pos = 0
        eod_sent = False

        for c in rows:
            t = c["time"].astimezone(MSK)
            candle_count += 1

            if candle_count <= range_candles:
                range_high = max(range_high, c["high"])
                range_low = min(range_low, c["low"])
                if candle_count == range_candles:
                    range_formed = True
                continue

            cutoff = t.replace(hour=cutoff_hour, minute=cutoff_min, second=0, microsecond=0)
            if t >= cutoff:
                if pos != 0 and not eod_sent:
                    d = "sell" if pos > 0 else "buy"
                    trades.append({"time": c["time"].isoformat(), "dir": d,
                                   "price": c["close"], "reason": "eod"})
                    eod_sent = True
                    pos = 0
                continue

            if not range_formed or range_low >= range_high:
                continue

            if c["close"] > range_high and pos <= 0:
                if pos < 0:
                    trades.append({"time": c["time"].isoformat(), "dir": "buy",
                                   "price": c["close"], "reason": "orb_reverse_long"})
                trades.append({"time": c["time"].isoformat(), "dir": "buy",
                               "price": c["close"], "reason": "orb_breakout_long"})
                pos = 1
            elif c["close"] < range_low and pos >= 0:
                if pos > 0:
                    trades.append({"time": c["time"].isoformat(), "dir": "sell",
                                   "price": c["close"], "reason": "orb_reverse_short"})
                trades.append({"time": c["time"].isoformat(), "dir": "sell",
                               "price": c["close"], "reason": "orb_breakout_short"})
                pos = -1

    return trades
