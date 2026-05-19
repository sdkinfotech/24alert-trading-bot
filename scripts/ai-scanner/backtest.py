#!/usr/bin/env python3
"""
Unified backtester for 24alert strategies: SMA Crossover and Level Bounce.
Fetches data live from the gateway API, runs strategy simulation with grid
optimization, and outputs JSON results.

Usage:
    python backtest.py --gateway-url http://gateway:8080 --uid <UID> --strategy sma --optimize
    python backtest.py --uid <UID> --strategy level_bounce --optimize --json
"""
import argparse
import json
import math
import sys
import urllib.request
from datetime import datetime, timedelta, timezone
from zoneinfo import ZoneInfo

MSK = ZoneInfo("Europe/Moscow")
TRADING_WINDOWS = [
    ("forts_day_before_clearing", 10 * 60, 14 * 60),
    ("forts_day_after_clearing", 14 * 60 + 5, 18 * 60 + 50),
    ("forts_evening", 19 * 60, 23 * 60 + 50),
]


def fetch_json(url):
    with urllib.request.urlopen(url, timeout=30) as r:
        return json.load(r)


def get_candles(base, uid, interval, days):
    to = datetime.now(timezone.utc)
    fr = to - timedelta(days=days + 5)
    url = (
        f"{base}/api/v1/candles?instrument_uid={uid}"
        f"&from={fr.strftime('%Y-%m-%dT%H:%M:%SZ')}"
        f"&to={to.strftime('%Y-%m-%dT%H:%M:%SZ')}"
        f"&interval={interval}"
    )
    try:
        data = fetch_json(url)
        raw = data.get("data", data) if isinstance(data, dict) else data
    except Exception as e:
        print(f"ERROR fetching candles: {e}", file=sys.stderr)
        return []
    candles = []
    for r in raw:
        candles.append({
            "open": r["open"], "high": r["high"], "low": r["low"],
            "close": r["close"], "volume": r.get("volume", 0),
            "time": datetime.fromisoformat(r["time"].replace("Z", "+00:00")),
        })
    return candles


def is_main_session(t):
    local = t.astimezone(MSK)
    if local.weekday() >= 5:
        return False
    minutes = local.hour * 60 + local.minute
    return any(start <= minutes < end for _, start, end in TRADING_WINDOWS)


# ─── PnL calculator ───

def calc_pnl(trades):
    entry = None
    pnl = 0.0
    wins = 0
    losses = 0
    max_dd = 0.0
    peak = 0.0
    equity = 0.0
    pnls = []

    for t in trades:
        if entry is None:
            entry = t
        elif t["dir"] != entry["dir"]:
            if entry["dir"] == "buy":
                p = t["price"] - entry["price"]
            else:
                p = entry["price"] - t["price"]
            pnl += p
            pnls.append(p)
            equity += p
            if equity > peak:
                peak = equity
            dd = peak - equity
            if dd > max_dd:
                max_dd = dd
            if p > 0:
                wins += 1
            else:
                losses += 1
            entry = t if "reverse" in t.get("reason", "") else None
        else:
            entry = t

    total = wins + losses
    win_rate = wins / total if total > 0 else 0

    gross_profit = sum(p for p in pnls if p > 0)
    gross_loss = abs(sum(p for p in pnls if p <= 0))
    profit_factor = gross_profit / gross_loss if gross_loss > 0 else (
        float("inf") if gross_profit > 0 else 0)

    if len(pnls) > 1:
        mean = sum(pnls) / len(pnls)
        var = sum((p - mean) ** 2 for p in pnls) / (len(pnls) - 1)
        std = var ** 0.5
        sharpe = (mean / std) * (252 ** 0.5) if std > 0 else 0
    else:
        sharpe = 0

    return {
        "pnl": round(pnl, 2),
        "trades": total,
        "wins": wins,
        "losses": losses,
        "win_rate": round(win_rate, 4),
        "max_drawdown": round(max_dd, 2),
        "sharpe": round(sharpe, 2),
        "profit_factor": round(profit_factor, 2),
    }


# ─── SMA Crossover ───

def run_sma(candles, fast_n, slow_n):
    closes = []
    pos = 0
    trades = []

    for c in candles:
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

        if prev_fast <= prev_slow and fast > slow and pos <= 0 and is_main_session(c["time"]):
            trades.append({"time": c["time"].isoformat(), "dir": "buy",
                           "price": c["close"], "reason": "golden_cross"})
            pos = 1
        elif prev_fast >= prev_slow and fast < slow and pos >= 0 and is_main_session(c["time"]):
            trades.append({"time": c["time"].isoformat(), "dir": "sell",
                           "price": c["close"], "reason": "death_cross"})
            pos = -1

    return trades


def optimize_sma(candles):
    best = None
    results = []
    for fast in range(3, 21):
        for slow in range(fast + 5, 61):
            trades = run_sma(candles, fast, slow)
            stats = calc_pnl(trades)
            entry = {"params": {"fast_period": fast, "slow_period": slow}, **stats}
            results.append(entry)
            if best is None or stats["pnl"] > best["pnl"]:
                best = entry
    return best, results


# ─── Level Bounce ───

def find_levels(daily_candles, n=10):
    if len(daily_candles) < n:
        return [], []
    highs = [c["high"] for c in daily_candles[-n:]]
    lows = [c["low"] for c in daily_candles[-n:]]
    return sorted(lows)[:3], sorted(highs, reverse=True)[:3]


def run_level_bounce(candles_15m, daily_candles, atr_mult=0.4, sl_mult=0.5,
                     tp_mult=1.5, cutoff_hour=18, cutoff_min=30, level_days=10):
    # atr_mult is kept for CLI/config compatibility. Production entry logic
    # requires an actual level touch; ATR only sizes stop-loss/take-profit.
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
    stop_loss = 0.0
    take_profit = 0.0
    current_day = ""
    eod_sent = False
    last_entry_day = ""
    last_entry_dir = ""
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
            duplicate_entry = (
                last_entry_day == day
                and last_entry_dir == "buy"
                and abs(last_entry_level - s) < 1e-9
            )
            if c["low"] <= s and c["close"] > s and not duplicate_entry:
                entry_price = c["close"]
                stop_loss = s - atr * sl_mult
                take_profit = entry_price + atr * tp_mult
                trades.append({"time": c["time"].isoformat(), "dir": "buy",
                               "price": entry_price, "reason": f"bounce_S={s:.1f}"})
                last_entry_day = day
                last_entry_dir = "buy"
                last_entry_level = s
                pos = 1
                break

        if pos != 0:
            continue

        for r in resistance:
            duplicate_entry = (
                last_entry_day == day
                and last_entry_dir == "sell"
                and abs(last_entry_level - r) < 1e-9
            )
            if c["high"] >= r and c["close"] < r and not duplicate_entry:
                entry_price = c["close"]
                stop_loss = r + atr * sl_mult
                take_profit = entry_price - atr * tp_mult
                trades.append({"time": c["time"].isoformat(), "dir": "sell",
                               "price": entry_price, "reason": f"reject_R={r:.1f}"})
                last_entry_day = day
                last_entry_dir = "sell"
                last_entry_level = r
                pos = -1
                break

    return trades


def optimize_level_bounce(candles_15m, candles_daily):
    best = None
    results = []
    for am in [0.0]:
        for sl in [0.3, 0.5, 0.7]:
            for tp in [1.0, 1.5, 2.0]:
                for ch in [17, 18, 23]:
                    for cm in [0, 30]:
                        trades = run_level_bounce(candles_15m, candles_daily,
                                                  am, sl, tp, ch, cm)
                        stats = calc_pnl(trades)
                        entry = {
                            "params": {
                                "atr_mult": am, "sl_mult": sl, "tp_mult": tp,
                                "cutoff_hour": ch, "cutoff_min": cm,
                            },
                            **stats,
                        }
                        results.append(entry)
                        if best is None or stats["pnl"] > best["pnl"]:
                            best = entry
    return best, results


def schedule_info():
    return {
        "timezone": "Europe/Moscow",
        "sessions": [
            {"name": name, "start": f"{start // 60:02d}:{start % 60:02d}", "end": f"{end // 60:02d}:{end % 60:02d}"}
            for name, start, end in TRADING_WINDOWS
        ],
        "weekdays_only": True,
    }


def main():
    parser = argparse.ArgumentParser(description="24alert strategy backtester")
    parser.add_argument("--gateway-url", default="http://127.0.0.1:18080",
                        help="Gateway base URL")
    parser.add_argument("--uid", required=True, help="T-Invest instrument UID")
    parser.add_argument("--strategy", required=True, choices=["sma", "level_bounce"],
                        help="Strategy type to backtest")
    parser.add_argument("--optimize", action="store_true",
                        help="Run parameter grid optimization")
    parser.add_argument("--json", action="store_true",
                        help="Output JSON only")
    parser.add_argument("--days", type=int, default=0,
                        help="History days (default: 90 for sma, 14 for level_bounce)")

    # SMA-specific
    parser.add_argument("--fast", type=int, default=12, help="SMA fast period")
    parser.add_argument("--slow", type=int, default=27, help="SMA slow period")

    # Level bounce-specific
    parser.add_argument("--atr-mult", type=float, default=0.4)
    parser.add_argument("--sl-mult", type=float, default=0.5)
    parser.add_argument("--tp-mult", type=float, default=1.5)
    parser.add_argument("--cutoff-hour", type=int, default=18)
    parser.add_argument("--cutoff-min", type=int, default=30)

    args = parser.parse_args()
    base = args.gateway_url.rstrip("/")

    if args.strategy == "sma":
        days = args.days if args.days > 0 else 90
        if not args.json:
            print(f"Fetching 1h candles for {days} days...", file=sys.stderr)
        candles = get_candles(base, args.uid, "1h", days)
        if len(candles) < 30:
            print(json.dumps({"error": "insufficient data", "candles": len(candles)}))
            sys.exit(1)

        if args.optimize:
            if not args.json:
                print("Running SMA grid optimization...", file=sys.stderr)
            best, all_results = optimize_sma(candles)
            top10 = sorted(all_results, key=lambda r: r["pnl"], reverse=True)[:10]
            output = {
                "strategy": "sma_crossover",
                "uid": args.uid,
                "candles": len(candles),
                "schedule": schedule_info(),
                "best": best,
                "top10": top10,
            }
        else:
            trades = run_sma(candles, args.fast, args.slow)
            stats = calc_pnl(trades)
            output = {
                "strategy": "sma_crossover",
                "uid": args.uid,
                "candles": len(candles),
                "schedule": schedule_info(),
                "params": {"fast_period": args.fast, "slow_period": args.slow},
                **stats,
                "trade_log": trades[-20:],
            }

    elif args.strategy == "level_bounce":
        days_15m = args.days if args.days > 0 else 14
        if not args.json:
            print(f"Fetching 15m ({days_15m}d) and daily (30d) candles...", file=sys.stderr)
        candles_15m = get_candles(base, args.uid, "15min", days_15m)
        candles_daily = get_candles(base, args.uid, "1d", 30)
        if len(candles_15m) < 50:
            print(json.dumps({"error": "insufficient data", "candles_15m": len(candles_15m)}))
            sys.exit(1)

        if args.optimize:
            if not args.json:
                print("Running Level Bounce grid optimization...", file=sys.stderr)
            best, all_results = optimize_level_bounce(candles_15m, candles_daily)
            top10 = sorted(all_results, key=lambda r: r["pnl"], reverse=True)[:10]
            output = {
                "strategy": "level_bounce",
                "uid": args.uid,
                "candles_15m": len(candles_15m),
                "candles_daily": len(candles_daily),
                "schedule": schedule_info(),
                "best": best,
                "top10": top10,
            }
        else:
            trades = run_level_bounce(
                candles_15m, candles_daily,
                args.atr_mult, args.sl_mult, args.tp_mult,
                args.cutoff_hour, args.cutoff_min,
            )
            stats = calc_pnl(trades)
            output = {
                "strategy": "level_bounce",
                "uid": args.uid,
                "candles_15m": len(candles_15m),
                "candles_daily": len(candles_daily),
                "schedule": schedule_info(),
                "params": {
                    "atr_mult": args.atr_mult, "sl_mult": args.sl_mult,
                    "tp_mult": args.tp_mult,
                    "cutoff_hour": args.cutoff_hour, "cutoff_min": args.cutoff_min,
                },
                **stats,
                "trade_log": trades[-20:],
            }

    json.dump(output, sys.stdout, ensure_ascii=False, indent=2, default=str)
    print()

    if not args.json and "best" in output and output["best"]:
        b = output["best"]
        print(f"\nBest: params={b['params']} PnL={b['pnl']} "
              f"Trades={b['trades']} WinRate={b['win_rate']:.0%} "
              f"Sharpe={b['sharpe']}", file=sys.stderr)


if __name__ == "__main__":
    main()
