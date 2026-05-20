#!/usr/bin/env bash
# Deploy 24alert with monitoring on srv03-cloud.
#
# Run on srv03-cloud (176.123.160.234):
#   cd /opt/24alert
#   sudo bash scripts/deploy-srv03.sh
#
set -euo pipefail

APP_DIR="/opt/24alert"
PASSWORD_FILE="$APP_DIR/config/remote_write_password"

echo "=== 24alert deploy (srv03-cloud) ==="

cd "$APP_DIR"

# --- Step 1: Pull latest code ---
echo "1. Pulling latest code..."
git pull origin main

# --- Step 2: Check password file ---
if [ ! -f "$PASSWORD_FILE" ] || grep -q "CHANGE_ME" "$PASSWORD_FILE" 2>/dev/null; then
    echo ""
    echo "WARNING: remote_write password not configured!"
    echo "Before starting monitoring, create the file:"
    echo "  echo 'YOUR_PASSWORD' > $PASSWORD_FILE"
    echo ""
    echo "This password must match the one set on srv01-cloud"
    echo "via scripts/setup-monitoring-access.sh."
    echo ""
fi

# --- Step 3: Shared Docker network (must exist before compose up) ---
echo "2. Ensuring Docker network..."
bash scripts/ensure-docker-network.sh

# --- Step 4: Rebuild Docker images ---
echo "3. Building Docker images..."
bash scripts/compose.sh build

# --- Step 5: Restart app services ---
echo "4. Restarting app services..."
bash scripts/compose.sh up -d

# --- Step 5b: Inter-service network check ---
echo "4b. Verifying container network..."
bash scripts/verify-docker-network.sh

# --- Step 6: Health check ---
echo "5. Waiting for health check..."
sleep 5
HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health || echo "000")
if [ "$HTTP_STATUS" = "200" ]; then
    echo "   Gateway health: OK (200)"
else
    echo "   WARNING: Gateway returned $HTTP_STATUS"
fi

# --- Step 7: Verify /metrics endpoint ---
METRICS_STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/metrics || echo "000")
if [ "$METRICS_STATUS" = "200" ]; then
    echo "   Metrics endpoint: OK (200)"
else
    echo "   WARNING: /metrics returned $METRICS_STATUS"
fi

# --- Step 8: Start monitoring (if password is set) ---
if [ -f "$PASSWORD_FILE" ] && ! grep -q "CHANGE_ME" "$PASSWORD_FILE" 2>/dev/null; then
    echo "6. Starting monitoring stack (Prometheus Agent + Promtail)..."
    bash scripts/compose.sh --profile monitoring up -d
    echo "   Monitoring started!"
else
    echo "6. Skipping monitoring start — set the password first (see step 2)."
fi

echo ""
echo "=== Done ==="
echo ""
echo "Verify:"
echo "  curl http://localhost:8080/health"
echo "  curl http://localhost:8080/metrics | head -20"
echo "  bash scripts/compose.sh ps"
echo "  bash scripts/verify-docker-network.sh"
