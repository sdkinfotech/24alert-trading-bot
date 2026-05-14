# Working Agreements — Qwen Agent Rules for 24alert Project

> Этот файл — основные правила, которые Qwen-агент обязан соблюдать при работе с проектом 24alert.

---

## 1. Язык
- Все комментарии, сообщения, handoff'ы, заметки — **на русском языке**.
- Код — на английском (как и сейчас).
- Commit messages — на русском.

## 2. Структура задач

### Каталог `.tasks/TASK-NNN/`

Каждая задача ОБЯЗАТЕЛЬНО содержит:

| Файл | Назначение | Кто создаёт |
|------|-----------|-------------|
| `task.md` | Постановка задачи | Планировщик или пользователь |
| `plan.md` | План работы, этапы, DoD, timeline | Планировщик |
| `backlog.md` | Строка бэклога (отслеживание статуса) | Планировщик |

Каждая роль создаёт в своей подпапке:

| Файл | Назначение | Кто создаёт |
|------|-----------|-------------|
| `prompt.md` | Промпт для роли | Планировщик (на основе plan.md) |
| `handoff.md` | Результат работы роли | Роль (backend, devops, и т.д.) |

### Артефакты
```
TASK-NNN/
├── task.md
├── plan.md
├── backlog.md
├── artifacts/           # Патчи, миграции, конфиги, SQL
│   └── ...
├── vault/               # Заметки Obsidian, связанные с задачей
│   └── ...
├── backend/
│   ├── prompt.md
│   └── handoff.md
├── devops/
│   ├── prompt.md
│   └── handoff.md
├── tester/
│   ├── prompt.md
│   └── handoff.md
└── tech-lead/
    ├── prompt.md
    └── handoff.md
```

## 3. Флоу задачи (Task Lifecycle)

```
CREATED → PLANNED → IN PROGRESS → REVIEW → DONE
                        ↑                │
                        └── NEEDS CORRECTION ──┘
```

### CREATED
- Задача появилась в BACKLOG.md
- Папка TASK-NNN ещё не создана

### PLANNED
- Создана папка TASK-NNN/
- Написан task.md (постановка)
- Написан plan.md (план, DoD, timeline, роли)
- Созданы prompt.md для каждой роли
- Статус в BACKLOG.md → "Planned"

### IN PROGRESS
- Роли работают последовательно по цепочке
- Каждая роль при завершении пишет handoff.md
- Артефакты складываются в artifacts/
- Заметки в vault/ обновляются или создаются
- Статус в BACKLOG.md → "In Progress"

### REVIEW
- Tech-lead проверяет все handoff'ы
- Если OK → APPROVED → DONE
- Если проблемы → NEEDS CORRECTION → обратно на нужную роль

### DONE
- Все handoff'ы написаны
- Tech-lead одобрил
- Статус в BACKLOG.md → "Done"
- Заметки в vault финализированы

## 4. Obsidian Vault

### Расположение
```
C:\vault\obsidian\devops\24alert\
```

### Правила ведения
- Каждая значимая задача получает заметку в `Knowledge/` или `journal/`
- Заметки имеют frontmatter (title, date, tags, status)
- Обязательны кросс-ссылки (wikilinks) на связанные заметки
- При обновлении задачи — обновлять MOC (Knowledge MOC.md)
- Формат: YAML frontmatter → заголовок → тело → связи

### Структура vault
```
24alert/
├── MOC.md                    # Карта проекта
├── Deployment.md             # Деплой
├── Operations.md             # Операции
├── Grafana.md               # Мониторинг
├── Troubleshooting.md        # Проблемы
├── Tokens.md                # Токены
├── OrderBook Stream.md       # WS стрим стакана
├── journal/                  # Ежедневники
└── Knowledge/
    ├── Knowledge MOC.md
    ├── Architecture/
    ├── Performance/
    └── T-Invest-API/         # 20+ заметок API
        ├── Services/
        └── SDKs/
```

## 5. Безопасность

- **НИКОГДА** не коммитить секреты (.env, токены, ключи)
- Токены — только через environment variables
- `.env` — chmod 0600, в .gitignore
- В handoff'ах и заметках — плейсхолдеры вместо реальных токенов

## 6. Качество кода

- `go fmt` + `goimports` перед коммитом
- `golangci-lint` должен проходить
- Unit-тесты для нового кода
- E2E-тесты для критичных путей
- Все новые эндпоинты — с тестами и документацией

## 7. Деплой

### Текущий процесс (Phase 2)
1. `git pull origin main` на сервере
2. `docker compose -p 24alert build <service>`
3. `docker compose -p 24alert up -d --no-deps --force-recreate <service>`
4. Smoke test: `curl https://gateway.24alert.ru:8080/health`
5. Проверить логи: `docker logs -f <container>`

### Будущий процесс (Phase 5 — CI/CD)
1. `git push origin main`
2. GitHub Actions: lint → test → build → push image → SSH deploy
3. Автоматический smoke test
4. Slack/Telegram уведомление

## 8. Роли

| Роль | Описание | Файлы (.cursor/rules/) |
|------|----------|----------------------|
| Planner | Декомпозиция, бэклог, timeline | role-planner.mdc |
| Backend Senior | Go-код, API, БД | role-backend-senior.mdc |
| DevOps Senior | Docker, K8s, мониторинг | role-devops.mdc |
| Frontend Senior | UI/UX, React | role-frontend-senior.mdc |
| Tester | QA, автотесты | role-tester.mdc |
| Tech Lead | Ревью, архитектура | role-tech-lead.mdc |
| Analyst | Исследования, Grafana, Vault | role-analyst.mdc |

## 9. Роадмап (ROADMAP.md)

Фаза 2 (Hardening) — текущая, 7 задач
Фаза 3 (Data/Persistence) — 3 задачи
Фаза 4 (Trading Features) — 3 задачи
Фаза 5 (Scaling) — 4 задачи

## 10. Напоминания

- При работе с Vault: использовать `[[wikilinks]]` для перекрёстных ссылок
- При обновлении репо: обновлять соответствующую заметку в Vault
- При деплое: вести журнал в `journal/YYYY-MM-DD-description.md`
- При ошибках: создавать/обновлять `Troubleshooting.md`