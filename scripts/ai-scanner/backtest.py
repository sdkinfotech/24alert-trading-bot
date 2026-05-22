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
from backtestlib.instruments import CORE_INSTRUMENTS  # noqa: E402
from backtestlib.metrics import calc_pnl, risk_score  # noqa: E402
from backtestlib.schedule import schedule_info  # noqa: E402
from backtestlib.strategies.sma import run_sma  # noqa: E402
from backtestlib.strategies.level_bounce import run_level_bounce  # noqa: E402

TRAILING_GRID_PCT = [x / 1000.0 for x in range(3, 16)]


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
    for pct in TRAILING_GRID_PCT:
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


def _tag_sma_row(entry, *, live_eligible, block_reason="", strategy="sma_crossover"):
    entry = dict(entry)
    entry["strategy"] = strategy
    entry["live_eligible"] = live_eligible
    if block_reason:
        entry["live_block_reason"] = block_reason
    return entry


def _tag_sma_rows(rows, live_eligible, reason=""):
    for e in rows:
        e["strategy"] = "sma_crossover"
        e["live_eligible"] = live_eligible
        if not live_eligible:
            e["live_block_reason"] = reason or "trailing_stop_pct required for live SMA"
    return rows


def prod_baseline_row(candles, prod_params):
    """Backtest current production SMA params for comparison."""
    if not prod_params:
        return None
    fast = int(prod_params["fast_period"])
    slow = int(prod_params["slow_period"])
    trail = float(prod_params.get("trailing_stop_pct", 0) or 0)
    swing = int(prod_params.get("initial_stop_swing_bars", 0) or 0)
    trades = run_sma(candles, fast, slow, trail, swing)
    stats = calc_pnl(trades)
    row = {
        "strategy": "sma_crossover",
        "mode": "prod",
        "params": {
            "fast_period": fast,
            "slow_period": slow,
            "trailing_stop_pct": trail,
            "initial_stop_swing_bars": swing,
            "interval": prod_params.get("interval", "1h"),
        },
        **stats,
        "risk_score": round(risk_score(stats), 4),
        "live_eligible": trail > 0,
    }
    return row


def optimize_sma_deployable(candles, swing_bars=0, prod_params=None):
    """Step 1: best fast/slow without trailing; step 2: trailing grid on that pair."""
    best_cross, _all_cross = optimize_sma(candles, 0.0, swing_bars)
    if not best_cross:
        return None, None, [], None, None
    fast = int(best_cross["params"]["fast_period"])
    slow = int(best_cross["params"]["slow_period"])
    best_trail, all_trail = optimize_sma_trailing(candles, fast, slow, swing_bars)
    _tag_sma_rows(all_trail, True)
    deployable = sorted(all_trail, key=lambda r: r.get("risk_score", -1e9), reverse=True)
    top = deployable[:12]
    best_deployable = top[0] if top else None
    optimization = {
        "kind": "sma_two_step",
        "fixed_fast": fast,
        "fixed_slow": slow,
        "trailing_grid_pct": TRAILING_GRID_PCT,
        "step1_best_risk_score": best_cross.get("risk_score"),
        "note_ru": (
            f"Сначала подобраны fast={fast} / slow={slow} без трейлинга (~{len(range(3, 21)) * 50} комбинаций), "
            f"затем перебран trailing 0.3%–1.5% ({len(TRAILING_GRID_PCT)} вариантов). "
            "Строки в таблице — не разные стратегии, а одна пара SMA с разным trailing."
        ),
        "note_en": (
            f"Step 1 picked fast={fast} / slow={slow} with no trailing; "
            f"step 2 swept trailing 0.3%–1.5% ({len(TRAILING_GRID_PCT)} values). "
            "Table rows are the same SMA pair, not different strategies."
        ),
    }
    production = prod_baseline_row(candles, prod_params)
    return best_deployable, best_deployable, top, optimization, production


def prod_params_for_uid(uid):
    for inst in CORE_INSTRUMENTS:
        if inst.get("uid") == uid:
            return inst.get("prod")
    return None


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
    parser.add_argument(
        "--optimize-deployable",
        action="store_true",
        help="SMA: optimize periods then trailing (live-ready)",
    )
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
        elif args.optimize_deployable:
            prod = prod_params_for_uid(args.uid)
            best_dep, best_any, combined, optimization, production = optimize_sma_deployable(
                candles, args.swing_bars, prod,
            )
            output = {
                "strategy": "sma_crossover", "uid": args.uid, "candles": len(candles),
                "schedule": schedule_info(), "mode": "optimize_deployable",
                "best_deployable": best_dep, "best": best_any,
                "top10": combined,
                "optimization": optimization,
                "production": production,
            }
        elif args.optimize:
            best, all_results = optimize_sma(candles, args.trailing_pct, args.swing_bars)
            _tag_sma_rows(all_results, False)
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
            for e in all_results:
                e["strategy"] = "level_bounce"
                e["live_eligible"] = True
            if best:
                best["strategy"] = "level_bounce"
                best["live_eligible"] = True
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
