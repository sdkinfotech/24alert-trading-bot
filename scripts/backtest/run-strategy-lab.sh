#!/usr/bin/env bash
# Full Strategy Lab: matrix → report → walk-forward (core3 futures).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:18080}"
DAYS="${DAYS:-90}"
DATE_TAG="$(date -u +%Y%m%d)"
REPORTS="${ROOT}/reports"
MATRIX_JSON="${REPORTS}/strategy-matrix-${DATE_TAG}.json"
WF_JSON="${REPORTS}/strategy-walkforward-${DATE_TAG}.json"
MD_OUT="${REPORTS}/strategy-matrix-ru-${DATE_TAG}.md"

mkdir -p "${REPORTS}"
export PYTHONPATH="${ROOT}/scripts:${PYTHONPATH:-}"

echo "=== Strategy Lab (gateway=${GATEWAY_URL}, days=${DAYS}) ==="

python3 "${ROOT}/scripts/backtest/strategy-matrix.py" \
  --gateway-url "${GATEWAY_URL}" \
  --tickers BMM6,NGM6,MCM6 \
  --days "${DAYS}" \
  --json-out "${MATRIX_JSON}" \
  > /dev/null

python3 "${ROOT}/scripts/backtest/strategy-report.py" \
  "${MATRIX_JSON}" \
  --out "${MD_OUT}"

python3 "${ROOT}/scripts/backtest/strategy-pick-deployable.py" "${MATRIX_JSON}"

python3 "${ROOT}/scripts/backtest/strategy-walk-forward.py" \
  "${MATRIX_JSON}" \
  --gateway-url "${GATEWAY_URL}" \
  --top 3 \
  --json-out "${WF_JSON}"

echo ""
echo "Artifacts:"
echo "  ${MATRIX_JSON}"
echo "  ${MD_OUT}"
echo "  ${WF_JSON}"
