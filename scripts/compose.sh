#!/usr/bin/env bash
# Canonical docker compose wrapper — always same project + network.
# Usage: scripts/compose.sh up -d strategy-runner
#        scripts/compose.sh ps
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-24alert}"

bash "$ROOT/scripts/ensure-docker-network.sh"

exec docker compose -p "$COMPOSE_PROJECT_NAME" -f deployments/docker-compose.yaml "$@"
