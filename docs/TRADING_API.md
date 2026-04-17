# 24Alert Trading API — Полная документация

**Base URL:** `http://<server>:8080`
**Swagger UI:** `GET /swagger/index.html`
**Health:** `GET /health` → `{"status":"ok"}`

---

## Содержание

1. [Аккаунты](#1-аккаунты)
2. [Ордера (акции, фьючерсы)](#2-ордера)
   - [Market (рыночный)](#21-market-order)
   - [Limit (лимитный)](#22-limit-order)
   - [Bestprice (лучшая цена)](#23-bestprice-order)
   - [Изменение ордера (Replace)](#24-replace-order)
   - [Отмена ордера](#25-cancel-order)
   - [Список активных ордеров](#26-list-orders)
   - [Статус ордера](#27-order-state)
3. [Стоп-ордера](#3-стоп-ордера)
   - [Stop-Loss](#31-stop-loss)
   - [Take-Profit](#32-take-profit)
   - [Stop-Limit](#33-stop-limit)
   - [Список стоп-ордеров](#34-list-stop-orders)
   - [Отмена стоп-ордера](#35-cancel-stop-order)
4. [Особенности по рынкам](#4-особенности-по-рынкам)
   - [Акции (TQBR)](#41-акции-tqbr)
   - [Фьючерсы (FORTS)](#42-фьючерсы-forts)
   - [Опционы (SPBOPT)](#43-опционы-spbopt)
5. [Маркетдата](#5-маркетдата)
   - [Свечи](#51-свечи)
   - [Стакан (Orderbook)](#52-стакан)
   - [Последняя цена](#53-последняя-цена)
   - [Статус торгов](#54-статус-торгов)
6. [Портфель и позиции](#6-портфель-и-позиции)
7. [Риск-менеджмент](#7-риск-менеджмент)
8. [Инструменты (UID)](#8-инструменты)
9. [Коды ошибок T-Invest](#9-коды-ошибок)
10. [Интеграция с Traderbook](#10-интеграция-с-traderbook)

---

## 1. Аккаунты

### Получить список счетов

```
GET /api/v1/accounts
```

**Ответ:**

```json
{
  "data": [
    {
      "id": "2004091456",
      "type": "ACCOUNT_TYPE_TINKOFF",
      "name": "Основной",
      "status": "ACCOUNT_STATUS_OPEN",
      "access_level": "ACCOUNT_ACCESS_LEVEL_FULL_ACCESS"
    }
  ]
}
```

Типы счетов: `ACCOUNT_TYPE_TINKOFF`, `ACCOUNT_TYPE_TINKOFF_IIS`, `ACCOUNT_TYPE_INVEST_BOX`.

### Маржинальные показатели

```
GET /api/v1/margin/{account_id}
```

**Ответ:**

```json
{
  "data": {
    "liquid_portfolio": 11069.15,
    "starting_margin": 4286.00,
    "minimal_margin": 2143.00,
    "funds_sufficiency_level": 2.58
  }
}
```

| Поле | Описание |
|------|----------|
| `liquid_portfolio` | Ликвидная стоимость портфеля |
| `starting_margin` | Начальная маржа (для открытия позиции) |
| `minimal_margin` | Минимальная маржа (для удержания позиции) |
| `funds_sufficiency_level` | Коэффициент достаточности средств (>1 = ОК) |

---

## 2. Ордера

### Общий формат запроса

```
POST /api/v1/orders
Content-Type: application/json

{
  "account_id":     "2004091456",
  "instrument_uid": "e6123145-9665-43e0-8413-cd61b8aa9b13",
  "quantity":       1,
  "direction":      "buy",
  "order_type":     "market",
  "price":          0
}
```

| Поле | Тип | Описание |
|------|-----|----------|
| `account_id` | string | **Обязательно.** ID счёта |
| `instrument_uid` | string | **Обязательно.** UID инструмента |
| `quantity` | int | Количество лотов |
| `direction` | string | `"buy"` или `"sell"` |
| `order_type` | string | `"market"`, `"limit"`, `"bestprice"` |
| `price` | float | Цена (для limit). Для market/bestprice — 0 или не передавать |

**Ответ (201 Created):**

```json
{
  "data": {
    "order_id": "77725574678",
    "execution_status": "EXECUTION_REPORT_STATUS_FILL",
    "lots_requested": 1,
    "lots_executed": 1,
    "total_price": 314.53,
    "direction": "ORDER_DIRECTION_BUY",
    "order_type": "ORDER_TYPE_MARKET"
  }
}
```

Статусы исполнения:

| Статус | Описание |
|--------|----------|
| `EXECUTION_REPORT_STATUS_FILL` | Полностью исполнен |
| `EXECUTION_REPORT_STATUS_NEW` | Размещён, ждёт исполнения |
| `EXECUTION_REPORT_STATUS_PARTIALLYFILL` | Частично исполнен |
| `EXECUTION_REPORT_STATUS_CANCELLED` | Отменён |
| `EXECUTION_REPORT_STATUS_REJECTED` | Отклонён |

---

### 2.1 Market Order

Покупка/продажа по текущей рыночной цене. Исполняется мгновенно.

```bash
curl -X POST http://server:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": "2004091456",
    "instrument_uid": "e6123145-9665-43e0-8413-cd61b8aa9b13",
    "quantity": 1,
    "direction": "buy",
    "order_type": "market"
  }'
```

**Поддержка по рынкам:**

| Рынок | Market order |
|-------|-------------|
| Акции (TQBR) | Да |
| Фьючерсы (FORTS) | Да |
| Опционы (SPBOPT) | **Нет** (ошибка 30094) |

---

### 2.2 Limit Order

Заявка с указанной ценой. Исполняется когда рынок достигнет указанной цены.

```bash
curl -X POST http://server:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": "2004091456",
    "instrument_uid": "e6123145-9665-43e0-8413-cd61b8aa9b13",
    "quantity": 1,
    "direction": "buy",
    "order_type": "limit",
    "price": 298.83
  }'
```

**Важно:**

- Цена **обязана быть кратна шагу цены** инструмента (для акций SBER = 0.01 руб)
- На MOEX нельзя ставить лимитку дальше ~5-10% от текущей рыночной цены (ошибка 30099)
- Лимитный ордер далеко от рынка → статус `NEW`, близко к рынку → может сразу `FILL`

**Поддержка по рынкам:**

| Рынок | Limit order |
|-------|-------------|
| Акции (TQBR) | Да |
| Фьючерсы (FORTS) | Да |
| Опционы (SPBOPT) | **Да** (единственный способ торговли опционами) |

---

### 2.3 Bestprice Order

Заявка по лучшей доступной цене. Отличается от market тем, что биржа исполняет по лучшей цене из стакана, а не по любой.

```bash
curl -X POST http://server:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": "2004091456",
    "instrument_uid": "e6123145-9665-43e0-8413-cd61b8aa9b13",
    "quantity": 1,
    "direction": "buy",
    "order_type": "bestprice"
  }'
```

---

### 2.4 Replace Order

Изменение цены и/или количества лотов активного **лимитного** ордера. Биржа отменяет старый и создаёт новый.

```bash
curl -X PUT "http://server:8080/api/v1/orders/77726105277?account_id=2004091456" \
  -H "Content-Type: application/json" \
  -d '{
    "quantity": 2,
    "price": 305.12
  }'
```

**Ответ:** новый `OrderResult` с новым `order_id`.

**Ограничения:**

- Работает только для ордеров в статусе `NEW` (не исполненных)
- Новая цена должна быть кратна шагу цены
- Если старый ордер уже исполнен → ошибка 30059

---

### 2.5 Cancel Order

Отмена активного ордера.

```bash
curl -X DELETE "http://server:8080/api/v1/orders/77726105277?account_id=2004091456"
```

**Ответ:**

```json
{
  "data": {
    "cancelled_at": "2026-04-03T14:05:12Z"
  }
}
```

Если ордер уже исполнен → ошибка 30059.
Если ордер не найден → ошибка 50005.

---

### 2.6 List Orders

Список **активных** (неисполненных) ордеров по счёту.

```bash
curl "http://server:8080/api/v1/orders?account_id=2004091456"
```

**Ответ:**

```json
{
  "data": [
    {
      "order_id": "77726105277",
      "instrument_uid": "e6123145-9665-43e0-8413-cd61b8aa9b13",
      "direction": "ORDER_DIRECTION_BUY",
      "order_type": "ORDER_TYPE_LIMIT",
      "lots": 1,
      "price": 298.83,
      "status": "EXECUTION_REPORT_STATUS_NEW"
    }
  ]
}
```

Исполненные/отменённые ордера **не** возвращаются. Для истории — `/api/v1/operations`.

---

### 2.7 Order State

Состояние конкретного ордера (включая уже исполненные, но не старше ~1 дня).

```bash
curl "http://server:8080/api/v1/orders/77726105277?account_id=2004091456"
```

---

## 3. Стоп-ордера

Условные заявки, которые хранятся на сервере T-Invest и выставляются на биржу при достижении указанной `stop_price`.

### Общий формат запроса

```
POST /api/v1/stop-orders
Content-Type: application/json

{
  "account_id":      "2004091456",
  "instrument_uid":  "e6123145-9665-43e0-8413-cd61b8aa9b13",
  "quantity":        1,
  "direction":       "sell",
  "stop_order_type": "stop_loss",
  "stop_price":      308.24,
  "price":           0
}
```

| Поле | Тип | Описание |
|------|-----|----------|
| `account_id` | string | **Обязательно** |
| `instrument_uid` | string | **Обязательно** |
| `quantity` | int | Количество лотов |
| `direction` | string | `"buy"` или `"sell"` |
| `stop_order_type` | string | `"stop_loss"`, `"take_profit"`, `"stop_limit"` |
| `stop_price` | float | Цена-триггер (при достижении → выставляется биржевая заявка) |
| `price` | float | Цена биржевой заявки (для stop_limit). Для stop_loss/take_profit = 0 |

**Ответ (201 Created):**

```json
{
  "data": {
    "stop_order_id": "f5779835-78e0-4d66-8ef5-0289f213e320"
  }
}
```

---

### 3.1 Stop-Loss

Защитный ордер: если цена падает до `stop_price` — продаёт по рынку.

**Сценарий:** Купил SBER по 314.50, ставлю стоп-лосс на 308.24 (≈ -2%).

```json
{
  "account_id": "2004091456",
  "instrument_uid": "e6123145-9665-43e0-8413-cd61b8aa9b13",
  "quantity": 1,
  "direction": "sell",
  "stop_order_type": "stop_loss",
  "stop_price": 308.24
}
```

**Как работает:**
1. Заявка хранится на сервере T-Invest
2. Когда рыночная цена SBER ≤ 308.24 → сервер выставляет рыночную заявку на продажу
3. Тип дочерней заявки: `EXCHANGE_ORDER_TYPE_MARKET`
4. Экспирация: `GOOD_TILL_CANCEL` (до отмены)

**Статус на бирже:** Работает для акций. Для опционов и некоторых фьючерсов может быть ошибка 30053.

---

### 3.2 Take-Profit

Ордер на фиксацию прибыли: если цена растёт до `stop_price` — продаёт по рынку.

**Сценарий:** Купил SBER по 314.50, ставлю тейк-профит на 320.81 (≈ +2%).

```json
{
  "account_id": "2004091456",
  "instrument_uid": "e6123145-9665-43e0-8413-cd61b8aa9b13",
  "quantity": 1,
  "direction": "sell",
  "stop_order_type": "take_profit",
  "stop_price": 320.81
}
```

**Как работает:**
1. Когда рыночная цена SBER ≥ 320.81 → сервер выставляет рыночную заявку на продажу
2. Тип дочерней заявки: `EXCHANGE_ORDER_TYPE_MARKET`
3. `TakeProfitType`: `TAKE_PROFIT_TYPE_REGULAR`

---

### 3.3 Stop-Limit

Условная заявка: при достижении `stop_price` выставляется **лимитная** заявка по `price`.

**Сценарий:** Если SBER упадёт до 305.12, продать по лимитной цене 301.98.

```json
{
  "account_id": "2004091456",
  "instrument_uid": "e6123145-9665-43e0-8413-cd61b8aa9b13",
  "quantity": 1,
  "direction": "sell",
  "stop_order_type": "stop_limit",
  "stop_price": 305.12,
  "price": 301.98
}
```

**Как работает:**
1. Когда цена ≤ 305.12 → выставляется лимитная заявка sell по 301.98
2. Тип дочерней заявки: `EXCHANGE_ORDER_TYPE_LIMIT`
3. Если цена уйдёт ниже 301.98 до исполнения — лимитная заявка может не исполниться

**Статус:** Может возвращать ошибку 30053 для некоторых инструментов.

---

### 3.4 List Stop Orders

```bash
curl "http://server:8080/api/v1/stop-orders?account_id=2004091456"
```

**Ответ:**

```json
{
  "data": [
    {
      "stop_order_id": "f5779835-78e0-4d66-8ef5-0289f213e320",
      "instrument_uid": "e6123145-9665-43e0-8413-cd61b8aa9b13",
      "direction": "STOP_ORDER_DIRECTION_SELL",
      "stop_order_type": "STOP_ORDER_TYPE_STOP_LOSS",
      "lots": 1,
      "stop_price": 308.24,
      "price": 0,
      "status": "STOP_ORDER_STATUS_ACTIVE"
    }
  ]
}
```

---

### 3.5 Cancel Stop Order

```bash
curl -X DELETE "http://server:8080/api/v1/stop-orders/f5779835-78e0-4d66-8ef5-0289f213e320?account_id=2004091456"
```

---

## 4. Особенности по рынкам

### 4.1 Акции (TQBR)

| Параметр | Значение |
|----------|----------|
| Секция | TQBR (Московская биржа) |
| Шаг цены | 0.01 руб |
| Торговые часы | 10:00 – 18:39:59 MSK (основная), 19:05 – 23:49:59 (вечерняя) |
| Типы ордеров | market, limit, bestprice |
| Стоп-ордера | stop_loss ✓, take_profit ✓, stop_limit (зависит от инструмента) |
| Замена ордера | ✓ (только limit в статусе NEW) |

**Пример: полный цикл торговли акциями**

```bash
# 1. Узнать цену
curl "http://server:8080/api/v1/prices?instrument_uid=e6123145-9665-43e0-8413-cd61b8aa9b13"

# 2. Купить 1 лот SBER по рынку
curl -X POST http://server:8080/api/v1/orders -H "Content-Type: application/json" \
  -d '{"account_id":"2004091456","instrument_uid":"e6123145-9665-43e0-8413-cd61b8aa9b13","quantity":1,"direction":"buy","order_type":"market"}'

# 3. Поставить стоп-лосс
curl -X POST http://server:8080/api/v1/stop-orders -H "Content-Type: application/json" \
  -d '{"account_id":"2004091456","instrument_uid":"e6123145-9665-43e0-8413-cd61b8aa9b13","quantity":1,"direction":"sell","stop_order_type":"stop_loss","stop_price":308.24}'

# 4. Поставить тейк-профит
curl -X POST http://server:8080/api/v1/stop-orders -H "Content-Type: application/json" \
  -d '{"account_id":"2004091456","instrument_uid":"e6123145-9665-43e0-8413-cd61b8aa9b13","quantity":1,"direction":"sell","stop_order_type":"take_profit","stop_price":320.81}'

# 5. Проверить позицию
curl "http://server:8080/api/v1/positions?account_id=2004091456"

# 6. Закрыть позицию (продать)
curl -X POST http://server:8080/api/v1/orders -H "Content-Type: application/json" \
  -d '{"account_id":"2004091456","instrument_uid":"e6123145-9665-43e0-8413-cd61b8aa9b13","quantity":1,"direction":"sell","order_type":"market"}'
```

---

### 4.2 Фьючерсы (FORTS)

| Параметр | Значение |
|----------|----------|
| Секция | FORTS (срочный рынок MOEX) |
| Маржа | Требуется. Проверяйте `/api/v1/margin/{account_id}` |
| Торговые часы | 10:00 – 18:50 MSK (основная), 19:05 – 23:50 (вечерняя) |
| Типы ордеров | market ✓, limit ✓, bestprice ✓ |
| Стоп-ордера | stop_loss ✓, take_profit ✓ |
| Расчёт | Вариационная маржа |

**Важно:**
- Для фьючерсов нужна **достаточная маржа** на счёте (ошибка 30042 при нехватке)
- Цена фьючерса — в пунктах, стоимость пункта зависит от инструмента
- Позиция может быть длинной (buy) или короткой (sell)

**Пример: торговля фьючерсами**

```bash
# 1. Проверить маржу
curl "http://server:8080/api/v1/margin/2004091456"

# 2. Купить 1 контракт KZU6 (Казахский тенге)
curl -X POST http://server:8080/api/v1/orders -H "Content-Type: application/json" \
  -d '{"account_id":"2004091456","instrument_uid":"3da86f70-25a7-4092-bc1b-05cf84dad9fe","quantity":1,"direction":"buy","order_type":"market"}'

# 3. Проверить позицию и маржу
curl "http://server:8080/api/v1/positions?account_id=2004091456"
curl "http://server:8080/api/v1/margin/2004091456"

# 4. Закрыть позицию
curl -X POST http://server:8080/api/v1/orders -H "Content-Type: application/json" \
  -d '{"account_id":"2004091456","instrument_uid":"3da86f70-25a7-4092-bc1b-05cf84dad9fe","quantity":1,"direction":"sell","order_type":"market"}'
```

---

### 4.3 Опционы (SPBOPT)

| Параметр | Значение |
|----------|----------|
| Секция | SPBOPT (опционы на FORTS) |
| Маржа | Требуется для продавцов; для покупателей — уплата премии |
| Типы ордеров | **Только limit** (market → ошибка 30094) |
| Стоп-ордера | Не тестировались |
| Ликвидность | Сильно зависит от инструмента. Используйте стакан! |

**Ключевые отличия опционов:**

1. **Только лимитные ордера** — рыночные запрещены на FORTS для опционов (ошибка 30094: "Options trading is not available at the moment")
2. **Используйте стакан** (`/api/v1/orderbook/{uid}`) для определения цены — `lastPrice` часто = 0
3. **Покупка**: ставьте limit по цене ask (мгновенное исполнение)
4. **Продажа**: ставьте limit по цене bid (мгновенное исполнение)
5. **Ликвидность**: торгуйте опционами с маркетмейкером (bid/ask по 50+ контрактов)

**Выбор ликвидного опциона:**
- Наиболее ликвидные: опционы на золото (GLDRUB_TOM), нефть, Si
- Выбирайте страйк близко к текущей цене базового актива (ATM)
- Ближайшая дата экспирации = больше ликвидности

**Пример: торговля опционами**

```bash
# 1. Проверить стакан опциона (НЕ lastPrice!)
curl "http://server:8080/api/v1/orderbook/66f3eea6-35f2-4347-8f1d-8f082588801c?depth=5"
# Ответ: {"bids":[{"price":236.9,"quantity":50}],"asks":[{"price":284.2,"quantity":50}]}

# 2. Купить 1 опцион CALL по цене ask (лимитный ордер!)
curl -X POST http://server:8080/api/v1/orders -H "Content-Type: application/json" \
  -d '{"account_id":"2004091456","instrument_uid":"66f3eea6-35f2-4347-8f1d-8f082588801c","quantity":1,"direction":"buy","order_type":"limit","price":284.20}'

# 3. Проверить позицию
curl "http://server:8080/api/v1/positions?account_id=2004091456"

# 4. Продать по bid
curl -X POST http://server:8080/api/v1/orders -H "Content-Type: application/json" \
  -d '{"account_id":"2004091456","instrument_uid":"66f3eea6-35f2-4347-8f1d-8f082588801c","quantity":1,"direction":"sell","order_type":"limit","price":236.90}'
```

---

## 5. Маркетдата

### 5.1 Свечи

```
GET /api/v1/candles?instrument_uid={uid}&from={RFC3339}&to={RFC3339}&interval={interval}
```

| Параметр | Обязательный | По умолчанию | Описание |
|----------|-------------|-------------|----------|
| `instrument_uid` | Да | — | UID инструмента |
| `from` | Нет | `to - 24h` | Начало периода (RFC3339) |
| `to` | Нет | `now` | Конец периода |
| `interval` | Нет | `1h` | Интервал свечей |

**Доступные интервалы:**

| Значение | Описание | Макс. период запроса |
|----------|----------|---------------------|
| `1min` | 1 минута | 1 день |
| `5min` | 5 минут | 1 день |
| `15min` | 15 минут | 1 день |
| `1h` | 1 час | 7 дней |
| `day` | 1 день | 1 год |
| `week` | 1 неделя | 2 года |
| `month` | 1 месяц | 10 лет |

**Пример:**

```bash
# Минутные свечи SBER за последний час
curl "http://server:8080/api/v1/candles?instrument_uid=e6123145-9665-43e0-8413-cd61b8aa9b13&from=2026-04-03T13:00:00Z&to=2026-04-03T14:00:00Z&interval=1min"
```

**Ответ:**

```json
{
  "data": [
    {
      "open": 314.52,
      "high": 314.59,
      "low": 314.50,
      "close": 314.54,
      "volume": 15234,
      "is_complete": true
    }
  ]
}
```

---

### 5.2 Стакан

```
GET /api/v1/orderbook/{instrument_uid}?depth={5|10|20}
```

| Параметр | По умолчанию | Описание |
|----------|-------------|----------|
| `depth` | 20 | Глубина стакана (количество уровней) |

**Ответ:**

```json
{
  "data": {
    "instrument_uid": "e6123145-9665-43e0-8413-cd61b8aa9b13",
    "depth": 5,
    "bids": [
      {"price": 314.52, "quantity": 19317},
      {"price": 314.51, "quantity": 8432}
    ],
    "asks": [
      {"price": 314.53, "quantity": 2930},
      {"price": 314.54, "quantity": 5612}
    ],
    "last_price": 314.53,
    "close_price": 313.80
  }
}
```

- `bids` — заявки на покупку (от лучшей к худшей)
- `asks` — заявки на продажу (от лучшей к худшей)
- **Для опционов:** стакан — единственный надёжный источник цены

---

### 5.3 Последняя цена

```
GET /api/v1/prices?instrument_uid={uid}
```

**Ответ:**

```json
{
  "data": [
    {
      "instrument_uid": "e6123145-9665-43e0-8413-cd61b8aa9b13",
      "price": 314.53
    }
  ]
}
```

**Ограничение:** Для опционов `price` часто = 0 (нет последней сделки). Используйте стакан.

---

### 5.4 Статус торгов

```
GET /api/v1/trading-status/{instrument_uid}
```

**Ответ:**

```json
{
  "data": {
    "instrument_uid": "e6123145-9665-43e0-8413-cd61b8aa9b13",
    "trading_status": "SECURITY_TRADING_STATUS_NORMAL_TRADING",
    "limit_order_available": true,
    "market_order_available": true,
    "api_trade_available": true
  }
}
```

| Статус | Описание |
|--------|----------|
| `NORMAL_TRADING` | Нормальные торги |
| `NOT_AVAILABLE_FOR_TRADING` | Торги закрыты |
| `OPENING_PERIOD` | Период открытия |
| `CLOSING_PERIOD` | Период закрытия |
| `BREAK_IN_TRADING` | Перерыв в торгах |
| `DEALER_NORMAL_TRADING` | Дилерские торги |

---

## 6. Портфель и позиции

### Позиции

```
GET /api/v1/positions?account_id={id}
```

Возвращает текущие позиции: акции, фьючерсы, опционы, валюту.

```json
{
  "data": [
    {
      "instrument_uid": "e6123145-9665-43e0-8413-cd61b8aa9b13",
      "instrument_type": "share",
      "figi": "BBG004730N88",
      "quantity": 1,
      "average_price": 314.53,
      "expected_yield": -0.01,
      "current_price": 314.52,
      "currency": "rub",
      "blocked": false
    }
  ]
}
```

### Портфель

```
GET /api/v1/portfolio?account_id={id}
```

Сводная информация по портфелю: суммы по классам активов.

### Лимиты на вывод

```
GET /api/v1/limits?account_id={id}
```

### История операций

```
GET /api/v1/operations?account_id={id}&instrument_uid={uid}&from={RFC3339}&to={RFC3339}
```

| Параметр | Обязательный | Описание |
|----------|-------------|----------|
| `account_id` | Да | ID счёта |
| `instrument_uid` | Нет | Фильтр по инструменту |
| `from` | Нет | Начало (default: -24h) |
| `to` | Нет | Конец (default: now) |

---

## 7. Риск-менеджмент

### Статус circuit breaker

```
GET /api/v1/risk/status
```

```json
{
  "data": {
    "circuit_breaker_tripped": false,
    "failure_count": 0,
    "threshold": 5
  }
}
```

Если `circuit_breaker_tripped: true` — все ордера блокируются.

### Сброс circuit breaker

```
POST /api/v1/risk/reset
```

---

## 8. Инструменты

### Протестированные инструменты (production)

| Тикер | UID | Тип | Секция | Шаг цены |
|-------|-----|-----|--------|----------|
| SBER | `e6123145-9665-43e0-8413-cd61b8aa9b13` | Акция | TQBR | 0.01 |
| KZU6 | `3da86f70-25a7-4092-bc1b-05cf84dad9fe` | Фьючерс | FORTS | 1.0 |
| GL11700CD6B | `66f3eea6-35f2-4347-8f1d-8f082588801c` | Опцион (CALL) | SPBOPT | 0.01 |

### Как найти UID инструмента

UID можно получить через T-Invest REST API напрямую:

```bash
# По тикеру
curl -X POST "https://invest-public-api.tinkoff.ru/rest/tinkoff.public.invest.api.contract.v1.InstrumentsService/GetInstrumentBy" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"idType":"INSTRUMENT_ID_TYPE_TICKER","classCode":"TQBR","id":"SBER"}'
```

---

## 9. Коды ошибок T-Invest

| Код | Описание | Что делать |
|-----|----------|------------|
| 30034 | InvalidArgument | Недостаточно средств / невалидный инструмент / нет прав |
| 30042 | Insufficient margin | Недостаточно маржи для фьючерсов — пополнить счёт |
| 30043 | InvalidArgument (stop) | Стоп-ордер не поддерживается для инструмента |
| 30049 | Price step violation | Цена не кратна шагу цены инструмента — округлить |
| 30053 | Stop order rejected | Стоп-ордер отклонён (тип не поддерживается для инструмента) |
| 30059 | Cancel/Replace failed | Ордер уже исполнен или не может быть отменён |
| 30094 | Options market order | Рыночные ордера запрещены для опционов — используйте limit |
| 30099 | Price out of range | Цена лимитного ордера слишком далеко от рынка (>5-10%) |
| 50005 | Order not found | Ордер не найден |
| 50006 | Stop order not found | Стоп-ордер не найден |
| 70001 | Internal error | Внутренняя ошибка T-Invest — повторить позже |

---

## Сводная таблица: что работает на каждом рынке

| Операция | Акции (TQBR) | Фьючерсы (FORTS) | Опционы (SPBOPT) |
|----------|:---:|:---:|:---:|
| Market order | ✅ | ✅ | ❌ (30094) |
| Limit order | ✅ | ✅ | ✅ |
| Bestprice order | ✅ | ✅ | не тестировалось |
| Replace order | ✅ | ✅ | не тестировалось |
| Cancel order | ✅ | ✅ | ✅ |
| Stop-Loss | ✅ | ✅ | не тестировалось |
| Take-Profit | ✅ | ✅ | не тестировалось |
| Stop-Limit | ⚠️ (30053) | не тестировалось | не тестировалось |
| Свечи | ✅ | ✅ | ✅ |
| Стакан | ✅ | ✅ | ✅ |
| Последняя цена | ✅ | ✅ | ⚠️ (часто 0) |
| Статус торгов | ✅ | ✅ | ✅ |
| Позиции | ✅ | ✅ | ✅ |
| Маржа | ✅ | ✅ | ✅ |

---

## 10. Интеграция с Traderbook

24Alert выступает **провайдером рыночных данных** для платформы [Traderbook](https://traderbook.ru) — монорепозитория на Next.js + Fastify + Prisma + PostgreSQL.

### Архитектура связи

```
┌─────────────────────────────────────────────────────────┐
│  Traderbook (TypeScript monorepo)                       │
│                                                         │
│  services/market-data (Fastify, порт 3021)              │
│    ├── gateway-client.ts  → HTTP-клиент к 24Alert       │
│    ├── sync.ts            → upsert в PostgreSQL         │
│    ├── cron.ts            → расписание синхронизации     │
│    └── server.ts          → API: heatmap, screener      │
│                                                         │
│  packages/db (Prisma)                                   │
│    └── instruments, instrument_prices                   │
│                                                         │
│  apps/web (Next.js)                                     │
│    └── потребляет /api/market/* для UI                  │
└────────────────┬────────────────────────────────────────┘
                 │ HTTP (env: ALERT_GATEWAY_URL)
                 ▼
┌─────────────────────────────────────────────────────────┐
│  24Alert Gateway (Go, порт 8080)                        │
│                                                         │
│  Endpoint'ы для интеграции:                             │
│    GET /api/v1/instruments/shares                       │
│    GET /api/v1/instruments/futures                      │
│    GET /api/v1/prices/bulk?uids=...                     │
│    GET /api/v1/prices/close?uids=...                    │
│                                                         │
│  internal/gateway/handlers/instruments.go               │
│  internal/gateway/adapter/instruments_adapter.go        │
└────────────────┬────────────────────────────────────────┘
                 │ gRPC
                 ▼
         T-Invest API (Тинькофф)
```

### Endpoint'ы интеграции

#### GET /api/v1/instruments/shares

Все торгуемые акции из T-Invest (статус `INSTRUMENT_STATUS_BASE`, доступные через API).

```bash
curl http://server:8080/api/v1/instruments/shares
```

Ответ:

```json
{
  "data": [
    {
      "uid": "e6123145-9665-43e0-8413-cd61b8aa9b13",
      "figi": "BBG004730N88",
      "ticker": "SBER",
      "class_code": "TQBR",
      "name": "Сбербанк",
      "currency": "rub",
      "exchange": "MOEX",
      "lot": 1,
      "instrument_type": "share",
      "sector": "financial",
      "min_price_increment": 0.01
    }
  ]
}
```

| Поле | Описание |
|------|----------|
| `uid` | Уникальный ID инструмента в T-Invest |
| `figi` | Идентификатор FIGI |
| `ticker` | Биржевой тикер |
| `class_code` | Секция биржи (`TQBR`, `FORTS`, ...) |
| `name` | Название компании/инструмента |
| `currency` | Валюта расчётов |
| `exchange` | Биржа |
| `lot` | Лотность |
| `instrument_type` | `share` или `future` |
| `sector` | Сектор (для акций) или базовый актив (для фьючерсов) |
| `min_price_increment` | Минимальный шаг цены |

#### GET /api/v1/instruments/futures

Все торгуемые фьючерсы. Формат аналогичен shares, `instrument_type: "future"`, `sector` = базовый актив.

```bash
curl http://server:8080/api/v1/instruments/futures
```

#### GET /api/v1/prices/bulk?uids=uid1,uid2,...

Последние цены для пачки инструментов за один запрос. Позволяет запросить до 100+ UID через запятую.

```bash
curl "http://server:8080/api/v1/prices/bulk?uids=e6123145-9665-43e0-8413-cd61b8aa9b13,3da86f70-25a7-4092-bc1b-05cf84dad9fe"
```

Ответ:

```json
{
  "data": [
    {"instrument_uid": "e6123145-...", "price": 314.28, "time": "2026-04-03T14:23:00Z"},
    {"instrument_uid": "3da86f70-...", "price": 16.20, "time": "2026-04-03T14:23:00Z"}
  ]
}
```

#### GET /api/v1/prices/close?uids=uid1,uid2,...

Цены закрытия предыдущей торговой сессии. Используются для расчёта дневного изменения `changePercent`.

```bash
curl "http://server:8080/api/v1/prices/close?uids=e6123145-9665-43e0-8413-cd61b8aa9b13"
```

Ответ:

```json
{
  "data": [
    {"instrument_uid": "e6123145-...", "price": 316.20}
  ]
}
```

### Как Traderbook потребляет данные

**1. Синхронизация инструментов** (`syncInstruments`)

- Вызывает `GET /api/v1/instruments/shares` + `GET /api/v1/instruments/futures`
- Upsert каждого инструмента в таблицу `instruments` (Prisma) по уникальному `uid`
- Расписание: при старте сервиса + ежедневно в 06:00 UTC (пн-пт, до открытия MOEX)

**2. Синхронизация цен** (`syncPrices`)

- Забирает из БД все `active=true` инструменты
- Вызывает `GET /api/v1/prices/bulk?uids=...` чанками по 100 UID
- Вызывает `GET /api/v1/prices/close?uids=...` чанками по 100 UID
- Считает: `changePercent = ((lastPrice - closePrice) / closePrice) * 100`
- Считает: `changeAbs = lastPrice - closePrice`
- Upsert в таблицу `instrument_prices` (1:1 к `instruments`)
- Расписание: каждые 2 мин (пн-пт, 04:00-18:00 UTC = торговые часы MOEX)

**3. API для фронтенда** (Fastify, порт 3021)

| Endpoint Traderbook | Описание |
|---------------------|----------|
| `GET /api/market/heatmap?type=shares` | Все инструменты с ценами и % изменения для тепловой карты |
| `GET /api/market/screener?type=shares&sort=changePercent&dir=desc&limit=50` | Скринер: фильтрация по типу/сектору/поиску, сортировка, пагинация |
| `GET /api/market/sectors` | Список секторов с количеством инструментов |
| `POST /api/market/sync/instruments` | Ручной триггер синхронизации инструментов |
| `POST /api/market/sync/prices` | Ручной триггер синхронизации цен |

### Схема БД (PostgreSQL, Prisma)

```
instruments (uid UNIQUE, ticker, name, sector, instrumentType, exchange, lot, active, ...)
     │
     └── 1:1 ── instrument_prices (lastPrice, closePrice, changePercent, changeAbs)
```

Индексы: `uid` (unique), `ticker`, `sector`, `instrumentType`, `changePercent`.

### Конфигурация

| Переменная | Сервис | Значение |
|-----------|--------|----------|
| `ALERT_GATEWAY_URL` | traderbook/market-data | URL до 24Alert gateway. Default: `http://localhost:8080`. В Docker: `http://host.docker.internal:8080` или IP сервера |
| `PORT` | traderbook/market-data | `3021` |
| `DATABASE_URL` | traderbook/market-data | PostgreSQL traderbook |

### Файлы 24Alert (этот репозиторий)

| Файл | Назначение |
|------|-----------|
| `internal/gateway/handlers/instruments.go` | HTTP-хендлеры: 4 роута, типы `InstrumentShort`, `ClosePrice`, интерфейс `InstrumentsService` |
| `internal/gateway/adapter/instruments_adapter.go` | Реализация: вызовы T-Invest SDK (`Shares`, `Futures`, `GetClosePrices`, `GetLastPrices`), фильтрация по `apiTradeAvailableFlag` |
| `internal/gateway/server.go` | Поле `Instruments` в `Services` struct, подключение роутов |
| `cmd/gateway/main.go` | Создание `InstrumentsAdapter`, передача в `Services` |

### Файлы Traderbook (соседний репозиторий ../traderbook)

| Файл | Назначение |
|------|-----------|
| `services/market-data/src/gateway-client.ts` | HTTP-клиент к 24Alert: `getShares`, `getFutures`, `getBulkPrices`, `getClosePrices` (чанки по 100) |
| `services/market-data/src/sync.ts` | `syncInstruments` + `syncPrices` — upsert в PostgreSQL через Prisma |
| `services/market-data/src/cron.ts` | Расписание: цены каждые 2 мин, инструменты ежедневно 06:00 UTC |
| `services/market-data/src/server.ts` | Fastify API: heatmap, screener, sectors, ручная синхронизация |
| `packages/db/prisma/schema.prisma` | Модели `Instrument`, `InstrumentPrice` |
