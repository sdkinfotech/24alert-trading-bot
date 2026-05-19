#!/usr/bin/env bash
set -euo pipefail
DB=$(docker exec 24alert-advisor-svc sh -c 'ls /app/data/*.db 2>/dev/null | head -1')
echo "db=$DB"
docker exec 24alert-advisor-svc sqlite3 "$DB" "SELECT session_id, status, ticker, datetime(started_at) FROM sessions ORDER BY started_at DESC LIMIT 8;"
echo "--- snapshots last 30m ---"
docker exec 24alert-advisor-svc sqlite3 "$DB" "SELECT session_id, COUNT(*) FROM micro_snapshots WHERE observed_at > datetime('now','-30 minutes') GROUP BY session_id;"
echo "--- reports 5m ---"
docker exec 24alert-advisor-svc sqlite3 "$DB" "SELECT session_id, status, datetime(period_end_ms/1000,'unixepoch') FROM analysis_reports WHERE timeframe='5m' ORDER BY period_end_ms DESC LIMIT 10;"
