# Связанные репозитории и ресурсы — TASK-002

## External Projects

### tinvest-api-bot
- **URL**: https://github.com/sdkinfotech/tinvest-api-bot
- **Git**: `git@github.com:sdkinfotech/tinvest-api-bot.git`
- **Описание**: Reference implementation trading bot на Go от sdkinfotech. Может содержать примеры стратегий и интеграции с T-Invest API.
- **Использование**: Изучение best practices, примеры gRPC + REST архитектуры, стратегий.

### invest-api-go-sdk
- **URL**: https://github.com/RussianInvestments/invest-api-go-sdk
- **Описание**: Official Go SDK от Тинькофф Инвестиций для работы с T-Invest API.
- **Использование**: Основная библиотека для работы с биржевым API. Используется в `pkg/tinvest/client.go`.

## Official Resources

### T-Invest Developer Portal
- **URL**: https://developer.tbank.ru/
- **Компоненты**:
  - [API Reference](https://developer.tbank.ru/invest/intro/intro/) — полная документация методов
  - [Rate Limits](https://russianinvestments.github.io/investAPI/limits/) — таблица ограничений
  - [Sandbox Guide](https://russianinvestments.github.io/investAPI/sandbox/) — как работать в песочнице
  - [Go SDK Docs](https://github.com/RussianInvestments/invest-api-go-sdk/tree/main/docs) — примеры кода

## Как использовать в проекте

1. **При исследовании API** (Аналитик): посмотреть в tinvest-api-bot как там организована архитектура, какие методы используются
2. **При разработке** (Бэкенд): использовать invest-api-go-sdk как основную зависимость
3. **При тестировании** (Тестировщик): обращаться к sandbox guide и rate limits docs

## Добавление в проект

Для клонирования tinvest-api-bot как reference:
```bash
git clone https://github.com/sdkinfotech/tinvest-api-bot.git external/tinvest-api-bot
```

Или просто использовать как reference при разработке.
