#!/usr/bin/env bash
# Go pre-add checks: format, lint, vet, tests, tidy, vulnerabilities.
#
# The single implementation of the pre-add rule in AGENTS.md, called from
# `make pre-add-check`.
# docs/decisions/0003-PLAN-install-fleet-agent-workspace.md
#
# Usage:
#   scripts/go-precheck.sh [file.go ...]
#
# With no arguments it checks every tracked Go file. Non-Go arguments are
# ignored. Lint, tidy, and govulncheck always cover both modules. Vet and
# test cover the packages of the named files, or both modules when no files
# are named.
#
# Exit codes: 0 all clear · 1 a check failed · 2 a required tool is missing.
#
# Env:
#   GOLANGCI_LINT=<path>      golangci-lint binary (default: $(go env GOPATH)/bin)
#   GO_PRECHECK_SKIP_VULN=1   skip govulncheck (offline work)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT" || exit 1

# On Go 1.26 the root tests' leak guard needs the goroutineleak profile, an
# experiment there that Go 1.27 rejects, so the tests set it on 1.26 only.
# golangci-lint, built with Go 1.27, rejects it, so nothing else does
# (docs/decisions/0024-PLAN-go-1-26-9-floor.md, G5).
test_experiment="${GOEXPERIMENT:-}"
case "$(go env GOVERSION)" in
go1.26*)
  case ",${test_experiment}," in
  *,goroutineleakprofile,*) ;;
  *) test_experiment="${test_experiment:+$test_experiment,}goroutineleakprofile" ;;
  esac
  ;;
esac

files=()
deleted_dirs=()
if [ "$#" -gt 0 ]; then
  for f in "$@"; do
    case "$f" in
    *.go)
      if [ -f "$f" ]; then
        files+=("$f")
      else
        deleted_dirs+=("$(dirname "$f")")
      fi
      ;;
    esac
  done
else
  while IFS= read -r f; do
    [ -n "$f" ] && files+=("$f")
  done < <(git ls-files '*.go')
fi

if [ "${#files[@]}" -eq 0 ] && [ "${#deleted_dirs[@]}" -eq 0 ]; then
  echo "go-precheck: no Go files to check."
  exit 0
fi

need() {
  command -v "$1" >/dev/null 2>&1 && return 0
  echo "go-precheck: $1 not found in PATH." >&2
  return 1
}

failed=0
fail() {
  [ "$failed" -lt "$1" ] && failed="$1"
}

need gofmt || exit 2
unformatted=""
if [ "${#files[@]}" -gt 0 ]; then
  unformatted="$(gofmt -l "${files[@]}")"
fi
if [ -n "$unformatted" ]; then
  echo "gofmt: these files are not formatted (run 'gofmt -w <file>'):" >&2
  printf '%s\n' "$unformatted" | sed 's/^/  /' >&2
  fail 1
fi

need go || exit 2
GOLANGCI="${GOLANGCI_LINT:-$(go env GOPATH)/bin/golangci-lint}"
lint_module() {
  local goos="$1" dir="$2" cfg="$3"
  local lint_out
  if ! lint_out="$(cd "$dir" && GOOS="$goos" "$GOLANGCI" run -c "$cfg" ./... 2>&1)"; then
    echo "golangci-lint (GOOS=$goos $dir):" >&2
    printf '%s\n' "$lint_out" | tail -40 | sed 's/^/  /' >&2
    fail 1
  fi
}
if [ -x "$GOLANGCI" ]; then
  for lint_goos in "$(go env GOOS)" windows; do
    lint_module "$lint_goos" "." ".golangci.yml"
    lint_module "$lint_goos" "cmd/generate" "../../.golangci.yml"
  done
else
  echo "go-precheck: golangci-lint not found at $GOLANGCI." >&2
  echo "  install: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0" >&2
  fail 2
fi

module_of() {
  case "$1" in
  cmd/generate | cmd/generate/*) printf '%s\n' cmd/generate ;;
  *) printf '%s\n' . ;;
  esac
}

pkg_of() {
  local f="$1" mod dir rel
  mod="$(module_of "$f")"
  dir="$(dirname "$f")"
  if [ "$mod" = . ]; then
    if [ "$dir" = . ]; then
      printf '%s\n' .
    else
      printf '%s\n' "./$dir"
    fi
    return
  fi
  rel="${dir#cmd/generate}"
  rel="${rel#/}"
  if [ -z "$rel" ]; then
    printf '%s\n' .
  else
    printf '%s\n' "./$rel"
  fi
}

check_module() {
  local dir="$1"
  shift
  local vet_out test_out
  if ! vet_out="$(cd "$dir" && go vet "$@" 2>&1)"; then
    echo "go vet ($dir):" >&2
    printf '%s\n' "$vet_out" | sed 's/^/  /' >&2
    fail 1
  fi
  if ! test_out="$(cd "$dir" && GOEXPERIMENT="$test_experiment" go test -race "$@" 2>&1)"; then
    echo "go test -race ($dir):" >&2
    printf '%s\n' "$test_out" | tail -40 | sed 's/^/  /' >&2
    fail 1
  fi
}

if [ "$#" -eq 0 ]; then
  check_module . ./...
  check_module cmd/generate ./...
else
  root_pkgs=""
  gen_pkgs=""
  root_whole=0
  gen_whole=0
  consider() {
    local d="$1" mod
    [ -n "$d" ] || return 0
    mod="$(module_of "$d")"
    if [ "$mod" = cmd/generate ]; then
      if compgen -G "$d/*.go" >/dev/null; then
        gen_pkgs="${gen_pkgs} $(pkg_of "$d/x.go")"
      else
        gen_whole=1
      fi
    else
      if compgen -G "$d/*.go" >/dev/null; then
        root_pkgs="${root_pkgs} $(pkg_of "$d/x.go")"
      else
        root_whole=1
      fi
    fi
  }
  if [ "${#files[@]}" -gt 0 ]; then
    for f in "${files[@]}"; do
      consider "$(dirname "$f")"
    done
  fi
  if [ "${#deleted_dirs[@]}" -gt 0 ]; then
    for d in "${deleted_dirs[@]}"; do
      consider "$d"
    done
  fi
  if [ "$root_whole" -eq 1 ]; then
    check_module . ./...
  elif [ -n "$root_pkgs" ]; then
    # shellcheck disable=SC2086
    check_module . $(printf '%s\n' $root_pkgs | sort -u)
  fi
  if [ "$gen_whole" -eq 1 ]; then
    check_module cmd/generate ./...
  elif [ -n "$gen_pkgs" ]; then
    # shellcheck disable=SC2086
    check_module cmd/generate $(printf '%s\n' $gen_pkgs | sort -u)
  fi
fi

tidy_module() {
  local dir="$1" tidy_out
  if ! tidy_out="$(cd "$dir" && go mod tidy -diff 2>&1)"; then
    echo "go mod tidy -diff ($dir):" >&2
    printf '%s\n' "$tidy_out" | tail -40 | sed 's/^/  /' >&2
    fail 1
  fi
}
tidy_module .
tidy_module cmd/generate

vuln_pin="$(grep -oE 'golang.org/x/vuln/cmd/govulncheck@v[0-9][0-9.]*' .github/workflows/ci.yml 2>/dev/null | head -1)"
vuln_skipped=""
run_vuln() {
  local dir="$1" vuln_out vuln_rc
  vuln_out="$(cd "$dir" && go run "$vuln_pin" ./... 2>&1)"
  vuln_rc=$?
  if [ "$vuln_rc" -eq 3 ]; then
    echo "govulncheck ($dir): vulnerabilities found:" >&2
    printf '%s\n' "$vuln_out" | tail -30 | sed 's/^/  /' >&2
    fail 1
  elif [ "$vuln_rc" -ne 0 ]; then
    if printf '%s' "$vuln_out" | grep -qiE 'no such host|connection refused|timeout|dial tcp|proxy'; then
      echo "govulncheck ($dir): could not reach the vulnerability database; skipped." >&2
      vuln_skipped="offline"
    else
      echo "govulncheck ($dir): failed (exit $vuln_rc):" >&2
      printf '%s\n' "$vuln_out" | tail -30 | sed 's/^/  /' >&2
      fail 1
    fi
  fi
}
if [ "${GO_PRECHECK_SKIP_VULN:-0}" = "1" ]; then
  echo "govulncheck: skipped (GO_PRECHECK_SKIP_VULN=1)" >&2
  vuln_skipped="GO_PRECHECK_SKIP_VULN=1"
elif [ -z "$vuln_pin" ]; then
  echo "govulncheck: no pin in .github/workflows/ci.yml" >&2
  fail 2
else
  run_vuln .
  run_vuln cmd/generate
fi

if [ "$failed" -eq 0 ]; then
  if [ -n "$vuln_skipped" ]; then
    echo "go-precheck: ${#files[@]} file(s) clean (gofmt, golangci-lint, go vet, go test); govulncheck skipped ($vuln_skipped)."
  else
    echo "go-precheck: ${#files[@]} file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)."
  fi
fi
exit "$failed"
