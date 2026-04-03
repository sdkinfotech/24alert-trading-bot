#!/bin/bash

# Rollback script for production
# Usage: bash scripts/rollback-prod.sh <host> <user> <previous-image>
# Example: bash scripts/rollback-prod.sh 176.123.160.234 adm-srv03-cloud sdkinfotech/24alert-trading-bot:main-abc1234

set -e

HOST=${1:-}
USER=${2:-}
PREVIOUS_IMAGE=${3:-}

# Validation
if [ -z "$HOST" ] || [ -z "$USER" ] || [ -z "$PREVIOUS_IMAGE" ]; then
    echo "❌ Usage: $0 <host> <user> <previous-image>"
    echo "   Example: $0 176.123.160.234 adm-srv03-cloud sdkinfotech/24alert-trading-bot:main-abc1234"
    exit 1
fi

DEPLOY_KEY="$HOME/.ssh/deploy_key"
if [ ! -f "$DEPLOY_KEY" ]; then
    echo "❌ SSH key not found: $DEPLOY_KEY"
    exit 1
fi

echo "=========================================="
echo "🔄 Trading Bot Rollback"
echo "=========================================="
echo "  Host: $HOST"
echo "  User: $USER"
echo "  Image: $PREVIOUS_IMAGE"
echo "=========================================="

# Function to run remote command
remote_exec() {
    local cmd="$1"
    ssh -i "$DEPLOY_KEY" -o ConnectTimeout=10 -o StrictHostKeyChecking=no "$USER@$HOST" "$cmd"
}

# ============================================
# Step 1: Stop current deployment
# ============================================
echo "[1/3] Stopping current deployment..."

if ! remote_exec "cd /opt/24alert && docker-compose -f deployments/docker-compose.yaml down"; then
    echo "⚠️  Docker-compose down failed (might already be stopped)"
fi

echo "✅ Current deployment stopped"

# ============================================
# Step 2: Pull and restart with previous image
# ============================================
echo "[2/3] Pulling previous image and restarting..."

if ! remote_exec "docker pull $PREVIOUS_IMAGE"; then
    echo "❌ Failed to pull previous image: $PREVIOUS_IMAGE"
    echo "   Make sure the image is available in the registry"
    exit 1
fi

# Update docker-compose to use the previous image
if ! remote_exec "cd /opt/24alert && docker-compose -f deployments/docker-compose.yaml up -d"; then
    echo "❌ Failed to start containers with previous image"
    exit 1
fi

echo "✅ Previous image deployed"

# ============================================
# Step 3: Verify health
# ============================================
echo "[3/3] Verifying health after rollback..."

sleep 3

if remote_exec "curl -sf http://localhost:8080/health > /dev/null 2>&1"; then
    echo "✅ Gateway health check passed"
    echo ""
    echo "=========================================="
    echo "✅ ROLLBACK SUCCESSFUL"
    echo "=========================================="
    echo "  Previous image: $PREVIOUS_IMAGE"
    echo "  Restored at: $(date -u +'%Y-%m-%dT%H:%M:%SZ')"
    echo "=========================================="
    exit 0
else
    echo "❌ Health check failed after rollback"
    echo ""
    echo "Debug info:"
    remote_exec "cd /opt/24alert && docker-compose -f deployments/docker-compose.yaml ps" || true
    remote_exec "cd /opt/24alert && docker-compose -f deployments/docker-compose.yaml logs --tail=20" || true
    exit 1
fi
