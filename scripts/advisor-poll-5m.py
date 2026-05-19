#!/usr/bin/env python3
import json
import os
import sys
import time
import urllib.request

ADVISOR = os.environ.get("ADVISOR_URL", "http://127.0.0.1:9030").rstrip("/")
SID = sys.argv[1] if len(sys.argv) > 1 else "ai-trader-bmm6-20260519-170007"
N = int(sys.argv[2]) if len(sys.argv) > 2 else 12

for i in range(N):
    url = f"{ADVISOR}/advisor/sessions/{SID}/analyses?tf=5m&limit=5"
    with urllib.request.urlopen(url, timeout=10) as resp:
        arr = json.loads(resp.read().decode())
    print(f"[{i+1}/{N}] 5m reports: {len(arr)}", end="")
    if arr:
        r = arr[0]
        print(f" | {r.get('status')} {r.get('period_start')} -> {r.get('period_end')}")
        print("   ", (r.get("summary_md") or "")[:120])
        break
    else:
        print()
    url2 = f"{ADVISOR}/advisor/sessions/{SID}/readiness"
    with urllib.request.urlopen(url2, timeout=10) as resp:
        rd = json.loads(resp.read().decode())
    print(f"     readiness: reports_ready={rd.get('reports_ready')} reason={rd.get('ready_reason')}")
    time.sleep(30)
