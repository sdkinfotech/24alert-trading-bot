#!/usr/bin/env python3
"""
Market scanner for 24alert AI-scanner.
Fetches MOEX shares, calculates intraday trading suitability metrics,
outputs JSON list of candidates ranked by composite score.

Usage:
    python scan_market.py --gateway-url http://gateway:8080 --top-n 10 --min-score 0.5
    python scan_market.py --json  # machine-readable output
"""
import argparse
import json
import sys
import urllib.request
from datetime import datetime, timedelta, timezone


def fetch_json(url):
    with urllib.request.urlopen(url, timeout=30) as r:
        return json.load(r)


TARGET_TICKERS = [
    "SBER", "GAZP", "LKOH", "ROSN", "GMKN",
    "YNDX", "VTBR", "NVTK", "TATN", "MGNT",
    "NLMK", "ALRS", "SNGS", "PLZL", "MTSS",
    "OZON", "PHOR", "CHMF", "POLY", "MAGN",
    "AFLT", "PIKK", "RUAL", "IRAO", "MOEX",
    "TRNFP", "SBERP", "FIVE", "TCSG", "SMLT",
]


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
        return data.get("data", data) if isinstance(data, dict) else data
    except Exception:
        return []


def analyze(candles_15m, candles_daily):
    if len(candles_15m) < 20 or len(candles_daily) < 10:
        return None

    trs = []
    for i in range(1, len(candles_15m)):
        c = candles_15m[i]
        prev_close = candles_15m[i - 1]["close"]
        tr = max(
            c["high"] - c["low"],
            abs(c["high"] - prev_close),
            abs(c["low"] - prev_close),
        )
        trs.append(tr)
    atr_15m = sum(trs[-20:]) / min(20, len(trs[-20:])) if trs else 0

    last_price = candles_15m[-1]["close"]
    atr_pct = (atr_15m / last_price * 100) if last_price > 0 else 0

    vols = [c.get("volume", 0) for c in candles_15m]
    avg_vol = sum(vols) / len(vols) if vols else 0
    recent_vol = sum(vols[-10:]) / min(10, len(vols[-10:])) if vols else 0
    vol_spike = recent_vol / avg_vol if avg_vol > 0 else 0

    daily_ranges = []
    for c in candles_daily[-10:]:
        rng = (c["high"] - c["low"]) / c["close"] * 100 if c["close"] > 0 else 0
        daily_ranges.append(rng)
    avg_daily_range = sum(daily_ranges) / len(daily_ranges) if daily_ranges else 0

    daily_vols = [c.get("volume", 0) for c in candles_daily]
    avg_daily_vol = sum(daily_vols[-10:]) / min(10, len(daily_vols[-10:])) if daily_vols else 0
    recent_daily_vol = sum(daily_vols[-3:]) / min(3, len(daily_vols[-3:])) if daily_vols else 0
    daily_vol_spike = recent_daily_vol / avg_daily_vol if avg_daily_vol > 0 else 0

    highs = sorted([c["high"] for c in candles_daily[-20:]], reverse=True)
    lows = sorted([c["low"] for c in candles_daily[-20:]])
    resistance = highs[:3]
    support = lows[:3]

    score = atr_pct * daily_vol_spike * avg_daily_range

    return {
        "last_price": round(last_price, 2),
        "atr_15m": round(atr_15m, 4),
        "atr_pct": round(atr_pct, 4),
        "avg_vol_15m": round(avg_vol, 0),
        "vol_spike": round(vol_spike, 2),
        "avg_daily_range_pct": round(avg_daily_range, 3),
        "daily_vol_spike": round(daily_vol_spike, 2),
        "support": [round(s, 2) for s in support],
        "resistance": [round(r, 2) for r in resistance],
        "score": round(score, 4),
    }


def main():
    parser = argparse.ArgumentParser(description="24alert market scanner")
    parser.add_argument("--gateway-url", default="http://127.0.0.1:18080",
                        help="Gateway base URL (default: http://127.0.0.1:18080)")
    parser.add_argument("--top-n", type=int, default=10,
                        help="Number of top candidates to return (default: 10)")
    parser.add_argument("--min-score", type=float, default=0.0,
                        help="Minimum score threshold (default: 0)")
    parser.add_argument("--json", action="store_true",
                        help="Output JSON only (for machine consumption)")
    parser.add_argument("--tickers", default="",
                        help="Comma-separated ticker filter (empty = use built-in list)")
    args = parser.parse_args()

    base = args.gateway_url.rstrip("/")
    tickers = [t.strip() for t in args.tickers.split(",") if t.strip()] if args.tickers else TARGET_TICKERS

    if not args.json:
        print("Fetching shares list...", file=sys.stderr)
    data = fetch_json(f"{base}/api/v1/instruments/shares")
    shares = data if isinstance(data, list) else data.get("data", data.get("instruments", []))

    rub_shares = {}
    for s in shares:
        if s.get("currency", "").lower() != "rub":
            continue
        ticker = s.get("ticker", "")
        uid = s.get("uid", s.get("instrument_uid", ""))
        if ticker in tickers and uid:
            rub_shares[ticker] = {
                "uid": uid,
                "ticker": ticker,
                "name": s.get("name", ""),
                "lot": s.get("lot", 1),
            }

    results = []
    for ticker in tickers:
        if ticker not in rub_shares:
            continue
        info = rub_shares[ticker]
        if not args.json:
            print(f"  Analyzing {ticker}...", end=" ", flush=True, file=sys.stderr)

        c15 = get_candles(base, info["uid"], "15min", 5)
        c1d = get_candles(base, info["uid"], "1d", 30)
        metrics = analyze(c15, c1d)

        if metrics is None:
            if not args.json:
                print("insufficient data", file=sys.stderr)
            continue

        if not args.json:
            print(f"score={metrics['score']:.2f}", file=sys.stderr)

        results.append({**info, **metrics})

    results.sort(key=lambda r: r["score"], reverse=True)
    results = [r for r in results if r["score"] >= args.min_score]
    results = results[: args.top_n]

    if args.json:
        json.dump(results, sys.stdout, ensure_ascii=False, indent=2)
        print()
    else:
        print(f"\n{'#':>2} {'Ticker':<7} {'Price':>8} {'ATR%':>7} {'DRange%':>8} "
              f"{'VolSpk':>7} {'Score':>8} {'Support':>24} {'Resistance':>24}", file=sys.stderr)
        print("-" * 110, file=sys.stderr)
        for i, r in enumerate(results, 1):
            sup_str = ", ".join(f"{s:.1f}" for s in r["support"][:3])
            res_str = ", ".join(f"{s:.1f}" for s in r["resistance"][:3])
            print(f"{i:2d} {r['ticker']:<7} {r['last_price']:8.2f} {r['atr_pct']:6.3f}% "
                  f"{r['avg_daily_range_pct']:7.2f}% "
                  f"{r['daily_vol_spike']:6.2f}x {r['score']:8.2f}  "
                  f"S[{sup_str}]  R[{res_str}]", file=sys.stderr)

        json.dump(results, sys.stdout, ensure_ascii=False, indent=2)
        print()


if __name__ == "__main__":
    main()
