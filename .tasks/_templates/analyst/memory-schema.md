# Memory Graph Schema — 24alert Analyst

## Entity Types

| entityType | Назначение | Примеры полей в observations |
|-----------|-----------|------------------------------|
| `Service` | Микросервис / компонент платформы | version, purpose, owner, health status, port, dependencies |
| `Server` | Физический / виртуальный сервер | OS, CPU cores, RAM GB, disk GB, IP, SSH user, status |
| `Metric` | Метрика мониторинга | unit, alert_threshold, current_value, trend (up/down/stable) |
| `Risk` | Выявленный риск | severity (HIGH/MEDIUM/LOW), probability, mitigation, source |
| `MarketData` | Рыночная информация | source, ticker, pattern, signal, date |
| `Decision` | Архитектурное / техническое решение | rationale, alternatives_considered, date, author |

## Relation Types (active voice)

| relationType | From → To | Описание |
|-------------|----------|----------|
| `depends_on` | Service → Service | Сервис зависит от другого |
| `deployed_on` | Service/Project → Server | Развёрнут на сервере |
| `exposes_metric` | Service → Metric | Сервис предоставляет метрику |
| `affects` | Risk → Service/Server | Риск влияет на компонент |
| `mitigates` | Decision → Risk | Решение снижает риск |
| `uses` | Service → MarketData | Сервис использует данные |
| `monitors` | Metric → Service/Server | Метрика мониторит компонент |

## Naming Conventions

- Prefix: `24alert-` для всех entities проекта
- Services: `24alert-<service-name>` (e.g. `24alert-api-gateway`)
- Servers: `24alert-<role>-server` (e.g. `24alert-prod-server`)
- Risks: `24alert-risk-<short-name>` (e.g. `24alert-risk-low-ram`)
- Metrics: `24alert-metric-<name>` (e.g. `24alert-metric-cpu-usage`)

## MCP Commands Reference

```
# Перед созданием — проверить существование
search_nodes({ query: "24alert service-name" })

# Создать новые сущности
create_entities({ entities: [{ name, entityType, observations: [...] }] })

# Добавить факты к существующей сущности
add_observations({ observations: [{ entityName, contents: [...] }] })

# Связать сущности
create_relations({ relations: [{ from, to, relationType }] })

# Обзор всего графа
read_graph()
```
