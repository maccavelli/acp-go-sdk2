#!/usr/bin/env bash
# Go pre-add checks: format, lint, vet, tests, tidy, vulnerabilities.
#
# The single implementation of the pre-add rule in AGENTS.md, called from
# `make pre-add-check`.
# docs/decisions/0003-PLAN-install-fleet-agent-workspace.md
#
# Usage:
#   scripts/go-precheck.sh [file ...]
#
# Lint, vet, the tests, tidy and govulncheck always cover both modules whole:
# a change reaches tests in other packages, in cmd/generate, through
# subprocess builds and through files that are not Go, and go test's cache
# does not see the last three, so the tests run uncached, with -race and
# -shuffle=on as CI's race step runs them. Named files decide two things:
# gofmt checks the named .go files that exist, and when every named file is
# docs-safe (CI's paths-ignore, by scripts/check_ci_filters.py --docs-safe)
# no Go check runs. With no file named, gofmt checks every tracked and every
# untracked, not ignored .go file. ACPTEST_UPDATE is unset, so no golden file
# is rewritten
# (docs/decisions/0047-MADR-pre-add-check-full-coverage.md).
#
# Exit codes: 0 all clear · 1 a check failed · 2 a required tool is missing,
# or the named files could not be classified.
#
# Env:
#   GOLANGCI_LINT=<path>      golangci-lint binary (default: $(go env GOPATH)/bin)
#   GO_PRECHECK_SKIP_VULN=1   skip govulncheck (offline work)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT" || exit 1

# A golden test given ACPTEST_UPDATE=1 rewrites its golden file instead of
# comparing it (acptest/golden.go).
unset ACPTEST_UPDATE

need() {
  command -v "$1" >/dev/null 2>&1 && return 0
  echo "go-precheck: $1 not found in PATH." >&2
  return 1
}

gofmt_files=()
if [ "$#" -gt 0 ]; then
  need python3 || exit 2
  docs_out="$(python3 -B scripts/check_ci_filters.py --docs-safe "$@" 2>&1)"
  case "$?" in
  0)
    echo "go-precheck: only docs-safe files named; no Go check runs."
    exit 0
    ;;
  1) ;;
  *)
    echo "go-precheck: could not tell whether the named files are docs-safe:" >&2
    printf '%s\n' "$docs_out" | sed 's/^/  /' >&2
    exit 2
    ;;
  esac
  for f in "$@"; do
    case "$f" in
    *.go) [ -f "$f" ] && gofmt_files+=("$f") ;;
    esac
  done
else
  while IFS= read -r f; do
    [ -n "$f" ] && [ -f "$f" ] && gofmt_files+=("$f")
  done < <(git ls-files --cached --others --exclude-standard -- '*.go')
fi

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

failed=0
fail() {
  [ "$failed" -lt "$1" ] && failed="$1"
}

need gofmt || exit 2
unformatted=""
if [ "${#gofmt_files[@]}" -gt 0 ]; then
  unformatted="$(gofmt -l "${gofmt_files[@]}")"
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

# check_module vets and tests the module in dir whole. The tests run with
# -shuffle=on, which go test does not cache, so no result recorded before a
# change answers for it; a failure prints the shuffle seed and keeps the whole
# output in a temporary file.
check_module() {
  local dir="$1" vet_out test_out log
  if ! vet_out="$(cd "$dir" && go vet ./... 2>&1)"; then
    echo "go vet ($dir):" >&2
    printf '%s\n' "$vet_out" | sed 's/^/  /' >&2
    fail 1
  fi
  if ! test_out="$(cd "$dir" && GOEXPERIMENT="$test_experiment" go test -race -shuffle=on ./... 2>&1)"; then
    echo "go test -race -shuffle=on ($dir):" >&2
    printf '%s\n' "$test_out" | tail -40 | sed 's/^/  /' >&2
    printf '%s\n' "$test_out" | grep -E '^-test\.shuffle ' | sed 's/^/  seed: /' >&2
    if log="$(mktemp "${TMPDIR:-/tmp}/go-precheck-test.XXXXXX")"; then
      printf '%s\n' "$test_out" >"$log"
      echo "  the whole output: $log" >&2
    fi
    fail 1
  fi
}

check_module .
check_module cmd/generate

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
  ran="both modules linted, vetted and tested (-race -shuffle=on, uncached), tidy"
  if [ -n "$vuln_skipped" ]; then
    echo "go-precheck: ${#gofmt_files[@]} file(s) gofmt-clean; ${ran}; govulncheck skipped ($vuln_skipped)."
  else
    echo "go-precheck: ${#gofmt_files[@]} file(s) gofmt-clean; ${ran}, govulncheck."
  fi
fi
exit "$failed"
