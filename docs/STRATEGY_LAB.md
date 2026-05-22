# Strategy Lab

Единый набор инструментов для сравнения **всех** типов стратегий 24alert на исторических свечах gateway.

## Стратегии в матрице

| ID | Интервал | Live runner | Примечание |
|----|----------|-------------|------------|
| `sma_crossover` | 1h | Да | Обязателен `trailing_stop_pct > 0` |
| `level_bounce` | 15m | Да | SL/TP от ATR |
| `orb_breakout` | 15m | **Нет** | Заблокирован до protective stop |
| `ema_1h` | 1h | Нет | Только research |
| `donchian_15m` | 15m | Нет | Только research |

AI Trader и `grpc` в матрицу не входят.

## Веб-интерфейс (дашборд)

Вкладка **«Лаборатория»** (`/dashboard/#lab`):

1. **Тикер** — поиск фьючерса или быстрые кнопки BMM6 / NGM6 / MCM6  
2. **Стратегия** — SMA, Level Bounce, ORB, EMA, Donchian (пометка live / research)  
3. **Оптимизация** — «Оптимизировать выбранную» или «Сравнить все стратегии» (1–4 мин)  
4. **Результаты** — таблица PnL / Sharpe / просадка; клик по строке  
5. **Запуск** — запись в `config.yaml`, reload, старт инстанса (только `live_eligible`)

API: `GET /strategy-lab/catalog`, `POST /strategy-lab/compare`, `POST /strategy-lab/optimize`, `POST /strategy-lab/apply`.

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
