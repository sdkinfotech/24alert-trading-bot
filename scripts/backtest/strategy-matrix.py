#!/usr/bin/env python3
"""Full strategy × parameter matrix for core futures."""
import argparse
import json
import sys
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "scripts"))

from backtestlib.data import get_candles  # noqa: E402
from backtestlib.instruments import CORE_INSTRUMENTS  # noqa: E402
from backtestlib.matrix import run_matrix_for_ticker, dedupe_best  # noqa: E402
from backtestlib.schedule import schedule_info  # noqa: E402


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--gateway-url", default="http://127.0.0.1:18080")
    ap.add_argument("--tickers", default="BMM6,NGM6,MCM6")
    ap.add_argument("--days", type=int, default=90)
    ap.add_argument("--json-out", default="")
    args = ap.parse_args()
    base = args.gateway_url.rstrip("/")
    tickers = {t.strip().upper() for t in args.tickers.split(",") if t.strip()}

    instruments = [i for i in CORE_INSTRUMENTS if i["ticker"] in tickers]
    if not instruments:
        print("No matching instruments", file=sys.stderr)
        sys.exit(1)

    out = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "gateway": base,
        "days": args.days,
        "schedule": schedule_info(),
        "instruments": [],
    }

    for inst in instruments:
        print(f"=== {inst['ticker']} ===", file=sys.stderr)
        uid = inst["uid"]
        print(f"  fetch 1h ({args.days}d)...", file=sys.stderr)
        c1h = get_candles(base, uid, "1h", args.days)
        print(f"  fetch 15m (14d)...", file=sys.stderr)
        c15 = get_candles(base, uid, "15min", 14)
        print(f"  fetch 1d (30d)...", file=sys.stderr)
        cdaily = get_candles(base, uid, "1d", 30)

        if len(c1h) < 30:
            out["instruments"].append({
                "ticker": inst["ticker"], "error": "insufficient 1h data",
                "candles_1h": len(c1h),
            })
            continue

        runs = run_matrix_for_ticker(inst, c1h, c15, cdaily)
        ranked = dedupe_best(runs)
        def is_deployable(r):
            if not r.get("live_eligible"):
                return False
            if r.get("strategy") == "sma_crossover":
                return float(r.get("params", {}).get("trailing_stop_pct", 0) or 0) > 0
            return True

        deployable = [r for r in ranked if is_deployable(r)]
        research = [r for r in ranked if not r.get("live_eligible")]

        prod = next((r for r in runs if r.get("mode") == "prod_baseline"), None)
        best_deploy = deployable[0] if deployable else None
        best_research = research[0] if research else None

        family_order = [
            "sma_crossover", "level_bounce", "orb_breakout", "ema_1h", "donchian_15m",
        ]
        family_best_map = {}
        for r in ranked:
            if r.get("mode") == "prod_baseline":
                continue
            sid = r.get("strategy")
            if sid not in family_best_map or r["risk_score"] > family_best_map[sid]["risk_score"]:
                family_best_map[sid] = r
        family_best = [family_best_map[s] for s in family_order if s in family_best_map]

        out["instruments"].append({
            "ticker": inst["ticker"],
            "uid": uid,
            "instance_id": inst["id"],
            "candles_1h": len(c1h),
            "candles_15m": len(c15),
            "candles_daily": len(cdaily),
            "total_runs": len(runs),
            "unique_configs": len(ranked),
            "production": prod,
            "best_deployable": best_deploy,
            "best_research": best_research,
            "top10_deployable": deployable[:10],
            "top10_research": research[:10],
            "top10_overall": ranked[:10],
            "family_best": family_best,
        })
        print(f"  done: {len(runs)} runs, best deploy risk={best_deploy['risk_score'] if best_deploy else 'n/a'}",
              file=sys.stderr)

    text = json.dumps(out, ensure_ascii=False, indent=2)
    if args.json_out:
        Path(args.json_out).parent.mkdir(parents=True, exist_ok=True)
        Path(args.json_out).write_text(text, encoding="utf-8")
        print(f"Wrote {args.json_out}", file=sys.stderr)
    print(text)


if __name__ == "__main__":
    main()
