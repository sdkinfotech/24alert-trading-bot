# AI Scanner (cron + Cursor Agent)

The `ai-scanner` service is **not** started by default: it lives under the Docker Compose **profile** `ai-scanner`.

## Start on the server

From `deployments/` (where `docker-compose.yaml` lives):

```bash
# Build and start ai-scanner with the rest of the stack (if not already up)
docker compose --profile ai-scanner up -d ai-scanner
```

## Requirements

- **`CURSOR_API_KEY`** in `deployments/.env` (must not be empty or `stub`). The container exits scans early if the key is missing (`run-scan.sh`).
- **`agent`** CLI is installed in the image (`deployments/ai-scanner/Dockerfile`).
- **Futures-only selection:** scanner candidates must come from `/api/v1/instruments/futures`; scanner `score` only ranks candidates. The advisor decides after optimized backtests.
- **Contract price limit:** `AI_SCANNER_MAX_CONTRACT_PRICE` defaults to `10000` and is checked before backtesting.
- **Production-aware backtests:** optimized backtests must respect FORTS Mon-Fri sessions `10:00–14:00`, `14:05–18:50`, `19:00–23:50 Europe/Moscow`; Level Bounce evening cutoff may be as late as `23:30`.

## Schedules (defaults, Europe/Moscow in container)

| Variable | Default | Job |
|----------|---------|-----|
| `SCAN_POSTMARKET_CRON` | `0 2 * * 2-6` | Post-market scan (Tue–Sat 02:00) |
| `SCAN_PREMARKET_CRON` | `50 9 * * 1-5` | Pre-market check (Mon–Fri 09:50) |
| `SCAN_HEALTH_CRON` | `0 */4 * * *` | Health every 4 hours |

Override via `.env` if needed.

## Logs

```bash
docker logs -f 24alert-ai-scanner
```

Cron writes job output to the container’s stdout (see `docker-compose.yaml` `>> /proc/1/fd/1`).

## Related

- Orchestration: [../docker-compose.yaml](../docker-compose.yaml) (`ai-scanner` service).
- Prompts: [run-scan.sh](run-scan.sh), [workspace/system-prompt.md](workspace/system-prompt.md).
