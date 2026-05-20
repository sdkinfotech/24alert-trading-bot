#!/usr/bin/env bash
# Ensure the shared Docker network exists before compose up.
set -euo pipefail

NET="${DOCKER_NETWORK_NAME:-24alert-trading-bot-net}"

if docker network inspect "$NET" >/dev/null 2>&1; then
  echo "Docker network $NET: OK"
else
  echo "Creating Docker network $NET ..."
  docker network create "$NET"
fi
