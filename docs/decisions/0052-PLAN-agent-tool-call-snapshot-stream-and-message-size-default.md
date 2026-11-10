---
status: in-progress
date: 2026-11-10
associated-madr: "0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md"
---

# Implement 0052: an agent's tool-call snapshot stream, and a 32 MiB default message size

Associated MADR: [0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md](0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md)

## Goal

Ship the MADR's O1 and M1 as `v1.35.0`. At the end of it:

* an agent starts a tool call with `StartToolCallStream`, writes its output to it as an `io.Writer`, changes it with
  `SetStatus` and `Update` without blocking, and ends it with `Finish`, which writes the final status and the whole state
  even after `session/cancel`, and honours the caller's deadline (D1);
* a stream's sender, counted by the connection's wait group and running only while the stream is active, writes the latest
  whole state at most once per 250 ms and coalesces under back-pressure (D2);
* the live text keeps a tail of at most 1 MiB, cut on a character boundary (D4);
* a turn's unfinished streams are written before its `session/prompt` answer, and turns do not wait on each other's (D3);
* the connection and `remote` read messages up to 32 MiB by default (D6).

## Scope

**Preconditions.**

* The MADR's questions 1 to 6 are answered as recommended, on 2026-11-10, and the MADR is `accepted`.
* The owner accepted what the MADR's two reviews of 2026-11-10 added, and approved this PLAN, on 2026-11-10.
* The pair and its `docs/README.md` rows are committed alone, as the bootstrap exception allows, on `957e60b` (`v1.34.2` plus
  the 0051 report). That commit is this PLAN's base. The base's tree is checked clean with `git status --short` before phase 1.

**Files changed.** Phase by phase:

| Phase | Files |
| :--- | :--- |
| 1, the message size (D6) | `connection.go`; `remote/client.go`; `remote/handler.go`; `jsonrpc_test.go`; `remote/` one test file; `README.md`; `docs/architecture.md` |
| 2, a start only while the connection lives (D2) | `connection.go`; `connection_end_test.go` |
| 3, the stream (D1, D2, D4, D5) | `internal/tailbuf/tailbuf.go`, `tailbuf_test.go` (new); `clienthost/terminals.go`; `clienthost/output.go`, `output_test.go` (deleted); `toolstream.go` (new); `errors.go`; `toolstream_test.go` (new); `example_toolstream_test.go` (new); `cmd/generate/engine_test.go` |
| 4, the turn's streams before the answer (D3) | `connection.go`; `toolstream.go`; `toolstream_test.go`; `cmd/generate/internal/emit/dispatch.go`; `cmd/generate/internal/emit/dispatch_test.go`; `agent_gen.go` (regenerated) |
| 5, the docs | `README.md`; `docs_test.go`; `docs/architecture.md`; `doc.go` |
| 6, the close | `tools/consumer.py`; `tools/test_tools.py`; `tools/README.md`; `docs/reports/0052-REPORT-release-notes-v1-35-0.md` (new); this PLAN; the MADR; `docs/README.md` |

Each phase also updates this PLAN's execution record and its row in `docs/README.md`. A file a phase needs that is not in its
row is a deviation: stop, record it, and ask.

**Not changed:** `go.mod`, `go.sum` and `cmd/generate/go.mod`, so no dependency; the schema; `.github/`; every existing
exported signature; `ToolCallTimeline`; `TestGoStatementsCounted`; what existing calls send on the wire.

**Out of scope:** an outbound size check; ACP v2 chunks; `example/agent` (the example test is the example); gobble-cli's and
magic-cli-remote's adoption, and the MADR's two observations for gobble-cli.

## Implementation Steps

**Planted breaks.** Every new test or check is first seen to fail on a planted break in a scratch copy, never in the tree:

* **In the root module,** a plant that edits a hand-written file runs through `python3 tools/plants.py SPEC.json`, one spec per
  phase, kept in the scratch directory, not committed.
* **In the generator,** a plant that needs a regeneration runs by a scratch script in a `git archive` copy: apply the edit,
  `cd cmd/generate && go run .`, then run the named tests, as
  [0050-PLAN-auth-method-ids-and-the-prompt-result-for-issue-14.md](0050-PLAN-auth-method-ids-and-the-prompt-result-for-issue-14.md)
  did. `TestEngineBehaviour` runs whole in such a plant, not filtered.
* For each plant, the execution record gives the edit, the command, its exit status and the failing test's line.

**End of each phase.**

1. `make pre-add-check FILES="<the phase's files>"`, with its output redirected to a scratch file and its exit status
   captured before any filter.
2. `make check-records`, `make markdownlint`, `make disclosure-check`; `make check-tools` when `tools/` or `scripts/` changed.
3. The execution record: what was done, each command with its exit status, each plant, and every deviation, dated.
4. What passed is staged. On `main` the agent commits only with the owner's explicit permission in that turn (`AGENTS.md`,
   "Commits"), with `git commit --no-edit`; otherwise the owner commits. The next phase starts from that commit.

### Phase 1: the message size, D6

1. **The constants.**
   * `connection.go:39`: `defaultMaxMessageSize = 32 * 1024 * 1024`. Its comment (`:37`-`:38`) gains "the TypeScript SDK's
     default too" and cites the MADR, D6.
   * `remote/client.go:13`: the same value; its comment (`:11`-`:12`) reads "the connection's own default, 32 MiB".
2. **The text that states the value.** Each of these, found by the review's search over the tracked files outside
   `docs/decisions/` and `docs/reports/`, changes from 10 MiB to 32 MiB:
   * `connection.go:721`, `WithMaxMessageSize`'s doc;
   * `remote/client.go:31` and `remote/handler.go:49`, the options' docs;
   * `README.md:992`-`:993`, "1024 notifications of up to 32 MiB each, about 32 GiB";
   * `README.md:1211`, `:1266` and `:1392`;
   * `docs/architecture.md:515`.

   The search is run again after the edit; it must find no 10 MiB default outside the records. Its output goes in the record.
3. **The tests that read the constant follow it unchanged:** `jsonrpc_test.go:239` (`TestMaxMessageSize`, "the default", which
   now sends a 32 MiB-plus-one line and a 32 MiB line), `options_test.go:43`, `remote/fakeserver_test.go:394` and
   `remote/httpserver_helpers_test.go:30`, `:201`. None asserts the value; step 4 does.
4. **New tests.**
   * `jsonrpc_test.go`, `TestDefaultMaxMessageSize`: `defaultMaxMessageSize == 32<<20`, and with no option a 20 MiB request
     line is read and answered `-32601` (method not found), through `rawWire` as `TestMaxMessageSize` does. Not parallel, to
     bound memory under `-race`. Plant: the constant at `10 * 1024 * 1024`; the test fails on both assertions.
   * `remote`, beside the existing large-message tests: `TestDefaultMaxMessageSize` asserts `remote`'s constant equals
     `32<<20`, and a 20 MiB message dialled with zero options crosses Streamable HTTP and WebSocket. Plant: `remote`'s
     constant at 10 MiB.
5. **Memory.** `TestMaxMessageSize`'s default case now allocates about 64 MiB per run, in parallel with its other case. The
   phase records `go test -race -run 'MaxMessageSize' -count=3 .` passing, with its time, on the host and the Linux test
   host. If either fails for memory or time, stop: that is a deviation.

### Phase 2: a start only while the connection lives, D2

1. **`connection.go`.** `Connection` gains `liveMu sync.Mutex` and `ended bool`, beside `goroutines` (`:429`), and two
   unexported methods, each commented with the argument below and citing the MADR, D2, and
   [0026-MADR-connection-end-cause-and-wait.md](0026-MADR-connection-end-cause-and-wait.md):

   ```go
   // goIfLive starts f on a goroutine the wait group counts when the
   // connection has not ended, and reports whether it did.
   func (c *Connection) goIfLive(f func()) bool

   // endLive marks the connection ended for goIfLive.
   func (c *Connection) endLive()
   ```

   `goIfLive` holds `liveMu`, returns false when `ended`, and calls `c.goroutines.Go(f)` under it. `endLive` sets `ended`
   under `liveMu`. A mutex, not an `atomic.Bool`: the check and the start must be one step against the mark. The MADR's
   second review gives why the anchor is `writeLoop` and not a watcher goroutine, the cancel sites or `context.AfterFunc`.
2. **`writeLoop`** (`:1041`) gains `defer c.endLive()` as its first statement. The argument: `writeLoop` is counted from
   `start` and returns only after the connection's context ends (MADR F8b); its deferred `endLive` runs before
   `goroutines.Go` counts it done. A `goIfLive` that reads `ended` false under `liveMu` therefore adds while `writeLoop` is
   counted, with the counter above zero.
3. **Tests,** `connection_end_test.go`:
   * `TestGoIfLiveAfterEnd`: after `Close` and `Wait`, `goIfLive` returns false and `f` never runs. Plant: the `ended` check
     removed; `f` runs.
   * `TestWriteLoopEndsLiveFirst`: parses `connection.go` and requires `writeLoop`'s first statement to be `defer c.endLive()`.
     Plants: the defer removed; the defer moved after the loop.
   * `TestGoIfLiveDuringEnd`: 32 goroutines the wait group does not count call `goIfLive` in a loop while the connection is
     closed and `Wait` runs, under `-race`; each `f` records that it ran. After `Wait` returns, every `f` that started has
     returned, and a further `goIfLive` returns false. Run with `-count=50`. Plant: the `ended` check removed; a call after
     `Wait` starts `f`.
   * `TestGoStatementsCounted` passes unchanged.

### Phase 3: the stream, D1, D2, D4, D5

1. **`internal/tailbuf`, new, from `clienthost/output.go` (D4).**
   * `internal/tailbuf/tailbuf.go`: `type Buffer struct` with an exported `Limit int` and unexported fields; `Write(p []byte)
     (int, error)`, which never fails and compacts once 64 KiB past the limit; `Snapshot(final bool) (text string, dropped
     int64)`, which cuts the head at a character boundary and, unless `final`, holds back an unfinished last character.
     `dropped` counts every byte cut from the head, compacted or cut in the snapshot. `cut`, `complete` and `compactAfter` move
     with it unchanged, and the package doc cites the schema's `outputByteLimit` rule and this MADR, D4.
   * `internal/tailbuf/tailbuf_test.go`: `clienthost/output_test.go`'s tests moved and adapted (`truncated` is `dropped > 0`),
     plus `TestDroppedCount`: the count equals the bytes written less the bytes kept and held back. Plant: compaction not
     adding to the count.
   * `clienthost/terminals.go`: `out *output` becomes `out *tailbuf.Buffer` (`:235`, `:269`); `:151` reads
     `text, dropped := term.out.Snapshot(exited)` and sets `resp.Truncated = dropped > 0`. `clienthost/output.go` and
     `clienthost/output_test.go` are deleted. `clienthost/terminals_test.go` passes unchanged: its truncation tests (`:101`,
     `:116`-`:147`) pin the behaviour across the move.
   * `make pre-add-check` covers `./internal/...`; `python3 tools/platforms.py` already runs over it.
2. **`errors.go`:** `ErrToolCallStreamFinished` and `ErrToolCallStreamField`, documented with the MADR's D1.
3. **`toolstream.go`, new: the API of the MADR's D1,** each declaration documented and citing the MADR. `StartToolCallStream`
   refuses a `start` without a `ToolCall` with an error wrapping `*ValidationError`, nothing sent. It builds the entity as
   `ToolCallTimeline.Apply` builds it from a `tool_call` (`tooltimeline.go:61`-`:75`). The options are
   `func(*toolCallStreamConfig)`, zero or less keeping each default (250 ms, 1 MiB).
4. **The stream's fields,** under `mu sync.Mutex` unless noted:
   * `entity ToolCallEntity`; `text tailbuf.Buffer`; `pending`, `running`, `finished bool`; `last time.Time`, the last write's
     end; `err error`, the first background error;
   * `slot chan struct{}`, capacity 1, outside `mu`: held while a snapshot is taken and written;
   * `conn *AgentSideConnection`, `base context.Context` (`context.WithoutCancel` of the start's context), `cfg`.
5. **The change path,** shared by `Write`, `WriteString`, `SetStatus` and `Update`: under `mu`, return
   `ErrToolCallStreamFinished` when `finished`; apply the change; set `pending`; when not `running`, set `running` and call
   `s.conn.Connection().goIfLive(s.run)`; when that refuses, clear `running` and keep `ErrConnectionClosed` in `err`.
   `Update` saves `Id` and `Status` before `f`, and after it restores either that changed and returns
   `fmt.Errorf("…: %s: %w", field, ErrToolCallStreamField)`. `SetStatus` refuses a status other than `pending` or
   `in_progress` with `ErrToolCallStreamField`.
6. **The sender, `run`:**

   ```go
   for {
       s.mu.Lock()
       if !s.pending || s.finished {
           s.running = false
           s.mu.Unlock()
           return
       }
       wait := s.cfg.interval - time.Since(s.last)
       s.mu.Unlock()
       if wait > 0 && !s.sleep(wait) { // a time.Timer, or the connection's end
           s.stop()                     // clears running under mu
           return
       }
       s.writePending(s.base)          // takes slot, snapshots, writes, releases
   }
   ```

   `writePending(ctx)` takes `slot` (or returns at the connection's end), then under `mu` takes the snapshot, clears
   `pending`, and releases `mu`. It writes with `s.conn.SessionUpdate(ctx, …)`, records the time in `last`, keeps the first
   error in `err` and sets `pending` again on an error, then releases `slot`.
7. **The snapshot,** taken under `mu`: one `tool_call_update` carrying `title`, `kind`, `status`, `name`, `content`
   (`deepCopy` of the entity's, then the text's `Snapshot(false)` as one text item when not empty; `[]` when empty),
   `locations` (`[]` when empty), and `rawInput` and `rawOutput` when set. `Finish`'s takes `Snapshot(true)`.
8. **The wrap-up context of `Flush` and `Finish`,** an unexported `wrapUp(ctx) (context.Context, context.CancelFunc)`:
   `context.WithCancelCause(context.WithoutCancel(ctx))`, and `context.AfterFunc(ctx, …)` that cancels it with
   `context.Cause(ctx)` unless that cause wraps `ErrPromptCancelled` or `ErrShutdown`. The returned cancel stops the
   `AfterFunc` and cancels. It is a pure function of `ctx`, tested alone.
9. **`Flush(ctx)`:** nil when nothing is pending; otherwise `wrapUp(ctx)`, take `slot` with a `select` on that context, write
   the pending state on it, release, and return the write's error. A context ended before the slot returns its cause and leaves
   `pending` set.
10. **`Finish(ctx, status)`:** refuse a status other than `completed` or `failed` with `ErrToolCallStreamField`; return
    `ErrToolCallStreamFinished` when finished; `wrapUp(ctx)`; take `slot`; under `mu`, save the status, set the new one, take
    the final snapshot; write it. On success, set `finished` under `mu`, remove the stream from its turn's set (phase 4), and
    release. On an error, restore the status, set `pending`, release, and return the error.
11. **Tests,** `toolstream_test.go`, through `acptest.Connect` with a recording client, each under `-race`. Timing tests run
    under `testing/synctest`, as `connection_overflow_block_test.go` and the other connection tests do. Same-package tests read
    `running` under `mu`; no package-level test hook.

    | Test | Requires | Plant |
    | :--- | :--- | :--- |
    | `TestToolCallStreamRoundTrip` and `FuzzToolCallStream` | scripted and fuzzed sequences of `Write`, `SetStatus`, `Update`, `Flush`, `Finish`: after `Finish`, the client's `ToolCallTimeline` entity equals `Entity()`, an empty and a nil collection counted equal | the live text left out of the snapshot |
    | `TestToolCallStreamExecWriter` | `cmd.Stdout = stream` for a command that prints 3 lines, then `Finish`: the client's text is the 3 lines | `Write` returning 0 |
    | `TestToolCallStreamCoalesces` | the client's handler blocks; 1,000 writes; after release, at most two writes follow, the last with all the text | a queue of snapshots in place of the pending flag |
    | `TestToolCallStreamInterval` | a burst within one interval gives at most two writes, the first at once | the interval wait removed |
    | `TestToolCallStreamIdle` | once the last write is done and nothing is pending, `running` is false; a later change sets it again and writes | the exit on nothing pending removed |
    | `TestToolCallStreamNoLostChange` | 10,000 rounds of a change racing the sender's exit, under `synctest`: every last change is written | `running` cleared outside the lock that checks `pending` |
    | `TestToolCallStreamFinishAfterCancel` | `Prompt` starts a stream; the client sends `session/cancel`; `Finish(ctx, failed)` on `Prompt`'s context writes the final update, received before the `cancelled` answer | `wrapUp` honouring every cause |
    | `TestToolCallStreamFinishDeadline` | `Finish` on a context past its deadline, with the write held by a blocking sending middleware: it returns `context.DeadlineExceeded`, the stream is not finished, and a second `Finish` writes | `wrapUp` ignoring every cause |
    | `TestWrapUp` | causes `ErrPromptCancelled`, `errSuperseded` and `ErrShutdown` leave the wrap-up context live; `context.Canceled`, a deadline and another cause end it with that cause | the `errors.Is` checks removed |
    | `TestToolCallStreamOwnedFields` | `Update` changing `Id`, and changing `Status`, return `ErrToolCallStreamField` naming the field, both restored, the other changes kept; `SetStatus(completed)` returns it | the restore removed |
    | `TestToolCallStreamBound` | a text past the bound keeps a valid UTF-8 tail of at most the bound, a 4-byte character at the cut; `Dropped` equals what was cut; a write ending inside a character is held back, then completed | the snapshot not holding back |
    | `TestToolCallStreamConcurrent` | 16 streams, 4 goroutines each writing, then `Finish`: every client entity holds all its text | the snapshot taken outside `slot` |
    | `TestToolCallStreamLifetime` | streams open, `Close`, then `Wait` returns, no `running` is true, and a later change leaves `ErrConnectionClosed` in `Err` | the sender's sleep not selecting on the connection's end |
    | `TestToolCallStreamRefusals` | a `start` that is not a `tool_call`; before `initialize` (`ErrNotInitialized`); `Finish(pending)`; a second `Finish`; `Write`, `Update`, `Flush` after `Finish` | each refusal removed in turn |
    | `TestToolCallStreamSendError` | a relative location: `Flush` returns the validation error, `Err` keeps a background one, and a later `Update` with an absolute one is written | the error not kept |
    | `TestToolCallStreamMiddleware` | `WithSendingMiddleware` and `WithObserver` see each snapshot as a `session/update` | none: it pins existing behaviour |

    `FuzzToolCallStream` runs for 60 seconds in the phase, its time recorded; a crasher it finds is a deviation.
    `BenchmarkToolCallStream` (1,000 writes of 100 bytes, one stream, and the same over 16 streams) is run and its result
    recorded, not gated.
12. **`example_toolstream_test.go`:** `ExampleStartToolCallStream`, an agent's `Prompt` that runs a command with
    `cmd.Stdout = stream`, sets `in_progress` with `SetStatus`, and calls `Finish`; it has an `// Output:` line, so
    `go test` runs it.
13. **`cmd/generate/engine_test.go`:** `TestRuntimeFilesCoverHandWritten`'s list gains `"toolstream.go"`, which uses ACP types
    and so is not a runtime file. Run in the `cmd/generate` module.

### Phase 4: the turn's streams before the answer, D3

1. **`connection.go`** (a runtime file, so no ACP type): `type turnStreams struct` with a mutex and the flush functions of the
   turn's open streams, keyed by a pointer for removal; `withTurnStreams(ctx context.Context) (context.Context, *turnStreams)`;
   `turnStreamsOf(ctx context.Context) *turnStreams`, nil when absent; `(*turnStreams).add(key any, flush func(context.Context)
   error)`, `remove(key any)`; and `(*turnStreams).flush(ctx context.Context)`, which calls each in the order added and logs a
   failure at Debug. All nil-safe.
2. **`toolstream.go`:** `StartToolCallStream` adds the stream's `Flush` to `turnStreamsOf(ctx)`; `Finish` removes it.
3. **The emitter,** `cmd/generate/internal/emit/dispatch.go`, `promptCase` (`:40`):
   * after the statement `reqCtx, cancel := context.WithCancelCause(ctx)`, it emits `reqCtx, streams := withTurnStreams(reqCtx)`;
   * directly after `resp, err := a.agent.Prompt(reqCtx, p)` and before `a.mu.Lock()`, it emits
     `streams.flush(context.WithoutCancel(ctx))`, so no lock of the side is held while it waits;
   * the comment above `promptCase` (`:30`) cites the MADR, D3.
4. **The pins,** `dispatch_test.go`: the sequence pinned at `:93` gains both statements, in that order, and the flush before
   `a.mu.Lock()`. Plant (generator, scratch script): the flush statement removed from the emitter; the pin fails.
5. **Regenerate:** `cd cmd/generate && go run .`. `git diff --stat` shows `agent_gen.go` alone; its diff is read in full and is
   the two statements. The engine cases that generate `promptCase`, `accept-prompt-cancellation`, `accept-prompt-overlap`,
   `accept-permission-cancellation` and `accept-session-close-cancels`, compile against the runtime files:
   `cd cmd/generate && go test -run 'TestEngineCases|TestEngineBehaviour' .` passes whole.
6. **Tests,** `toolstream_test.go`:

   | Test | Requires | Plant |
   | :--- | :--- | :--- |
   | `TestToolCallStreamFlushedBeforeAnswer` | `Prompt` starts a stream, appends, returns without `Finish`; the client receives the text before the `session/prompt` answer | the generated flush removed, regenerated in a copy |
   | `TestToolCallStreamTurnsSeparate` | under `PromptOverlapConcurrent`, turn A's answer arrives while turn B's stream is held by a blocking sending middleware | one set per session in place of one per turn |
   | `TestToolCallStreamCancelDuringFlush` | the flush is held by a blocking sending middleware; a `session/cancel` reaches the agent's `Cancel` before the hold is released | the flush emitted after `a.mu.Lock()`, regenerated in a copy; the test fails by its bound, not by hanging the run |
   | `TestToolCallStreamNoTurn` | a stream started on `context.Background()` is flushed by no answer and works | none: it pins D3's last rule |
   | `TestToolCallStreamFinishedLeavesTurn` | after `Finish`, the answer writes nothing more for that stream | `Finish` not removing the stream |

### Phase 5: the docs

* **`README.md`:**
  * "Explore the API" (`:227`): the tool-call line gains `acp.StartToolCallStream` and `acp.ToolCallStream`.
  * A new `### Streaming a tool call` after "Cancellation" (`:660`) and before "Closing and limits" (`:728`). It covers:
    the snapshot rule (F1); the writer and `cmd.Stdout`; the defaults; the fields the stream owns; `Finish` after
    `session/cancel` and under a deadline; the flush before the answer and the turn rule; the cost (the MADR's Neutral
    consequence); and D5's list. Its code block is
    `ExampleStartToolCallStream`'s body, between `<!-- toolstream: ExampleStartToolCallStream -->` and
    `<!-- /toolstream -->`.
* **`docs_test.go`:** `TestReadmeToolCallStreamIsTheExample`, the pattern of `TestReadmeQuickstartIsTheExample` (`:169`).
  Plant: one line of the README block changed.
* **`docs/architecture.md`:** the stream; its sender under `goIfLive`, anchored on `writeLoop`; the turn's stream set in the
  generated `promptCase`.
* **`doc.go`:** one sentence naming the stream beside the tool-call constructors, if `doc.go` names them; otherwise nothing,
  and the record says so.

### Phase 6: the close

1. **`tools/consumer.py`:** the consumer module starts a `ToolCallStream` on an `acptest` pair, appends, finishes, and reads
   the entity back with `ToolCallTimeline`; its floor becomes `v1.35.0`. `tools/test_tools.py` and `tools/README.md`'s table
   follow. `make check-tools`.
2. **The gates:** `python3 tools/gates.py`; `python3 tools/remote.py tree --host linux` and `--host windows`, the copy made
   without macOS `._` files; `python3 tools/interop.py`, since `remote`'s limit changed, when `node`, `npm`, `npx` and `uv`
   are present, else recorded as not run, with the reason. Each output goes in the record.
3. **`make api`:** only additions: `StartToolCallStream`; `ToolCallStream` and its nine methods (`Write`, `WriteString`,
   `SetStatus`, `Update`, `Entity`, `Dropped`, `Err`, `Flush`, `Finish`); `ToolCallStreamOption`,
   `WithToolCallStreamInterval`, `WithToolCallStreamMaxText`; `ErrToolCallStreamFinished`, `ErrToolCallStreamField`.
   `clienthost`'s exported API is unchanged.
4. **`docs/reports/0052-REPORT-release-notes-v1-35-0.md`,** against `v1.34.2`, in the form of
   `0050-REPORT-release-notes-v1-34-0.md`: the stream; the 32 MiB default as a widening; the generated flush before the
   prompt answer as a behaviour change for agents that leave streams open; no change to existing calls.
5. **The records:** this PLAN `complete` only when every Verification item holds; the MADR's Consequences checked against
   what was built, with an amendment for any difference; `docs/README.md`'s rows.
6. **The release,** only on the owner's ask in that turn: `git fetch`, `make push-guard`, the tag, then
   `python3 tools/release_body.py v1.35.0 <rev>` and `python3 tools/publication.py`.

## Verification

* Each phase: `make pre-add-check FILES="…"` passes, which runs `gofmt`; `golangci-lint` with `.golangci.yml` for the host and
  for `GOOS=windows`; `go vet`; `go test -race -shuffle=on` uncached in both modules; `go mod tidy -diff`; and `govulncheck`.
* Every test named in phases 1 to 5 exists and was seen to fail on its plant, recorded with the failure line.
* `go test -race -count=20 -run 'ToolCallStream|WrapUp|GoIfLive|WriteLoopEndsLiveFirst' .` and
  `go test -race -count=20 ./internal/tailbuf/ ./clienthost/` pass, for flakiness.
* `FuzzToolCallStream` ran 60 seconds with no crasher.
* `make api` reports exactly phase 6's additions.
* `make check-records`, `make markdownlint`, `make disclosure-check` and, where changed, `make check-tools` pass.
* The Linux and Windows hosts pass the tree's tests; interop passes or is recorded as not run.
* The search of phase 1, step 2 finds no 10 MiB default outside the records.

## Rollout and Rollback

* **Rollout:** one minor, `v1.35.0`, after phase 6, on the owner's ask. Consumers adopt under their own records: gobble-cli
  needs an output hook in its tools and a client view that redraws (MADR F15, F19).
* **Rollback before the tag:** revert the phase's commits, newest first; phase 4 depends on phase 3, and phase 3 on phase 2.
* **Rollback after the tag:** a patch. The 32 MiB default returns to 10 MiB alone, one constant per package. The generated flush
  is removed by the emitter step alone. The stream's exported API stays under the Go 1 promise.

## Execution record

### Approval (2026-11-10)

The owner accepted the MADR with its two reviews and approved this PLAN. The MADR, this PLAN and their two rows in
`docs/README.md` were staged alone for the bootstrap commit, which is phase 1's base. Phase 1 starts once that commit exists
and `git status --short` is clean.

### Phase 1, the message size (2026-11-10)

**Base.** The owner committed the pair as `c630d2f`; `git status --short` was empty before the first edit.

**Done.**

* `connection.go`: `defaultMaxMessageSize = 32 * 1024 * 1024`, its comment naming the TypeScript SDK's default and citing
  the MADR, D6; `WithMaxMessageSize`'s doc says 32 MiB and cites D6 beside 0017.
* `remote/client.go`: `defaultMaxMessageSize = 32 * 1024 * 1024`, its comment "the connection's own default, 32 MiB" citing
  D6; `DialOptions.MaxMessageSize`'s doc says 32 MiB. `remote/handler.go`: `HandlerOptions.MaxMessageSize`'s doc says 32 MiB.
* `README.md`: the byte budget's paragraph, "1024 notifications of up to 32 MiB each, about 32 GiB"; `remote`'s two
  `MaxMessageSize` bullets; "Errors on the wire", 32 MiB, the TypeScript SDK's default too. `docs/architecture.md`, `receive`'s
  limit, 32 MiB, citing D6.
* `jsonrpc_test.go`: `TestDefaultMaxMessageSize`, not parallel. `remote/ws_test.go`: `TestDefaultMaxMessageSize`, not parallel,
  a 20 MiB request and its 20 MiB answer over Streamable HTTP and WebSocket with nil options on both sides, after
  `initialize`, as `TestHTTPServerOverlongAnswer` orders them.

**Step 2's search,** a Python script over the tracked and untracked files outside `docs/decisions/` and `docs/reports/`, for
`10 MiB`, `10 * 1024 * 1024`, `10 << 20`, `10485760` and `10 GiB`, after the edits: exit 1, two hits, both the new tests'
comments naming "the former 10 MiB default". No statement of the default is left.

**Tests.** `go test -race -count=1 -run 'MaxMessageSize' . ./remote/`: exit 0; the root package 7.3 s, `remote` 17.8 s.

**Plants,** `python3 tools/plants.py` over a two-plant spec in the scratch directory: exit 0, "2/2 as required".

| Plant | Failure |
| :--- | :--- |
| `connection.go`'s constant at `10 * 1024 * 1024` | `jsonrpc_test.go:264: the default limit is 10485760 bytes, want 32 MiB`; `jsonrpc_test.go:272: a 20 MiB line under the default limit answered null {"code":-32600,…,"data":{"error":"message exceeds 10485760 bytes"}}, want -32601 with id 1` |
| `remote/client.go`'s constant at `10 * 1024 * 1024` | `ws_test.go:253: the default limit is 10485760 bytes, want 32 MiB`; `ws_test.go:294: http: writing the 20 MiB request under the default limit: remote: message exceeds the size limit: more than 10485760 bytes` |

The `remote` plant stops at the Streamable HTTP write, which is fatal, so its WebSocket half is not reached under the plant;
the unplanted run passes both.

**Step 5, memory and time.**

* This host: `go test -race -count=3 -run 'MaxMessageSize' .`: exit 0, 15.2 s, wall 16.3 s.
* A Linux host reached over SSH (`Linux x86_64`, go1.27.3): `go test -race -count=3 -run 'MaxMessageSize' . ./remote`, in a
  fresh directory removed after: exit 0; the root package 41.2 s, `remote` 118.0 s, 151 s in all.
* **Deviation, 2026-11-10, the tool:** step 5 asks for `-race` on the Linux host, and `tools/remote.py pkg` cross-compiles
  the test binary, which `-race` cannot (it needs cgo), while `remote.py tree` runs without `-race`. The run copied the
  working tree's tracked and untracked files, without macOS `._` files, by a scratch script that follows `remote.py`'s
  method: the host from `ACP_LINUX_HOST`, a fresh `acp-tools-<random>` directory, removed after, its removal checked. No
  file of the tree changed for it. Recorded for the owner's confirmation at the phase's hand-off.

**Pre-add.** `make pre-add-check FILES="connection.go remote/client.go remote/handler.go jsonrpc_test.go remote/ws_test.go
README.md docs/architecture.md"`: exit 0, "5 file(s) gofmt-clean; both modules linted, vetted and tested (-race -shuffle=on,
uncached), tidy, govulncheck." `make check-records`, `make markdownlint` and `make disclosure-check`: exit 0 each.

**Not done in this phase:** step 4's last item needed no new test. 0045's D4 test,
`TestQueuedBytesClampAndSingleLine` (`connection_queue_bytes_test.go:167`), is unchanged and passed in the pre-add run, and
the 64 MiB default still holds two 32 MiB messages.
