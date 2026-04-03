# TASK-003: Развёртывание торгового робота на production и smoke testing

## Описание
Развернуть торгового робота (TASK-002) на prod-сервере (srv03-cloud), настроить конфигурацию для production режима (не sandbox), и провести ручное smoke testing через CLI и REST API.

## Требования

### Функциональные
- Развёртывание через docker-compose на srv03-cloud (176.123.160.234)
- Настройка T-Invest API ключа (production account)
- Проверка всех 5 сервисов (order, marketdata, portfolio, risk, gateway)
- CLI smoke tests: несколько базовых команд
- REST API tests через curl: основные endpoints
- Swagger UI доступен и работает
- Логи структурированы, доступны через docker logs

### Нефункциональные
- Конфигурация через config.yaml + env vars (.env)
- Health checks для всех сервисов
- Graceful shutdown
- Мониторинг базовых метрик (CPU, memory, network)
- Документирование: deployment guide, troubleshooting

## Критерии готовности (DoD)
- [ ] Docker-compose поднят на prod-сервере
- [ ] Все 5 контейнеров запущены и healthy
- [ ] CLI работает с production-аккаунтом T-Invest
- [ ] REST API доступен на :8080/api/v1/*
- [ ] Swagger UI работает на :8080/swagger/
- [ ] Smoke tests пройдены (order/cancel, market data, portfolio)
- [ ] Логи доступны и структурированы
- [ ] Документирован процесс развёртывания

## Зависимости
- TASK-001 (сервер готов, Docker установлен)
- TASK-002 (торговый робот готов, одобрен техлидом)

## Приоритет
HIGH — готовность к MVP
