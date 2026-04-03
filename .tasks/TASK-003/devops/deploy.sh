#!/bin/bash
# Deploy script for trading-bot on srv03-cloud
# Usage: bash deploy.sh [production|sandbox]

set -e

MODE=${1:-production}
TARGET_DIR="/opt/24alert"

echo "========================================="
echo "Trading Bot Deployment Script"
echo "Mode: $MODE"
echo "Target: $TARGET_DIR"
echo "========================================="

# Step 1: SSH & Preparation
echo "[1/8] SSH и подготовка..."
cd $TARGET_DIR

# Step 2: Git clone (if empty)
if [ ! -f "go.mod" ]; then
    echo "[2/8] Git clone..."
    git clone https://github.com/24alert/trading-bot.git .
else
    echo "[2/8] Код уже на месте, обновляю..."
    git pull origin main || true
fi

# Step 3: Configure .env
echo "[3/8] Конфигурирование .env..."
if [ "$MODE" = "production" ]; then
    cat > deployments/.env << 'EOF'
TINVEST_SANDBOX=false
TINVEST_PROD_TOKEN=t.gr4w_xSRuwyOBiLlGHs7Hm7MTATMWVhBDsfLJmn1uccXIvuK20sbpIp_6crH1RJ6rjAjwmLcB2I5fqmFKUGPxw
TINVEST_SANDBOX_TOKEN=t.haxQpgLAVgCxmP_cP7Zb9fLNRrjbmgdp8nmidr_an85UJpRsgrGVQ3SzxcfswYzk5b9yfNLoIzzEt-R5XwHWZQ
LOG_LEVEL=info
EOF
    echo "✓ Production mode enabled (TINVEST_SANDBOX=false)"
else
    cat > deployments/.env << 'EOF'
TINVEST_SANDBOX=true
TINVEST_PROD_TOKEN=t.gr4w_xSRuwyOBiLlGHs7Hm7MTATMWVhBDsfLJmn1uccXIvuK20sbpIp_6crH1RJ6rjAjwmLcB2I5fqmFKUGPxw
TINVEST_SANDBOX_TOKEN=t.haxQpgLAVgCxmP_cP7Zb9fLNRrjbmgdp8nmidr_an85UJpRsgrGVQ3SzxcfswYzk5b9yfNLoIzzEt-R5XwHWZQ
LOG_LEVEL=info
EOF
    echo "✓ Sandbox mode enabled (TINVEST_SANDBOX=true)"
fi

# Step 4: Pre-deployment checks
echo "[4/8] Pre-deployment checks..."
echo "  - Docker: $(docker version --format '{{.Server.Version}}')"
echo "  - Disk: $(df -h /opt/24alert | tail -1 | awk '{print $4}') free"
echo "  - Port 8080: $(sudo netstat -tuln 2>/dev/null | grep :8080 | wc -l) (0 = free)"

# Step 5: Build Docker images
echo "[5/8] Docker build..."
make docker-build > /dev/null 2>&1
docker images | grep 24alert | head -1 | awk '{print "  ✓", $1, $3}'

# Step 6: Stop old containers
echo "[6/8] Stopping old containers..."
docker-compose -f deployments/docker-compose.yaml down 2>/dev/null || true
sleep 2

# Step 7: Start new containers
echo "[7/8] Starting services..."
make docker-up
sleep 5

# Step 8: Health checks
echo "[8/8] Health checks..."
for i in {1..6}; do
    STATUS=$(curl -s http://localhost:8080/health | jq -r '.status // "offline"' 2>/dev/null || echo "offline")
    if [ "$STATUS" = "ok" ]; then
        echo "✓ Gateway health check PASSED"
        break
    fi
    if [ $i -lt 6 ]; then
        echo "  Waiting... ($i/5)"
        sleep 5
    else
        echo "✗ Gateway health check FAILED"
        exit 1
    fi
done

# Summary
echo ""
echo "========================================="
echo "Deployment Complete!"
echo "========================================="
docker-compose -f deployments/docker-compose.yaml ps
echo ""
echo "API: http://176.123.160.234:8080"
echo "Swagger: http://176.123.160.234:8080/swagger/"
echo "Logs: docker-compose logs -f"
