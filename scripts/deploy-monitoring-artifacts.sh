#!/usr/bin/env bash
# Deploy 24alert monitoring artifacts to srv01-cloud.
# Copies dashboard JSON and alert rules, then reloads services.
#
# Run from the repo root on your local machine:
#   bash scripts/deploy-monitoring-artifacts.sh
#
set -euo pipefail

REMOTE_USER="admin-cloud-srv-01"
REMOTE_HOST="213.171.27.217"
MONITORING_DIR="/opt/monitoring"

echo "=== Deploying 24alert monitoring artifacts to $REMOTE_HOST ==="

echo "1. Copying dashboard..."
scp monitoring/dashboards/24alert.json \
  "${REMOTE_USER}@${REMOTE_HOST}:${MONITORING_DIR}/grafana/dashboards/24alert.json"

echo "2. Copying alert rules..."
scp monitoring/rules/24alert.yml \
  "${REMOTE_USER}@${REMOTE_HOST}:${MONITORING_DIR}/prometheus/rules/24alert.yml"

echo "3. Reloading services..."
ssh "${REMOTE_USER}@${REMOTE_HOST}" "cd ${MONITORING_DIR} && sudo docker compose restart prometheus grafana"

echo ""
echo "=== Done ==="
echo "Dashboard: http://${REMOTE_HOST}:3535/d/24alert-trading-bot"
echo "Alert rules applied to Prometheus."
