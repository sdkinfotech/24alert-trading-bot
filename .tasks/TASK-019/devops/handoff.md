# Handoff: devops | ticket_id: TASK-019 | slug: TASK-019

## Тикет
- **ticket_id:** TASK-019
- **slug:** TASK-019

## Статус
DONE

## Что сделано

### 1. Контейнер
- `deployments/docker-compose.yaml`: gateway publish изменён с `"8080:8080"` на `"127.0.0.1:18080:8080"`. Внешний порт 8080 освобождён для nginx (TLS terminator).
- `docker compose -p 24alert build gateway && docker compose -p 24alert up -d --no-deps --force-recreate gateway` — пересобрано и перезапущено.

### 2. DNS
- A-record `gateway.24alert.ru → 176.123.160.234` создан через `twc.exe domain subdomain add gateway.24alert.ru` + A-record (при необходимости).
- DNS-01 challenge: TXT `_acme-challenge.gateway.24alert.ru` добавляется/удаляется хуками:
  - `/usr/local/bin/certbot-txt-add.sh` — `twc.exe domain record add` + ожидание propagation.
  - `/usr/local/bin/certbot-txt-del.sh` — cleanup.
- Оба скрипта очищены от CRLF (`sed -i 's/\r$//' ...`).

### 3. TLS (Let's Encrypt)
- HTTP-01 challenge не работает (провайдер блокирует 80/443 наружу).
- `certbot certonly --manual --preferred-challenges dns --manual-auth-hook /usr/local/bin/certbot-txt-add.sh --manual-cleanup-hook /usr/local/bin/certbot-txt-del.sh --manual-public-ip-logging-ok -d gateway.24alert.ru` — сертификат выдан.
- Автопродление: systemd timer `certbot.timer` активен.

### 4. Nginx
- Файл `/etc/nginx/sites-available/gateway.24alert.ru` + symlink в `sites-enabled`:
  - `listen 8080 ssl` (Timeweb пропускает наружу только 8080 для этого VPS).
  - TLS-сертификаты из `/etc/letsencrypt/live/gateway.24alert.ru/`.
  - `proxy_pass http://127.0.0.1:18080;`
  - WS headers (`Upgrade`, `Connection`, `Host`, `X-Real-IP`, `X-Forwarded-For`, `X-Forwarded-Proto`).
  - `proxy_read_timeout 120s;` / `proxy_send_timeout 120s;` — комфорт для редких пауз в рынке.
  - Location `/api/v1/stream/` с `allow 72.56.243.146; allow 127.0.0.1; deny all;` (ACL).
  - Location `/health` — без ACL, для external smoke.
- `nginx -t && systemctl reload nginx` — зелёные.

### 5. Smoke
- `curl -sS https://gateway.24alert.ru:8080/health` — 200 снаружи.
- `python3 wss_smoke.py` с traderbook VPS (72.56.243.146): handshake 101, ≥ 3 snapshot-фреймов за 5 секунд.
- Traderbook `market-data` подключился: логи `[orderbook-stream] connected, 5 uids`, 12332 snapshot'ов за 11 минут, density_active > 0 по SBER.

## Артефакты
- Конфиги:
  - gateway: `/opt/24alert/deployments/docker-compose.yaml`
  - nginx: `/etc/nginx/sites-enabled/gateway.24alert.ru`
  - LE certs: `/etc/letsencrypt/live/gateway.24alert.ru/`
  - hooks: `/usr/local/bin/certbot-txt-{add,del}.sh`
- Snapshot'ы конфигов сохранены в `.tmp/gateway.24alert.ru.conf`, `.tmp/fix-compose.sh` в репозитории (локально у devops-агента).

## Корректировки для следующих ролей
Для **tester**: smoke поверх `wss://gateway.24alert.ru:8080/api/v1/stream/orderbook` должен запускаться с IP, который в ACL. Для добавления нового IP — отредактировать `allow` в nginx-конфиге, `systemctl reload nginx`.

## Блокеры
НЕТ
