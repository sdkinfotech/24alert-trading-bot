#!/bin/sh
# 24alert AI Scanner: runs Cursor Agent CLI for market analysis tasks.
# Usage: run-scan.sh post-market | pre-market | health

set -eu

KIND="${1:-health}"
DATE="$(date +%Y-%m-%d)"
MSG_FILE="/tmp/scanner-${KIND}-${DATE}.txt"

START_TS=$(date +%s)
echo "[$(date -Iseconds)] ai-scanner kind=$KIND start"

# Validate environment
if [ -z "${CURSOR_API_KEY:-}" ] || [ "${CURSOR_API_KEY}" = "stub" ]; then
  echo "[$(date -Iseconds)] ai-scanner kind=$KIND FAILED reason=missing_env CURSOR_API_KEY" >&2
  exit 3
fi
WS="/workspace"
if [ ! -d "$WS" ] || [ ! -f "$WS/system-prompt.md" ]; then
  echo "[$(date -Iseconds)] ai-scanner kind=$KIND FAILED reason=bad_workspace $WS" >&2
  exit 3
fi

if ! command -v agent >/dev/null 2>&1; then
  echo "[$(date -Iseconds)] ai-scanner kind=$KIND FAILED reason=agent_not_found PATH=$PATH" >&2
  exit 4
fi

case "$KIND" in
  post-market)
    PROMPT="Выполни пост-маркет анализ за $DATE по инструкции из system-prompt.md: 1) Сканируй рынок с помощью scan_market.py 2) Для каждого кандидата из топ-5 запусти бэктест с --optimize 3) Отбери инструменты с Sharpe>1 и PnL>0 4) Обнови config.yaml — добавь новые стратегии, убери убыточные 5) Вызови POST /config/reload на strategy-runner 6) Запиши отчёт в vault. Действуй автономно, без подтверждений."
    ;;
  pre-market)
    PROMPT="Выполни пре-маркет проверку за $DATE по инструкции из system-prompt.md: 1) Проверь здоровье всех сервисов (gateway, strategy-runner) 2) Проверь что все enabled стратегии запущены через GET /instances 3) Проверь PnL каждой стратегии за вчера 4) Запиши краткий отчёт в vault. Действуй автономно."
    ;;
  health)
    PROMPT="Быстрая проверка здоровья 24alert за $DATE: 1) GET /health на gateway и strategy-runner 2) GET /instances — убедись что все enabled стратегии running 3) Если что-то не так — запиши в vault и попробуй перезапустить через API. Кратко."
    ;;
  *)
    echo "Unknown scan kind: $KIND (expected: post-market | pre-market | health)" >&2
    exit 2
    ;;
esac

export CURSOR_API_KEY

AGENT_MODEL="${AI_SCANNER_MODEL:-composer-2}"

: > "$MSG_FILE"
set +e
timeout 1800 agent -p --force --approve-mcps --trust \
  --workspace "$WS" \
  --model "$AGENT_MODEL" \
  --output-format text \
  "$PROMPT" >> "$MSG_FILE" 2>&1
EXIT=$?
set -e

END_TS=$(date +%s)
DUR=$((END_TS - START_TS))
MSG_BYTES=$(wc -c < "$MSG_FILE" 2>/dev/null || echo 0)

if [ "$EXIT" -eq 0 ]; then
  echo "[$(date -Iseconds)] ai-scanner kind=$KIND exit=0 dur=${DUR}s bytes=$MSG_BYTES status=ok"
else
  echo "[$(date -Iseconds)] ai-scanner kind=$KIND exit=$EXIT dur=${DUR}s bytes=$MSG_BYTES status=FAILED log=$MSG_FILE" >&2
  head -c 2000 "$MSG_FILE" 2>/dev/null || true
  echo "" >&2
  exit 1
fi
