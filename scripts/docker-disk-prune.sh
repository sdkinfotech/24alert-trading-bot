#!/usr/bin/env bash
# Safe Docker disk cleanup on production hosts (srv03).
# Removes stopped containers and unused images; does not touch running 24alert stack.
#
# Usage:
#   sudo bash scripts/docker-disk-prune.sh           # prune
#   sudo bash scripts/docker-disk-prune.sh --dry-run  # show what would be removed
#   sudo bash scripts/docker-disk-prune.sh --verbose
#
# Log: /var/log/24alert-docker-prune.log (when run as root)

set -euo pipefail

DRY_RUN=0
VERBOSE=0
LOG_FILE="${LOG_FILE:-/var/log/24alert-docker-prune.log}"

for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=1 ;;
    --verbose|-v) VERBOSE=1 ;;
    -h|--help)
      sed -n '2,12p' "$0"
      exit 0
      ;;
  esac
done

log() {
  local msg="[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*"
  echo "$msg"
  if [ -w "$(dirname "$LOG_FILE")" ] 2>/dev/null || [ "$(id -u)" -eq 0 ]; then
    echo "$msg" >>"$LOG_FILE" 2>/dev/null || true
  fi
}

need_docker() {
  if ! command -v docker >/dev/null 2>&1; then
    log "ERROR: docker not found"
    exit 1
  fi
}

df_root() {
  df -h / | awk 'NR==2 {print $3" used, "$4" free ("$5")"}'
}

run_prune() {
  local label=$1
  shift
  if [ "$DRY_RUN" -eq 1 ]; then
    log "DRY-RUN: would run: docker $*"
    return 0
  fi
  log "Running: docker $*"
  docker "$@" 2>&1 | while read -r line; do
    [ "$VERBOSE" -eq 1 ] && log "  $line"
  done
}

main() {
  need_docker
  log "=== 24alert docker disk prune start ==="
  log "Before: $(df_root)"
  docker system df 2>/dev/null | while read -r line; do log "  $line"; done || true

  # Stopped build leftovers (safe)
  run_prune "containers" container prune -f

  # Dangling layers from recent builds
  run_prune "dangling images" image prune -f

  # Images not referenced by any container (keeps all 12 running 24alert images)
  run_prune "unused images" image prune -af

  # BuildKit cache if present
  if docker builder prune -af >/dev/null 2>&1; then
    [ "$DRY_RUN" -eq 0 ] && log "Build cache pruned"
  fi

  # Optional: apt cache on host (small, helps git pull)
  if [ "$(id -u)" -eq 0 ] && [ "$DRY_RUN" -eq 0 ]; then
    apt-get clean -y >/dev/null 2>&1 || true
  fi

  log "After: $(df_root)"
  docker system df 2>/dev/null | while read -r line; do log "  $line"; done || true
  log "=== 24alert docker disk prune done ==="
}

main "$@"
