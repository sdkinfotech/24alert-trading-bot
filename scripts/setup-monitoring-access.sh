#!/usr/bin/env bash
# Setup script for srv01-cloud (213.171.27.217) monitoring server.
# Configures UFW rules and Prometheus basic auth for 24alert remote_write.
#
# Run on srv01-cloud as root or with sudo:
#   ssh admin-cloud-srv-01@213.171.27.217
#   sudo bash /path/to/setup-monitoring-access.sh
#
set -euo pipefail

SRV03_IP="176.123.160.234"
PROMETHEUS_PORT=9191
LOKI_PORT=3100
MONITORING_DIR="/opt/monitoring"

echo "=== 24alert Monitoring Access Setup ==="
echo "Granting srv03-cloud ($SRV03_IP) access to monitoring ports..."

# --- UFW Rules ---
echo ""
echo "--- UFW Rules ---"

ufw allow from "$SRV03_IP" to any port "$PROMETHEUS_PORT" proto tcp comment "24alert remote_write"
echo "Allowed $SRV03_IP -> port $PROMETHEUS_PORT (Prometheus remote_write)"

ufw allow from "$SRV03_IP" to any port "$LOKI_PORT" proto tcp comment "24alert promtail"
echo "Allowed $SRV03_IP -> port $LOKI_PORT (Loki log push)"

ufw status numbered | grep -E "$SRV03_IP|24alert"

# --- Prometheus Basic Auth ---
echo ""
echo "--- Prometheus Basic Auth ---"

if ! command -v htpasswd &> /dev/null; then
    echo "Installing apache2-utils for htpasswd..."
    apt-get update -qq && apt-get install -y -qq apache2-utils
fi

WEB_YML="$MONITORING_DIR/prometheus/web.yml"

if [ ! -f "$WEB_YML" ]; then
    echo "ERROR: $WEB_YML not found. Check MONITORING_DIR."
    exit 1
fi

echo ""
echo "Creating basic auth user '24alert' for Prometheus remote_write."
echo "Enter the password for the 24alert user:"
read -rs PASSWORD
echo ""

BCRYPT_HASH=$(htpasswd -nbBC 10 "" "$PASSWORD" | tr -d ':\n' | sed 's/^\$//')
BCRYPT_HASH="\$$BCRYPT_HASH"

if grep -q "24alert:" "$WEB_YML"; then
    echo "User '24alert' already exists in $WEB_YML — updating..."
    sed -i "s|24alert:.*|24alert: '$BCRYPT_HASH'|" "$WEB_YML"
else
    echo "Adding user '24alert' to $WEB_YML..."
    # Append under basic_auth_users section
    if grep -q "basic_auth_users:" "$WEB_YML"; then
        sed -i "/basic_auth_users:/a\\  24alert: '$BCRYPT_HASH'" "$WEB_YML"
    else
        echo "" >> "$WEB_YML"
        echo "basic_auth_users:" >> "$WEB_YML"
        echo "  24alert: '$BCRYPT_HASH'" >> "$WEB_YML"
    fi
fi

echo "Reloading Prometheus config..."
cd "$MONITORING_DIR"
docker compose restart prometheus

echo ""
echo "=== Done ==="
echo ""
echo "IMPORTANT: Save this password — you need to put it in"
echo "  /opt/24alert/config/remote_write_password"
echo "on srv03-cloud ($SRV03_IP)."
echo ""
echo "Then start the monitoring stack on srv03-cloud:"
echo "  cd /opt/24alert/deployments"
echo "  docker compose --profile monitoring up -d"
