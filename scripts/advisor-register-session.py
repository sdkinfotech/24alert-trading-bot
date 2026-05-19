#!/usr/bin/env python3
"""Register AI trader session in advisor-svc (prod debug)."""
import json
import os
import sys
import urllib.request

ADVISOR = os.environ.get("ADVISOR_URL", "http://127.0.0.1:9030").rstrip("/")
SID = sys.argv[1] if len(sys.argv) > 1 else "ai-trader-bmm6-20260519-170007"

body = {
    "session_id": SID,
    "account_id": "2001673385",
    "instrument_uid": "dc1ffa30-70a4-4a7b-807a-4f31c2951f7e",
    "ticker": "BMM6",
    "mode": "level_intraday",
    "started_at": "2026-05-19T17:00:07Z",
}
data = json.dumps(body).encode()
req = urllib.request.Request(
    f"{ADVISOR}/advisor/sessions/register",
    data=data,
    headers={"Content-Type": "application/json"},
    method="POST",
)
with urllib.request.urlopen(req, timeout=10) as resp:
    print("register:", resp.status, resp.read().decode())

req2 = urllib.request.Request(f"{ADVISOR}/advisor/sessions/{SID}/readiness")
with urllib.request.urlopen(req2, timeout=10) as resp:
    print("readiness:", resp.read().decode())

req3 = urllib.request.Request(f"{ADVISOR}/advisor/sessions/{SID}/analyses?tf=5m&limit=5")
with urllib.request.urlopen(req3, timeout=10) as resp:
    print("5m reports:", resp.read().decode())
