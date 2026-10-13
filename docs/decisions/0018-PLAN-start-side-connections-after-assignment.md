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
