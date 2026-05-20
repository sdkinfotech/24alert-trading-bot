#!/usr/bin/env python3
"""Diagnose broker vs ledger vs strategy for an instance."""
import json
import sys
import urllib.request

BASE = sys.argv[2] if len(sys.argv) > 2 else "http://127.0.0.1:9020"
IID = sys.argv[1] if len(sys.argv) > 1 else "fut-brent-mini-lb"


def get(path):
    with urllib.request.urlopen(f"{BASE}{path}", timeout=15) as r:
        return json.loads(r.read().decode())


port = get(f"/instances/{IID}/portfolio")
led = get(f"/instances/{IID}/ledger")
ind = get(f"/instances/{IID}/indicator")
stops = get(f"/instances/{IID}/stop-orders")

print(f"=== {IID} ===")
print("portfolio_error:", port.get("portfolio_error"))
for p in port.get("positions") or []:
    if not p.get("in_instance"):
        continue
    uid = p["instrument_uid"]
    lq = (led.get("quantities") or {}).get(uid, 0)
    print(f"\n{ticker} ({uid[:8]}...)")
    print(f"  broker qty={p['quantity']} avg={p.get('average_price')}")
    print(f"  ledger qty={lq}")
    print(f"  mismatch={abs(p['quantity'] - lq) > 1e-6}")
    prot = [s for s in stops if s.get("instrument_uid") == uid]
    print(f"  broker stop orders: {len(prot)}")
    for s in prot:
        print(f"    {s.get('stop_order_id')} {s.get('direction')} stop={s.get('stop_price')}")

print(f"\nstrategy indicator position={ind.get('position')} (0=flat 1=long -1=short)")
print(f"trailing_stop_pct={ind.get('trailing_stop_pct')} active={ind.get('trailing_stop_active')}")

# AI trader sessions on same account
try:
    sessions = get("/ai-trader/sessions")
    for s in sessions:
        if s.get("status") == "running" or (s.get("ticker") or "").upper() == "BMM6":
            print(f"\nAI session: {s.get('id')} phase={s.get('phase')} exec={s.get('execution_mode')}")
            ls = s.get("live_state") or {}
            print(f"  live position_lots={ls.get('position_lots')} working_orders={len(ls.get('working_orders') or [])}")
except Exception as e:
    print("ai-trader sessions:", e)
