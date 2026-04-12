#!/usr/bin/env bash
# Ensure every AST testdata *.input.* file has a matching *.approved.{json,md} file.
# Logic lives in mairu/internal/ast/approved_test.go (TestFixtureCompleteness).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
MAIRU_DIR="${ROOT_DIR}/mairu"

if [[ ! -f "${MAIRU_DIR}/go.mod" ]]; then
  echo "check-fixtures: cannot find ${MAIRU_DIR}/go.mod" >&2
  exit 1
fi

cd "${MAIRU_DIR}"
exec go test ./internal/ast/... -run '^TestFixtureCompleteness$' -count=1
