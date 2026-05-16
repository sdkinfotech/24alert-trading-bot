#!/usr/bin/env python3
"""
Market scanner for 24alert AI-scanner.
Fetches MOEX futures, calculates intraday trading suitability metrics,
filters candidates by contract price, and outputs a JSON list ranked by
composite score.

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


DEFAULT_FUTURE_PREFIXES = ["BM", "NG", "MC"]
MONTH_CODES = {code: i for i, code in enumerate("FGHJKMNQUVXZ", start=1)}


def unwrap_data(data):
    if isinstance(data, dict):
        return data.get("data", data.get("instruments", []))
    return data


def fetch_last_prices(base, uids):
    if not uids:
        return {}
    url = f"{base}/api/v1/prices/bulk?uids={','.join(uids)}"
    try:
        data = unwrap_data(fetch_json(url))
    except Exception:
        return {}

    out = {}
    for p in data:
        uid = p.get("instrument_uid", "")
        price = p.get("price", 0)
        if uid and price:
            out[uid] = price
    return out


def contract_rank(ticker):
    """Sort futures by year/month code so nearest contracts come first."""
    if len(ticker) < 2:
        return (99, 99, ticker)
    month_code = ticker[-2]
    year_code = ticker[-1]
    if month_code not in MONTH_CODES or not year_code.isdigit():
        return (99, 99, ticker)
    return (int(year_code), MONTH_CODES[month_code], ticker)


def months_ahead(ticker, now):
    if len(ticker) < 2:
        return 999
    month_code = ticker[-2]
    year_code = ticker[-1]
    if month_code not in MONTH_CODES or not year_code.isdigit():
        return 999
    return (int(year_code) - (now.year % 10)) * 12 + (MONTH_CODES[month_code] - now.month)


def select_nearest_by_prefix(futures, prefixes, min_months_ahead):
    now = datetime.now(timezone.utc)
    selected = {}
    for prefix in prefixes:
        matches = [
            f for f in futures
            if f.get("ticker", "").startswith(prefix)
            and months_ahead(f.get("ticker", ""), now) >= min_months_ahead
        ]
        if not matches:
            continue
        selected[prefix] = sorted(matches, key=lambda f: contract_rank(f.get("ticker", "")))[0]
    return list(selected.values())


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
                        help="Comma-separated exact futures ticker filter (e.g. BMM6,NGM6,MCM6)")
    parser.add_argument("--ticker-prefixes", default=",".join(DEFAULT_FUTURE_PREFIXES),
                        help="Comma-separated futures prefixes if --tickers is empty (default: BM,NG,MC)")
    parser.add_argument("--min-contract-price", type=float, default=0.0,
                        help="Minimum contract price in instrument currency (default: 0)")
    parser.add_argument("--max-contract-price", type=float, default=10000.0,
                        help="Maximum contract price in instrument currency (default: 10000)")
    parser.add_argument("--all-expirations", action="store_true",
                        help="Analyze all matched expirations instead of nearest contract per prefix")
    parser.add_argument("--min-months-ahead", type=int, default=1,
                        help="For prefix selection, skip contracts closer than this many months ahead (default: 1)")
    args = parser.parse_args()

    base = args.gateway_url.rstrip("/")
    tickers = [t.strip().upper() for t in args.tickers.split(",") if t.strip()]
    prefixes = [p.strip().upper() for p in args.ticker_prefixes.split(",") if p.strip()]

    if not args.json:
        print("Fetching futures list...", file=sys.stderr)
    data = fetch_json(f"{base}/api/v1/instruments/futures")
    futures = unwrap_data(data)

    candidates = []
    for f in futures:
        if f.get("class_code") != "SPBFUT":
            continue
        ticker = f.get("ticker", "").upper()
        uid = f.get("uid", f.get("instrument_uid", ""))
        if not uid:
            continue
        if tickers and ticker not in tickers:
            continue
        if not tickers and prefixes and not any(ticker.startswith(p) for p in prefixes):
            continue
        candidates.append({
            "uid": uid,
            "ticker": ticker,
            "name": f.get("name", ""),
            "lot": f.get("lot", 1),
            "class_code": f.get("class_code", ""),
            "instrument_type": f.get("instrument_type", "future"),
            "currency": f.get("currency", ""),
            "min_price_increment": f.get("min_price_increment", 0),
        })

    if not tickers and not args.all_expirations:
        candidates = select_nearest_by_prefix(candidates, prefixes, args.min_months_ahead)

    last_prices = fetch_last_prices(base, [c["uid"] for c in candidates])
    filtered = []
    for info in candidates:
        price = last_prices.get(info["uid"], 0)
        contract_price = price * info.get("lot", 1) if price else 0
        if price and contract_price < args.min_contract_price:
            continue
        if price and args.max_contract_price > 0 and contract_price > args.max_contract_price:
            continue
        info["last_price"] = round(price, 4) if price else 0
        info["contract_price"] = round(contract_price, 4) if contract_price else 0
        filtered.append(info)

    results = []
    for info in sorted(filtered, key=lambda x: x["ticker"]):
        ticker = info["ticker"]
        if not args.json:
            cp = info.get("contract_price", 0)
            print(f"  Analyzing {ticker} contract_price={cp}...", end=" ", flush=True, file=sys.stderr)

        c15 = get_candles(base, info["uid"], "15min", 5)
        c1d = get_candles(base, info["uid"], "1d", 30)
        metrics = analyze(c15, c1d)

        if metrics is None:
            if not args.json:
                print("insufficient data", file=sys.stderr)
            continue

        if not args.json:
            print(f"score={metrics['score']:.2f}", file=sys.stderr)

        if not info["last_price"]:
            info["last_price"] = metrics["last_price"]
            info["contract_price"] = round(metrics["last_price"] * info.get("lot", 1), 4)

        results.append({**metrics, **info})

    results.sort(key=lambda r: r["score"], reverse=True)
    results = [r for r in results if r["score"] >= args.min_score]
    results = results[: args.top_n]

    if args.json:
        json.dump(results, sys.stdout, ensure_ascii=False, indent=2)
        print()
    else:
        print(f"\n{'#':>2} {'Ticker':<7} {'Contract':>10} {'Price':>8} {'ATR%':>7} {'DRange%':>8} "
              f"{'VolSpk':>7} {'Score':>8} {'Support':>24} {'Resistance':>24}", file=sys.stderr)
        print("-" * 122, file=sys.stderr)
        for i, r in enumerate(results, 1):
            sup_str = ", ".join(f"{s:.1f}" for s in r["support"][:3])
            res_str = ", ".join(f"{s:.1f}" for s in r["resistance"][:3])
            print(f"{i:2d} {r['ticker']:<7} {r['contract_price']:10.2f} {r['last_price']:8.2f} "
                  f"{r['atr_pct']:6.3f}% "
                  f"{r['avg_daily_range_pct']:7.2f}% "
                  f"{r['daily_vol_spike']:6.2f}x {r['score']:8.2f}  "
                  f"S[{sup_str}]  R[{res_str}]", file=sys.stderr)

        json.dump(results, sys.stdout, ensure_ascii=False, indent=2)
        print()


if __name__ == "__main__":
    main()
