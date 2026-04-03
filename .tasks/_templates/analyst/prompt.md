# Промпт: Аналитик → TASK-NNN

## Контекст
Ты выступаешь как **Аналитик**. Задача — исследовать артефакты задачи, собрать данные из всех доступных источников, структурировать выводы и сохранить в хранилища знаний.

**Исходная постановка**: `.tasks/TASK-NNN/task.md`
**План выполнения**: `.tasks/TASK-NNN/plan.md`
**Handoff ролей**: прочитай все доступные `handoff.md` из подпапок задачи.

---

## 1. Сбор данных

### Из Grafana (MCP `user-grafana`)
- [ ] Запросить список дашбордов (`search_dashboards`)
- [ ] Получить ключевые метрики за период: CPU, RAM, disk, throughput, latency, error rate
- [ ] Выгрузить результаты в `.tasks/TASK-NNN/analyst/data/metrics.json`

### Из логов (Loki / Tempo)
- [ ] Собрать error patterns за последние 24h / 7d
- [ ] Идентифицировать slow requests (>1s P95)
- [ ] Сохранить summary в `.tasks/TASK-NNN/analyst/data/logs-summary.json`

### Из кода и API
- [ ] Извлечь структуру API endpoints (swagger / OpenAPI если есть)
- [ ] Проанализировать архитектуру компонентов (сервисы, зависимости)
- [ ] Сохранить в `.tasks/TASK-NNN/analyst/data/api-structure.json`

### Из инфраструктуры (DevOps handoff)
- [ ] Ресурсы сервера: CPU cores, RAM, disk, network
- [ ] Установленные инструменты и версии
- [ ] Capacity planning: достаточно ли ресурсов для целевой нагрузки

### Рыночные данные (если применимо)
- [ ] Конкурентный анализ
- [ ] Торговые паттерны и сигналы
- [ ] Источники данных и API

---

## 2. Анализ

### Что искать
- **Риски**: что может сломаться, какова вероятность и влияние
- **Bottlenecks**: где узкие места (CPU, RAM, диск, сеть, БД, API)
- **Тренды**: растёт нагрузка? деградирует latency? растёт error rate?
- **Аномалии**: отклонения от нормы, unexplained patterns
- **Gaps**: что не покрыто мониторингом, документацией, тестами

### Формат выводов
Каждый вывод оформляй как:
```
[FINDING-NNN] Краткое описание
- Источник: (откуда данные)
- Данные: (числа, графики, логи)
- Влияние: HIGH | MEDIUM | LOW
- Рекомендация: (конкретное действие)
```

---

## 3. Структурирование

### Wiki (Obsidian) — приоритет 1
- Обнови или создай заметки в `traderbook/Knowledge/`
- Подпапки: `Architecture/`, `Performance/`, `Market Research/`, `Incidents/`
- Каждая заметка: frontmatter, wikilinks, ссылка из MOC
- MOC: `traderbook/Knowledge/Knowledge MOC.md`

### Memory graph (MCP) — приоритет 2
- Перед созданием: `search_nodes` по имени сущности
- Если есть: `add_observations` с новыми фактами
- Если нет: `create_entities` + `create_relations`
- Типы: Service, Server, Metric, Risk, MarketData, Decision

### Data files — приоритет 3
- JSON/CSV в `.tasks/TASK-NNN/analyst/data/`
- Именование: `metrics.json`, `logs-summary.json`, `api-structure.json`

---

## 4. Handoff

По завершении создай `.tasks/TASK-NNN/analyst/handoff.md` по стандартному формату (см. шаблон).

**Успешное завершение**:
- ✓ Данные собраны из всех источников
- ✓ Риски и bottlenecks задокументированы
- ✓ Wiki обновлена (заметки + MOC)
- ✓ Memory graph обновлён (entities + relations)
- ✓ Data файлы сохранены
- ✓ Handoff написан
