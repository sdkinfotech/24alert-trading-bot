# Strategy Lab

Единый набор инструментов для сравнения **всех** типов стратегий 24alert на исторических свечах gateway.

## Стратегии в матрице

| ID | Интервал | Live runner | Примечание |
|----|----------|-------------|------------|
| `sma_crossover` | 1h | Да | Обязателен `trailing_stop_pct > 0`; кнопка «Оптимизировать» в лаборатории подбирает периоды **и** трейлинг (`--optimize-deployable`) |
| `level_bounce` | 15m | Да | SL/TP от ATR |
| `orb_breakout` | 15m | **Нет** | Заблокирован до protective stop |
| `ema_1h` | 1h | Нет | Только research |
| `donchian_15m` | 15m | Нет | Только research |

AI Trader и `grpc` в матрицу не входят.

## Веб-интерфейс (дашборд)

Вкладка **«Лаборатория»** (`/dashboard/#lab`):

1. **Тикер** — BMM6 / NGM6 / MCM6 или поиск  
2. **Анализ** — `POST /strategy-lab/analyze` (полная матрица ~90 дн., 1–4 мин)  
3. **Результаты** — вердикт, сравнение с config.yaml, таблица топ-конфигов  
4. **Запуск** — процедура rollout (см. ниже), без автостарта live из UI

API:

| Метод | Путь | Назначение |
|--------|------|------------|
| GET | `/strategy-lab/catalog` | Справочник стратегий |
| POST | `/strategy-lab/analyze` | **Основной анализ** (матрица + вердикт + rollout plan) |
| POST | `/strategy-lab/compare` | Только матрица (JSON) |
| POST | `/strategy-lab/optimize` | Узкая оптимизация одной семьи (legacy) |
| POST | `/strategy-lab/apply` | `phase=stage` или `phase=enable_live` |
| POST | `/strategy-lab/interpret` | Опционально LLM поверх результатов |

### Процедура выката на прод

См. [`docs/PRODUCTION_TRADING_POLICY.md`](PRODUCTION_TRADING_POLICY.md).

1. **Анализ** — вердикт `deploy_candidate` и осмысленное преимущество над прод (≥2 пт PnL в бэктесте, не только Sharpe).  
2. **Stage** — `POST /strategy-lab/apply` с `"phase":"stage"`: пишет instance в `config.yaml` с **`enabled: false`**, reload runner.  
3. **Git** — commit + push `config/config.yaml` (и код при необходимости).  
4. **VPS** — `git pull`, `docker compose build strategy-runner`, `up -d`.  
5. **Smoke** — `/instances`, portfolio flat, events (команды в `rollout.smoke_commands`).  
6. **Enable live** — только после smoke: `phase=enable_live`, `confirm_live=true`, на runner **`STRATEGY_LAB_ALLOW_LIVE_START=true`**. UI по умолчанию **не** стартует instance.

После таблицы на шаге «Результаты» UI вызывает `POST /strategy-lab/interpret` — LLM (OpenRouter, `STRATEGY_LAB_MODEL` или `ASSISTANT_MODEL`) пишет выводы: что лучше, выкатывать ли на прод, сравнение с текущим продом. Без ключа — шаблон по правилам.

**Nginx:** на `gateway.24alert.ru` нужен `location /strategy-lab` → `:9020` (см. `deployments/nginx-strategy-lab-location.snippet`, `deployments/patch-nginx-strategy-lab.sh`). Без этого вкладка «Лаборатория» покажет `404 Not Found nginx` и список стратегий будет пустым.

**Docker:** в образе strategy-runner нужен полный `python3` (не `python3-minimal`) — иначе бэктест падает с `ModuleNotFoundError: No module named 'json'`. См. `deployments/Dockerfile`.

## Быстрый старт (prod VPS)

```bash
export GATEWAY_URL=http://127.0.0.1:18080
export DAYS=90
bash scripts/backtest/run-strategy-lab.sh
```

Артефакты: `reports/strategy-matrix-YYYYMMDD.json`, `reports/strategy-matrix-ru-YYYYMMDD.md`, `reports/strategy-walkforward-YYYYMMDD.json`.

## Скрипты

| Скрипт | Назначение |
|--------|------------|
| [scripts/backtest/strategy-matrix.py](../scripts/backtest/strategy-matrix.py) | Полная сетка параметров |
| [scripts/backtest/strategy-report.py](../scripts/backtest/strategy-report.py) | RU markdown отчёт |
| [scripts/backtest/strategy-pick-deployable.py](../scripts/backtest/strategy-pick-deployable.py) | Краткий вывод победителей |
| [scripts/backtest/strategy-walk-forward.py](../scripts/backtest/strategy-walk-forward.py) | Train/test split |
| [scripts/backtest/run-strategy-lab.sh](../scripts/backtest/run-strategy-lab.sh) | One-shot pipeline |
| [scripts/ai-scanner/backtest.py](../scripts/ai-scanner/backtest.py) | Одиночный бэктест (docker) |

Библиотека: [scripts/backtestlib/](../scripts/backtestlib/).

## Интерпретация

- **PnL** — пункты цены фьючерса, не рубли GO/комиссии.
- **risk_score** — Sharpe×PF минус штраф за просадку; отсекает &lt;5 сделок и отрицательный PnL.
- **deployable** — можно включить в `config.yaml` без смены типа стратегии в runner.
- **walk-forward** — последние 30 дней out-of-sample; `holds_on_test=false` → возможное переобучение.

## Ограничения

- Не меняет прод-конфиг автоматически.
- Расширенная корзина фьючерсов: см. [research/forts-strategy-lab/](../research/forts-strategy-lab/).
