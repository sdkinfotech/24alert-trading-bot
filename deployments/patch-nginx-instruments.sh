#!/bin/bash
set -euo pipefail
CONF=/etc/nginx/sites-available/gateway.24alert.ru.conf
SNIP=/tmp/nginx-instruments.snippet
if grep -q 'location /instruments' "$CONF"; then
  echo 'already patched'
  exit 0
fi
cp "$CONF" "${CONF}.bak.$(date +%Y%m%d%H%M%S)"
python3 <<'PY'
from pathlib import Path
conf = Path("/etc/nginx/sites-available/gateway.24alert.ru.conf")
snip = Path("/tmp/nginx-instruments.snippet")
text = conf.read_text()
needle = "    # AI Trader prompt-driven sessions.\n"
if needle not in text:
    raise SystemExit("needle not found")
conf.write_text(text.replace(needle, snip.read_text() + "\n" + needle))
print("patched", conf)
PY
nginx -t
systemctl reload nginx
echo OK
