#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_HOOK="${ROOT_DIR}/.githooks/pre-commit"
TARGET_HOOK="${ROOT_DIR}/.git/hooks/pre-commit"

if [[ ! -d "${ROOT_DIR}/.git/hooks" ]]; then
  echo "Git hooks directory not found. Run inside a git repository."
  exit 1
fi

if [[ ! -f "${SOURCE_HOOK}" ]]; then
  echo "Source hook missing: ${SOURCE_HOOK}"
  exit 1
fi

install -m 0755 "${SOURCE_HOOK}" "${TARGET_HOOK}"
echo "Installed pre-commit hook to ${TARGET_HOOK}"
