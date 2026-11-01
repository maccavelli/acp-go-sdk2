---
status: proposed
date: 2026-11-01
associated-madr: "0047-MADR-pre-add-check-full-coverage.md"
---

# Implement 0047: the pre-add check tests both modules, uncached

Associated MADR: [0047-MADR-pre-add-check-full-coverage.md](0047-MADR-pre-add-check-full-coverage.md)

## Goal

`make pre-add-check` runs every test a change can reach, never from a stale cache: vet and `go test -race -shuffle=on` over both
modules whenever a named file is outside CI's docs-safe set, as the MADR's D1 to D7 state. `make test` stops reading the cache
too. The owner answered on 2026-11-01: Q1 (a), Q2 (a) and Q3 (a), and this PLAN runs before the 0046 PLAN's phase 8.

Base: `cc2a72d`. Every file and line named below is at that commit.

## Scope

In scope:

* `scripts/check_ci_filters.py`: a `--docs-safe PATH...` mode (D2), and its tests in `scripts/test_check_ci_filters.py`.
* `scripts/go-precheck.sh`: D1 to D4, and its header.
* `Makefile`: the `test` target (D6).
* `tools/gates.py`: the `newfiles` gate and the docstring (D5).
* `AGENTS.md`, `docs/architecture.md`, `docs/README.md`, and annotations in
  [0046-PLAN-sending-and-receiving-middleware.md](0046-PLAN-sending-and-receiving-middleware.md) (deviation 3) and
  [0036-PLAN-session-update-constructors-kind-and-client-refusals.md](0036-PLAN-session-update-constructors-kind-and-client-refusals.md)
  pointing here (D7).

Out of scope:

* any Go file, the exported API, a release;
* the other gates, `tools/floor.py`, `tools/remote.py` and CI, which already run uncached (MADR F5);
* Windows-only files, the floor toolchain and `GOEXPERIMENT=nojsonv2`, which stay their gates' (MADR, "Not decided here");
* the unexplained failure of MADR F8, beyond keeping the output of any failure met while executing this PLAN.

## Implementation Steps

Every phase ends staged for the owner, who commits. A phase begins only on the owner's word. Every new check is seen to fail on a
planted break, in a scratch clone or copy, never in the tree.

### Phase 1: `--docs-safe`

1. **`check_ci_filters.py --docs-safe PATH...`.**
   * Reads `.github/workflows/ci.yml` with the module's `load`, and takes `on.push.paths-ignore`.
   * Each path is made relative to `--root` and normalized lexically (`os.path.normpath`, no symlink resolution): a `./` prefix
     and `..` segments are resolved, an absolute path under the root is made relative, and a path outside the root is not
     docs-safe. Backslashes become slashes. The path need not exist: a deleted file is named too.
   * Exits 0 when there is at least one path and every one matches a pattern (`_matches`); 1 otherwise, printing the first path
     that does not; 2 when the workflow cannot be read, as `main` does today.
   * The docstring's list of parts gains the mode.
2. **Tests,** `scripts/test_check_ci_filters.py`, against the `CI` fixture's `docs/**` and `AGENTS.md`:
   * `docs/README.md` and `AGENTS.md`: safe;
   * `./docs/x.md`, and an absolute path under the root: safe;
   * `README.md`, `docs/../README.md`, `x/docs/y.md`, `connection.go`, and a path outside the root: not safe;
   * a mix of safe and unsafe: not safe; no path: not safe (exit 1);
   * this repository's own `ci.yml`: `docs/decisions/0047-MADR-pre-add-check-full-coverage.md` safe, `README.md` not.
3. **Seen to fail,** each in a scratch copy of `scripts/`, the unit tests run there:
   * the normalization dropped (paths matched as given): the `./` and absolute cases fail;
   * the empty list taken as safe: its case fails;
   * the patterns searched for rather than matched from the path's start (`glob_re(p).search`): `x/docs/y.md` fails;
   * `docs/../README.md` not normalized: its case fails.
4. **Checks:** `make check-tools`; the record, markdown and disclosure checks; the execution record; staging.

### Phase 2: the check, `make test` and the gate

1. **`scripts/go-precheck.sh`.**
   * Header (`:1-20`): what runs, D1 to D4, and this record.
   * `unset ACPTEST_UPDATE` before any command (D4).
   * The file list (`:40-63`): with files named, `gofmt_files` are the named `.go` files that exist, and when
     `python3 scripts/check_ci_filters.py --docs-safe "$@"` exits 0 it prints "go-precheck: only docs-safe files named; no Go
     check runs." and exits 0; exit 2 from it fails the check with 2. With no file named, `gofmt_files` are
     `git ls-files --cached --others --exclude-standard '*.go'` (D3).
   * `deleted_dirs`, `module_of`, `pkg_of`, `consider` and the selection (`:41`, `:49`, `:109-135`, `:153-200`) are removed.
     `check_module . ./...` and `check_module cmd/generate ./...` always run.
   * `check_module`'s test line (`:146`): `go test -race -shuffle=on` (Q2 a).
   * The last line names what ran: "go-precheck: N file(s) formatted; both modules linted, vetted and tested (-race -shuffle=on,
     uncached); tidy; govulncheck." and its form with govulncheck skipped.
   * `shellcheck`, where installed, is clean; the script stays bash, in the file's existing style.
   * Both modules run uncached, one after the other (Q1 a), and the docs-safe skip uses phase 1's mode (Q3 a).
2. **`Makefile`'s `test`** (`:62-65`): `go test -shuffle=on ./...` in both modules; the example build unchanged.
3. **`tools/gates.py`'s `newfiles`** (`:73-77`): `gofmt -l` on the untracked `.go` files, failing on any output; the docstring's
   line becomes "gofmt on the untracked .go files". `make check-tools` covers the module.
4. **The words** (D7): `AGENTS.md` "Pre-add checks"; `docs/architecture.md:697-705`; the 0046 PLAN's deviation 3 and its last
   paragraph, and the 0036 PLAN's entry, each gain a dated line pointing here, without rewriting what they say.
5. **Checks:** `make check-tools`; `make pre-add-check` itself, with no file named, then with `FILES=connection.go`, each exit 0,
   their output kept; the record, markdown and disclosure checks; the execution record; staging.

### Phase 3: confirmation

Each item of the MADR's Confirmation, in scratch clones of the tree with phase 2's commit, with the old script beside it where the
item compares them. A scratch script (`confirm.py` in the session's scratch directory) clones, plants, runs, and keeps each run's
output and exit status.

1. **Deviation 3's tree:** a clone at `f8e7ecb` with phase 2's script copied in, `FILES=connection.go`: the new check exits 1,
   naming `cmd/generate` and `TestEngineBehaviour`; the old script, in the same clone: exit 0.
2. **The example probe:** `example/client/main.go` printing `stopped, reason`, with a warm cache, `FILES=example/client/main.go`:
   exit 1, `TestExamplePair`; the old script: exit 0.
3. **`README.md` alone:** a change that breaks `TestReadmeMiddlewareIsTheExample`, `FILES=README.md`: exit 1; the old script: exit
   0, "no Go files to check".
4. **`go.mod` alone:** a `require` of a module the code does not import, `FILES=go.mod`: exit 1, from `go mod tidy -diff`.
5. **Docs-safe alone:** `FILES="docs/README.md AGENTS.md"`: exit 0, the docs-safe line, and no `go` command run (timed: under a
   second). `FILES="docs/README.md README.md"`: the checks run.
6. **No cache:** two runs in a row: no `(cached)` in either's output.
7. **The environment:** `ACPTEST_UPDATE=1 make pre-add-check`, then `git status --porcelain`: empty.
8. **A new file:** an untracked, unformatted `x.go` at the root, no file named: exit 1, gofmt naming it.
9. **The timing,** as the MADR's F7 measured it: the new check with `FILES=connection.go` and with none, recorded beside 50.1 s.
10. **Gates:** `tools/gates.py --all`, its output in the record.
11. **Push and CI,** on the owner's ask: `make push-guard` after a fetch; after the owner's push, `tools/watch_ci.py --branch
    main` on the head. Then this PLAN and its row `complete`, and the MADR's status with them.

## Verification

* `make pre-add-check` with any named file outside the docs-safe set runs vet and `go test -race -shuffle=on ./...` in both
  modules, uncached, and gofmt on the named `.go` files.
* Each of the MADR's Confirmation items 1 to 8 holds as phase 3 states, and the old script fails items 1 to 3 as stated.
* `--docs-safe`'s tests pass, and each was seen to fail.
* `make check-tools`, `tools/gates.py --all` and CI pass.

## Rollout and Rollback

* **Rollout.** Tooling only, for the agents and the owner; nothing a consumer sees. The first runs after the change take about two
  minutes here.
* **Rollback.** Revert phase 2's commit, then phase 1's; the old selection returns, with the gaps the MADR lists.
