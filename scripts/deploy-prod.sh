#!/bin/bash
# Deploy 24alert to production (GitHub Actions or manual).
# Usage: bash scripts/deploy-prod.sh <host> <user> <commit>
# Example: bash scripts/deploy-prod.sh 176.123.160.234 adm-srv03-cloud faaac39

set -euo pipefail

HOST=${1:-}
USER=${2:-}
COMMIT=${3:-}

if [ -z "$HOST" ] || [ -z "$USER" ] || [ -z "$COMMIT" ]; then
    echo "Usage: $0 <host> <user> <commit>"
    exit 1
fi

DEPLOY_KEY="${DEPLOY_KEY:-$HOME/.ssh/deploy_key}"
SSH_OPTS=(-i "$DEPLOY_KEY" -o ConnectTimeout=15 -o StrictHostKeyChecking=no)

if [ ! -f "$DEPLOY_KEY" ]; then
    echo "SSH key not found: $DEPLOY_KEY (set DEPLOY_KEY to override)"
    exit 1
fi

remote() {
    ssh "${SSH_OPTS[@]}" "$USER@$HOST" "$@"
}

echo "=========================================="
echo "24alert production deploy"
echo "  Host:   $USER@$HOST"
echo "  Commit: $COMMIT"
echo "=========================================="

echo "[1/6] Pre-checks..."
remote "docker ps >/dev/null" || { echo "Docker unavailable"; exit 1; }
remote "test -f /opt/24alert/scripts/compose.sh" || { echo "Missing /opt/24alert/scripts/compose.sh"; exit 1; }
echo "OK"

echo "[2/6] git pull..."
remote "cd /opt/24alert && git fetch origin main && git checkout main && git pull origin main"
remote "cd /opt/24alert && git rev-parse --short HEAD"

echo "[3/6] Docker network + build (strategy-runner, advisor-svc)..."
remote "cd /opt/24alert && sudo bash scripts/ensure-docker-network.sh"
# Remove stale named containers that block compose recreate
remote "sudo docker rm -f 24alert-strategy-runner 24alert-advisor-svc 2>/dev/null || true"
remote "cd /opt/24alert && sudo bash scripts/compose.sh build strategy-runner advisor-svc"

echo "[4/6] compose up..."
remote "cd /opt/24alert && sudo bash scripts/compose.sh up -d"
remote "cd /opt/24alert && sudo bash scripts/verify-docker-network.sh"

echo "[5/6] config reload + health..."
sleep 3
remote "curl -sf http://127.0.0.1:8080/health >/dev/null && echo gateway_ok || echo gateway_FAIL"
remote "curl -sf http://127.0.0.1:9020/health >/dev/null && echo runner_ok || echo runner_FAIL"
remote "curl -sf http://127.0.0.1:9030/health >/dev/null && echo advisor_ok || echo advisor_FAIL"
remote "curl -s -X POST http://127.0.0.1:9020/config/reload || true"

echo "[6/6] container status..."
remote "cd /opt/24alert && sudo bash scripts/compose.sh ps"

echo "=========================================="
echo "DEPLOYMENT SUCCESSFUL"
echo "  Commit: $COMMIT"
echo "=========================================="
