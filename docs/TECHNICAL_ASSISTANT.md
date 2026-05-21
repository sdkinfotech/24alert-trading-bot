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

### Правила построения уровней

**Опорная цена одна** для всего отчёта и всех графиков: последний close **5m** (fallback 1h → 1d).

Построение **сверху вниз** (старший TF задаёт зоны, младший не дублирует):

1. **Daily (D1)** — год 2H+2L, свинги 90d (wing 3, до 4), ближайший high/low квартала к цене, POC 1d → кластер.
2. **Hourly** — 2H+2L за ~5 сессий, POC 1h → отбрасываются, если уже внутри daily-зоны (~0,25%).
3. **Intraday** — зеркала 1h (до 5) и POC 5m → только если не покрыты старшими зонами.

Список: до **18** уровней (микс «у цены» и «дальние daily»). Касания: **вход в зону** на TF источника (не каждый бар внутри зоны). Сила 1–5 пересчитывается после касаний.

#### Графики (та же опорная цена)

| TF | Макс. | Состав |
|----|-------|--------|
| **1d** | 8 | только `daily_*`, `daily90_*`, `daily_swing_*`, `volume_poc_1d` |
| **1h** | 10 | hourly, POC 1h, зеркала ≤8%; ближайшие daily ≤12% |
| **5m** | 8 | всё в коридоре **≤2%** от опорной цены |

**LLM** — только текст (`summary_md`, `report_md`, сценарии); цены уровней не меняет.

Реализация: `assistant_levels.go`, `assistant_mirror.go`, `assistant_level_rules.go`.

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
