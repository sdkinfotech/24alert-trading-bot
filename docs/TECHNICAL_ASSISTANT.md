# Технический ИИ-ассистент

Read-only анализ инструмента по тикеру: много-TF история, уровни, зеркалки, объёмы, сценарии пробой/отскок. **Не торгует** и не меняет config.

## Dashboard

Вкладка **«Ассистент»** (`http://127.0.0.1:9020/dashboard/#assistant` или `https://gateway.24alert.ru:8080/dashboard/#assistant`):

**Nginx:** на `gateway.24alert.ru` нужен `location /assistant` → `:9020` (см. `deployments/nginx-assistant-location.snippet`, скрипт `deployments/patch-nginx-assistant.sh`). Без этого UI покажет `404 Not Found nginx`.

1. Выберите тикер (каталог MOEX).
2. **Анализировать** — async job 30–90 с.
3. График `1d | 1h | 5m`, панель уровней (accordion), блок сценариев.

## API (strategy-runner :9020)

| Method | Path | Body / query |
|--------|------|----------------|
| POST | `/assistant/analyses` | `{"ticker":"NGM6"}` → `202` `{analysis_id, status, ticker}` |
| GET | `/assistant/analyses/{id}` | полный результат или `running` + `progress_pct` |
| GET | `/assistant/analyses/{id}/chart?tf=1h` | свечи + уровни для TF |
| DELETE | `/assistant/analyses/{id}` | удалить из кэша (TTL 24h) |

### Горизонты данных

| Горизонт | Интервал | Окно |
|----------|----------|------|
| Год | 1d | ~365 дней |
| Квартал | 1d | ~90 дней |
| Месяц | 1h | ~32 дня |
| Неделя | 1h | ~7 дней |
| Час (intraday) | 5m | 7 дней |

### Уровни (детерминированно)

- Daily / hourly highs & lows (top 3)
- Volume POC (5m, 1h)
- **Зеркалки** — swing-уровни с реакциями с обеих сторон
- Сила 1–5, касания, объём в зоне, средняя реакция (bps)

LLM дополняет `report_md` по каждому уровню и 2+ сценария (`bounce`, `breakout`, `range`).

## Переменные окружения

| Env | Default | Описание |
|-----|---------|----------|
| `ASSISTANT_ENABLED` | `true` | `false` — API отклоняет новые анализы |
| `ASSISTANT_MODEL` | `AI_CHAT_MODEL` или Claude Sonnet | OpenRouter model |
| `ASSISTANT_MODEL_FALLBACKS` | — | запасные модели через запятую |
| `OPENROUTER_API_KEY` | — | обязателен для LLM-отчётов (иначе fallback) |

## Пример curl

```bash
curl -s -X POST http://127.0.0.1:9020/assistant/analyses \
  -H 'Content-Type: application/json' \
  -d '{"ticker":"NGM6"}'

curl -s http://127.0.0.1:9020/assistant/analyses/asst-ngm6-1730000000 | jq '.status, .levels | length, .scenarios | length'
```

## Отличие от AI Trader

| | AI Trader | Ассистент |
|--|-----------|-----------|
| Торговля | да (архив) | **нет** |
| Live стакан | да | **нет (v1)** |
| advisor-svc | да | **нет** |
| API | `/ai-trader/*` | `/assistant/*` |
