#!/usr/bin/env python3
import argparse
import json
import math
import os
import urllib.error
import urllib.request
from datetime import datetime, timedelta, timezone
from zoneinfo import ZoneInfo

MSK = ZoneInfo("Europe/Moscow")
TRADING_WINDOWS = [(10 * 60, 14 * 60), (14 * 60 + 5, 18 * 60 + 50), (19 * 60, 23 * 60 + 50)]
CORE_TICKERS = {"BMM6", "NGM6", "MCM6"}
PREFIXES = ["BM", "BR", "NG", "MC", "GZ", "RI", "MX", "SR"]
MONTH_CODES = {code: idx for idx, code in enumerate("FGHJKMNQUVXZ", start=1)}


def fetch_json(url, timeout=45):
    with urllib.request.urlopen(url, timeout=timeout) as resp:
        return json.load(resp)


def unwrap(data):
    if isinstance(data, dict):
        return data.get("data", data.get("instruments", data))
    return data


def request_json(base, path):
    try:
        return fetch_json(base.rstrip("/") + path)
    except Exception as exc:
        return {"error": repr(exc)}


def fetch_last_prices(base, uids):
    prices = {}
    if not uids:
        return prices
    chunk_size = 80
    for idx in range(0, len(uids), chunk_size):
        chunk = uids[idx:idx + chunk_size]
        try:
            data = unwrap(fetch_json(f"{base.rstrip()}/api/v1/prices/bulk?uids={','.join(chunk)}"))
            for row in data:
                uid = row.get("instrument_uid") or row.get("uid")
                price = row.get("price", 0)
                if uid and price:
                    prices[uid] = float(price)
        except Exception:
            continue
    return prices


def contract_rank(ticker):
    if len(ticker) < 2:
        return (99, 99, ticker)
    month = MONTH_CODES.get(ticker[-2], 99)
    year = int(ticker[-1]) if ticker[-1].isdigit() else 99
    return (year, month, ticker)


def is_main_session(ts):
    local = ts.astimezone(MSK)
    if local.weekday() >= 5:
        return False
    minutes = local.hour * 60 + local.minute
    return any(start <= minutes < end for start, end in TRADING_WINDOWS)


def candle_url(base, uid, interval, days):
    to = datetime.now(timezone.utc)
    fr = to - timedelta(days=days)
    return (
        f"{base.rstrip('/')}/api/v1/candles?instrument_uid={uid}"
        f"&from={fr.strftime('%Y-%m-%dT%H:%M:%SZ')}"
        f"&to={to.strftime('%Y-%m-%dT%H:%M:%SZ')}"
        f"&interval={interval}"
    )


def get_candles(base, uid, interval, days):
    attempts = [days]
    if interval == "15min":
        attempts += [21, 14, 10, 7, 5]
    elif interval == "1h":
        attempts += [120, 90, 60, 30, 14]
    else:
        attempts += [60, 45, 30, 14]
    last_error = None
    for attempt in dict.fromkeys(d for d in attempts if d > 0):
        try:
            raw = unwrap(fetch_json(candle_url(base, uid, interval, attempt), timeout=60))
            candles = []
            for row in raw:
                candles.append({
                    "time": datetime.fromisoformat(row["time"].replace("Z", "+00:00")),
                    "open": float(row["open"]),
                    "high": float(row["high"]),
                    "low": float(row["low"]),
                    "close": float(row["close"]),
                    "volume": float(row.get("volume", 0) or 0),
                })
            candles.sort(key=lambda x: x["time"])
            return candles, attempt, None
        except urllib.error.HTTPError as exc:
            last_error = f"HTTP {exc.code}"
        except Exception as exc:
            last_error = repr(exc)
    return [], None, last_error


def pair_stats(events):
    entry = None
    pnls = []
    equity = 0.0
    peak = 0.0
    max_dd = 0.0
    for event in events:
        if entry is None:
            entry = event
            continue
        if event["dir"] == entry["dir"]:
            entry = event
            continue
        profit = event["price"] - entry["price"] if entry["dir"] == "buy" else entry["price"] - event["price"]
        pnls.append(profit)
        equity += profit
        peak = max(peak, equity)
        max_dd = max(max_dd, peak - equity)
        entry = None
    wins = len([p for p in pnls if p > 0])
    losses = len(pnls) - wins
    gross_profit = sum(p for p in pnls if p > 0)
    gross_loss = abs(sum(p for p in pnls if p <= 0))
    if len(pnls) > 1:
        mean = sum(pnls) / len(pnls)
        variance = sum((p - mean) ** 2 for p in pnls) / (len(pnls) - 1)
        std = math.sqrt(variance)
        sharpe = mean / std * math.sqrt(252) if std > 0 else 0.0
    else:
        sharpe = 0.0
    return {
        "pnl": round(sum(pnls), 4),
        "trades": len(pnls),
        "signals": len(events),
        "wins": wins,
        "losses": losses,
        "win_rate": round(wins / len(pnls), 4) if pnls else 0,
        "max_drawdown": round(max_dd, 4),
        "sharpe": round(sharpe, 4),
        "profit_factor": round(gross_profit / gross_loss, 4) if gross_loss > 0 else (999 if gross_profit > 0 else 0),
    }


def sma(values, n):
    return sum(values[-n:]) / n if len(values) >= n else None


def ema(prev, value, n):
    alpha = 2 / (n + 1)
    return value if prev is None else prev + alpha * (value - prev)


def average_true_range(candles, n=14):
    trs = []
    for idx in range(1, len(candles)):
        c = candles[idx]
        prev_close = candles[idx - 1]["close"]
        trs.append(max(c["high"] - c["low"], abs(c["high"] - prev_close), abs(c["low"] - prev_close)))
    return sum(trs[-n:]) / min(n, len(trs)) if trs else 0


def daily_levels(daily, n=10):
    if len(daily) < n:
        return [], []
    last = daily[-n:]
    return sorted(c["low"] for c in last)[:3], sorted((c["high"] for c in last), reverse=True)[:3]


def run_sma(candles, fast, slow):
    closes, pos, trades = [], 0, []
    for c in candles:
        closes.append(c["close"])
        if len(closes) < slow + 1 or not is_main_session(c["time"]):
            continue
        f = sma(closes, fast)
        s = sma(closes, slow)
        pf = sma(closes[:-1], fast)
        ps = sma(closes[:-1], slow)
        if pf is None or ps is None:
            continue
        if pf <= ps and f > s and pos <= 0:
            trades.append({"time": c["time"].isoformat(), "dir": "buy", "price": c["close"], "reason": "sma_cross"})
            pos = 1
        elif pf >= ps and f < s and pos >= 0:
            trades.append({"time": c["time"].isoformat(), "dir": "sell", "price": c["close"], "reason": "sma_cross"})
            pos = -1
    return trades


def run_ema(candles, fast, slow):
    ef = es = pf = ps = None
    pos, trades = 0, []
    for c in candles:
        pf, ps = ef, es
        ef = ema(ef, c["close"], fast)
        es = ema(es, c["close"], slow)
        if pf is None or ps is None or not is_main_session(c["time"]):
            continue
        if pf <= ps and ef > es and pos <= 0:
            trades.append({"time": c["time"].isoformat(), "dir": "buy", "price": c["close"], "reason": "ema_cross"})
            pos = 1
        elif pf >= ps and ef < es and pos >= 0:
            trades.append({"time": c["time"].isoformat(), "dir": "sell", "price": c["close"], "reason": "ema_cross"})
            pos = -1
    return trades


def run_donchian(candles, lookback, atr_stop=1.0):
    pos, entry, stop, trades = 0, 0.0, 0.0, []
    for idx, c in enumerate(candles):
        if idx < lookback + 1 or not is_main_session(c["time"]):
            continue
        prev = candles[idx - lookback:idx]
        high = max(x["high"] for x in prev)
        low = min(x["low"] for x in prev)
        atr = average_true_range(candles[max(0, idx - 30):idx + 1], 14)
        if pos > 0 and c["low"] <= stop:
            trades.append({"time": c["time"].isoformat(), "dir": "sell", "price": stop, "reason": "atr_stop"})
            pos = 0
        elif pos < 0 and c["high"] >= stop:
            trades.append({"time": c["time"].isoformat(), "dir": "buy", "price": stop, "reason": "atr_stop"})
            pos = 0
        if pos == 0 and c["close"] > high:
            entry = c["close"]
            stop = entry - atr * atr_stop
            trades.append({"time": c["time"].isoformat(), "dir": "buy", "price": entry, "reason": "donchian_breakout"})
            pos = 1
        elif pos == 0 and c["close"] < low:
            entry = c["close"]
            stop = entry + atr * atr_stop
            trades.append({"time": c["time"].isoformat(), "dir": "sell", "price": entry, "reason": "donchian_breakout"})
            pos = -1
    return trades


def run_level_bounce(candles, daily, sl_mult, tp_mult, cutoff_hour=23, cutoff_min=30, level_days=10):
    support, resistance = daily_levels(daily, level_days)
    atr = average_true_range(daily, 14)
    pos, stop, target, trades = 0, 0.0, 0.0, []
    current_day, eod_sent, last_entry = "", False, None
    if not support or not resistance or atr <= 0:
        return trades
    for c in candles:
        local = c["time"].astimezone(MSK)
        day = local.strftime("%Y-%m-%d")
        if day != current_day:
            current_day, eod_sent = day, False
        cutoff = local.replace(hour=cutoff_hour, minute=cutoff_min, second=0, microsecond=0)
        if local >= cutoff:
            if pos != 0 and not eod_sent and is_main_session(c["time"]):
                trades.append({"time": c["time"].isoformat(), "dir": "sell" if pos > 0 else "buy", "price": c["close"], "reason": "eod"})
                pos, eod_sent = 0, True
            continue
        if not is_main_session(c["time"]):
            continue
        if pos > 0:
            if c["low"] <= stop:
                trades.append({"time": c["time"].isoformat(), "dir": "sell", "price": stop, "reason": "stop"})
                pos = 0
                continue
            if c["high"] >= target:
                trades.append({"time": c["time"].isoformat(), "dir": "sell", "price": target, "reason": "tp"})
                pos = 0
                continue
        if pos < 0:
            if c["high"] >= stop:
                trades.append({"time": c["time"].isoformat(), "dir": "buy", "price": stop, "reason": "stop"})
                pos = 0
                continue
            if c["low"] <= target:
                trades.append({"time": c["time"].isoformat(), "dir": "buy", "price": target, "reason": "tp"})
                pos = 0
                continue
        if pos != 0:
            continue
        for level in support:
            key = (day, "buy", level)
            if c["low"] <= level and c["close"] > level and key != last_entry:
                stop = level - atr * sl_mult
                target = c["close"] + atr * tp_mult
                trades.append({"time": c["time"].isoformat(), "dir": "buy", "price": c["close"], "reason": f"bounce_S={level:.1f}"})
                pos, last_entry = 1, key
                break
        if pos:
            continue
        for level in resistance:
            key = (day, "sell", level)
            if c["high"] >= level and c["close"] < level and key != last_entry:
                stop = level + atr * sl_mult
                target = c["close"] - atr * tp_mult
                trades.append({"time": c["time"].isoformat(), "dir": "sell", "price": c["close"], "reason": f"reject_R={level:.1f}"})
                pos, last_entry = -1, key
                break
    return trades


def run_orb(candles, range_minutes=60, rr=1.5):
    by_day = {}
    for c in candles:
        local = c["time"].astimezone(MSK)
        by_day.setdefault(local.strftime("%Y-%m-%d"), []).append(c)
    trades = []
    for day, rows in by_day.items():
        opening = []
        for c in rows:
            local = c["time"].astimezone(MSK)
            minutes = local.hour * 60 + local.minute
            if 10 * 60 <= minutes < 10 * 60 + range_minutes:
                opening.append(c)
        if len(opening) < 2:
            continue
        high = max(c["high"] for c in opening)
        low = min(c["low"] for c in opening)
        rng = high - low
        if rng <= 0:
            continue
        pos, stop, target = 0, 0.0, 0.0
        for c in rows:
            local = c["time"].astimezone(MSK)
            if local.hour * 60 + local.minute < 10 * 60 + range_minutes or not is_main_session(c["time"]):
                continue
            if pos > 0:
                if c["low"] <= stop:
                    trades.append({"time": c["time"].isoformat(), "dir": "sell", "price": stop, "reason": "orb_stop"})
                    break
                if c["high"] >= target:
                    trades.append({"time": c["time"].isoformat(), "dir": "sell", "price": target, "reason": "orb_target"})
                    break
            elif pos < 0:
                if c["high"] >= stop:
                    trades.append({"time": c["time"].isoformat(), "dir": "buy", "price": stop, "reason": "orb_stop"})
                    break
                if c["low"] <= target:
                    trades.append({"time": c["time"].isoformat(), "dir": "buy", "price": target, "reason": "orb_target"})
                    break
            elif c["close"] > high:
                stop, target = low, c["close"] + rng * rr
                trades.append({"time": c["time"].isoformat(), "dir": "buy", "price": c["close"], "reason": "orb_breakout"})
                pos = 1
            elif c["close"] < low:
                stop, target = high, c["close"] - rng * rr
                trades.append({"time": c["time"].isoformat(), "dir": "sell", "price": c["close"], "reason": "orb_breakout"})
                pos = -1
    return trades


def enrich_futures(futures, prices):
    rows = []
    for item in futures:
        ticker = (item.get("ticker") or "").upper()
        uid = item.get("uid") or item.get("instrument_uid")
        if item.get("class_code") != "SPBFUT" or not ticker or not uid:
            continue
        price = float(prices.get(uid, 0) or 0)
        lot = float(item.get("lot", 1) or 1)
        rows.append({
            **item,
            "ticker": ticker,
            "uid": uid,
            "last_price": round(price, 6),
            "contract_price": round(price * lot, 6) if price else 0,
            "is_core": ticker in CORE_TICKERS,
            "rank": contract_rank(ticker),
        })
    return rows


def choose_candidates(futures, prices, universe="core", max_instruments=80, min_contract_price=0.0, max_contract_price=0.0):
    enriched = enrich_futures(futures, prices)
    if universe == "core":
        selected = {}
        for f in enriched:
            ticker = f["ticker"]
            if ticker in CORE_TICKERS or any(ticker.startswith(prefix) for prefix in PREFIXES):
                prefix = next((p for p in PREFIXES if ticker.startswith(p)), ticker)
                selected.setdefault(prefix, []).append(f)
        out = []
        for _, rows in selected.items():
            rows = sorted(rows, key=lambda f: f["rank"])
            out.extend(rows[:2])
        by_ticker = {f["ticker"]: f for f in out}
        for f in enriched:
            if f["ticker"] in CORE_TICKERS:
                by_ticker[f["ticker"]] = f
        return sorted(by_ticker.values(), key=lambda f: (not f["is_core"], f["ticker"])), enriched

    filtered = []
    for f in enriched:
        cp = f.get("contract_price", 0)
        if min_contract_price and cp and cp < min_contract_price:
            continue
        if max_contract_price and cp and cp > max_contract_price:
            continue
        filtered.append(f)

    # Keep production contracts, then test a broad expensive-contract slice.
    filtered.sort(key=lambda f: (f["is_core"], f.get("contract_price", 0), -f["rank"][0], -f["rank"][1]), reverse=True)
    selected = []
    seen = set()
    for f in filtered:
        if f["ticker"] in seen:
            continue
        selected.append(f)
        seen.add(f["ticker"])
        if len(selected) >= max_instruments:
            break
    for f in enriched:
        if f["is_core"] and f["ticker"] not in seen:
            selected.append(f)
            seen.add(f["ticker"])
    selected.sort(key=lambda f: (not f["is_core"], -f.get("contract_price", 0), f["ticker"]))
    return selected, enriched


def legacy_choose_candidates(futures):
    selected = {}
    for f in futures:
        ticker = (f.get("ticker") or "").upper()
        if f.get("class_code") != "SPBFUT" or not ticker:
            continue
        if ticker in CORE_TICKERS or any(ticker.startswith(prefix) for prefix in PREFIXES):
            prefix = next((p for p in PREFIXES if ticker.startswith(p)), ticker)
            selected.setdefault(prefix, []).append(f)
    out = []
    for prefix, rows in selected.items():
        rows = sorted(rows, key=lambda f: contract_rank((f.get("ticker") or "").upper()))
        out.extend(rows[:2])
    by_ticker = {(f.get("ticker") or "").upper(): f for f in out}
    for f in futures:
        ticker = (f.get("ticker") or "").upper()
        if ticker in CORE_TICKERS:
            by_ticker[ticker] = f
    return sorted(by_ticker.values(), key=lambda f: (0 if (f.get("ticker") or "").upper() in CORE_TICKERS else 1, (f.get("ticker") or "")))


def evaluate(ticker, uid, c15, c1h, daily):
    runs = []
    for fast, slow in [(5, 17), (9, 26), (12, 27), (4, 9), (7, 17)]:
        runs.append({"ticker": ticker, "strategy": "sma_1h", "params": {"fast": fast, "slow": slow}, **pair_stats(run_sma(c1h, fast, slow))})
        runs.append({"ticker": ticker, "strategy": "ema_1h", "params": {"fast": fast, "slow": slow}, **pair_stats(run_ema(c1h, fast, slow))})
    for lookback in [8, 12, 16, 20]:
        runs.append({"ticker": ticker, "strategy": "donchian_15m", "params": {"lookback": lookback, "atr_stop": 1.0}, **pair_stats(run_donchian(c15, lookback, 1.0))})
    for sl in [0.3, 0.5, 0.7, 1.0]:
        for tp in [1.0, 1.5, 2.0]:
            runs.append({"ticker": ticker, "strategy": "level_bounce_15m", "params": {"sl_mult": sl, "tp_mult": tp, "cutoff_hour": 23, "cutoff_min": 30}, **pair_stats(run_level_bounce(c15, daily, sl, tp))})
    for minutes in [30, 60]:
        for rr in [1.0, 1.5, 2.0]:
            runs.append({"ticker": ticker, "strategy": "orb_15m", "params": {"range_minutes": minutes, "rr": rr}, **pair_stats(run_orb(c15, minutes, rr))})
    for run in runs:
        run["uid"] = uid
        if run["strategy"] in ("sma_1h", "ema_1h"):
            run["family"] = "trend_following"
        elif run["strategy"] in ("level_bounce_15m",):
            run["family"] = "level_reversal"
        else:
            run["family"] = "breakout_momentum"
        run["score"] = round(run["pnl"] * max(run["profit_factor"], 0) + run["sharpe"], 4)
        if run["trades"] < 5:
            run["quality"] = "LOW_SAMPLE"
        elif run["pnl"] > 0 and run["sharpe"] > 1 and run["profit_factor"] > 1.3:
            run["quality"] = "PASS"
        elif run["pnl"] > 0:
            run["quality"] = "WATCH"
        else:
            run["quality"] = "REJECT"
    return runs


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--gateway-url", default="http://127.0.0.1:18080")
    parser.add_argument("--strategy-url", default="http://127.0.0.1:9020")
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--universe", choices=["core", "all"], default="core")
    parser.add_argument("--max-instruments", type=int, default=80)
    parser.add_argument("--min-contract-price", type=float, default=0.0)
    parser.add_argument("--max-contract-price", type=float, default=0.0)
    args = parser.parse_args()
    os.makedirs(args.output_dir, exist_ok=True)
    os.makedirs(os.path.join(args.output_dir, "data"), exist_ok=True)
    os.makedirs(os.path.join(args.output_dir, "reports"), exist_ok=True)

    futures = unwrap(fetch_json(args.gateway_url.rstrip("/") + "/api/v1/instruments/futures"))
    futures_base = []
    for item in futures:
        uid = item.get("uid") or item.get("instrument_uid")
        if item.get("class_code") == "SPBFUT" and uid:
            futures_base.append(item)
    prices = fetch_last_prices(args.gateway_url, [item.get("uid") or item.get("instrument_uid") for item in futures_base])
    candidates, all_futures = choose_candidates(
        futures,
        prices,
        universe=args.universe,
        max_instruments=args.max_instruments,
        min_contract_price=args.min_contract_price,
        max_contract_price=args.max_contract_price,
    )
    baseline = {"instances": request_json(args.strategy_url, "/instances")}
    if isinstance(baseline["instances"], list):
        for inst in baseline["instances"]:
            iid = inst.get("id")
            if not iid:
                continue
            baseline[iid] = {
                "pnl": request_json(args.strategy_url, f"/instances/{iid}/pnl"),
                "events": request_json(args.strategy_url, f"/instances/{iid}/events?limit=100"),
                "indicator": request_json(args.strategy_url, f"/instances/{iid}/indicator"),
            }

    instrument_rows = []
    all_results = []
    candle_meta = {}
    for item in candidates:
        ticker = (item.get("ticker") or "").upper()
        uid = item.get("uid") or item.get("instrument_uid")
        if not ticker or not uid:
            continue
        c15, used15, err15 = get_candles(args.gateway_url, uid, "15min", 21)
        c1h, used1h, err1h = get_candles(args.gateway_url, uid, "1h", 120)
        daily, usedd, errd = get_candles(args.gateway_url, uid, "1d", 60)
        candle_meta[ticker] = {
            "uid": uid,
            "candles_15m": len(c15),
            "candles_1h": len(c1h),
            "candles_daily": len(daily),
            "used_days_15m": used15,
            "used_days_1h": used1h,
            "used_days_daily": usedd,
            "errors": {"15m": err15, "1h": err1h, "1d": errd},
        }
        instrument_rows.append({
            "ticker": ticker,
            "uid": uid,
            "name": item.get("name", ""),
            "class_code": item.get("class_code", ""),
            "lot": item.get("lot", 1),
            "last_price": item.get("last_price", 0),
            "contract_price": item.get("contract_price", 0),
            "is_core": item.get("is_core", False),
            "avg_volume_15m": round(sum(c["volume"] for c in c15) / len(c15), 2) if c15 else 0,
            "avg_volume_1h": round(sum(c["volume"] for c in c1h) / len(c1h), 2) if c1h else 0,
            **candle_meta[ticker],
        })
        if len(c15) >= 50 and len(c1h) >= 50 and len(daily) >= 10:
            evaluated = evaluate(ticker, uid, c15, c1h, daily)
            for row in evaluated:
                row["last_price"] = item.get("last_price", 0)
                row["contract_price"] = item.get("contract_price", 0)
                row["avg_volume_15m"] = instrument_rows[-1]["avg_volume_15m"]
                row["avg_volume_1h"] = instrument_rows[-1]["avg_volume_1h"]
                row["name"] = item.get("name", "")
                row["is_core"] = item.get("is_core", False)
            all_results.extend(evaluated)

    ranked = sorted(all_results, key=lambda r: (r["quality"] == "PASS", r["score"], r["pnl"], r["trades"]), reverse=True)
    outputs = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "universe": args.universe,
        "baseline": baseline,
        "instruments": instrument_rows,
        "all_futures_count": len(all_futures),
        "candle_meta": candle_meta,
        "results": ranked,
        "top20": ranked[:20],
    }
    with open(os.path.join(args.output_dir, "data", "prod-baseline.json"), "w", encoding="utf-8") as fh:
        json.dump(baseline, fh, ensure_ascii=False, indent=2, default=str)
    with open(os.path.join(args.output_dir, "data", "instruments.json"), "w", encoding="utf-8") as fh:
        json.dump(instrument_rows, fh, ensure_ascii=False, indent=2, default=str)
    with open(os.path.join(args.output_dir, "data", "all-futures-universe.json"), "w", encoding="utf-8") as fh:
        json.dump(all_futures, fh, ensure_ascii=False, indent=2, default=str)
    with open(os.path.join(args.output_dir, "reports", "backtest-results.json"), "w", encoding="utf-8") as fh:
        json.dump(outputs, fh, ensure_ascii=False, indent=2, default=str)
    print(json.dumps({
        "all_futures": len(all_futures),
        "instruments": len(instrument_rows),
        "results": len(ranked),
        "top20": ranked[:20],
    }, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
