# Регрессионный чек-лист 24alert

> Last revision: 2026-05-16  
> Run after changes touching gateway, nginx, strategy-runner, ai-scanner, market data, order flow, or production config.

## Critical Path

Run on `srv03-cloud` unless stated otherwise.

### Production Health

- [ ] `curl -fsS https://gateway.24alert.ru:8080/health` returns OK.
- [ ] `curl -fsS http://127.0.0.1:18080/health` returns OK on VPS loopback.
- [ ] `bash /opt/24alert/scripts/compose.sh ps` shows expected containers.
- [ ] `bash /opt/24alert/scripts/verify-docker-network.sh` — advisor ↔ strategy-runner HTTP 200 (см. `deployments/DOCKER_NETWORK.md`).
- [ ] `docker logs 24alert-gateway --since 5m` has no panic/OOM/auth loop.
- [ ] `docker logs 24alert-strategy-runner --since 5m` has no panic/OOM/order loop.
- [ ] `docker logs 24alert-ai-scanner --since 1h` has no repeated scanner/runtime failure.

### Public nginx Contract

- [ ] `https://gateway.24alert.ru:8080/health` is public.
- [ ] Public `https://gateway.24alert.ru:8080/api/v1/accounts` returns nginx 404/blocked behavior, not full REST.
- [ ] `/api/v1/stream/orderbook` is accessible only from allowlisted IPs.
- [ ] `https://gateway.24alert.ru:8080/dashboard/` loads strategy dashboard.
- [ ] `https://gateway.24alert.ru:8080/instances` proxies to strategy-runner.

### Gateway REST On VPS

- [ ] `curl -fsS http://127.0.0.1:18080/api/v1/accounts` returns accounts.
- [ ] `curl -fsS 'http://127.0.0.1:18080/api/v1/portfolio?account_id=2001673385'` returns portfolio.
- [ ] `curl -fsS 'http://127.0.0.1:18080/api/v1/candles?instrument_uid=<UID>&interval=15min'` returns candles.
- [ ] `curl -fsS 'http://127.0.0.1:18080/api/v1/trading-status/<UID>'` returns trading status.

### Strategy Runner

- [ ] `curl -fsS http://127.0.0.1:9020/health` returns OK.
- [ ] `curl -fsS http://127.0.0.1:9030/health` returns OK (advisor-svc).
- [ ] `https://gateway.24alert.ru:8080/advisor/health` proxies to advisor-svc (after nginx snippet).
- [ ] `curl -fsS http://127.0.0.1:9020/instances` shows futures instances running.
- [ ] `curl -fsS 'http://127.0.0.1:9020/instances/fut-gas-mini-sma/indicator'` returns indicator data.
- [ ] `curl -fsS 'http://127.0.0.1:9020/instances/fut-mechel-lb/events?limit=20'` returns event timeline.
- [ ] Outside FORTS sessions `10:00–14:00`, `14:05–18:50`, `19:00–23:50 Europe/Moscow`, signals are not sent as live orders.
- [ ] Clearing break `14:00–14:05 Europe/Moscow` is blocked.
- [ ] Warmup-only signals do not appear as live dashboard markers.

### AI Scanner

- [ ] `docker logs 24alert-ai-scanner --since 1h` shows cron/env startup and no repeated failures.
- [ ] Scanner uses `/api/v1/instruments/futures`, not shares.
- [ ] `AI_SCANNER_MAX_CONTRACT_PRICE` is set in container env.
- [ ] Auto changes only affect `auto-fut-*`; manual `fut-*` instances are not changed automatically.

### Port Hardening

- [ ] Service ports are loopback-only: `9001`, `9002`, `9003`, `9004`, `6379`, `9020`, `9120`, `9030`, `9130`.
- [ ] External access goes through nginx `:8080` only.
- [ ] If `monitoring` profile is enabled, `9090` must be loopback-only.

## Local Checks

- [ ] `go test ./...` or targeted package tests for the touched area.
- [ ] `make dashboard-build` if dashboard source changed.
- [ ] `python scripts/ai-scanner/backtest.py --help` and `python scripts/ai-scanner/scan_market.py --help` if scanner changed.

## Reporting

Record results in the task handoff or PR description:

- commit SHA;
- production host;
- commands run;
- pass/fail summary;
- known residual risks.
