#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MAIRU_DIR="${ROOT_DIR}/mairu"

# ui/embed.go uses //go:embed all:dist — the directory must exist and contain at least one file
# or go vet / go test fail. CI runs `bun run build` first; locally we create a tiny stub if needed.
ensure_ui_embed_dist() {
  local d="${MAIRU_DIR}/ui/dist"
  mkdir -p "${d}"
  local n
  n=$(find "${d}" -mindepth 1 -maxdepth 1 2>/dev/null | wc -l | tr -d ' ')
  if [[ "${n}" -eq 0 ]]; then
    printf '%s\n' '<!doctype html><meta charset="utf-8"><title>mairu</title><!-- stub: run bun run build in mairu/ui -->' >"${d}/index.html"
  fi
}

list_go_files() {
  git -C "${ROOT_DIR}" ls-files -- "*.go"
}

fmt_go() {
  echo "Formatting Go packages..."
  go_files=()
  while IFS= read -r file; do
    go_files+=("${file}")
  done < <(list_go_files)
  if [[ ${#go_files[@]} -gt 0 ]]; then
    (cd "${ROOT_DIR}" && gofmt -w "${go_files[@]}")
  fi
}

fmt_check_go() {
  go_files=()
  while IFS= read -r file; do
    go_files+=("${file}")
  done < <(list_go_files)
  if [[ ${#go_files[@]} -eq 0 ]]; then
    echo "No tracked Go files found."
    return 0
  fi

  unformatted=()
  while IFS= read -r file; do
    unformatted+=("${file}")
  done < <(cd "${ROOT_DIR}" && gofmt -l "${go_files[@]}")
  if [[ ${#unformatted[@]} -gt 0 ]]; then
    echo "Go formatting check failed. Run: make fmt-go"
    printf ' - %s\n' "${unformatted[@]}"
    return 1
  fi

  echo "Go formatting is clean."
}

lint_go() {
  ensure_ui_embed_dist
  if command -v golangci-lint >/dev/null 2>&1; then
    echo "Linting with golangci-lint..."
    (cd "${MAIRU_DIR}" && golangci-lint run ./...)
    return 0
  fi

  echo "golangci-lint not found, falling back to go vet."
  echo "Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
  (cd "${MAIRU_DIR}" && go vet ./...)
}

test_go() {
  ensure_ui_embed_dist
  echo "Running Go tests..."
  (cd "${MAIRU_DIR}" && go test ./...)
}

test_race_go() {
  ensure_ui_embed_dist
  echo "Running Go tests with race detector..."
  (cd "${MAIRU_DIR}" && go test -race ./...)
}

coverage_go() {
  ensure_ui_embed_dist
  echo "Running Go coverage..."
  (
    cd "${MAIRU_DIR}"
    go test -coverprofile=coverage.out ./...
    go tool cover -func=coverage.out
  )
}

check_go() {
  fmt_check_go
  lint_go
  test_go
}

check_ci_go() {
  fmt_check_go
  lint_go
  test_race_go
}

usage() {
  cat <<'EOF'
Usage: go-dev.sh <command>

Commands:
  fmt        Format Go code
  fmt-check  Verify gofmt cleanliness
  lint       Run golangci-lint (or fallback go vet)
  test       Run go test ./...
  test-race  Run go test -race ./...
  coverage   Run go test coverage and report
  check      Run fmt-check + lint + tests
  check-ci   Run fmt-check + lint + race tests
EOF
}

case "${1:-}" in
  fmt)
    fmt_go
    ;;
  fmt-check)
    fmt_check_go
    ;;
  lint)
    lint_go
    ;;
  test)
    test_go
    ;;
  test-race)
    test_race_go
    ;;
  coverage)
    coverage_go
    ;;
  check)
    check_go
    ;;
  check-ci)
    check_ci_go
    ;;
  *)
    usage
    exit 1
    ;;
esac
