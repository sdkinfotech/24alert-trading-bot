# Подбор инструментов для стратегий

Руководство по выбору подходящего инструмента (акции, ETF) при запуске торговой стратегии на `strategy-runner`.

## Критерии отбора

| Критерий | Почему важно | Как проверить |
|----------|-------------|---------------|
| **Ликвидность** | Узкий спред, быстрое исполнение рыночных заявок. В стакане тысячи лотов по bid/ask. | `GET /api/v1/orderbook/{uid}?depth=5` — смотреть `quantity` в bids/asks. |
| **Цена лота** | Лот должен быть доступен на балансе счёта. Для тестов чем дешевле, тем больше «ходов». | `GET /api/v1/prices?instrument_uid={uid}` — `price` это цена **за 1 акцию**; стоимость лота = `price * lot_size`. |
| **Торговый статус** | `api_trade_available: true` — через API можно выставлять заявки. | `GET /api/v1/trading-status/{uid}` |
| **Рыночные заявки** | `market_order_available: true` — стратегия SMA шлёт market-ордера. | Из того же `/trading-status`. |
| **Размер лота** | Для большинства крупных российских акций лот = 1 акция. Но бывают исключения (особенно после сплитов/консолидаций). | `InstrumentByUid` через SDK (runner вызывает при старте). |
| **Волатильность** | SMA crossover генерирует сигналы на трендовых движениях; на «плоском» инструменте сигналов не будет. | Визуально: дневной/часовой график; программно: ATR, стандартное отклонение по свечам. |
| **Совместимость со счётом** | На ИИС маржинальная торговля недоступна; InvestBox не поддерживает стандартный API ордеров. | Документация по типу счёта (см. `Accounts.md`). |

## Пошаговый процесс

### 1. Определить бюджет

Посмотреть баланс счёта:

```bash
ssh adm-srv03-cloud@176.123.160.234 \
  "curl -s 'http://127.0.0.1:18080/api/v1/portfolio?account_id=ID_СЧЁТА'" | jq
```

Пример: ИИС `2001673385` — баланс 2 000 RUB. Значит цена 1 лота должна быть не выше ~300-400 RUB (с запасом на комиссию и несколько итераций).

### 2. Выбрать кандидатов

Ликвидные дешёвые акции MOEX (актуально на май 2026):

| Тикер | instrument_uid | Цена акции (RUB) | Лот | Стоимость лота |
|-------|----------------|-------------------|-----|----------------|
| **ВТБ (VTBR)** | `8e2b0325-0292-4654-8a18-4f63ed3b0e09` | ~122 | 1 | ~122 |
| **Газпром (GAZP)** | `962e2a95-02a9-4171-abd7-aa198dbe643a` | — | 1 | — |
| **Сбер (SBER)** | `e6123145-9665-43e0-8413-cd61b8aa9b13` | ~326 | 1 | ~326 |

Как найти UID нового инструмента:
- В e2e-тестах проекта (`tests/e2e/orders_test.go` — SBER UID зашит как `testInstrumentUID`).
- Через T-Invest SDK: `InstrumentByFigi(figi)` или `FindInstrument(query)`.
- Если gateway запущен, проверить цену: `GET /api/v1/prices?instrument_uid=UUID`.

### 3. Проверить ликвидность

```bash
ssh adm-srv03-cloud@176.123.160.234 \
  "curl -s 'http://127.0.0.1:18080/api/v1/orderbook/UUID?depth=5'"
```

Что искать:
- **bids** и **asks** не пустые.
- **quantity** на лучших уровнях — сотни/тысячи лотов.
- **Спред** (ask - bid) минимальный (для ВТБ/SBER: 1-2 копейки).

Пример ВТБ:
```json
{
  "bids": [{"price": 121.89, "quantity": 1122}, ...],
  "asks": [{"price": 121.90, "quantity": 6}, ...]
}
```
Спред 1 копейка, тысячи лотов — отлично.

### 4. Проверить торговый статус

```bash
ssh adm-srv03-cloud@176.123.160.234 \
  "curl -s 'http://127.0.0.1:18080/api/v1/trading-status/UUID'"
```

Ожидаемый ответ:
```json
{
  "trading_status": "SECURITY_TRADING_STATUS_NORMAL_TRADING",
  "limit_order_available": true,
  "market_order_available": true,
  "api_trade_available": true
}
```

Если `api_trade_available: false` — инструмент сейчас не торгуется (выходные, клиринг, отдельная сессия).

### 5. Рассчитать risk-бюджет

С балансом 2 000 RUB и ВТБ по ~122 RUB/лот:
- **max_position_lots** в конфиге = 10 → максимум 10 лотов = ~1 220 RUB.
- **quantity** = 1 → каждый сигнал покупает/продаёт 1 лот (~122 RUB).
- **max_daily_loss_rub** = 500 → watchdog остановит инстанс при потере 500 RUB за день.

## Почему ВТБ (VTBR) для тестов

1. **Дешевле Сбера в 2.7 раза** → на 2 000 RUB можно совершить больше сделок.
2. **Суперликвидный** — тысячи лотов в стакане, спред 1-2 коп.
3. **Лот = 1 акция** — просто считать.
4. **Волатильность** достаточная для SMA crossover на часовых свечах.
5. **`api_trade_available: true`** — подтверждено.

## Неподходящие инструменты

| Инструмент | Проблема |
|------------|----------|
| ETF из InvestBox | Тип счёта `INVEST_BOX` не поддерживает PostOrder. |
| Инструменты с `market_order_available: false` | SMA crossover шлёт market-ордера. |
| Акции с ценой лота > баланса | Ордер будет отклонён risk-проверкой `balance_check`. |
| Малоликвидные бумаги (3-й эшелон) | Широкий спред → большой slippage → ложный P&L. |
| Фьючерсы / опционы | Другая механика, маржинальные требования. |

## Как добавить новый инструмент в стратегию

### Вариант A: через `config/config.yaml`

```yaml
strategies:
  instances:
    - id: my-new-instance
      type: sma_crossover
      account_id: "2001673385"
      instruments:
        - "NEW_INSTRUMENT_UID"
      enabled: true
      params:
        interval: "1h"
        quantity: "1"
```

### Вариант B: через CLI-флаги (без правки YAML)

```bash
go run ./cmd/strategy-runner \
  --strategy-account "2001673385" \
  --strategy-instrument-uid "NEW_INSTRUMENT_UID" \
  --strategy-interval "1h" \
  --strategy-quantity "1"
```

### Вариант C: через management API (на лету)

```bash
# Остановить текущий
curl -X POST http://127.0.0.1:9020/instances/iis-vtb-sma/stop

# Запустить новый (если в конфиге есть)
curl -X POST http://127.0.0.1:9020/instances/my-new-instance/start
```

## См. также

- [`docs/STRATEGY_RUNNER.md`](STRATEGY_RUNNER.md) — конфигурация, API, watchdog, метрики.
- Obsidian: `Accounts.md` — баланс и типы счетов.
