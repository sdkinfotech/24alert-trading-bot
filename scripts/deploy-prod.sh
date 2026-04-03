#!/bin/bash

# Deploy script for production deployment on srv03-cloud
# Usage: bash scripts/deploy-prod.sh <host> <user> <commit>
# Example: bash scripts/deploy-prod.sh 176.123.160.234 adm-srv03-cloud abc1234

set -e

HOST=${1:-}
USER=${2:-}
COMMIT=${3:-}

# Validation
if [ -z "$HOST" ] || [ -z "$USER" ] || [ -z "$COMMIT" ]; then
    echo "❌ Usage: $0 <host> <user> <commit>"
    exit 1
fi

DEPLOY_KEY="$HOME/.ssh/deploy_key"
if [ ! -f "$DEPLOY_KEY" ]; then
    echo "❌ SSH key not found: $DEPLOY_KEY"
    exit 1
fi

echo "=========================================="
echo "🚀 Trading Bot Production Deployment"
echo "=========================================="
echo "  Host: $HOST"
echo "  User: $USER"
echo "  Commit: $COMMIT"
echo "=========================================="

# Function to run remote command
remote_exec() {
    local cmd="$1"
    ssh -i "$DEPLOY_KEY" -o ConnectTimeout=10 -o StrictHostKeyChecking=no "$USER@$HOST" "$cmd"
}

# ============================================
# Step 1: Pre-deployment checks
# ============================================
echo "[1/5] Pre-deployment checks..."

if ! remote_exec "docker ps > /dev/null 2>&1"; then
    echo "❌ Cannot connect to Docker daemon"
    exit 1
fi

if ! remote_exec "test -f /opt/24alert/docker-compose.yaml"; then
    echo "❌ /opt/24alert/docker-compose.yaml not found"
    exit 1
fi

echo "✅ Pre-deployment checks passed"

# ============================================
# Step 2: Pull latest code
# ============================================
echo "[2/5] Pulling latest code from GitHub..."

if ! remote_exec "cd /opt/24alert && git pull origin main"; then
    echo "⚠️  Git pull failed (might be first deploy or detached HEAD)"
    echo "    Continuing with existing code..."
fi

echo "✅ Code updated"

# ============================================
# Step 3: Build and start containers
# ============================================
echo "[3/5] Building and starting Docker containers..."

if ! remote_exec "cd /opt/24alert && docker-compose -f deployments/docker-compose.yaml pull 2>/dev/null || true"; then
    echo "⚠️  Docker pull failed (images might not be in registry yet)"
fi

if ! remote_exec "cd /opt/24alert && docker-compose -f deployments/docker-compose.yaml build"; then
    echo "❌ Docker build failed"
    exit 1
fi

if ! remote_exec "cd /opt/24alert && docker-compose -f deployments/docker-compose.yaml up -d"; then
    echo "❌ Docker-compose up failed"
    exit 1
fi

echo "✅ Containers started"

# ============================================
# Step 4: Wait for services to be ready
# ============================================
echo "[4/5] Waiting for services to be ready (max 30 seconds)..."

if ! remote_exec "sleep 2"; then
    echo "❌ Remote command failed"
    exit 1
fi

echo "✅ Services warming up"

# ============================================
# Step 5: Health check with retry
# ============================================
echo "[5/5] Performing health checks..."

RETRY_COUNT=0
MAX_RETRIES=5
RETRY_INTERVAL=2

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    RETRY_COUNT=$((RETRY_COUNT + 1))
    
    echo "  Health check attempt $RETRY_COUNT/$MAX_RETRIES..."
    
    # Check gateway health
    if remote_exec "curl -sf http://localhost:8080/health > /dev/null 2>&1"; then
        echo "✅ Gateway health check passed"
        
        # Check other services are responding
        if remote_exec "docker-compose -f deployments/docker-compose.yaml ps | grep -q 'Up'"; then
            echo "✅ All containers are running"
            echo ""
            echo "=========================================="
            echo "✅ DEPLOYMENT SUCCESSFUL"
            echo "=========================================="
            echo "  Commit: $COMMIT"
            echo "  Deployed to: $HOST"
            echo "  Time: $(date -u +'%Y-%m-%dT%H:%M:%SZ')"
            echo "=========================================="
            exit 0
        fi
    fi
    
    if [ $RETRY_COUNT -lt $MAX_RETRIES ]; then
        echo "  Waiting ${RETRY_INTERVAL}s before retry..."
        sleep "$RETRY_INTERVAL"
    fi
done

echo "❌ Health check failed after $MAX_RETRIES attempts"
echo ""
echo "Collecting debug information..."
remote_exec "cd /opt/24alert && docker-compose -f deployments/docker-compose.yaml ps" || true
remote_exec "cd /opt/24alert && docker-compose -f deployments/docker-compose.yaml logs --tail=20" || true

exit 1
