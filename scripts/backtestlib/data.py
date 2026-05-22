"""Gateway candle fetch."""
import sys
import urllib.request
from datetime import datetime, timedelta, timezone


def fetch_json(url, timeout=60):
    import json
    with urllib.request.urlopen(url, timeout=timeout) as r:
        return json.load(r)


def get_candles(base: str, uid: str, interval: str, days: int) -> list:
    to = datetime.now(timezone.utc)
    fr = to - timedelta(days=days + 5)
    url = (
        f"{base.rstrip('/')}/api/v1/candles?instrument_uid={uid}"
        f"&from={fr.strftime('%Y-%m-%dT%H:%M:%SZ')}"
        f"&to={to.strftime('%Y-%m-%dT%H:%M:%SZ')}"
        f"&interval={interval}"
    )
    try:
        import json
        with urllib.request.urlopen(url, timeout=60) as r:
            data = json.load(r)
        raw = data.get("data", data) if isinstance(data, dict) else data
    except Exception as e:
        print(f"ERROR fetching candles: {e}", file=sys.stderr)
        return []
    candles = []
    for row in raw:
        candles.append({
            "open": float(row["open"]),
            "high": float(row["high"]),
            "low": float(row["low"]),
            "close": float(row["close"]),
            "volume": float(row.get("volume", 0) or 0),
            "time": datetime.fromisoformat(row["time"].replace("Z", "+00:00")),
        })
    candles.sort(key=lambda x: x["time"])
    return candles


def slice_candles_by_days(candles: list, days: int) -> list:
    if not candles or days <= 0:
        return candles
    cutoff = datetime.now(timezone.utc) - timedelta(days=days)
    return [c for c in candles if c["time"] >= cutoff]
