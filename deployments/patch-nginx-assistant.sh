#!/bin/bash
# Add nginx locations for /assistant and /config → strategy-runner :9020
set -euo pipefail
CONF=/etc/nginx/sites-available/gateway.24alert.ru.conf
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SNIP=/tmp/nginx-assistant.snippet
cp "$ROOT/deployments/nginx-assistant-location.snippet" "$SNIP"
if grep -q 'location /assistant' "$CONF"; then
  echo 'already patched (assistant)'
  exit 0
fi
cp "$CONF" "${CONF}.bak.$(date +%Y%m%d%H%M%S)"
python3 <<'PY'
from pathlib import Path
conf = Path("/etc/nginx/sites-available/gateway.24alert.ru.conf")
snip = Path("/tmp/nginx-assistant.snippet")
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
