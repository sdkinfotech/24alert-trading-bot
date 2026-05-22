#!/usr/bin/env python3
"""
Unified backtester for 24alert strategies (thin wrapper over backtestlib).
Docker path: /opt/ai-scanner/backtest.py — keep this file for compatibility.
"""
import argparse
import json
import sys
from pathlib import Path

_SCRIPTS = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(_SCRIPTS))

from backtestlib.data import get_candles  # noqa: E402
from backtestlib.metrics import calc_pnl, risk_score  # noqa: E402
from backtestlib.schedule import schedule_info  # noqa: E402
from backtestlib.strategies.sma import run_sma  # noqa: E402
from backtestlib.strategies.level_bounce import run_level_bounce  # noqa: E402


def optimize_sma(candles, trailing_pct=0.0, swing_bars=0):
    best = None
    results = []
    for fast in range(3, 21):
        for slow in range(fast + 5, 61):
            trades = run_sma(candles, fast, slow, trailing_pct, swing_bars)
            stats = calc_pnl(trades)
            entry = {
                "mode": "sma_cross",
                "params": {
                    "fast_period": fast, "slow_period": slow,
                    "trailing_stop_pct": trailing_pct,
                    "initial_stop_swing_bars": swing_bars,
                },
                **stats,
                "risk_score": round(risk_score(stats), 4),
            }
            results.append(entry)
            if best is None or entry["risk_score"] > best["risk_score"]:
                best = entry
    return best, results


def optimize_sma_trailing(candles, fast_n, slow_n, swing_bars=0):
    best = None
    results = []
    for pct_milli in range(3, 16):
        pct = pct_milli / 1000.0
        trades = run_sma(candles, fast_n, slow_n, pct, swing_bars)
        stats = calc_pnl(trades)
        entry = {
            "mode": "sma_trailing",
            "params": {
                "fast_period": fast_n, "slow_period": slow_n,
                "trailing_stop_pct": pct, "initial_stop_swing_bars": swing_bars,
            },
            **stats,
            "risk_score": round(risk_score(stats), 4),
        }
        results.append(entry)
        if best is None or entry["risk_score"] > best["risk_score"]:
            best = entry
    return best, results


def optimize_sma_sl_tp(candles, fast_n, slow_n, swing_bars=0):
    best = None
    results = []
    for sl_m in range(5, 21, 3):
        sl = sl_m / 1000.0
        for tp_m in range(10, 41, 5):
            tp = tp_m / 1000.0
            trades = run_sma(candles, fast_n, slow_n, 0, swing_bars, sl, tp)
            stats = calc_pnl(trades)
            entry = {
                "mode": "sma_sl_tp",
                "params": {
                    "fast_period": fast_n, "slow_period": slow_n,
                    "stop_loss_pct": sl, "take_profit_pct": tp,
                    "initial_stop_swing_bars": swing_bars,
                },
                **stats,
                "risk_score": round(risk_score(stats), 4),
            }
            results.append(entry)
            if best is None or entry["risk_score"] > best["risk_score"]:
                best = entry
    return best, results


def optimize_level_bounce(candles_15m, candles_daily):
    best = None
    results = []
    for sl in [0.3, 0.5, 0.7]:
        for tp in [1.0, 1.5, 2.0]:
            for ch in [17, 18, 23]:
                for cm in [0, 30]:
                    trades = run_level_bounce(candles_15m, candles_daily, 0.0, sl, tp, ch, cm)
                    stats = calc_pnl(trades)
                    entry = {
                        "mode": "level_bounce",
                        "params": {
                            "atr_mult": 0.0, "sl_mult": sl, "tp_mult": tp,
                            "cutoff_hour": ch, "cutoff_min": cm,
                        },
                        **stats,
                        "risk_score": round(risk_score(stats), 4),
                    }
                    results.append(entry)
                    if best is None or entry["risk_score"] > best["risk_score"]:
                        best = entry
    return best, results


def main():
    parser = argparse.ArgumentParser(description="24alert strategy backtester")
    parser.add_argument("--gateway-url", default="http://127.0.0.1:18080")
    parser.add_argument("--uid", required=True)
    parser.add_argument("--strategy", required=True, choices=["sma", "level_bounce"])
    parser.add_argument("--optimize", action="store_true")
    parser.add_argument("--json", action="store_true")
    parser.add_argument("--days", type=int, default=0)
    parser.add_argument("--fast", type=int, default=12)
    parser.add_argument("--slow", type=int, default=27)
    parser.add_argument("--trailing-pct", type=float, default=0.0)
    parser.add_argument("--swing-bars", type=int, default=0)
    parser.add_argument("--optimize-trailing", action="store_true")
    parser.add_argument("--atr-mult", type=float, default=0.4)
    parser.add_argument("--sl-mult", type=float, default=0.5)
    parser.add_argument("--tp-mult", type=float, default=1.5)
    parser.add_argument("--cutoff-hour", type=int, default=18)
    parser.add_argument("--cutoff-min", type=int, default=30)
    args = parser.parse_args()
    base = args.gateway_url.rstrip("/")

    if args.strategy == "sma":
        days = args.days or 90
        candles = get_candles(base, args.uid, "1h", days)
        if len(candles) < 30:
            print(json.dumps({"error": "insufficient data", "candles": len(candles)}))
            sys.exit(1)
        if args.optimize_trailing:
            best, all_results = optimize_sma_trailing(candles, args.fast, args.slow, args.swing_bars)
            output = {"strategy": "sma_crossover", "uid": args.uid, "candles": len(candles),
                      "schedule": schedule_info(), "mode": "optimize_trailing",
                      "best": best, "top10": sorted(all_results, key=lambda r: r["sharpe"], reverse=True)[:10]}
        elif args.optimize:
            best, all_results = optimize_sma(candles, args.trailing_pct, args.swing_bars)
            output = {"strategy": "sma_crossover", "uid": args.uid, "candles": len(candles),
                      "schedule": schedule_info(), "best": best,
                      "top10": sorted(all_results, key=lambda r: r["pnl"], reverse=True)[:10]}
        else:
            trades = run_sma(candles, args.fast, args.slow, args.trailing_pct, args.swing_bars)
            stats = calc_pnl(trades)
            output = {
                "strategy": "sma_crossover", "uid": args.uid, "candles": len(candles),
                "schedule": schedule_info(),
                "params": {"fast_period": args.fast, "slow_period": args.slow,
                           "trailing_stop_pct": args.trailing_pct,
                           "initial_stop_swing_bars": args.swing_bars},
                **stats, "trade_log": trades[-20:],
            }
    else:
        days_15m = args.days or 14
        candles_15m = get_candles(base, args.uid, "15min", days_15m)
        candles_daily = get_candles(base, args.uid, "1d", 30)
        if len(candles_15m) < 50:
            print(json.dumps({"error": "insufficient data", "candles_15m": len(candles_15m)}))
            sys.exit(1)
        if args.optimize:
            best, all_results = optimize_level_bounce(candles_15m, candles_daily)
            output = {"strategy": "level_bounce", "uid": args.uid,
                      "candles_15m": len(candles_15m), "candles_daily": len(candles_daily),
                      "schedule": schedule_info(), "best": best,
                      "top10": sorted(all_results, key=lambda r: r["pnl"], reverse=True)[:10]}
        else:
            trades = run_level_bounce(
                candles_15m, candles_daily, args.atr_mult, args.sl_mult, args.tp_mult,
                args.cutoff_hour, args.cutoff_min)
            stats = calc_pnl(trades)
            output = {
                "strategy": "level_bounce", "uid": args.uid,
                "candles_15m": len(candles_15m), "candles_daily": len(candles_daily),
                "schedule": schedule_info(),
                "params": {"atr_mult": args.atr_mult, "sl_mult": args.sl_mult,
                           "tp_mult": args.tp_mult, "cutoff_hour": args.cutoff_hour,
                           "cutoff_min": args.cutoff_min},
                **stats, "trade_log": trades[-20:],
            }

    print(json.dumps(output, ensure_ascii=False, indent=2, default=str))


if __name__ == "__main__":
    main()
