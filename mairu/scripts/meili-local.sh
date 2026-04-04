#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TOOLS_DIR="${ROOT_DIR}/.tools/meilisearch"
DATA_DIR="${ROOT_DIR}/.data/meilisearch"
LOG_DIR="${ROOT_DIR}/.logs"
PID_FILE="${TOOLS_DIR}/meilisearch.pid"
VERSION_FILE="${TOOLS_DIR}/version.txt"
MEILI_BIN="${TOOLS_DIR}/meilisearch"
MEILI_VERSION="${MEILI_VERSION:-v1.12.3}"
MEILI_HOST="${MEILI_HOST:-127.0.0.1}"
MEILI_PORT="${MEILI_PORT:-7700}"
MEILI_URL="http://${MEILI_HOST}:${MEILI_PORT}"
MEILI_API_KEY="${MEILI_API_KEY:-contextfs-dev-key}"
HEALTH_URL="${MEILI_URL}/health"
LOG_FILE="${LOG_DIR}/meilisearch.log"

detect_asset_name() {
  local os
  local arch

  os="$(uname -s)"
  arch="$(uname -m)"

  case "${os}-${arch}" in
    Darwin-arm64)
      echo "meilisearch-macos-apple-silicon"
      ;;
    Darwin-x86_64)
      echo "meilisearch-macos-amd64"
      ;;
    Linux-aarch64|Linux-arm64)
      echo "meilisearch-linux-aarch64"
      ;;
    Linux-x86_64)
      echo "meilisearch-linux-amd64"
      ;;
    *)
      echo "Unsupported platform: ${os}-${arch}" >&2
      exit 1
      ;;
  esac
}

is_running() {
  if [[ ! -f "${PID_FILE}" ]]; then
    return 1
  fi

  local pid
  pid="$(cat "${PID_FILE}")"
  kill -0 "${pid}" >/dev/null 2>&1
}

download_meilisearch() {
  local asset_name
  local download_url
  asset_name="$(detect_asset_name)"
  download_url="https://github.com/meilisearch/meilisearch/releases/download/${MEILI_VERSION}/${asset_name}"

  mkdir -p "${TOOLS_DIR}"
  if [[ -x "${MEILI_BIN}" && -f "${VERSION_FILE}" ]] && [[ "$(cat "${VERSION_FILE}")" == "${MEILI_VERSION}" ]]; then
    return 0
  fi

  echo "Downloading Meilisearch ${MEILI_VERSION} (${asset_name})..."
  curl -fL "${download_url}" -o "${MEILI_BIN}"
  chmod +x "${MEILI_BIN}"
  printf "%s" "${MEILI_VERSION}" > "${VERSION_FILE}"
}

wait_for_health() {
  local retries=30
  local delay_seconds=1
  local i

  for ((i = 1; i <= retries; i++)); do
    if curl -fsS "${HEALTH_URL}" >/dev/null 2>&1; then
      return 0
    fi
    sleep "${delay_seconds}"
  done

  echo "Meilisearch did not become healthy in time. Check ${LOG_FILE}" >&2
  return 1
}

start_meilisearch() {
  mkdir -p "${DATA_DIR}" "${LOG_DIR}" "${TOOLS_DIR}"
  download_meilisearch

  if is_running; then
    echo "Meilisearch is already running at ${MEILI_URL}"
    return 0
  fi

  echo "Starting Meilisearch at ${MEILI_URL}"
  nohup "${MEILI_BIN}" \
    --http-addr "${MEILI_HOST}:${MEILI_PORT}" \
    --master-key "${MEILI_API_KEY}" \
    --db-path "${DATA_DIR}" \
    > "${LOG_FILE}" 2>&1 &

  local pid=$!
  printf "%s" "${pid}" > "${PID_FILE}"

  wait_for_health
  echo "Meilisearch is healthy at ${MEILI_URL}"
}

stop_meilisearch() {
  if ! is_running; then
    echo "Meilisearch is not running"
    rm -f "${PID_FILE}"
    return 0
  fi

  local pid
  pid="$(cat "${PID_FILE}")"
  echo "Stopping Meilisearch (pid ${pid})..."
  kill "${pid}" >/dev/null 2>&1 || true
  rm -f "${PID_FILE}"
}

status_meilisearch() {
  if is_running; then
    local pid
    pid="$(cat "${PID_FILE}")"
    echo "Meilisearch is running (pid ${pid}) at ${MEILI_URL}"
  else
    echo "Meilisearch is stopped"
  fi
}

clean_meilisearch() {
  stop_meilisearch
  rm -rf "${DATA_DIR}" "${LOG_DIR}/meilisearch.log"
  echo "Deleted local Meilisearch data and logs"
}

usage() {
  cat <<'EOF'
Usage: mairu/contextfs/scripts/meili-local.sh <command>

Commands:
  up      Download (if needed) and start Meilisearch locally
  down    Stop local Meilisearch
  status  Print Meilisearch status
  clean   Stop Meilisearch and delete local data/logs
EOF
}

main() {
  local cmd="${1:-}"
  case "${cmd}" in
    up)
      start_meilisearch
      ;;
    down)
      stop_meilisearch
      ;;
    status)
      status_meilisearch
      ;;
    clean)
      clean_meilisearch
      ;;
    *)
      usage
      exit 1
      ;;
  esac
}

main "${1:-}"
