#!/bin/bash
# Resolve current futures UIDs for 24alert strategy config.
# Run on VPS where the gateway is available at 127.0.0.1:18080.
# Usage: bash scripts/resolve-futures.sh
set -euo pipefail

GATEWAY="${GATEWAY_URL:-http://127.0.0.1:18080}"

curl -s "${GATEWAY}/api/v1/instruments/futures" | python3 -c '
import json, sys
resp = json.load(sys.stdin)
data = resp.get("data", resp) if isinstance(resp, dict) else resp
prefixes = {"BM": "Brent mini", "NG": "Gas", "MC": "Mechel"}
for prefix, label in prefixes.items():
    matches = [
        i for i in data
        if isinstance(i, dict) and i.get("ticker", "").startswith(prefix)
    ]
    matches.sort(key=lambda x: x.get("ticker", ""))
    print(f"\n=== {label} (prefix={prefix}) ===")
    for m in matches:
        t = m.get("ticker", "")
        uid = m.get("uid", "")
        lot = m.get("lot", "")
        name = m.get("name", "")
        cc = m.get("class_code", "")
        mpi = m.get("min_price_increment", "")
        cur = m.get("currency", "")
        print(f"  {t:12s}  uid={uid}  lot={lot}  mpi={mpi}  cur={cur}  class={cc}  name={name}")
    if not matches:
        print("  (no matches)")
'
