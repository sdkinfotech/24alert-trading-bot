#!/usr/bin/env bash
# Smoke: armed_live session API + kill-switch (run on srv03).
set -euo pipefail
API="${1:-http://127.0.0.1:9020}"
BODY='{"ticker":"BMM6","instrument_uid":"dc1ffa30-70a4-4a7b-807a-4f31c2951f7e","account_id":"2001673385","strategy_kind":"level_intraday","mode":"armed_live","confirm_live":true}'

echo "=== POST session armed_live ==="
SESSION=$(curl -sf -X POST "$API/ai-trader/sessions" -H 'Content-Type: application/json' -d "$BODY")
echo "$SESSION" | head -c 500
echo
SID=$(echo "$SESSION" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('instance_id') or d['id'])")
echo "session_id=$SID"

echo "=== GET config ==="
curl -sf "$API/ai-trader/config"
echo

echo "=== poll session (6x10s) ==="
for i in 1 2 3 4 5 6; do
  ST=$(curl -sf "$API/ai-trader/sessions/$SID")
  echo "$ST" | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'poll {sys.argv[1]}: phase={d.get(\"phase\")} trading_ready={d.get(\"trading_ready\")} exec={d.get(\"execution_mode\")}')" "$i"
  if echo "$ST" | python3 -c "import sys,json; d=json.load(sys.stdin); raise SystemExit(0 if d.get('trading_ready') else 1)"; then
    break
  fi
  sleep 10
done

echo "=== start-trading ==="
curl -s -w "\nHTTP:%{http_code}\n" -X POST "$API/ai-trader/sessions/$SID/start-trading" || true
echo

echo "=== kill-switch ON ==="
curl -sf -X POST "$API/ai-trader/kill-switch" -H 'Content-Type: application/json' -d '{"active":true}'
echo
curl -sf http://127.0.0.1:9120/metrics | grep alert24_ai_trader_kill_switch || true

echo "=== kill-switch OFF ==="
curl -sf -X POST "$API/ai-trader/kill-switch" -H 'Content-Type: application/json' -d '{"active":false}'
echo

echo "=== stop session ==="
curl -sf -X POST "$API/ai-trader/sessions/$SID/stop" | head -c 300
echo
echo "=== OK ==="
