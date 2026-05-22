#!/usr/bin/env python3
"""Walk-forward: train 60d / test 30d for top matrix candidates."""
import argparse
import json
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "scripts"))

from backtestlib.data import get_candles  # noqa: E402
from backtestlib.metrics import enrich_run, risk_score  # noqa: E402
from backtestlib.instruments import CORE_INSTRUMENTS  # noqa: E402
from backtestlib.strategies import run_sma, run_level_bounce, run_ema, run_donchian, run_orb  # noqa: E402


def simulate(run: dict, c1h, c15, cdaily):
    s = run["strategy"]
    p = run.get("params", {})
    mode = run.get("mode", "")

    if s == "sma_crossover" or mode.startswith("sma") or mode == "prod_baseline":
        trades = run_sma(
            c1h, int(p.get("fast_period", p.get("fast", 5))),
            int(p.get("slow_period", p.get("slow", 17))),
            float(p.get("trailing_stop_pct", 0)),
            int(p.get("initial_stop_swing_bars", 0)),
            float(p.get("stop_loss_pct", 0)),
            float(p.get("take_profit_pct", 0)),
        )
        return enrich_run(s, p, trades, live_eligible=run.get("live_eligible", False),
                          mode=mode, interval="1h")
    if s == "level_bounce":
        trades = run_level_bounce(
            c15, cdaily,
            float(p.get("atr_mult", 0)),
            float(p["sl_mult"]), float(p["tp_mult"]),
            int(p["cutoff_hour"]), int(p["cutoff_min"]),
            int(p.get("level_days", 10)),
        )
        return enrich_run(s, p, trades, live_eligible=True, mode=mode, interval="15min")
    if s == "ema_1h":
        trades = run_ema(c1h, int(p["fast_period"]), int(p["slow_period"]))
        return enrich_run(s, p, trades, live_eligible=False, mode=mode, interval="1h")
    if s == "donchian_15m":
        trades = run_donchian(c15, int(p["lookback"]), float(p["atr_stop"]))
        return enrich_run(s, p, trades, live_eligible=False, mode=mode, interval="15min")
    if s == "orb_breakout":
        trades = run_orb(c15, int(p["range_candles"]),
                         int(p["cutoff_hour"]), int(p["cutoff_min"]))
        return enrich_run(s, p, trades, live_eligible=False, mode=mode, interval="15min")
    return None


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("matrix_json")
    ap.add_argument("--gateway-url", default="http://127.0.0.1:18080")
    ap.add_argument("--train-days", type=int, default=60)
    ap.add_argument("--test-days", type=int, default=30)
    ap.add_argument("--top", type=int, default=3)
    ap.add_argument("--json-out", default="")
    args = ap.parse_args()
    base = args.gateway_url.rstrip("/")
    matrix = json.loads(Path(args.matrix_json).read_text(encoding="utf-8"))

    out = {"train_days": args.train_days, "test_days": args.test_days, "tickers": []}

    for inst in matrix.get("instruments", []):
        if inst.get("error"):
            continue
        ticker = inst["ticker"]
        meta = next((i for i in CORE_INSTRUMENTS if i["ticker"] == ticker), None)
        if not meta:
            continue
        uid = meta["uid"]
        print(f"Walk-forward {ticker}...", file=sys.stderr)
        total_days = args.train_days + args.test_days + 5
        c1h = get_candles(base, uid, "1h", total_days)
        c15 = get_candles(base, uid, "15min", total_days)
        cdaily = get_candles(base, uid, "1d", 45)
        cutoff = datetime.now(timezone.utc) - timedelta(days=args.test_days)
        test1h = [c for c in c1h if c["time"] >= cutoff]
        train1h = [c for c in c1h if c["time"] < cutoff]
        test15 = [c for c in c15 if c["time"] >= cutoff]
        train15 = [c for c in c15 if c["time"] < cutoff]

        candidates = []
        for key in ("production", "best_deployable", "best_research"):
            if inst.get(key):
                candidates.append(inst[key])
        for key in ("top10_deployable", "top10_overall"):
            for r in inst.get(key, [])[: args.top]:
                if r not in candidates:
                    candidates.append(r)
        seen = set()
        unique = []
        for c in candidates:
            k = (c.get("strategy"), c.get("mode"), str(c.get("params")))
            if k not in seen:
                seen.add(k)
                unique.append(c)

        wf_rows = []
        for cand in unique[: args.top * 2]:
            tr = simulate(cand, train1h, train15, cdaily)
            te = simulate(cand, test1h, test15, cdaily)
            if not tr or not te:
                continue
            holds = te["pnl"] > 0 and te["risk_score"] > -1e8
            wf_rows.append({
                "strategy": cand["strategy"],
                "mode": cand["mode"],
                "params": cand["params"],
                "train_pnl": tr["pnl"],
                "train_risk": tr["risk_score"],
                "test_pnl": te["pnl"],
                "test_risk": te["risk_score"],
                "test_trades": te["trades"],
                "holds_on_test": holds,
            })
        wf_rows.sort(key=lambda x: x["test_risk"], reverse=True)
        out["tickers"].append({"ticker": ticker, "walk_forward": wf_rows[: args.top]})

    text = json.dumps(out, ensure_ascii=False, indent=2)
    if args.json_out:
        Path(args.json_out).write_text(text, encoding="utf-8")
    print(text)


if __name__ == "__main__":
    main()
