#!/bin/sh
# 24alert AI Scanner: runs Cursor Agent CLI for market analysis tasks.
# Usage: run-scan.sh post-market | pre-market | health

set -eu

KIND="${1:-health}"
DATE="$(date +%Y-%m-%d)"
MSG_FILE="/tmp/scanner-${KIND}-${DATE}.txt"
WS="/workspace"
METRICS_DIR="$WS/metrics"
METRICS_FILE="$METRICS_DIR/ai-scanner-$KIND.prom"

START_TS=$(date +%s)
mkdir -p "$METRICS_DIR" 2>/dev/null || true
cat > "$METRICS_FILE" <<EOF
alert24_ai_scanner_last_start_timestamp{kind="$KIND"} $START_TS
alert24_ai_scanner_run_in_progress{kind="$KIND"} 1
EOF
echo "[$(date -Iseconds)] ai-scanner kind=$KIND start"

fail_before_agent() {
  code="$1"
  reason="$2"
  END_TS=$(date +%s)
  DUR=$((END_TS - START_TS))
  cat > "$METRICS_FILE" <<EOF
alert24_ai_scanner_last_start_timestamp{kind="$KIND"} $START_TS
alert24_ai_scanner_last_failure_timestamp{kind="$KIND"} $END_TS
alert24_ai_scanner_run_duration_seconds{kind="$KIND"} $DUR
alert24_ai_scanner_last_exit_code{kind="$KIND"} $code
alert24_ai_scanner_last_response_bytes{kind="$KIND"} 0
alert24_ai_scanner_last_response_lines{kind="$KIND"} 0
alert24_ai_scanner_reports_total{kind="$KIND"} 0
alert24_ai_scanner_run_in_progress{kind="$KIND"} 0
alert24_ai_scanner_runs_total{kind="$KIND",status="failed"} 1
EOF
  echo "[$(date -Iseconds)] ai-scanner kind=$KIND FAILED reason=$reason" >&2
  exit "$code"
}

# Validate environment
if [ -z "${CURSOR_API_KEY:-}" ] || [ "${CURSOR_API_KEY}" = "stub" ]; then
  fail_before_agent 3 "missing_env CURSOR_API_KEY"
fi
if [ ! -d "$WS" ] || [ ! -f "$WS/system-prompt.md" ]; then
  fail_before_agent 3 "bad_workspace $WS"
fi
REF="$WS/reference/ai-scanner-reference.md"
LOG_SKILL="$WS/reference/log-reading-skill.md"
MEM="$WS/memory/agent-memory.md"
REPORTS="$WS/reports"
if [ ! -f "$REF" ]; then
  fail_before_agent 3 "missing_reference $REF"
fi
if [ ! -f "$LOG_SKILL" ]; then
  fail_before_agent 3 "missing_log_skill $LOG_SKILL"
fi
mkdir -p "$WS/memory" "$REPORTS" "$METRICS_DIR"
if [ ! -s "$MEM" ]; then
  cp "$WS/reference/agent-memory.seed.md" "$MEM"
fi

if ! command -v agent >/dev/null 2>&1; then
  fail_before_agent 4 "agent_not_found PATH=$PATH"
fi

case "$KIND" in
  post-market)
    PROMPT="Выполни пост-маркет анализ за $DATE по инструкции из system-prompt.md. Сначала прочитай справочник $REF и память $MEM. Затем: 1) Сканируй ТОЛЬКО фьючерсы с помощью scan_market.py, учитывай contract_price <= AI_SCANNER_MAX_CONTRACT_PRICE 2) Не отсекай кандидатов по score/atr_pct до бэктеста — score только ранжирует очередь 3) Для каждого futures-кандидата из результата сканера запусти бэктест sma и level_bounce с --optimize 4) Отбери только стратегии с Sharpe>1, PnL>0, win_rate>45%, trades>=5, profit_factor>1.3 5) Обнови config.yaml — добавляй только SPBFUT futures стратегии, убери/замени убыточные auto-стратегии 6) Вызови POST /config/reload на strategy-runner 7) Запиши отчёт в $REPORTS 8) Дополни $MEM краткой записью о решениях и метриках. Действуй автономно, без подтверждений."
    ;;
  pre-market)
    PROMPT="Выполни пре-маркет проверку за $DATE по инструкции из system-prompt.md. Сначала прочитай справочник $REF, skill логов $LOG_SKILL и память $MEM. Затем: 1) Проверь здоровье всех сервисов (gateway, strategy-runner) 2) Проверь что все enabled стратегии запущены через GET /instances 3) Проверь PnL каждой стратегии за вчера 4) Проверь journal events по каждой стратегии через /instances/<id>/events?limit=50 5) Запиши краткий отчёт в $REPORTS 6) При значимых событиях дополни $MEM. Действуй автономно."
    ;;
  health)
    PROMPT="Быстрая проверка здоровья 24alert за $DATE: сначала прочитай $REF, $LOG_SKILL и $MEM, затем 1) GET /health на gateway и strategy-runner 2) GET /instances — убедись что все enabled стратегии running 3) Проверь journal events через /instances/<id>/events?limit=20 4) Если что-то не так — запиши в $REPORTS, попробуй перезапустить через API и дополни $MEM. Кратко."
    ;;
  *)
    fail_before_agent 2 "unknown_kind $KIND"
    ;;
esac

export CURSOR_API_KEY
export AI_SCANNER_MAX_CONTRACT_PRICE="${AI_SCANNER_MAX_CONTRACT_PRICE:-10000}"

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
MSG_LINES=$(wc -l < "$MSG_FILE" 2>/dev/null || echo 0)
REPORT_COUNT=$(find "$REPORTS" -type f 2>/dev/null | wc -l | tr -d ' ')

if [ "$EXIT" -eq 0 ]; then
  cat > "$METRICS_FILE" <<EOF
alert24_ai_scanner_last_start_timestamp{kind="$KIND"} $START_TS
alert24_ai_scanner_last_success_timestamp{kind="$KIND"} $END_TS
alert24_ai_scanner_run_duration_seconds{kind="$KIND"} $DUR
alert24_ai_scanner_last_exit_code{kind="$KIND"} $EXIT
alert24_ai_scanner_last_response_bytes{kind="$KIND"} $MSG_BYTES
alert24_ai_scanner_last_response_lines{kind="$KIND"} $MSG_LINES
alert24_ai_scanner_reports_total{kind="$KIND"} $REPORT_COUNT
alert24_ai_scanner_run_in_progress{kind="$KIND"} 0
alert24_ai_scanner_runs_total{kind="$KIND",status="ok"} 1
EOF
  echo "[$(date -Iseconds)] ai-scanner kind=$KIND exit=0 dur=${DUR}s bytes=$MSG_BYTES status=ok"
else
  cat > "$METRICS_FILE" <<EOF
alert24_ai_scanner_last_start_timestamp{kind="$KIND"} $START_TS
alert24_ai_scanner_last_failure_timestamp{kind="$KIND"} $END_TS
alert24_ai_scanner_run_duration_seconds{kind="$KIND"} $DUR
alert24_ai_scanner_last_exit_code{kind="$KIND"} $EXIT
alert24_ai_scanner_last_response_bytes{kind="$KIND"} $MSG_BYTES
alert24_ai_scanner_last_response_lines{kind="$KIND"} $MSG_LINES
alert24_ai_scanner_reports_total{kind="$KIND"} $REPORT_COUNT
alert24_ai_scanner_run_in_progress{kind="$KIND"} 0
alert24_ai_scanner_runs_total{kind="$KIND",status="failed"} 1
EOF
  echo "[$(date -Iseconds)] ai-scanner kind=$KIND exit=$EXIT dur=${DUR}s bytes=$MSG_BYTES status=FAILED log=$MSG_FILE" >&2
  head -c 2000 "$MSG_FILE" 2>/dev/null || true
  echo "" >&2
  exit 1
fi
