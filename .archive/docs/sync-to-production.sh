#!/bin/bash
# Sync script: Push local → GitHub → Pull on server
# Usage: bash sync-to-production.sh

set -e

echo "========================================="
echo "Git Sync: Local → GitHub → Server"
echo "========================================="

# Step 1: Verify local repo
echo "[1/4] Verifying local repository..."
cd c:\Users\sdk\proj\24alert
STATUS=$(git status --porcelain)
if [ ! -z "$STATUS" ]; then
    echo "⚠️  Uncommitted changes detected:"
    echo "$STATUS"
    echo "Please commit or stash changes first."
    exit 1
fi
echo "✓ Local repo clean"

# Step 2: Verify remote
echo "[2/4] Verifying GitHub remote..."
REMOTE=$(git remote get-url origin)
echo "  Remote: $REMOTE"
echo "✓ Remote configured"

# Step 3: Push to GitHub
echo "[3/4] Pushing to GitHub..."
git push -u origin main
echo "✓ Pushed to GitHub"

# Step 4: Instructions for server sync
echo "[4/4] Server sync instructions..."
echo ""
echo "Next, SSH to server and run:"
echo ""
echo "  ssh adm-srv03-cloud@176.123.160.234"
echo "  cd /opt/24alert"
echo "  git pull origin main"
echo "  make docker-build"
echo "  make docker-up"
echo ""
echo "========================================="
echo "✓ Local sync complete!"
echo "========================================="
