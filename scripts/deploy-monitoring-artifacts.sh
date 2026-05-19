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

echo "1. Copying dashboards..."
scp monitoring/dashboards/24alert-gateway-api.json \
    monitoring/dashboards/24alert-strategy-runner.json \
    monitoring/dashboards/24alert-ai-scanner.json \
    monitoring/dashboards/24alert-infrastructure.json \
    monitoring/dashboards/24alert-llm-observability.json \
  "${REMOTE_USER}@${REMOTE_HOST}:/tmp/"

echo "2. Copying alert rules..."
scp monitoring/rules/24alert.yml \
  "${REMOTE_USER}@${REMOTE_HOST}:/tmp/24alert.yml"

echo "3. Validating and installing artifacts..."
ssh "${REMOTE_USER}@${REMOTE_HOST}" "\
  docker cp /tmp/24alert.yml prometheus:/tmp/24alert.yml && \
  docker exec prometheus promtool check rules /tmp/24alert.yml && \
  sudo cp /tmp/24alert.yml ${MONITORING_DIR}/prometheus/rules/24alert.yml && \
  sudo cp /tmp/24alert-gateway-api.json ${MONITORING_DIR}/grafana/dashboards-24alert/24alert-gateway-api.json && \
  sudo cp /tmp/24alert-strategy-runner.json ${MONITORING_DIR}/grafana/dashboards-24alert/strategy-runner.json && \
  sudo cp /tmp/24alert-ai-scanner.json ${MONITORING_DIR}/grafana/dashboards-24alert/24alert-ai-scanner.json && \
  sudo cp /tmp/24alert-infrastructure.json ${MONITORING_DIR}/grafana/dashboards-24alert/24alert-infrastructure.json && \
  sudo cp /tmp/24alert-llm-observability.json ${MONITORING_DIR}/grafana/dashboards-24alert/24alert-llm-observability.json && \
  docker kill -s HUP prometheus && \
  docker restart grafana >/dev/null"

echo ""
echo "=== Done ==="
echo "Dashboards:"
echo "  Strategy: http://${REMOTE_HOST}:3535/d/24alert-strategy/24alert-strategy-runner"
echo "  LLM:      http://${REMOTE_HOST}:3535/d/24alert-llm/24alert-llm-openrouter"
echo "Alert rules applied to Prometheus."
