#!/usr/bin/env bash
# Verify 24alert containers share one network and advisor reaches strategy-runner.
set -euo pipefail

NET="${DOCKER_NETWORK_NAME:-24alert-trading-bot-net}"
ERR=0

echo "=== Docker network: $NET ==="

if ! docker network inspect "$NET" >/dev/null 2>&1; then
  echo "FAIL: network $NET does not exist (run scripts/ensure-docker-network.sh)"
  exit 1
fi

echo "--- Containers on $NET (24alert*) ---"
docker network inspect "$NET" --format '{{range .Containers}}{{.Name}} {{end}}' | tr ' ' '\n' | grep -E '^24alert-' | sort || true

check_pair() {
  local from="$1" to_url="$2" label="$3"
  if ! docker ps --format '{{.Names}}' | grep -qx "$from"; then
    echo "SKIP: $from not running"
    return 0
  fi
  local code
  code=$(docker exec "$from" curl -sf -o /dev/null -w '%{http_code}' "$to_url" 2>/dev/null || echo "000")
  if [ "$code" = "200" ]; then
    echo "OK: $label ($from -> $to_url)"
  else
    echo "FAIL: $label ($from -> $to_url) HTTP=$code"
    ERR=1
  fi
}

check_pair "24alert-advisor-svc" "http://24alert-strategy-runner:9020/health" "advisor -> strategy-runner"
check_pair "24alert-strategy-runner" "http://24alert-advisor-svc:9030/health" "strategy-runner -> advisor"

echo "--- Orphan networks (should be empty or unused) ---"
for n in deployments_trading-bot-net 24alert_trading-bot-net; do
  if docker network inspect "$n" >/dev/null 2>&1; then
    cnt=$(docker network inspect "$n" --format '{{len .Containers}}' 2>/dev/null || echo 0)
    if [ "${cnt:-0}" -gt 0 ]; then
      echo "WARN: $n still has $cnt container(s) — run scripts/reconcile-docker-network.sh"
      ERR=1
    fi
  fi
done

if [ "$ERR" -eq 0 ]; then
  echo "=== Network verification passed ==="
else
  echo "=== Network verification FAILED ==="
  exit 1
fi
