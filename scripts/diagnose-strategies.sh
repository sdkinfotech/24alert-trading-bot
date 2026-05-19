#!/usr/bin/env sh
# Run on the VPS (or any host with Docker + curl) to diagnose ai-scanner + strategy-runner.
# Usage: ./scripts/diagnose-strategies.sh
# Optional: STRATEGY_RUNNER_URL=http://127.0.0.1:9020

set -eu

RUNNER="${STRATEGY_RUNNER_URL:-http://127.0.0.1:9020}"
RUNNER="${RUNNER%/}"

echo "=== Docker: ai-scanner ==="
docker ps -a --filter name=ai-scanner 2>/dev/null || echo "(docker not available)"

echo ""
echo "=== Docker logs: 24alert-ai-scanner (last 120 lines) ==="
docker logs 24alert-ai-scanner --tail 120 2>/dev/null || echo "(container missing or no permission)"

echo ""
echo "=== strategy-runner: mounted config (futures instances) ==="
docker exec 24alert-strategy-runner cat /app/config/config.yaml 2>/dev/null | sed -n '/fut-brent-mini-lb/,/notifications:/p' | head -120 || echo "(exec failed — is strategy-runner running?)"

echo ""
echo "=== GET ${RUNNER}/instances ==="
curl -sS "${RUNNER}/instances" | head -c 4000 || echo "(curl failed)"

echo ""
echo "=== GET ${RUNNER}/instances/fut-mechel-lb/pnl (404 if not running) ==="
curl -sS -w "\nHTTP %{http_code}\n" "${RUNNER}/instances/fut-mechel-lb/pnl" || true

echo ""
echo "=== GET ${RUNNER}/instances/fut-mechel-lb/events?limit=20 ==="
curl -sS -w "\nHTTP %{http_code}\n" "${RUNNER}/instances/fut-mechel-lb/events?limit=20" || true

echo ""
echo "=== Restart an instance (POST .../start) ==="
echo "curl -sS -X POST ${RUNNER}/instances/fut-mechel-lb/start -w \"\\nHTTP %{http_code}\\n\""
