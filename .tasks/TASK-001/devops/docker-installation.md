# Docker Installation Report — TASK-001

## Установка Docker на srv03-cloud (176.123.160.234)

**Статус:** ✅ УСПЕШНО  
**Дата:** 2026-04-02 18:53 MSK  
**Версия Docker:** 28.2.2-0ubuntu1~22.04.1  
**Версия Containerd:** 1.7.28-0ubuntu1~22.04.1  

---

## Что было установлено

```
✓ docker.io (28.2.2-0ubuntu1~22.04.1)
✓ containerd (1.7.28-0ubuntu1~22.04.1)  
✓ runc (1.3.3-0ubuntu1~22.04.3)
✓ bridge-utils (1.7-1ubuntu3)
✓ dnsmasq-base (2.90-0ubuntu0.22.04.1)
✓ pigz (2.6-1)
✓ ubuntu-fan (0.12.16)
✓ dns-root-data (2024071801~ubuntu0.22.04.1)
```

**Размер установки:** 289 MB дополнительно  
**Скачано:** 76.3 MB  
**Скорость:** 16.5 MB/s

---

## Конфигурация

### Системные сервисы
```
✓ docker.service — active (running)
✓ docker.socket — active
✓ containerd.service — active (running)
✓ ubuntu-fan.service — active (running)
```

### Пользователь и права
```
✓ Пользователь adm-srv03-cloud добавлен в группу docker
✓ docker ps работает без ошибок
✓ docker system df доступен
```

### Дисковое пространство
```
Диск до установки: 4.9 GB используется / 24 GB свободно
Диск после установки: 5.3 GB используется / 23 GB свободно
Использовано Docker: ~0.4 GB (приемлемо)
```

---

## Проверка работоспособности

```bash
# Версия
$ docker --version
Docker version 28.2.2, build 28.2.2-0ubuntu1~22.04.1

# Статус демона
$ sudo systemctl status docker
● docker.service - Docker Application Container Engine
     Loaded: loaded (/etc/systemd/system/docker.service; enabled; vendor preset: enabled)
     Active: active (running) since Thu 2026-04-02 18:53:20 MSK; 12s ago

# Список контейнеров
$ docker ps
CONTAINER ID   IMAGE     COMMAND   CREATED   STATUS    PORTS     NAMES

# Дисковое использование
$ docker system df
TYPE            TOTAL     ACTIVE    SIZE      RECLAIMABLE
Images          0         0         0B        0B
Containers      0         0         0B        0B
Local Volumes   0         0         0B        0B
Build Cache     0         0         0B        0B
```

**Статус:** ✅ Все системы работают корректно

---

## Примечания

1. **Kernel upgrade pending** — уведомление о доступности нового ядра (не критично)
   - Текущее: 5.15.0-161-generic
   - Доступно: 5.15.0-174-generic
   - Рекомендация: Перезагрузка после установки обновлений (не требуется срочно)

2. **CDI warnings** — незначительные предупреждения о CDI конфиге
   - Не влияют на работу Docker
   - Можно игнорировать

3. **Storage driver** — overlay2 (стандартный и оптимальный для Linux)

4. **BuildKit** — инициализирован успешно (необходим для `docker build`)

---

## Готовность к deployment

| Компонент | Статус | Проверка |
|-----------|--------|----------|
| Docker daemon | ✅ Работает | `systemctl status docker` |
| User access | ✅ Настроен | `docker ps` успешен |
| Storage | ✅ Доступна | 23 GB свободно |
| Network | ✅ Настроена | docker0 и bridge работают |
| Containerd | ✅ Работает | `systemctl status containerd` |
| Runc | ✅ Установлен | Версия 1.3.3 |

**ИТОГ:** 🟢 **ПОЛНАЯ ГОТОВНОСТЬ** — Docker готов к использованию

---

## Использование

### Запустить контейнер
```bash
docker run -d --name test-container nginx:latest
```

### Проверить контейнеры
```bash
docker ps -a
```

### Просмотреть логи
```bash
docker logs test-container
```

### Управление образами
```bash
docker pull ubuntu:22.04
docker images
docker tag ubuntu:22.04 myapp:latest
```

### Docker Compose (если нужен)
Установить через apt:
```bash
sudo apt-get install -y docker-compose
```

Или использовать встроенный `docker compose`:
```bash
docker compose --version
```

---

**Установка завершена успешно. Docker готов к production использованию.**
