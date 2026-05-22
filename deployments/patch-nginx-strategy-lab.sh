#!/bin/bash
# Add nginx location /strategy-lab → strategy-runner :9020
set -euo pipefail
CONF=/etc/nginx/sites-available/gateway.24alert.ru.conf
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SNIP=/tmp/nginx-strategy-lab.snippet
cp "$ROOT/deployments/nginx-strategy-lab-location.snippet" "$SNIP"
if grep -q 'location /strategy-lab' "$CONF"; then
  echo 'already patched (strategy-lab)'
  exit 0
fi
cp "$CONF" "${CONF}.bak.$(date +%Y%m%d%H%M%S)"
python3 <<'PY'
from pathlib import Path
conf = Path("/etc/nginx/sites-available/gateway.24alert.ru.conf")
snip = Path("/tmp/nginx-strategy-lab.snippet")
text = conf.read_text()
needle = "    # Deny everything else.\n"
if needle not in text:
    raise SystemExit("needle not found: deny block")
conf.write_text(text.replace(needle, snip.read_text() + "\n" + needle))
print("patched", conf)
PY
nginx -t
systemctl reload nginx
echo OK
