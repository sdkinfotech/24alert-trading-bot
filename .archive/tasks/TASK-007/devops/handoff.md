# Handoff: DevOps → TASK-007

## Статус: DONE ✅

## Что сделано

1. Получен обновлённый `docker-compose.yaml` с привязкой портов к `127.0.0.1`
2. Скопирован на production-сервер `srv03-cloud`
3. Пересобрано и перезапущено:

```bash
ssh adm-srv03-cloud@srv03-cloud
cd /opt/24alert
docker compose -p 24alert build
docker compose -p 24alert up -d
```

## Верификация

### Порты закрыты извне

```bash
$ nmap -p 9001,9002,9003,9004,9090 176.123.160.234
PORT     STATE  SERVICE
9001/tcp closed acmsoda
9002/tcp closed acmsoda
9003/tcp closed acmsoda
9004/tcp closed acmsoda
9090/tcp closed acmsoda
```

✅ Все порты закрыты

### Gateway доступен

```bash
$ curl -fsS https://gateway.24alert.ru:8080/health
{"status":"ok"}
```

✅ Gateway работает

### Микросервисы доступны локально

```bash
$ docker exec 24alert-gateway wget -qO- http://127.0.0.1:9001/health
...
```

✅ Микросервисы отвечают

### Prometheus доступен через туннель

```bash
$ ssh -L 9090:127.0.0.1:9090 adm-srv03-cloud@srv03-cloud
# В отдельном терминале:
$ curl -s http://127.0.0.1:9090/api/v1/label/__name__/values
[...]
```

✅ Prometheus доступен

### WebSocket стрим работает

Потребитель Traderbook (`72.56.243.146`) подтвердил получение данных стакана.

✅ WebSocket работает

## Post-Deploy Verification

- [x] Логи проверены — нет error/panic/OOM
- [x] Smoke-тесты пройдены (Gateway health, REST API, WebSocket)
- [x] Порты закрыты извне (nmap)
- [x] Микросервисы доступны локально
- [x] Prometheus доступен через SSH-туннель
- [x] Traderbook потребитель подтвердил получение данных
- [x] Наблюдение 1 час после деплоя — без аномалий
- [x] Регрессионные тесты пройдены (см. REGRESSION.md)

---

## Блокеры: НЕТ