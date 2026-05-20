#!/usr/bin/env bash
# Attach all 24alert-* containers to the canonical network; disconnect legacy split networks.
set -euo pipefail

NET="${DOCKER_NETWORK_NAME:-24alert-trading-bot-net}"

bash "$(dirname "$0")/ensure-docker-network.sh"

LEGACY_NETS=(deployments_trading-bot-net 24alert_trading-bot-net)

echo "=== Reconcile containers to $NET ==="

mapfile -t CONTAINERS < <(docker ps -a --format '{{.Names}}' | grep -E '^24alert-' || true)

if [ "${#CONTAINERS[@]}" -eq 0 ]; then
  echo "No 24alert-* containers found."
  exit 0
fi

for c in "${CONTAINERS[@]}"; do
  if docker inspect "$c" --format '{{range $k, $v := .NetworkSettings.Networks}}{{$k}} {{end}}' | grep -qw "$NET"; then
    echo "OK: $c already on $NET"
  else
    echo "CONNECT: $c -> $NET"
    docker network connect "$NET" "$c" 2>/dev/null || echo "  (connect failed — is container running?)"
  fi
  for leg in "${LEGACY_NETS[@]}"; do
    if docker network inspect "$leg" >/dev/null 2>&1; then
      if docker inspect "$c" --format '{{range $k, $v := .NetworkSettings.Networks}}{{$k}} {{end}}' | grep -qw "$leg"; then
        echo "DISCONNECT: $c from $leg"
        docker network disconnect "$leg" "$c" 2>/dev/null || true
      fi
    fi
  done
done

echo "=== Done. Run scripts/verify-docker-network.sh ==="
