# Промпт: DevOps → TASK-001

## Контекст
Ты выступаешь как **DevOps инженер**. Задача — первое подключение к production-серверу алготрейдинговой платформы и полная диагностика ресурсов и окружения.

**Исходная постановка**: `.tasks/TASK-001/task.md`
**План выполнения**: `.tasks/TASK-001/plan.md`

---

## Что делать

### 1. Подготовка и безопасность
- Убедись, что у тебя есть SSH-ключ или пароль для подключения к prod-серверу.
  - Ищи credencials в: `.env`, `secrets.json`, HashiCorp Vault, AWS Secrets Manager или другом хранилище проекта.
  - **Важно**: не коммитить и не логировать sensitive данные.
- Проверь, что у тебя есть доступ (sudo/root если требуется для системных проверок).

### 2. Подключение и базовая информация
Подключись к серверу и собери следующее:
```bash
# OS и ядро
uname -a
lsb_release -a

# CPU
nproc
lscpu

# Память (RAM)
free -h

# Диск
df -h
du -sh /*

# Сетевые интерфейсы
ip addr
ip route
netstat -tuln  # или ss -tuln

# Текущий пользователь и права
whoami
groups
sudo -l  # если есть sudo access
```

### 3. Проверка необходимых инструментов
Убедись, что установлены:
- `git` (версия)
- `docker` (версия, статус демона)
- `kubectl` (версия, если нужен k8s)
- `ssh-keygen`, `curl`, `wget`, `jq`

Команды проверки:
```bash
git --version
docker version
kubectl version
which ssh-keygen curl wget jq
```

### 4. Тестирование сетевой доступности
- Проверить доступность внешних сервисов (если известны):
  - DNS: `dig google.com` или `nslookup`
  - HTTP/HTTPS: `curl -I https://google.com`
  - NTP (синхронизация времени): `timedatectl` или `ntpq -p`

### 5. Проверка прав и конфигурации деплоя
- Проверить, может ли текущий пользователь запускать Docker: `docker ps`
- Проверить наличие SSH публичного ключа для автоматизации: `cat ~/.ssh/authorized_keys`
- Проверить файрвол / selinux (если применимо): `sudo firewall-cmd --list-all` или `getenforce`

### 6. Сбор логов для диагностики
- Системные логи: `journalctl -n 50 --priority err` (последние ошибки)
- Дата и время на сервере: `date`
- Uptime: `uptime`

---

## Форма результата: handoff.md

Когда закончишь, создай файл `.tasks/TASK-001/devops/handoff.md` со следующей структурой:

```markdown
# Handoff: DevOps → TASK-001

## Статус
DONE | BLOCKED | NEEDS_CORRECTION

## Что сделано
- Подключение по SSH установлено и протестировано
- Собрана информация о ресурсах (OS, CPU, RAM, диск, сеть)
- Проверены все необходимые инструменты (git, docker, kubectl)
- Валидированы права доступа для деплоя
- Собраны диагностические логи

## Артефакты
### Документация
- Server Info Summary (OS, CPU cores, RAM, диск, сетевые интерфейсы)
- List of installed tools with versions
- Network connectivity status
- Access & permissions report

### Команды и результаты
```
# (вставь вывод ключевых команд, скрыв sensitive data)
OS: Ubuntu 20.04 LTS
CPU: 8 cores @ 2.5 GHz
RAM: 32 GB
Disk: 500 GB available (/dev/sda1)
Docker: v20.10.x running
Git: v2.x.x installed
```

## Корректировки для следующих ролей
НЕ ТРЕБУЕТСЯ
<!-- или: -->
<!-- Для техлида: обновить security checklist — SSH ключи требуют ротации каждые 90 дней -->

## Блокеры
НЕТ
<!-- или: -->
<!-- - SSH ключ истёк; требуется обновление в vault -->
```

---

## Успешное завершение
- ✓ Подключение работает воспроизводимо
- ✓ Ресурсы документированы и соответствуют ожиданиям
- ✓ Все инструменты установлены
- ✓ Права доступа проверены
- ✓ handoff.md написан и готов к передаче техлиду

**После завершения**: передай handoff техлиду для code review (роль Техлид).
