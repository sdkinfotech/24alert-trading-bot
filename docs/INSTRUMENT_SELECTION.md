# Подбор инструментов для стратегий

Канон по операционным правилам и текущим решениям хранится в Obsidian: `24alert/Strategies.md` и `24alert/Strategy Optimization Methodology.md`.

Текущий режим 24alert: **только фьючерсы MOEX FORTS**. Shares/equities больше не являются кандидатами для боевых стратегий.

## Обязательные фильтры

| Критерий | Правило |
|----------|---------|
| Тип инструмента | `class_code = SPBFUT`, `instrument_type = future` |
| Contract price | `last_price * lot <= AI_SCANNER_MAX_CONTRACT_PRICE` |
| Данные | есть свечи для scan и backtest |
| API trading | инструмент торгуется и доступен через API |
| Экспирация | использовать актуальный текущий контракт, не дальний/истекающий без решения |

## Текущая корзина

| Фьючерс | Тикер | UID | Стратегия |
|---------|-------|-----|-----------|
| Brent mini | `BMM6` | `dc1ffa30-70a4-4a7b-807a-4f31c2951f7e` | `sma_crossover 1h fast=4 slow=9 trailing_stop_pct=0.005` |
| Natural Gas mini | `NGM6` | `117a1408-431f-4ba0-a041-5bba3123d4a8` | `sma_crossover 1h fast=5 slow=17` |
| Mechel futures | `MCM6` | `6f4563c0-e853-46f2-98c7-3abce3cc7517` | `sma_crossover 1h fast=4 slow=9` |

`fut-brent-mini-lb` and `fut-mechel-lb` keep legacy IDs for continuity; do not infer strategy type from the ID suffix.

UID фьючерсов меняются при ролловере. Проверка актуальных контрактов:

```bash
bash scripts/resolve-futures.sh
```

## Как отбирает AI Scanner

Scanner:

1. Берёт инструменты из `/api/v1/instruments/futures`.
2. Получает last prices bulk.
3. Считает `contract_price`, `atr_pct`, объёмы, S/R уровни и `score`.
4. Использует `score` только для ранжирования очереди.
5. Для каждого кандидата запускает backtest `sma` и `level_bounce` с `--optimize`.
6. Решает по backtest thresholds: `Sharpe > 1`, `PnL > 0`, `win_rate > 45%`, `trades >= 5`, `profit_factor > 1.3`.

Backtest обязан учитывать production FORTS schedule guard: только Mon-Fri, `10:00–14:00`, `14:05–18:50`, `19:00–23:50 Europe/Moscow`. Для `level_bounce` вечерний cutoff допускается до `23:30`.

Пример ручной проверки на VPS:

```bash
docker exec 24alert-ai-scanner python3 /opt/ai-scanner/scan_market.py \
  --gateway-url http://gateway:8080 \
  --top-n 10 \
  --max-contract-price ${AI_SCANNER_MAX_CONTRACT_PRICE:-10000} \
  --json
```

## Проверка конкретного фьючерса

```bash
ssh adm-srv03-cloud@176.123.160.234

# статус runner
curl -s http://127.0.0.1:9020/instances

# свечи через gateway loopback
curl -s 'http://127.0.0.1:18080/api/v1/candles?instrument_uid=<UID>&interval=15min'

# indicator/dashboard data
curl -s 'http://127.0.0.1:9020/instances/<INSTANCE_ID>/indicator'
```

## Риски фьючерсов

- PnL в текущем backtest/ledger приблизительный до точного учёта стоимости шага цены.
- `EstimatedCost` для `SPBFUT` сейчас `0`, чтобы balance check не блокировал маржинальные инструменты; ГО нужно контролировать отдельно.
- На ИИС `2001673385` API margin может возвращать `30051`, поэтому увеличивать `quantity` без отдельной проверки нельзя.
- Нужен регулярный ролловер UID на новый контракт.

## Management API

```bash
# остановить / запустить существующий instance
curl -X POST http://127.0.0.1:9020/instances/fut-mechel-lb/stop
curl -X POST http://127.0.0.1:9020/instances/fut-mechel-lb/start
```

Ручные инстансы `fut-*` не меняются советником автоматически. Auto-инстансы должны называться `auto-fut-<ticker>-<strategy>`.
