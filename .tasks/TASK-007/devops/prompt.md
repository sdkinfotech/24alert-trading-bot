# DevOps — TASK-007: Закрытие портов (деплой)

## Контекст

После изменения docker-compose.yaml бэкендом — нужно применить изменения на production-сервере и убедиться, что:
1. Порты больше не доступны извне
2. Сервисы внутри Docker-сети продолжают общаться
3. Gateway продолжает работать через nginx

## Что сделать

1. Подключиться к серверу: `ssh adm-srv03-cloud@srv03-cloud`
2. Перейти в `/opt/24alert`
3. Скопировать обновлённый docker-compose.yaml (если менялся локально)
4. Пересобрать и рестартануть:
```bash
cd /opt/24alert
docker compose -p 24alert build
docker compose -p 24alert up -d
```
5. Проверить:
```bash
# Порты должны быть закрыты извне
netstat -tlnp 2>/dev/null | grep -E '900[1-4]|9090'
# Должно быть: 127.0.0.1:9001, 127.0.0.1:9002, ...
# НЕ должно быть: 0.0.0.0:9001, ...

# Gateway по-прежнему доступен
curl -fsS https://gateway.24alert.ru:8080/health

# Микросервисы доступны локально
docker exec 24alert-gateway wget -qO- http://127.0.0.1:9001/health 2>&1
```
6. Настроить SSH-туннель для доступа к Prometheus (если нужен удалённый доступ):
```bash
ssh -L 9090:127.0.0.1:9090 adm-srv03-cloud@srv03-cloud
```
7. Результаты в `artifacts/` (nmap-сканы до/после)

## Чек-лист перед handoff

- [ ] Порты закрыты извне (nmap/similar)
- [ ] Gateway работает
- [ ] Микросервисы доступны локально
- [ ] Prometheus доступен через туннель
- [ ] handoff.md написан

## Handoff

Создай `handoff.md` в этой папке с результатами.