# Tester — TASK-007: Закрытие портов

## Контекст

После деплоя изменений нужно верифицировать, что:
1. Микросервисные порты закрыты извне
2. Gateway продолжает работать
3. Внутренняя связь между сервисами не нарушена

## Сценарии тестирования

### TC-1: Порты закрыты извне
```bash
# Извне сервера (не на самом сервере!)
nmap -p 9001,9002,9003,9004,9090 176.123.160.234
# Ожидание: все порты filtered/closed
```

### TC-2: Gateway доступен
```bash
curl -fsS https://gateway.24alert.ru:8080/health
# Ожидание: 200 OK, {"status":"ok"}
```

### TC-3: Микросервисы доступны локально
```bash
# На самом сервере
curl http://127.0.0.1:9001/health 2>&1
curl http://127.0.0.1:9002/health 2>&1
curl http://127.0.0.1:9003/health 2>&1
curl http://127.0.0.1:9004/health 2>&1
# Ожидание: 200 OK или корректный ответ сервиса
```

### TC-4: REST API работает (через gateway)
```bash
curl -s https://gateway.24alert.ru:8080/api/v1/accounts \
  -H "Authorization: Bearer <test-token>" | head -5
# Ожидание: ответ JSON (может быть ошибка авторизации — это OK,
# главное что endpoint отвечает)
```

### TC-5: WebSocket стрим работает
```bash
python3 -c "
import ssl, websocket
ws = websocket.create_connection(
    'wss://gateway.24alert.ru:8080/api/v1/stream/candles',
    sslopt={'cert_reqs': ssl.CERT_NONE}
)
ws.send('{\"method\":\"SUBSCRIBE\",\"params\":{\"uids\":[\"e6123145-9665-43e0-8413-cd61b8aa9b13\"],\"interval\":\"1h\"}}')
print(ws.recv())
ws.close()
"
# Ожидание: ответ 101 или корректный JSON
```

### TC-6: Prometheus доступен через туннель
```bash
ssh -L 9090:127.0.0.1:9090 adm-srv03-cloud@srv03-cloud &
sleep 2
curl -s http://127.0.0.1:9090/api/v1/label/__name__/values | head -3
# Ожидание: JSON со списком метрик
```

## Результаты

Все TC-1..TC-6 пройдены ✅ / Частично ❌ / Не пройдены ❌

Ответственные: DevOps — TC-1, TC-3, TC-6; Tester — TC-2, TC-4, TC-5