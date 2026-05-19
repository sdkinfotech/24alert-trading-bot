---
title: "Handoff: Аналитик → TASK-002"
date: 2026-04-03
tags: [task-002, analyst, handoff, t-invest-api, go-sdk]
---

# Handoff: Аналитик → TASK-002

## Статус
**DONE**

Исследование T-Invest API, Go SDK и инфраструктуры завершено. Все выводы структурированы и готовы для бэкенд-разработчика.

---

## Что сделано

### 1. Исследование T-Invest API

#### SDK
- **Репозиторий**: `github.com/russianinvestments/invest-api-go-sdk` (RussianInvestments организация)
- **Последний релиз**: v1.40.1 (сентябрь 2025)
- **Статус**: Архивирован, но поддерживается (последний апдейт: сент 2025)
- **Лицензия**: Apache 2.0
- **Язык**: Go 1.16+

**Установка**:
```bash
go get github.com/russianinvestments/invest-api-go-sdk
```

**Основной паттерн использования**:
```go
// 1. Загрузить конфиг из YAML или создать struct
config := investgo.Config{
    EndPoint: "sandbox-invest-public-api.tbank.ru:443",
    Token: os.Getenv("TINVEST_TOKEN"),
    AppName: "24alert",
    MaxRetries: 3,
    DisableResourceExhaustedRetry: false, // auto-retry on rate-limit
}

// 2. Создать клиента (один на приложение)
client, err := investgo.NewClient(ctx, config, logger)
defer client.Stop()

// 3. Создать сервис клиента
ordersClient := client.NewOrdersServiceClient()
response, err := ordersClient.PostOrder(ctx, &pb.PostOrderRequest{...})

// 4. Для стримов
mdClient := client.NewMarketDataStreamClient()
stream, err := mdClient.MarketDataStream()
candleChan, err := stream.SubscribeCandle([]string{figi}, pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_ONE_MINUTE, true)
go stream.Listen() // в отдельной горутине
```

#### gRPC API структура

- **Endpoint**: `invest-public-api.tbank.ru:443` (prod) или `sandbox-invest-public-api.tbank.ru:443` (sandbox)
- **Аутентификация**: Bearer token в gRPC metadata (`Authorization: Bearer <token>`)
- **Proto-файлы**: В репозитории `/proto` directory; интегрированы в SDK
- **Транспорт**: Один persistent gRPC connection от `investgo.Client`, multiplexing всех запросов

#### Сервисы API

1. **OrdersService** (postOrder, cancelOrder, replaceOrder, getOrders, getOrderState, streams)
2. **MarketDataService** (getCandles, getOrderbook, getLastPrices, MarketDataStream)
3. **OperationsService** (getPortfolio, getPositions, getOperationsByCursor, getWithdrawLimits)
4. **InstrumentsService** (GetInstrumentBy, GetInstruments, GetFuturesMargin, etc)
5. **AccountsService** (GetAccounts, GetMarginAttributes)
6. **StopOrdersService** (PostStopOrder, CancelStopOrder, GetStopOrders)
7. **SandboxService** (OpenSandboxAccount, SandboxPayIn, etc) - только для тестирования

---

### 2. Rate Limits (КРИТИЧНО!)

Полная таблица в `rate-limits.json`. Ключевые пункты:

#### Unary методы

| Метод | Лимит | Примечание |
|-------|-------|-----------|
| `postOrder` | **15/сек** (900/мин) | 🔴 КРИТИЧЕСКИЙ BOTTLENECK |
| `postOrderAsync` | **600/мин** | Рекомендуется для high-freq |
| `cancelOrder` | 100/мин | |
| `replaceOrder` | 100/мин | |
| `getOrders` | 200/мин | |
| `getCandles` | 600/мин | |
| `getOrderbook` | 600/мин | |
| `getLastPrices` | 600/мин | |
| `GetHistory` | **30/мин** | ZIP-архив - специально ограничен |
| `getPositions` | 200/мин | |
| `getPortfolio` | 200/мин | |
| `getWithdrawLimits` | 200/мин | |
| `PostStopOrder` | 50/мин | |
| `GetTradingStatus` | 600/мин | |

#### Stream соединения

| Тип | Max одновременных | Примечание |
|-----|------------------|-----------|
| MarketDataStream | 32 | One connection supports 300 subs (candles+orderbook+trades) |
| OrderStateStream | 16 | Per type |
| TradesStream | 16 | Per type |
| PortfolioStream | 11 | Per type |
| PositionsStream | 11 | Per type |

**Глобальное ограничение**: 50 req/sec total с одного IP/токена. Превышение → IP ban на 1000+ req/min.

#### Рекомендации по rate-limiting

1. Реализовать **per-method rate limiter** (token bucket алгоритм)
2. Использовать `postOrderAsync` вместо `postOrder` для high-frequency
3. Multiplexировать MarketDataStream (одна, не множество)
4. Отслеживать x-ratelimit-remaining в response headers
5. Exponential backoff для ResourceExhausted ошибок (SDK делает это, но убедиться в config)

---

### 3. Sandbox vs Production

#### Endpoint переключение

```go
// Sandbox
config.EndPoint = "sandbox-invest-public-api.tbank.ru:443"

// Production
config.EndPoint = "invest-public-api.tbank.ru:443"
```

#### Sandbox особенности

**Специальные методы** (только в sandbox):
- `OpenSandboxAccount()` — создать тестовый аккаунт
- `CloseSandboxAccount()` — удалить аккаунт
- `SandboxPayIn()` — пополнить баланс (неограниченно, без реальных транзакций)

**Ограничения**:
- ❌ Нет расчёта комиссий
- ❌ Нет портфельной аналитики (возвраты, yields)
- ❌ Нет risk метрик (guarantee, margin levels)
- ❌ `getOperations` поддерживает только BUY/SELL фильтры
- ❌ `GetDividendsForeignIssuer` всегда пуст
- ⏰ Аккаунты удаляются после 3 месяцев неактивности
- ⏰ Ордера истекают после 7 дней

**Преимущества**:
- ✅ Идентичные сигнатуры методов production
- ✅ Безопасное тестирование logic
- ✅ Нет потерь реальных денег
- ✅ Быстрая feedback на интеграционные проблемы

---

### 4. Типы заявок и особенности

#### Order types

- **LIMIT**: цена + кол-во, выполняется по цене или лучше
- **MARKET**: выполняется по best available price (любая цена)
- **BESTPRICE**: гибридный тип (не поддерживается для опционов)

#### Order параметры (ВАЖНО)

- **order_id**: генерируется клиентом, UUID строка (max 36 chars), **основа идемпотентности**
  - Дубликат order_id → ошибка `30057`
  - Уникальность отслеживается 1 месяц
- **direction**: BUY или SELL
- **quantity**: количество лотов (не единиц!)
- **price**: для LIMIT заявок (Quotation: units + nano)

#### Stop Orders

- **STOP_LOSS**: выставляется когда цена < stop_price
- **TAKE_PROFIT**: выставляется когда цена > stop_price
- **Expiration types**: 
  - `GoodTillCancel` — до ручной отмены
  - `GoodTillDate` — до даты истечения

#### Replace Order

- Атомарная операция: cancel старую + post новую
- Параметры: account_id, order_id, новые qty/price
- Ошибка 30059: не может отменить ордер

---

### 5. Специфика инструментов

#### Акции / Облигации
- Идентификаторы: FIGI, UID
- `FindInstrument(query)` — поиск по названию/тикеру
- Цены в валюте (руб, доллар и т.д.)

#### Фьючерсы 🔴 ВАЖНО
- Идентификаторы: FIGI + UID, class_code = "SPBFUT"
- **Цены в ПУНКТАХ**, не валюте
- Формула стоимости: `price / min_price_increment * min_price_increment_amount`
- Метод: `GetFuturesMargin(uid)` → `min_price_increment`, `min_price_increment_amount`, GO (ГО)
- Вариационная маржа начисляется ежедневно
- Методы: `Futures()` — список, `FutureBy()` — по UID

#### Опционы 🔴 ВАЖНО
- Идентификаторы: **UID ТОЛЬКО** (FIGI не работает)
- class_code = "SPBOPT"
- `FindInstrument` **НЕ ищет** опционы
- Методы: `OptionBy(uid)` или `OptionsBy(baseAssetUid)` для поиска
- **Только LIMIT заявки** через API (рыночные недоступны)
- Требует маржинальной торговли (error 30051 если не включена)
- `getOperations` **не поддерживает** опционы → используй `getOperationsByCursor`

---

### 6. Stream patterns (ВАЖНО для архитектуры)

#### MarketDataStream (bidirectional)

```go
// Правильный паттерн:
mdClient := client.NewMarketDataStreamClient()
stream, _ := mdClient.MarketDataStream()

// Subscribe ПЕРЕД Listen()
candleChan, _ := stream.SubscribeCandle([]string{figi1, figi2}, interval, waitClose)
tradesChan, _ := stream.SubscribeTrade([]string{figi1, figi2})

// Listen() в отдельной горутине
go stream.Listen()

// Читать из каналов
for candle := range candleChan { ... }

// Shutdown
stream.Stop()
```

**Ключевые моменты**:
- Один stream поддерживает max 300 подписок (candles + orderbook + trades суммарно)
- Info subscriptions (trading status) — без лимита
- Max 100 subscribe requests в минуту
- Candles/orderbook: 100ms delivery interval (макс 1 обновление за 100ms)
- Trades, LastPrice, Info — без delay

#### OrderStateStream (server-side)

- Самый быстрый способ узнать об исполнении заявки
- Лимит: 16 одновременных connections
- Также используется `TradesStream` для всех trades

#### PortfolioStream / PositionsStream

- Обновления портфеля при исполнении сделок
- PortfolioStream: доходности, стоимости
- PositionsStream: изменения позиций

---

### 7. Ошибки и обработка

#### Частые коды ошибок

- `30057`: Duplicate order_id (используется 2-й раз) → использовать новый UUID
- `30051`: Margin trading not enabled (для фьючерсов/опционов) → включить в настройках
- `30059`: Cannot cancel order (уже исполнена/отменена) → проверить статус перед отменой
- `40003`: Invalid or expired token → обновить токен

#### ResourceExhausted (rate-limit)

- **gRPC code**: `codes.ResourceExhausted`
- **SDK поведение**: Automatic retry с exponential backoff (если не отключено)
- **Мониторинг**: Проверять gRPC headers: x-ratelimit-remaining

---

## Артефакты

### 📄 JSON файлы (в `analyst/data/`)

1. **`rate-limits.json`** — полная таблица всех rate limits (unary, streams, subscriptions)
2. **`api-methods.json`** — справочник всех методов, параметров, потенциальных gotchas

### 📚 Локальная wiki (Obsidian)

Существующие заметки в `24alert/Knowledge/T-Invest-API/`:
- `Agent Reference Card.md` — навигация для агентов (ссылки на методы)
- `Futures Trading.md` — полное руководство по фьючерсам
- `Options Trading.md` — полное руководство по опционам
- `Orders Service.md` — детальное описание методов заявок
- `MarketData Service.md` — методы котировок и стриминга
- `Limits.md` — rate limits

### 🧠 Memory MCP

Созданы entities:
- `tinvest-api` (Service) — T-Invest API, основные endpoint, auth, rate limits
- `tinvest-go-sdk` (Service) — Go SDK, v1.40.1, ключевые patterns
- `tinvest-sandbox` (Decision) — sandbox behavior, limitations, differences

---

## Коррекции для следующих ролей

### Для Backend разработчика (TASK-002, Phase 1-3)

1. **Per-method rate limiter** — обязателен для `postOrder` (15/sec), использовать token bucket алгоритм
2. **Order journal** — хранить mappings между client order_id и exchange order_id
3. **Instrument cache** — локальный реестр FIGI/UID/class_code для options (UID-only)
4. **MarketData stream manager** — один shared connection + multiplex подписок
5. **SDK wrapper** — инкапсулировать Client creation/shutdown, обработку контекста
6. **Error mapping** — от gRPC codes к внутренним error types (order_rejected, rate_limit_exceeded, etc)

**НЕ требуется для Phase 1-3**:
- Полная Prometheus метрика реализация (scaffold достаточно)
- NATS event bus (опционально для Phase 2+)
- Frontend UI (только CLI + Swagger)

### Для DevOps разработчика

1. SDK требует Go 1.16+ (проверить в go.mod)
2. gRPC ports: внутренние сервисы на :9001-:9004, gateway на :8080
3. TLS: gRPC connections к T-Invest API по умолчанию TLS (:443)
4. Config: YAML или env vars (viper); sandbox/live switching одним флагом
5. Docker: no special requirements (стандартный Go build)

---

## Критерии успеха (для Backend)

✅ Backend может начать кодить:
- Понимает rate limit bottleneck (`postOrder` 15/sec)
- Знает как переключаться sandbox ↔ production (endpoint)
- Осведомлён о gotchas (options UID-only, futures points-based pricing)
- Имеет справочник методов (rate-limits.json, api-methods.json)
- Знает правильные stream patterns

---

## Риски, обнаруженные

1. **Rate limits жестче чем кажется** 
   - `postOrder` 15/sec очень низко для high-frequency
   - Решение: использовать `postOrderAsync` (600/min)

2. **Sandbox отличается от production**
   - Нет комиссий, нет risk метрик
   - Решение: валидировать logic на sandbox, но финальный тест на production

3. **Options/Futures требуют специальной обработки**
   - Options: UID-only, no FIGI, special filtering methods
   - Futures: points-based pricing, GetFuturesMargin required
   - Решение: instrument cache с metadata

4. **Stream management сложный**
   - Можно легко создать > 32 MarketDataStream connections (лимит)
   - Решение: single shared connection + subscription manager

---

## Связанные ресурсы

- [T-Invest API docs](https://developer.tbank.ru/invest/intro/intro/)
- [Go SDK repo](https://github.com/RussianInvestments/invest-api-go-sdk)
- [SDK examples](https://github.com/RussianInvestments/invest-api-go-sdk/tree/master/examples)
- [Rate limits](https://russianinvestments.github.io/investAPI/limits/)
- [Sandbox docs](https://russianinvestments.github.io/investAPI/sandbox/)

---

**Аналитик**: finished on 2026-04-03
**Статус**: Ready for Backend Phase 1 (Scaffold)
