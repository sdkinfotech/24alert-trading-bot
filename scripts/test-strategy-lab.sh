#!/usr/bin/env bash
# Unit tests for Strategy Lab analyzer and apply gates.
set -euo pipefail
cd "$(dirname "$0")/.."
echo "== go test strategy lab =="
go test ./internal/strategy/ -run 'Lab|StrategyLab' -count=1
echo "== python syntax backtest =="
python3 -m py_compile scripts/ai-scanner/backtest.py
echo "OK"
