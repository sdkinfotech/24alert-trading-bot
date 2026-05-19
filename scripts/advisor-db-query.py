#!/usr/bin/env python3
import sqlite3
import sys

db = sys.argv[1] if len(sys.argv) > 1 else "/tmp/advisor_memory.db"
sid = sys.argv[2] if len(sys.argv) > 2 else "ai-trader-bmm6-20260519-170007"
con = sqlite3.connect(db)
cur = con.cursor()
print("=== sessions ===")
for row in cur.execute("SELECT session_id, status, ticker FROM advisor_sessions ORDER BY started_at_ms DESC LIMIT 8"):
    print(row)
print("=== snapshots for", sid, "===", cur.execute("SELECT COUNT(*) FROM micro_snapshots WHERE session_id=?", (sid,)).fetchone()[0])
for row in cur.execute(
    "SELECT datetime(captured_at_ms/1000,'unixepoch'), length(payload_json) FROM micro_snapshots WHERE session_id=? ORDER BY captured_at_ms DESC LIMIT 5",
    (sid,),
):
    print(" ", row)
print("=== reports ===")
for row in cur.execute(
    "SELECT timeframe, status, datetime(period_end_ms/1000,'unixepoch') FROM analysis_reports WHERE session_id=? ORDER BY period_end_ms DESC LIMIT 10",
    (sid,),
):
    print(row)
print("=== scheduler_state ===")
for row in cur.execute("SELECT timeframe, datetime(last_period_end_ms/1000,'unixepoch') FROM scheduler_state WHERE session_id=?", (sid,)):
    print(row)
