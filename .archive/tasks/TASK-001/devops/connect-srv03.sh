#!/bin/bash
# DevOps SSH Helper для srv03-cloud (176.123.160.234)
# Использование: ./connect-srv03.sh [command]

HOST="176.123.160.234"
USER="adm-srv03-cloud"
KEY="$HOME/.ssh/id_rsa"
PORT="22"

# Если аргумент передан — выполнить команду
if [ -z "$1" ]; then
    # Интерактивная сессия
    ssh -i "$KEY" -o StrictHostKeyChecking=no "$USER@$HOST"
else
    # Выполнить команду
    ssh -i "$KEY" -o StrictHostKeyChecking=no "$USER@$HOST" "$@"
fi
