#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

test -z "$(gofmt -l cmd internal content scripts/update-integrity.go)"
go vet ./...
go test ./...
go build ./cmd/mysetup
build_dir="$(mktemp -d)"
trap 'rm -rf "$build_dir"' EXIT
GOOS=windows GOARCH=amd64 go build -o "$build_dir/mysetup.exe" ./cmd/mysetup
go run ./cmd/mysetup catalog validate

if command -v gitleaks >/dev/null 2>&1; then
  gitleaks git --no-banner
else
  printf 'warning: gitleaks is not installed; CI secret scanning remains authoritative\n' >&2
fi
