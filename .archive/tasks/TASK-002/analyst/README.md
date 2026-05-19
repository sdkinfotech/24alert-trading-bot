# Аналитика TASK-002: T-Invest API Research

## Содержание

### 📄 Основной отчёт
- **`handoff.md`** — полный handoff от Аналитика к Backend разработчику
  - Исследование T-Invest API и Go SDK
  - Rate limits (таблицы + рекомендации)
  - Sandbox vs Production
  - Специфика инструментов (фьючерсы, опционы)
  - Stream patterns
  - Ошибки и обработка
  - Коррекции для следующих ролей

### 📊 JSON справочники (в папке `data/`)

#### 1. `rate-limits.json`
Полная таблица rate limits:
- **Global limits**: 50 req/sec, IP ban при > 1000 req/min
- **Unary methods**: Все методы с лимитами (постоянный, per-second, per-minute)
- **Stream connections**: Лимиты на одновременные connections (32 MD, 16 Orders, 11 Ops)
- **Stream subscriptions**: Лимиты на подписки (300 total per MD connection, Info unlimited)
- **Data delivery intervals**: Delay между обновлениями в стримах (100ms для candles/orderbook)
- **Важные замечания**: SDK retry logic, IP ban threshold, stream counter refresh time

**Использование**: Backend разработчик копирует в свой rate-limiter реализацию.

#### 2. `api-methods.json`
Справочник всех методов и patterns:
- **SDK overview**: Client creation, Config structure, lifecycle
- **Core services**: Orders, MarketData, Operations, StopOrders, Instruments, Accounts (7 сервисов)
- **Stream patterns**: Правильный способ использования MarketDataStream, OrderStateStream
- **Error handling**: Код ошибок (30057, 30051, 30059, 40003), rate-limit detection
- **Sandbox vs Production**: Endpoint переключение, специальные методы
- **Proto file location**: Где найти proto-контракты в SDK
- **Gotchas & Recommendations**: 9 критических gotchas + решения

**Использование**: Backend разработчик обращается при реализации каждого сервиса.

## Ключевые выводы для Backend

### 🔴 КРИТИЧНЫЕ для Phase 1 (Scaffold)

1. **Rate Limiter architecture**
   - `postOrder` — 15/sec (используй token bucket per-method)
   - `postOrderAsync` — 600/min (рекомендуется для high-freq)
   - Global — 50/sec (общий лимит на приложение)

2. **Stream management**
   - **ОДНА** MarketDataStream (max 32 connections)
   - Max 300 subscriptions (candles + orderbook + trades combined)
   - Правильный pattern: subscribe → listen in goroutine → read from channels

3. **Idempotency**
   - order_id — UUID, генерируется клиентом
   - Дубликат → error 30057
   - Требует order journal для mapping client_id → exchange_id

### 🟡 ВАЖНЫЕ для Phase 2 (Core services)

4. **Options vs Futures identification**
   - **Options**: UID-only, class_code="SPBOPT", `OptionBy(uid)` / `OptionsBy(baseAssetUid)`
   - **Futures**: FIGI+UID, class_code="SPBFUT", points-based pricing
   - Оба требуют маржинальную торговлю (error 30051)

5. **Sandbox testing strategy**
   - Endpoint переключение: одним флагом конфига
   - Limitations: нет комиссий, нет risk метрик, accounts удаляются через 3 месяца
   - Best practice: вся логика на sandbox, финальный test на production

### 🟢 ОПЦИОНАЛЬНЫЕ для Phase 3+

6. **Performance optimizations**
   - Use `postOrderAsync` для high-freq
   - OrderStateStream (fastest order updates)
   - Parallel stream subscriptions (но не более 32)

---

## Как использовать

### Для Backend разработчика при реализации Rate Limiter

```
1. Откроет rate-limits.json
2. Реализует per-method token bucket:
   - postOrder: 15/sec
   - cancelOrder: 100/min
   - getOrders: 200/min
   - и т.д.
3. Добавит global limiter: 50/sec total
4. Включит SDK retry logic: DisableResourceExhaustedRetry = false
```

### Для Backend разработчика при реализации каждого сервиса

```
1. Откроет api-methods.json
2. Найдёт свой сервис (Orders, MarketData, etc)
3. Скопирует key_methods, stream_methods, notes
4. Посмотрит примеры использования в README SDK
5. Реализует в Go с proper error handling
```

### Для DevOps разработчика при containerization

```
1. Откроет handoff.md, раздел "Для DevOps разработчика"
2. Заметит: Go 1.16+, gRPC ports :9001-:9004, TLS к T-Invest API
3. Создаст Dockerfile + docker-compose.yaml
```

---

## Связь с остальной документацией

- **Локальная wiki** (`24alert/Knowledge/T-Invest-API/`):
  - `Agent Reference Card.md` — навигация для всех агентов
  - `Futures Trading.md` — full guide по фьючерсам
  - `Options Trading.md` — full guide по опционам
  - `Limits.md` — rate limits для референса

- **Memory MCP** (entities):
  - `tinvest-api` — основной API, endpoints, rate limits
  - `tinvest-go-sdk` — SDK facts и patterns
  - `tinvest-sandbox-behavior` — sandbox limitations
  - `options-trading-go-sdk` — options gotchas
  - `futures-trading-go-sdk` — futures gotchas
  - `marketdatastream-pattern` — correct stream usage

---

## Что дальше (для Backend Phase 1)

1. Backend разработчик читает `handoff.md` целиком (30 мин)
2. Backend размещает `rate-limits.json` в своём rate-limiter пакете
3. Backend размещает `api-methods.json` в wiki/docs проекта
4. Backend начинает Phase 1 (Scaffold): создание go.mod, proto структуры, shared packages

---

**Аналитик**: [завершено] 2026-04-03
**Статус для Backend**: Ready for Phase 1
