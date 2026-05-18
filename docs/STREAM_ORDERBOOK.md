# Stream · OrderBook (WebSocket)

Gateway-endpoint поверх T-Invest `OrderBookStream`. Позволяет внешним потребителям (в первую очередь — Traderbook `services/market-data`) получать обновления стакана в реальном времени в унифицированном JSON-формате без прямого gRPC-соединения с T-Invest.

Introduced in `TASK-019` (Traderbook TB-036 dependency), прод с 2026-04-17.

## 1. Общая архитектура

```
T-Invest gRPC (OrderBookStream)
           │
           ▼
┌───────────────────────────────────────────────┐
│  24alert-gateway (:8080 inside container)     │
│    internal/gateway/handlers/stream.go        │
│       ├─ StreamCandles   (уже существовал)    │
│       └─ StreamOrderBook ← NEW                │
│    internal/gateway/services/stream/          │
│       └─ StreamManager.SubscribeOrderbook     │
└───────────────────────────────────────────────┘
           │   WS JSON frames
           ▼
┌───────────────────────────────────────────────┐
│  nginx (host, TCP:8080 TLS)                   │
│    /etc/nginx/sites-enabled/gateway.24alert.ru│
│    - TLS termination (Let's Encrypt, DNS-01)  │
│    - ACL: IP whitelist для /api/v1/stream/*   │
│    - proxy_pass http://127.0.0.1:18080        │
└───────────────────────────────────────────────┘
           │
           ▼ wss://gateway.24alert.ru:8080/api/v1/stream/orderbook
    Внешние потребители (Traderbook VPS 72.56.243.146 и др.)
```

Gateway-контейнер публикуется только на `127.0.0.1:18080` (host-side). Публичный выход наружу — **только** через nginx на `:8080/tcp` с TLS и ACL.

> **Почему порт 8080, а не 443?** Timeweb Cloud блокирует внешний доступ к портам 80/443 для данного VPS (`srv03-cloud`, IP `176.123.160.234`). Из доступных портов единственный свободный для сырого TCP — 8080, поэтому весь HTTPS/WSS уехал на него. См. § Траблшутинг.

## 2. Endpoint

- **URL:** `wss://gateway.24alert.ru:8080/api/v1/stream/orderbook`
- **Handshake:** стандартный WebSocket upgrade (RFC 6455). HTTP-маршрут также отвечает `400 Bad Request` при отсутствии `uids`.
- **ACL:** только IP из whitelist (`/etc/nginx/sites-enabled/gateway.24alert.ru`). На 2026-04-17:
  - `72.56.243.146` — traderbook prod VPS
  - `127.0.0.1` — локальные smoke-тесты на сервере
- **Health:** `https://gateway.24alert.ru:8080/health` (TLS, тот же nginx, без ACL).

Смежные microstructure endpoints для AI Trader:

| Endpoint | Назначение |
|----------|------------|
| `/api/v1/stream/trades?uids=...` | public trades / prints: `type=trade`, `uid`, `direction`, `price`, `quantity`, `time`, `ts` |
| `/api/v1/stream/last-price?uids=...` | last price heartbeat: `type=last_price`, `uid`, `price`, `time`, `ts` |

### Query parameters

| Параметр | Тип | По умолчанию | Описание |
|----------|-----|--------------|----------|
| `uids`   | CSV string (required) | — | UID инструментов T-Invest. Текущая реализация gateway обрезает список до **50 UID** на WS; 300 — лимит T-Invest, но не текущий контракт gateway. Пустой или только-пробельный → HTTP 400. |
| `depth`  | int | `20` | Глубина стакана (10/20/30/40/50 — по контракту T-Invest). |

Пример:

```
wss://gateway.24alert.ru:8080/api/v1/stream/orderbook?uids=e6123145-...,962e2a95-...&depth=20
```

## 3. Протокол сообщений

Все сообщения — текстовые JSON-фреймы. Server → Client. Client → Server отсутствует (это односторонний стрим; для пинг-понга сервер сам рассылает `ping`).

### 3.1 `snapshot`

Полный снимок стакана после каждого обновления от T-Invest. Поле `depth` отражает фактическую глубину, полученную в кадре (может быть меньше запрошенной при тонких книгах).

```json
{
  "type": "snapshot",
  "uid": "e6123145-9665-43e0-8413-cd61b8aa9b13",
  "depth": 20,
  "bids": [
    {"price": 323.12, "quantity": 37153},
    {"price": 323.00, "quantity": 106407}
  ],
  "asks": [
    {"price": 323.37, "quantity": 33174},
    {"price": 323.55, "quantity": 21414}
  ],
  "ts": 1776428924790
}
```

Поля:

| Поле | Тип | Описание |
|------|-----|----------|
| `type` | `"snapshot"` | Литерал |
| `uid`  | string | UID инструмента T-Invest |
| `depth`| int   | Фактическая глубина (len(bids), len(asks) могут быть меньше, если одной из сторон в пределах depth меньше) |
| `bids` | array | Покупки, убывающе по цене. `{price: float64, quantity: int64}` |
| `asks` | array | Продажи, возрастающе по цене. Такой же формат |
| `ts`   | int64 | Event time от T-Invest, unix-ms (UTC) |

> **Важно для AI Trader / scalping:** текущий stream отдаёт полные snapshots, а не delta-book. В контракте пока нет sequence number, receive timestamp, spread/imbalance, counters dropped frames и признака устаревшего состояния. При медленном потребителе gateway может drop-нуть snapshot в буфере. Для live AI-скальпера это нужно расширить до microstructure event schema, см. [`docs/AI_TRADER_SCALPER.md`](AI_TRADER_SCALPER.md).

### 3.2 `ping`

Пинг от сервера ~каждые 15 секунд. Используется как heartbeat и для keep-alive через nginx.

```json
{"type": "ping", "ts": 1776428940000}
```

Клиенту отвечать не нужно. Стандартных WebSocket control-frames (ping/pong opcode) gateway также шлёт средствами библиотеки (`gorilla/websocket`).

### 3.3 `error`

Сообщение об ошибке подписки / T-Invest gRPC. После него соединение обычно закрывается; клиент должен реконнектить с backoff.

```json
{"type": "error", "error": "t-invest rpc: RESOURCE_EXHAUSTED (30042)", "ts": 1776428950000}
```

## 4. Сценарии реконнекта

- Соединение рвётся nginx при `proxy_read_timeout` (60s) — mitigation: `ping` каждые 15с.
- T-Invest может прервать стрим при ротации токена, сетевой ошибке, rate-limit: gateway шлёт `error`, закрывает WS. Клиент реконнектит с экспоненциальным backoff (`services/market-data/src/orderbook/orderbook-stream.ts`: 1s → 2s → 4s → 8s → 16s, потолок 16s, jitter ±20%).
- При реконнекте `snapshot` приходит заново (не требуется persistent state на клиенте).

## 5. Нагрузка и лимиты

| Параметр | Значение |
|----------|---------|
| Макс. подписок на один WS | **50 UID в текущем gateway code path**; T-Invest допускает до 300 UID на одно WS-соединение совокупно с `OrderBookStream` + `MarketDataStream` |
| Частота `snapshot` | зависит от активности; SBER в пике 50 Гц, средне 5–20 Гц. В RPS-метриках gateway ≤ 5 000/s для 200 UID |
| Ширина кадра | ~400 байт для depth=20 |
| Сетевой объём | ≈ 200 KB/s исходящего для 200 UID при 1–5 Hz |

При необходимости более 50 UID — разнесение по нескольким WS-соединениям либо доработка gateway limit/hub. Для AI Trader предпочтительнее shared `OrderBookHub` с ref-count, reconnect и drop counters.

## 6. Метрики

Gateway экспортирует метрики по `:8080/metrics` (внутри контейнера). Ключевые:

- `gateway_orderbook_clients` — число активных WS-соединений к `/stream/orderbook`
- `gateway_orderbook_frames_total{type="snapshot|ping|error"}` — отправленные фреймы
- `gateway_orderbook_subscribers` — число уникальных UID в активных подписках
- `gateway_orderbook_reconnects_total` — вход T-Invest gRPC reconnect

В Traderbook стороне:

- `orderbook_updates_total{uid}` — количество принятых snapshot'ов (Counter)
- `orderbook_densities_active{uid,side}` — количество активных плотностей (Gauge)
- `orderbook_detect_duration_ms_bucket{uid}` — гистограмма времени работы детектора

## 7. Прод-операции

### 7.1 Проверка работоспособности (localhost, с VPS)

```bash
# на srv03-cloud
curl -sS https://gateway.24alert.ru:8080/health
# → 200 ok

# nginx ACL blocks наружный сырой доступ, но snapshot можно проверить локально через gateway-контейнер:
docker exec 24alert-gateway wget -qO- 'http://127.0.0.1:8080/api/v1/stream/orderbook?uids='
# → 400 (uids required) — endpoint жив
```

Снаружи (с traderbook VPS) smoke см. § 7.3.

### 7.2 Логи

```bash
ssh root@srv03-cloud
docker logs -f 24alert-gateway                       # приложение
sudo journalctl -u nginx -n 200                      # TLS/ACL
sudo tail -f /var/log/letsencrypt/letsencrypt.log    # сертификаты
```

Полезные грепы:

```bash
docker logs 24alert-gateway 2>&1 | grep -Ei 'orderbook|stream|subscribe|rpc error'
```

### 7.3 Внешний smoke

С IP, находящегося в ACL (например, `72.56.243.146`):

```bash
# минимальный WebSocket клиент на Python (без зависимостей)
python3 .tmp/wss_smoke.py
# ожидается: handshake 101, затем несколько snapshot-фреймов
```

Скрипт лежит в `.tmp/wss_smoke.py` на обеих VPS (шаблон в `c:\Users\sdk\proj\24alert\.tmp\wss_smoke.py`).

### 7.4 Ротация TLS

Let's Encrypt, DNS-01 challenge через Timeweb Cloud. Автоматический рестарт:

```bash
sudo certbot renew --dry-run                         # dry run
sudo systemctl list-timers | grep certbot            # systemd timer
```

Manual-auth/manual-cleanup hooks (оставлены в `/usr/local/bin/certbot-txt-add.sh` и `certbot-txt-del.sh`) обрабатывают добавление/удаление `TXT _acme-challenge.gateway.24alert.ru` через Timeweb CLI.

## 8. Траблшутинг

| Симптом | Что смотреть |
|---------|--------------|
| Handshake 101 OK, но `snapshot` не идёт | `docker logs 24alert-gateway` — нет `T-Invest token` / expired. Перезалить токен в `.env`. |
| `error: RESOURCE_EXHAUSTED (30042)` | Превышен лимит 300 подписок. Разнести UID по нескольким WS. |
| `error: NotFound (50002)` | Некорректный UID (после ротации инструментов T-Invest). Проверить `instruments`. |
| Handshake 403 | IP клиента не в ACL. Обновить `allow` в nginx-конфиге и reload. |
| Handshake 502/504 | gateway-контейнер не отвечает. `docker ps` + рестарт. |
| `TLS handshake error` снаружи, `curl` таймаут | Проверить, что nginx слушает **8080/tcp** с TLS (Timeweb блокирует 80/443). `ss -ltnp | grep 8080`. |
| Сертификат истёк | `certbot renew`. Если DNS-01 сломан — ручной `certbot certonly --manual` с TXT-хуками. |
| Клиент получает `ping`, но никакого `snapshot` | Рынок закрыт, либо UID без активности. Подтвердить по другому активному UID (SBER). |

## 9. Безопасность

- Единственный путь наружу — TLS 1.2+ через nginx. Сырой TCP к gateway закрыт (bind `127.0.0.1`).
- ACL в nginx — whitelist по IP. Любой новый потребитель добавляется вручную + `nginx -s reload`.
- Токены T-Invest лежат только в `.env` gateway (не коммитятся, права `0600`).
- Логи nginx обезличены (нет заголовков `Authorization`). T-Invest gRPC использует mTLS внутри контейнера.

## 10. Связанные файлы

| Файл | Назначение |
|------|-----------|
| `internal/gateway/handlers/stream.go` | HTTP → WS adapter, JSON-кодирование `StreamOrderBookMsg`/`StreamLevel` |
| `internal/gateway/handlers/stream_orderbook_test.go` | Unit-тесты на контракт JSON + валидация параметров |
| `internal/gateway/services/stream/manager.go` | `SubscribeOrderbook`, мультиплексирование подписок |
| `deployments/docker-compose.yaml` | gateway publish `127.0.0.1:18080:8080` |
| `/etc/nginx/sites-available/gateway.24alert.ru` (на srv03) | TLS + ACL + `proxy_pass` |

См. также: `.tasks/TASK-019/task.md`, `README.md` раздел «OrderBook WebSocket Stream», Traderbook `docs/runbooks/orderbook-density.md`.
