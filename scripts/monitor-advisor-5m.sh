#!/usr/bin/env bash
# Monitor advisor 5m rollup: ingest snapshots + 5m/15m reports for an AI Trader session.
# Usage: monitor-advisor-5m.sh [session_id]
# Env: RUNNER_URL (default http://127.0.0.1:9020), ADVISOR_URL (default http://127.0.0.1:9030)
set -euo pipefail

RUNNER="${RUNNER_URL:-http://127.0.0.1:9020}"
ADVISOR="${ADVISOR_URL:-http://127.0.0.1:9030}"

SID="${1:-}"
if [ -z "$SID" ]; then
  SID=$(curl -sf "$RUNNER/ai-trader/sessions" | python3 -c "
import sys, json
arr = json.load(sys.stdin)
running = [x for x in arr if x.get('status') != 'stopped']
if not running:
    running = arr
if not running:
    sys.exit(1)
print(running[0]['id'])
")
fi

echo "=== Session: $SID ==="
curl -sf "$RUNNER/ai-trader/sessions/$SID" | python3 -c "
import sys, json
d = json.load(sys.stdin)
pp = d.get('phase_progress') or {}
print('status:', d.get('status'), 'phase:', d.get('phase'))
print('trading_ready:', pp.get('trading_ready'), 'reports_ready:', pp.get('reports_ready'))
bs = pp.get('buffer_stats') or {}
print('chart_bars(1m):', bs.get('chart_bars'), 'book:', bs.get('book_samples'), 'prints:', bs.get('print_samples'))
"

echo
echo "=== Advisor readiness ==="
curl -sf "$ADVISOR/advisor/sessions/$SID/readiness" | python3 -m json.tool 2>/dev/null || curl -sf "$ADVISOR/advisor/sessions/$SID/readiness"

echo
echo "=== Advisor reports 5m ==="
curl -sf "$ADVISOR/advisor/sessions/$SID/analyses?tf=5m&limit=5" | python3 -c "
import sys, json
arr = json.load(sys.stdin)
print('count:', len(arr))
for r in arr[:5]:
    print(' -', r.get('status'), r.get('period_start'), '->', r.get('period_end'), '|', (r.get('summary_md') or '')[:80])
"

echo
echo "=== Advisor reports 15m ==="
curl -sf "$ADVISOR/advisor/sessions/$SID/analyses?tf=15m&limit=3" | python3 -c "
import sys, json
arr = json.load(sys.stdin)
print('count:', len(arr))
for r in arr[:3]:
    print(' -', r.get('status'), r.get('period_start'), '->', r.get('period_end'))
"

echo
echo "=== Advisor metrics (ingest / reports) ==="
curl -sf "$ADVISOR/metrics" 2>/dev/null | grep -E 'advisor_(ingest|reports)' || echo "(no /metrics on advisor)"

echo
echo "=== Advisor DB (host copy) ==="
if sudo docker cp 24alert-advisor-svc:/app/data/advisor_memory.db /tmp/advisor_memory.db 2>/dev/null; then
  python3 "$(dirname "$0")/advisor-db-query.py" /tmp/advisor_memory.db "$SID" 2>/dev/null || \
    python3 /tmp/advisor-db-query.py /tmp/advisor_memory.db "$SID"
else
  echo "could not copy advisor_memory.db"
fi

echo "=== advisor -> runner connectivity ==="
sudo docker exec 24alert-advisor-svc curl -s -o /dev/null -w "runner_http=%{http_code}\n" http://strategy-runner:9020/health 2>/dev/null || \
  echo "curl from advisor container failed"
