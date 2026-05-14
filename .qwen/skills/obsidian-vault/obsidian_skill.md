# Obsidian Vault — 24alert: Навигация и правила

Этот скилл позволяет агенту ориентироваться в Obsidian vault проекта 24alert и правильно создавать/обновлять заметки.

---

## 1. Расположение vault

```
C:\vault\obsidian\devops\24alert\
```

## 2. Структура vault

```
24alert/
├── MOC.md                              # Главная карта проекта
├── Deployment.md                       # Деплой gateway на srv03-cloud
├── Grafana.md                          # Мониторинг и дашборды
├── Operations.md                       # Рутинные операции, smoke, логи
├── OrderBook Stream.md                 # WS стрим стакана (TASK-019)
├── Tokens.md                           # Токены T-Invest, ротация, безопасность
├── Troubleshooting.md                  # Типовые проблемы и решения
├── journal/                            # Ежедневные журналы
│   └── 2026-04-04-antigravity-proxy-setup.md (пример)
├── Knowledge/                          # Справочные знания
│   ├── Knowledge MOC.md               # Карта знаний T-Invest API
│   ├── Architecture/
│   │   └── 24alert Platform Overview.md
│   ├── Performance/
│   │   └── Server Capacity 24alert.md  # Ресурсы и capacity
│   └── T-Invest-API/
│       ├── T-Invest API MOC.md         # Главный MOC API
│       ├── Agent Reference Card.md     # Шпаргалка для агентов
│       ├── Overview.md                 # Адреса, protobuf
│       ├── Authentication.md           # Токены
│       ├── Limits.md                   # Rate limits, лимиты
│       ├── Protocols.md               # gRPC, REST, WebSocket
│       ├── Sandbox.md                 # Песочница
│       ├── Error Codes.md             # Коды ошибок
│       ├── Trading Statuses.md         # Статусы торгов
│       ├── Options Trading.md          # Полное руководство по опционам
│       ├── Futures Trading.md          # Полное руководство по фьючерсам
│       ├── Algorithmic Trading.md      # Этапы, стратегии, риски
│       ├── Custom Data Types.md        # Quotation, MoneyValue, Timestamp
│       ├── Instrument Identification.md # UID, FIGI, ISIN, ticker
│       ├── Glossary.md                 # Биржевой глоссарий
│       ├── Performance.md             # Оптимизация API-взаимодействия
│       ├── Robot Contest.md            # Конкурсные роботы
│       ├── Contest Robot Architecture.md # Архитектура конкурсных роботов
│       └── SDKs/                       # SDK по языкам
│           ├── Golang SDK.md
│           ├── Python SDK.md
│           ├── Java SDK.md
│           └── ...
│       └── Services/                   # Сервисы API
│           ├── Accounts Service.md
│           ├── Instruments Service.md
│           ├── Orders Service.md
│           ├── Operations Service.md
│           ├── MarketData Service.md
│           ├── Stop Orders Service.md
│           ├── Signals Service.md
│           └── Autofollow Service.md
└── News/                               # Рыночные новости (опционально)
    ├── ContentAnalysis.md
    └── RssSources-Catalog.md
```

---

## 3. Формат заметок

### Frontmatter (обязательный)

```yaml
---
title: "Заголовок заметки"
date: 2026-04-16
updated: 2026-04-17          # при обновлении
tags: [24alert, api, trading]
aliases: ["Короткий псевдоним"]
status: active | draft | archived
---
```

### Тело заметки

- **Markdown** с Obsidian-расширениями.
- Ссылки — через `[[Вики-ссылки]]`.
- Код — в тройных бэктиках с языком.
- Callout-блоки: `> [!info]`, `> [!warning]`, `> [!note]`, `> [!danger]`, `> [!todo]`.
- Задачи: `- [ ]` / `- [x]`.

---

## 4. Правила создания заметок

### Где создавать

| Тип контента | Папка |
|-------------|-------|
| Постоянные знания | `Knowledge/` (или подпапки) |
| Инцидент / постмортем | `Knowledge/Incidents/` |
| Операционная инструкция | корневой уровень (как `Deployment.md`) |
| Ежедневный журнал | `journal/` |
| Мониторинг | корневой уровень (как `Grafana.md`) |

### Именование файлов

- Транслитерация на русском для русскоязычных заметок: `Мониторинг-и-Observability.md`
- Английские имена для технических: `Server-Capacity.md`
- Camel Case или «Предложение с большой буквы» единообразно.

### Обязательные поля

1. **Title** — понятный заголовок
2. **Date** — дата создания
3. **Tags** — минимум 1 тег
4. **Status** — `active`, `draft` или `archived`
5. **Связи** — ссылки на [[MOC]], [[Deployment]] и другие релевантные заметки

---

## 5. Обновление MOC

При создании новой заметки:

1. Открыть `Knowledge/Knowledge MOC.md`
2. Добавить ссылку в соответствующий раздел
3. Если раздела нет — создать

Формат:
```markdown
- [[Knowledge/T-Invest-API/Новая-заметка|Название отображения]] — краткое описание
```

---

## 6. Формат ежедневника

**Файл**: `journal/YYYY-MM-DD-description.md`

```yaml
---
title: "Описание действий"
date: 2026-04-04
tags: [journal, deploy, maintenance]
status: active
---
```

---

## 7. Кросс-ссылки

- Всегда связывай новые заметки с существующими через `[[wikilinks]]`.
- При переименовании — обнови все ссылки.
- Между подразделами Knowledge и корневыми заметками (Deployment, Operations) — двусторонние ссылки.

## 8. Пример заметки в vault

```markdown
---
title: "Новый эндпоинт /stream/spreads"
date: 2026-04-17
tags: [24alert, endpoint, websocket]
status: draft
---

# Endpoint /stream/spreads

Новый WS-эндпоинт для стриминга спредов между инструментами.

## Архитектура

...

## Связи

- [[24alert/MOC]]
- [[24alert/OrderBook Stream]]
- [[24alert/Deployment]]
```

---

## 9. Интеграция с 24alert-репозиторием

Все заметки в vault должны отражать текущее состояние репозитория `24alert-trading-bot`. При обновлении кода — обновлять соответствующие заметки в vault.

Ключевые файлы репо и соответствующие заметки:

| Репо | Vault |
|------|-------|
| `README.md` | [[24alert/MOC]] § Обзор |
| `docs/TRADING_API.md` | [[24alert/Knowledge/T-Invest-API/T-Invest API MOC]] |
| `BACKLOG.md` | Отслеживается в таск-трекере, не в vault |
| `deployments/docker-compose.yaml` | [[24alert/Deployment]] |
| `config/config.yaml` | [[24alert/Deployment]] § Конфиг |
| `.tasks/TASK-NNN/` | Журналы в `journal/` + заметки по необходимости |