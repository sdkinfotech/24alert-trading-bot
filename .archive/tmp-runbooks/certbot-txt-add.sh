#!/bin/bash
set -e
mkdir -p /tmp/acme
echo "$CERTBOT_VALIDATION" > "/tmp/acme/${CERTBOT_DOMAIN}.txt"
chmod 644 "/tmp/acme/${CERTBOT_DOMAIN}.txt"
echo "wrote /tmp/acme/${CERTBOT_DOMAIN}.txt = $CERTBOT_VALIDATION"
for i in $(seq 1 120); do
  val=$(dig @8.8.8.8 +short TXT "_acme-challenge.${CERTBOT_DOMAIN}" | tr -d '"')
  if [ "$val" = "$CERTBOT_VALIDATION" ]; then
    echo "TXT propagated after $((i*5))s"
    sleep 10
    exit 0
  fi
  sleep 5
done
echo "timeout waiting for TXT"
exit 1
