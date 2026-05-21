#!/usr/bin/env bash
# Canonical docker compose wrapper — always same project + network.
# Usage: scripts/compose.sh up -d strategy-runner
#        scripts/compose.sh ps
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-24alert}"
# AI Scanner: nightly scan + pre-market + health (see deployments/ai-scanner/README.md)
export COMPOSE_PROFILES="${COMPOSE_PROFILES:-ai-scanner}"

bash "$ROOT/scripts/ensure-docker-network.sh"

exec docker compose -p "$COMPOSE_PROJECT_NAME" -f deployments/docker-compose.yaml "$@"
