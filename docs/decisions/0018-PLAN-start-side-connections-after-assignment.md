---
status: in-progress
date: 2026-10-13
associated-madr: "0018-MADR-start-side-connections-after-assignment.md"
---

# Implement the side connections' start after assignment

Associated MADR: [0018-MADR-start-side-connections-after-assignment.md](0018-MADR-start-side-connections-after-assignment.md)

## Goal

`NewAgentSideConnection` and `NewClientSideConnection` assign their `Connection` before it starts reading, so a peer that writes at once
causes no data race and no nil-pointer panic (issue #3). The exported API and `NewConnection`'s behaviour do not change. The fix ships as
`v0.17.1`.

## Scope

In scope:

* `connection.go`: `newConnection`, `(*Connection).start`, `NewConnection` built from them, and its documentation.
* `cmd/generate/internal/emit/dispatch.go`: `emitSideConnection`'s constructor body.
* `agent_gen.go`, `client_gen.go`: regenerated.
* `cmd/generate/main_test.go`: the hand-written stub gains `newConnection` and `start`, so the generator's type check sees what the
  generated constructors call.
* A new root test file, `construct_test.go`.
* `docs/architecture.md`: one sentence on the construction order.
* `docs/reports/0018-REPORT-release-notes-v0-17-1.md` and `docs/README.md`.

Out of scope: any other change to `Connection`'s goroutines; the exported API; the generator's engine cases (their `want.api` lists
signatures, which do not change, and their behaviour tests run without `-race`).

## Implementation Steps

1. **Records.** This pair, indexed in `docs/README.md`. Gates: `make check-records`, `make markdownlint`, the identifier scan.
   The owner commits.
2. **Runtime.** In `connection.go`:
   * move the body of `NewConnection` up to its four `go` statements into `func newConnection(handler MethodHandler, peerInput io.Writer,
     peerOutput io.Reader, opts ...ConnectionOption) *Connection`;
   * move the four `go` statements, unchanged and in the same order, into `func (c *Connection) start()`;
   * make `NewConnection` `c := newConnection(...); c.start(); return c`;
   * add to `NewConnection`'s documentation that the handler may be called before `NewConnection` returns, so it must not read a
     variable assigned from the result;
   * change the `logger` field's comment from "which NewConnection starts" to "which start starts".
3. **Generator.** In `emitSideConnection`, emit `x.conn = newConnection(x.handleWithExtensions, peerInput, peerOutput, opts...)`,
   then `x.conn.start()`, then `return x`. Add `newConnection` and `start` to the stub in `cmd/generate/main_test.go`. Regenerate with
   `cd cmd/generate && env -u GOPATH -u GOMODCACHE go run .`; the diff of `agent_gen.go` and `client_gen.go` is exactly those two lines in
   each constructor.
4. **Test.** `construct_test.go` with `TestConstructorAssignsBeforeReading`, two subtests:
   * agent: 100 times, an input that already holds an `initialize` line, an output that signals the first line written; build with
     `NewAgentSideConnection`, wait for the answer, check it is a result;
   * client: 100 times, an input that already holds an `fs/read_text_file` request; build with `NewClientSideConnection`, wait for the
     answer, check it is the `-32601` that a request before `initialize` gets.

   Seen to fail: in a scratch copy, revert the generated constructors to the old order (assign from `NewConnection`) and run
   `go test -race -run TestConstructorAssignsBeforeReading`; it must report a `DATA RACE` on each side. Record the output. Also run it
   there without `-race` and record the result, whether it fails or not.
5. **Docs.** `docs/architecture.md`: in the paragraph on the side connections, that the constructors assign every field, the
   `Connection` included, before calling `start`.
6. **Release notes.** `docs/reports/0018-REPORT-release-notes-v0-17-1.md`, shaped like
   `docs/reports/0017-REPORT-release-notes-v0-17-0.md`: one Fixes entry, the affected versions, the commits, the API listing (unchanged).
   Indexed in `docs/README.md`.
7. **Verification and close.** The gates below, the Windows run, the execution record; after CI passes on the pushed commit, the PLAN
   and its index row become `complete`. The owner pushes, tags `v0.17.1` and publishes the release; the issue is closed with a reference
   to it.

Steps 2 to 4 land in one commit, because the generator's output, its stub and the runtime must agree for either module's tests to pass.

## Verification

For each code step:

* regenerate, and `git diff --exit-code` shows no change beyond the step's;
* `make pre-add-check` (it runs `go test -race`), and `GO_PRECHECK_SKIP_VULN=1 scripts/go-precheck.sh` on new `.go` files;
* `GOEXPERIMENT=nojsonv2 go test ./...` at the root;
* `GOOS=windows go vet ./...` at the root and in `cmd/generate`;
* `go test -race -count=50 -run TestConstructorAssignsBeforeReading .`, the issue's reproduction count;
* `make check-records`, `make markdownlint`, and the identifier scan of changed files;
* the exported API listing against `v0.17.0`: no difference;
* `go test -count=1 -shuffle=on ./...` at the root and in `cmd/generate` on Windows.

## Rollout and Rollback

`v0.17.1` is a patch: no exported identifier changes, and `NewConnection` behaves as before. Rollback is reverting the step 2 to 4
commit, which brings the race back; a consumer pinned to `v0.17.1` can return to `v0.17.0` with no code change.

## Execution record

### Step 1, records, 2026-10-13

The owner accepted the MADR and approved the PLAN on 2026-10-13. This pair is indexed in `docs/README.md`, the MADR `accepted` and
the PLAN `in-progress`. `make check-records` and `make markdownlint` exit 0; the identifier scan of the three files finds no local
account name, hostname, real-machine path or downstream project name.

### Steps 2 to 4 deviation, 2026-10-13: the builder is `buildConnection`

Found: `make pre-add-check` failed in both modules' lint runs with `connection.go:429:6: confusing-naming: Method 'newConnection' differs
only by capitalization to function 'NewConnection' in the same source file (revive)`. The name came from this PLAN, so the failure is new,
not pre-existing. Decision (owner, 2026-10-13): the unexported builder is `buildConnection`, in `connection.go`, the generator, the stub in
`cmd/generate/main_test.go` and the regenerated files. Where steps 2 and 3 say `newConnection`, read `buildConnection`. A `nolint`
directive was not considered, since it would only silence the check. The MADR's outcome 1 carries the same amendment.

### Steps 2 to 4 deviation, 2026-10-13: a generator test asserts the constructor

Found: `TestSideConnectionsFollowTheModel` in `cmd/generate/internal/emit/dispatch_test.go:192`, which the scope did not list, expects
the old text `asc.conn = NewConnection(asc.handleWithExtensions, peerInput, peerOutput, opts...)\n\treturn asc\n}\n` and failed in both
its cases ("with prompt turns", "without"). Decision (owner, 2026-10-13): `dispatch_test.go` joins step 3's scope. Its agent expectation
becomes the builder line, `asc.conn.start()`, then `return asc`, and the client constructor, not asserted before, gains the same
expectation, so the generator's own test pins the order the MADR decides.

### Steps 2 to 4, runtime, generator and test, 2026-10-13

What was built:

* `connection.go`: `NewConnection` is `buildConnection`, then `start`, then `return`. `buildConnection` holds the old body up to the
  `go` statements; `start` holds the four `go` statements, unchanged and in the same order. `NewConnection`'s documentation says
  reading starts before it returns, so the handler must not read a variable assigned from the result. The `logger` field's comment
  names `start`.
* `cmd/generate/internal/emit/dispatch.go`: `emitSideConnection` emits `x.conn = buildConnection(...)`, then `x.conn.start()`, then
  `return x`, with a comment citing the MADR.
* `cmd/generate/main_test.go`: the stub declares `buildConnection` and `(*Connection).start`.
* `cmd/generate/internal/emit/dispatch_test.go`: as the deviation entry above records.
* `agent_gen.go`, `client_gen.go`: regenerated. The diff of each is the one assignment changed to `buildConnection` and one
  `start()` line added. Regenerating again changes nothing.
* `construct_test.go`: `TestConstructorAssignsBeforeReading`, as step 4 describes. The input is an `io.Pipe` whose line is written by a
  goroutine started before the constructor, so the line is waiting when reading starts. The output hands over the first line written.
  The client's request carries `testAbsPath("x")`: the first Windows run failed with `-32602` (`"/x" is not an absolute path`), because
  validation runs before the gate and a POSIX literal is not absolute there. That was a defect in the new test's input, fixed before
  any record of the test passing.

Seen to fail, each in a scratch copy of the finished tree, never in the tree:

* Generated constructors back to the old order (assign from `NewConnection`, no `start`): `go test -race -count=1 -run
  TestConstructorAssignsBeforeReading` exits 1 with 2 `DATA RACE` reports, read at `agent_gen.go:214` and `client_gen.go:115`, written
  at `agent_gen.go:33` and `client_gen.go:27`, and `--- FAIL` on both subtests ("race detected during execution of test"). Rerun after
  the Windows fix, with the same result.
* The same copy without `-race`, `-count=20` (4000 constructions): exit 0. As the MADR's consequences say, this test alone catches the
  old order only under `-race`.
* The generator emitting the old order (`NewConnection`, no `start` call): `go test -run TestSideConnectionsFollowTheModel
  ./internal/emit` exits 1, with the four expectations ("with prompt turns" and "without", agent and client) reported missing. This
  check needs no `-race`, so the generator's order is also pinned deterministically.

Verification on the tree:

* `make pre-add-check`: exit 0, "110 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)". Its first run failed as the
  two deviation entries above record.
* `GO_PRECHECK_SKIP_VULN=1 scripts/go-precheck.sh construct_test.go`: exit 0.
* `GOEXPERIMENT=nojsonv2 go test -count=1 ./...`: exit 0.
* `GOOS=windows go vet ./...`: exit 0 at the root and in `cmd/generate`.
* `go test -race -count=50 -run TestConstructorAssignsBeforeReading .`: exit 0 (10000 constructions).
* The exported API listing of `v0.17.0` and of the tree: 2411 lines each, no difference.
* Windows: `go test -count=1 -shuffle=on ./...` exit 0 at the root and in `cmd/generate`, and `-count=20` of the new test exit 0.

The owner committed steps 2 to 4 as `3a9b5c4`.

### Step 5, architecture, 2026-10-13

`docs/architecture.md`, in the list entry on the generated side connections: each constructor assigns every field, the `Connection`
from `buildConnection` included, before it calls `start`, so a message already waiting never reaches the dispatch with a field unset.

### Step 6, release notes, 2026-10-13

`docs/reports/0018-REPORT-release-notes-v0-17-1.md`, observed on `3a9b5c4`, indexed in `docs/README.md`: one fix, the
`NewConnection` documentation note, the generator's constructor order, the commits since `v0.17.0` (`b5f13fb`, `3a9b5c4`), an empty
schema diff and an unchanged API listing.

### Step 7, verification, 2026-10-13

No Go file changed after `3a9b5c4`, so the code verification recorded for steps 2 to 4, the Windows run included, stands for this
commit. For steps 5 and 6: `make check-records` and `make markdownlint` exit 0, and the identifier scan of `docs/architecture.md`,
`docs/README.md` and the report finds no local account name, hostname, real-machine path or downstream project name. The pre-push
disclosure guard, run over `b5f13fb` and `3a9b5c4` against `origin/main`, exits 0.

The close is in two stages, as for 0017: once the owner has pushed `main` and CI has passed on it, the PLAN and its `docs/README.md`
row become `complete` in a close commit, and then the owner tags `v0.17.1`, publishes the release from the report's "Release text",
and closes issue #3 with a reference to the release.
