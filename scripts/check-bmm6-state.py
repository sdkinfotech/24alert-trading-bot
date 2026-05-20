#!/usr/bin/env python3
import json
import urllib.request

BASE = "http://127.0.0.1:9020"

def get(path):
    with urllib.request.urlopen(BASE + path, timeout=15) as r:
        return json.loads(r.read().decode())

instances = get("/instances")
print("=== instances ===")
for i in instances:
    print(f"  {i['id']}: running={i['running']} ticker={i.get('tickers')}")

port = get("/instances/fut-brent-mini-lb/portfolio")
print("\n=== fut-brent-mini-lb broker ===")
for p in port.get("positions") or []:
    if p.get("in_instance"):
        print(f"  {p.get('ticker')}: qty={p['quantity']} yield={p.get('expected_yield')}")

try:
    led = get("/instances/fut-brent-mini-lb/ledger")
    print("ledger:", led.get("quantities"))
except Exception as e:
    print("ledger:", e)

stops = get("/instances/fut-brent-mini-lb/stop-orders")
print("broker stop orders:", len(stops))

sessions = get("/ai-trader/sessions")
print("\n=== AI trader sessions ===")
for s in sessions:
    ls = s.get("live_state") or {}
    print(f"  {s['id']}: status={s.get('status')} phase={s.get('phase')} exec={s.get('execution_mode')}")
    print(f"    live position_lots={ls.get('position_lots')} sl={ls.get('stop_loss')} tp={ls.get('take_profit')}")
