---
status: complete
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
| 3, the stream (D1, D2, D4, D5) | `internal/tailbuf/tailbuf.go`, `tailbuf_test.go` (new); `clienthost/terminals.go`; `clienthost/output.go`, `output_test.go` (deleted); `toolstream.go` (new); `errors.go`; `toolstream_test.go` (new); `example_toolstream_test.go` (new); `cmd/generate/engine_test.go`; `cmd/generate/internal/output/check.go`, `check_test.go` (added by the phase's deviation of 2026-11-10) |
| 4, the turn's streams before the answer (D3) | `connection.go`; `toolstream.go`; `toolstream_test.go`; `cmd/generate/internal/emit/dispatch.go`; `cmd/generate/internal/emit/dispatch_test.go`; `agent_gen.go` (regenerated) |
| 5, the docs | `README.md`; `docs_test.go`; `docs/architecture.md`; `doc.go` |
| 6, the close | `tools/consumer.py`; `tools/test_tools.py`; `tools/README.md`; `docs/reports/0052-REPORT-release-notes-v1-35-0.md` (new); this PLAN; the MADR; `docs/README.md`; `toolstream_test.go` (added by the phase's deviation of 2026-11-10) |

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
  file of the tree changed for it. Recorded for the owner's confirmation at the phase's hand-off. The owner had issue #15
  filed, "tools/remote.py: run go test -race (and -run, -count) on a test host", for a permanent fix in `remote.py`.

**Pre-add.** `make pre-add-check FILES="connection.go remote/client.go remote/handler.go jsonrpc_test.go remote/ws_test.go
README.md docs/architecture.md"`: exit 0, "5 file(s) gofmt-clean; both modules linted, vetted and tested (-race -shuffle=on,
uncached), tidy, govulncheck." `make check-records`, `make markdownlint` and `make disclosure-check`: exit 0 each.

**Not done in this phase:** step 4's last item needed no new test. 0045's D4 test,
`TestQueuedBytesClampAndSingleLine` (`connection_queue_bytes_test.go:167`), is unchanged and passed in the pre-add run, and
the 64 MiB default still holds two 32 MiB messages.

**Commit.** The owner committed the phase as `8e7caad`.

### Phase 2, a start only while the connection lives (2026-11-10)

**Base.** `8e7caad`; `git status --short` was empty before the first edit.

**Done.**

* `connection.go`: `liveMu sync.Mutex` and `ended bool` after `goroutines`, their comment giving why a mutex; `goIfLive(f
  func()) bool` and `endLive()` before `wakeNotificationWaits`, `goIfLive`'s comment stating the `sync.WaitGroup` argument and
  citing the MADR, D2, and 0026; `writeLoop`'s first statement `defer c.endLive()`, with a comment naming
  `TestWriteLoopEndsLiveFirst`.
* `connection_end_test.go`: `TestGoIfLiveAfterEnd`, `TestWriteLoopEndsLiveFirst` and `TestGoIfLiveDuringEnd`, after
  `TestGoStatementsCounted`, which is unchanged; the imports gain `sync` and `sync/atomic`.

**Found during the phase.** `TestGoIfLiveDuringEnd` as first written let its 32 callers loop while `goIfLive` accepted. Under
its plant, `goIfLive` never refused, so the callers never ended and the test hung until `go test`'s 120 s timeout
("panic: test timed out after 2m0s"); the plant reported "NOT AS REQUIRED". The test was rewritten within the step's terms so
that a broken `goIfLive` fails it and does not hang it: the callers also stop on a channel the test closes; `Wait` is bounded
through `WaitContext` at 10 s; one `goIfLive` after `Wait` returns must refuse. No other file changed for it.

**Tests.** `go test -race -count=50 -run '^TestGoIfLiveDuringEnd$|^TestGoIfLiveAfterEnd$|^TestWriteLoopEndsLiveFirst$|^TestGoStatementsCounted$' .`:
exit 0, 17.2 s.

**Plants,** `python3 tools/plants.py` over a four-plant spec in the scratch directory: exit 0, "4/4 as required", each test
failing in about 1.3 s.

| Plant | Failure |
| :--- | :--- |
| `goIfLive` without the `ended` check | `connection_end_test.go:511: goIfLive started a goroutine after Wait returned` |
| the same, under `TestGoIfLiveDuringEnd` | `connection_end_test.go:597: Wait returned acp: connection closed and goIfLive after it true: goroutines started after the end` |
| `writeLoop` without the defer | `connection_end_test.go:542: writeLoop's first statement is not defer c.endLive()` |
| the defer after a first statement `_ = 0` | `connection_end_test.go:542: writeLoop's first statement is not defer c.endLive()` |

The step named the defer "moved after the loop"; the loop only returns, so a defer after it is unreachable and runs never.
The plant puts a statement before the defer instead, which tests the same rule: the defer must come first.

**Pre-add.** `make pre-add-check FILES="connection.go connection_end_test.go"`: exit 0, "2 file(s) gofmt-clean; both modules
linted, vetted and tested (-race -shuffle=on, uncached), tidy, govulncheck." `make check-records`, `make markdownlint` and
`make disclosure-check`: exit 0 each.

**Not done in this phase:** nothing calls `goIfLive` outside its tests yet; phase 3's stream is its first caller.

**Commit.** The owner committed the phase as `8969f86`.

### Phase 3, the stream (2026-11-10)

**Base.** `8969f86`; `git status --short` was empty before the first edit.

**Done.**

* `internal/tailbuf/tailbuf.go`, new: `Buffer` with `Limit`, `Write`, `WriteString`, `Snapshot(final) (text, dropped)` and
  `Dropped()`; `cut`, `complete` and `compactAfter` moved from `clienthost/output.go` unchanged. `internal/tailbuf/tailbuf_test.go`:
  `clienthost`'s two tests moved as `TestTruncatesAtCharacter` and `TestHoldsBackUnfinishedCharacter`, and
  `TestDroppedCount` and `TestWriteNeverFails`, new.
* `clienthost/terminals.go`: `out *tailbuf.Buffer`; `TerminalOutput` reads `Snapshot(exited)` and sets `Truncated` from
  `dropped > 0`. `clienthost/output.go` and `clienthost/output_test.go` deleted with `git rm`.
* `errors.go`: `ErrToolCallStreamFinished` and `ErrToolCallStreamField`.
* `toolstream.go`, new: the MADR's D1 API, the change path, the sender `run`, `writePending`, the slot, `toolCallUpdateOf` and
  `wrapUp`, as steps 3 to 10 give them. No `go` statement: the sender starts through `goIfLive`.
* `toolstream_test.go`, new: the sixteen tests of step 11 and `TestToolCallStreamHelperProcess`, the command
  `TestToolCallStreamExecWriter` runs, which skips unless its variable is set; `FuzzToolCallStream`, three seeds;
  `BenchmarkToolCallStream`.
* `example_toolstream_test.go`, new: `ExampleStartToolCallStream`, its agent built on `AgentRefusals` and its client on
  `ClientRefusals`, with an `// Output:` the test checks.
* `cmd/generate/engine_test.go`: `TestRuntimeFilesCoverHandWritten`'s list gains `"toolstream.go"`.

**Found during the phase, each within the step's terms unless marked a deviation.**

* **The MADR's two rules,** `Flush` with nothing pending and a failed write, were wrong as written; the MADR carries "Amended
  during the PLAN's phase 3, 2026-11-10", and its D1 and D2 bullets point to it. `writePending` takes the slot before it
  looks at `pending`, and sets `pending` again only when its own context ended.
* **`tailbuf` has two methods step 1 did not list,** `WriteString` (the stream's `WriteString` without a `[]byte` copy) and
  `Dropped` (the stream's `Dropped` without copying the text), and its compaction moves the kept tail within its array where
  `clienthost`'s allocated a new one. The moved tests pin the behaviour, and `clienthost/terminals_test.go` passes unchanged.
* **The example takes its output from `io.Copy` of a reader, not a command,** with a comment that `cmd.Stdout = stream` does
  the same: an example's `// Output:` must be the same on every host, and no command prints the same on all of them.
  `TestToolCallStreamExecWriter` runs a real command, the test binary itself, as Go's helper-process pattern does.
* **The test helper is `waitUntil`;** `eventually` was taken, by `refused_messages_test.go:41`, with another signature.
* **Two plants changed from the step's table,** both because the written plant would hang the test, not fail it:
  * `TestToolCallStreamIdle`'s plant: the table's "the exit on nothing pending removed" makes the sender spin without blocking,
    and `synctest.Wait` then never returns. The plant leaves the sender's `running` mark set when it exits, which the test
    reads.
  * `TestToolCallStreamCoalesces`' plant: "a queue of snapshots" is two edits, every change starting a sender and every sender
    writing whether pending or not.
* **Deviation, 2026-11-10, the race plant:** `TestToolCallStreamConcurrent`'s plant is a data race, the change path without the
  stream's lock, which only `-race` shows, and `tools/plants.py` runs `go test` without it. A scratch script, as phase 1's,
  copied the working tree's tracked and untracked files to a scratch directory, applied the edit (matched once), ran
  `go test -race -count=1 -run '^TestToolCallStreamConcurrent$' .`, and removed the copy. It is the same gap as issue #15's,
  in another tool.

**Tests.**

* `go test -race -count=1 ./internal/tailbuf/ ./clienthost/`: exit 0; 1.7 s and 6.0 s.
* `go test -race -count=1 -v -run 'ToolCallStream|WrapUp' .`: exit 0; seventeen PASS, `TestToolCallStreamHelperProcess` SKIP, as
  it must outside its command.
* `go test -race -count=20 -run 'ToolCallStream|WrapUp' .`: exit 0, 23.5 s.
* `go test -race -count=3 -run '^ExampleStartToolCallStream$' .`: exit 0.
* `cd cmd/generate && go test -count=1 -run '^TestRuntimeFilesCoverHandWritten$' .`: exit 0.
* `go test -run '^$' -fuzz '^FuzzToolCallStream$' -fuzztime 60s .`: exit 0, 189,173 executions, 285 corpus entries, no
  crasher; nothing written under `testdata/`.
* `go test -run '^$' -bench '^BenchmarkToolCallStream$' -benchmem .`, recorded, not gated: one stream, 1,000 writes of
  100 bytes and `Finish`, 1,068,181 ns/op, 1,826,421 B/op, 201 allocs/op; sixteen at once, 19,383,844 ns/op, 34,318,661 B/op,
  4,160 allocs/op (darwin/arm64).

**Plants,** `python3 tools/plants.py` over a sixteen-plant spec in the scratch directory: exit 0, "16/16 as required", none by
timeout.

| Plant | Failure |
| :--- | :--- |
| `tailbuf`: compaction not counted | `tailbuf_test.go:92: dropped 3000, want 599001: 600000 written, 999 kept` |
| the live text left out of the snapshot | `toolstream_test.go:189: the client's live text is "summary"` |
| `Write` returning 0 | `toolstream_test.go:265: short write` |
| every change sent, as a queue would | `toolstream_test.go:304: 384 writes for 1,001 changes held by back-pressure, want at most 2` |
| the interval wait removed | `toolstream_test.go:345: 100 writes for a burst within one interval, want 2` |
| the sender's mark not cleared when it exits | `toolstream_test.go:379: a sender still runs with nothing pending` |
| the mark cleared outside the lock that checks `pending` | `toolstream_test.go:415: round 0: the client's text is "b0", want "b0c0": a change was lost` |
| `wrapUp` honouring every cause (`FinishAfterCancel`) | `toolstream_test.go:454: Finish after session/cancel: acp: prompt turn cancelled` |
| `wrapUp` honouring every cause (`TestWrapUp`) | `toolstream_test.go:527: cause acp: prompt turn cancelled, cancelled before false: live false, want true` |
| `wrapUp` ignoring every cause | `toolstream_test.go:484: Finish ignored its deadline` |
| `Update`'s restore removed | `toolstream_test.go:569: after the refused changes: id "other", status "completed", title "kept"; want o1, pending, kept` |
| the snapshot not holding back | `toolstream_test.go:600: a character not finished: kept text valid false, ends with it false; want it held back` |
| the sender's sleep not ended by the connection's end | `toolstream_test.go:666: Wait did not return after Close: a sender outlived its connection` |
| `Finish` without its status check | `toolstream_test.go:697: Finish(pending) = <nil>, want ErrToolCallStreamField` |
| changes taken after `Finish` | `toolstream_test.go:710: Update after Finish = <nil>, want ErrToolCallStreamFinished` |
| the background error not kept | `toolstream_test.go:735: the background write's error did not happen` |
| the change path without the stream's lock, under `-race` (scratch script) | four `WARNING: DATA RACE`; `testing.go:1867: race detected during execution of test`; exit 1 |

`TestToolCallStreamMiddleware` has no plant, as the step says: it pins existing behaviour.

**Deviation, 2026-11-10, the generator's check of the root.** The phase's first `make pre-add-check` exited 2. Besides four lint
findings in the phase's own files, `cmd/generate`'s `TestCheckAcceptsRepository` failed:
`../../toolstream.go:10:2: could not import github.com/maccavelli/acp-go-sdk2/internal/tailbuf (no required module provides
package …)`. `output.Check` (`cmd/generate/internal/output/check.go:110`-`:118`) type-checks the root package through
`sharedImporter`, a source importer that resolves in the `cmd/generate` module and cannot see the root module's other
packages. The root had imported only the standard library until `toolstream.go`, so nothing had needed it. `run` calls
`Check` before it writes, so phase 4's regeneration would fail the same way; the failure is the phase's, not an older one.
The owner chose, of three resolutions offered (fix the check; keep a copy of the buffer in the root and reverse D4; stop and
write a separate pair), to fix the check: the root, as built and as tested, is type-checked through `moduleImporter`, which
already resolves the module's packages from source for the external tests (0049 G27). A scratch copy with that change
passed `TestCheck*` and the `output` package's tests before the decision. The phase's files gain
`cmd/generate/internal/output/check.go` and `check_test.go`.

**The same run's other findings.** `errcheck`: `_, _ = s.text.Write(p)` and `_, _ = s.text.WriteString(text)`; `tailbuf`
gains `Append` and `AppendString`, which return nothing, and its `Write` and `WriteString` call them. `perfsprint`: two
`fmt.Sprint(i)` in `toolstream_test.go` become `strconv.Itoa(i)`. `agentproc`'s `TestCloseContextBoundsHandlers` failed once
under the full parallel run, with the line 0042's PLAN records for its timing case ("CloseContext = context deadline exceeded
after 301.587708ms, want the bound's cause and acp.ErrHandlersRunning"); alone, `go test -race -count=20` passed it 20 times.
The phase does not touch `agentproc`; it is a timing flake under load, left as it is.

**The resolution, done.**

* `cmd/generate/internal/output/check.go`: `checkHost` checks the root as built and as tested through a `moduleImporter` with
  no root package to serve; that importer refuses an import of the root when it has none. `Check`'s and `moduleImporter`'s
  comments say so and cite this deviation.
* `cmd/generate/internal/output/check_test.go`: `TestCheckRootImportsModulePackage`. A root whose hand-written file and
  in-package test import `internal/buf` of its module is accepted, and one whose `internal/buf` does not check is refused,
  naming it. `cd cmd/generate && go test -race -count=1 -run '^TestCheck' . ./internal/output/`: exit 0, 16.1 s and 7.1 s.
* Plant, by a scratch script in the copy's `cmd/generate`, since `tools/plants.py` runs from the root module: the built check
  back on `imp`. `check_test.go:354: Check refused a root that imports a package of its module: generated code does not
  type-check with …`; exit 1, failed as required.
* `tailbuf.Buffer` gained `Append` and `AppendString`; the stream calls them. `toolstream_test.go` imports `strconv`.
* After these changes: `go test -race -count=1 -run 'ToolCallStream|WrapUp' .`: exit 0; the sixteen plants again, "16/16 as
  required"; the race plant again, failed as required.

**Pre-add.** `make pre-add-check FILES="internal/tailbuf/tailbuf.go internal/tailbuf/tailbuf_test.go clienthost/terminals.go
cmd/generate/engine_test.go cmd/generate/internal/output/check.go cmd/generate/internal/output/check_test.go errors.go
toolstream.go toolstream_test.go example_toolstream_test.go"`: `make` exit 0, "10 file(s) gofmt-clean; both modules linted,
vetted and tested (-race -shuffle=on, uncached), tidy, govulncheck." `agentproc` passed in this run. `make check-records`,
`make markdownlint` and `make disclosure-check`: exit 0 each.

**Not done in this phase:** the turn's stream set and the flush before the answer, phase 4's; `make api`'s listing, phase 6's.

**Commit.** The owner committed the phase as `38e9772`.

### Phase 4, the turn's streams before the answer (2026-11-10)

**Base.** `38e9772`; `git status --short` was empty before the first edit.

**Done.**

* `connection.go`: `turnStreams`, `turnStream`, `turnStreamsKey`, `withTurnStreams`, `turnStreamsOf`, and the nil-safe `add`,
  `remove`, `size` and `flush`, after `receiptOf`, with no ACP type. `flush` copies the entries under its lock and runs them
  with none held.
* `toolstream.go`: the stream's `turn` field; `StartToolCallStream` adds `flushForTurn` to `turnStreamsOf(ctx)`; `Finish`
  removes the stream on success.
* `cmd/generate/internal/emit/dispatch.go`: `promptCase` emits `reqCtx, streams := withTurnStreams(reqCtx)` after the
  `WithCancelCause`, and `streams.flush(context.WithoutCancel(ctx))` directly after the handler, before `a.mu.Lock()`; its
  comment cites D3. `dispatch_test.go`: `TestDispatchFollowsTheModel`'s pins gain both, each with its neighbour.
* `cd cmd/generate && go run .`: exit 0. `git diff --stat` showed `agent_gen.go` as the one generated file, and its diff, read
  in full, is the two statements at `:758` and `:774`.
* `toolstream_test.go`: `holdCall`, a middleware that holds one call's updates, and the five tests of step 6.

**Found during the phase.** Step 1 had the turn's `flush` log a failure at Debug, which needs the connection's logger in a
runtime type. The stream's own `flushForTurn` keeps the error for `Err` instead, as a background write's is: the SDK, not the
agent, made the write, and `Err` is where the agent reads such errors (D1). `turnStreams` therefore runs flushes that return
nothing.

**Tests.**

* `go test -race -count=1 -v -run 'ToolCallStream(FlushedBeforeAnswer|TurnsSeparate|CancelDuringFlush|NoTurn|FinishedLeavesTurn)' .`:
  exit 0, five PASS.
* `cd cmd/generate && go test -count=1 -run 'TestEngineCases|TestEngineBehaviour' .`, whole: exit 0, 37.5 s. The engine cases
  that render `promptCase` compile with `withTurnStreams` from the runtime files.
* `go test -race -count=20 -run 'ToolCallStream|WrapUp|Prompt.*Cancel|Cancel.*Prompt|PromptOverlap' .`: exit 0, 24.2 s.

**Plants.**

| Plant | How | Failure |
| :--- | :--- | :--- |
| one stream set for every turn | `tools/plants.py` | `toolstream_test.go:856: turn A's answer while turn B's flush is held did not happen` |
| `Finish` not leaving its turn | `tools/plants.py` | `toolstream_test.go:967: the turn held 1 streams after the start and 1 after Finish, want 1 and 0` |
| the flush statement removed from the emitter | scratch script in the copy's `cmd/generate` | `dispatch_test.go:166: agent_gen.go lacks "resp, err := a.agent.Send(reqCtx, p)\n\t\tstreams.flush(…)\n\t\ta.mu.Lock()\n"` |
| the generated flush replaced by `_ = streams`, regenerated in the copy | scratch script: edit, `go run .`, root test | `toolstream_test.go:815: when the answer arrived the client had text "" and status "in_progress", want "late text" and in_progress` |
| the flush emitted after `a.mu.Lock()`, regenerated in the copy | the same | `toolstream_test.go:905: session/cancel did not reach the agent while the turn's flush was held`, by its 5 s bound |

The step's "the generated flush removed" leaves `streams` unused, which the generator's check refuses before any test runs; the
plant keeps the variable with `_ = streams`, which removes the flush and compiles. The regenerating plants print the copy's
`agent_gen.go` diff, which was the one statement each.

**Pre-add.** The first `make pre-add-check` exited 2 at lint, before its tests: `errcheck` on
`ts, _ := ctx.Value(turnStreamsKey{}).(*turnStreams)` (`connection.go:131`), and `unconvert` on a `string(…)` in
`TestToolCallStreamTurnsSeparate`. `turnStreamsOf` checks the assertion's `ok`, and the conversion is gone. The plants of
`tools/plants.py` were run again after: "2/2 as required". `make pre-add-check FILES="connection.go toolstream.go
toolstream_test.go agent_gen.go cmd/generate/internal/emit/dispatch.go cmd/generate/internal/emit/dispatch_test.go"` again:
`make` exit 0, "6 file(s) gofmt-clean; both modules linted, vetted and tested (-race -shuffle=on, uncached), tidy,
govulncheck." `make check-records`, `make markdownlint` and `make disclosure-check`: exit 0 each.

**Not done in this phase:** the README section and the docs, phase 5's.

**Commit.** The owner committed the phase as `4c7f943`.

### Phase 5, the docs (2026-11-10)

**Base.** `4c7f943`; `git status --short` was empty before the first edit.

**Done.**

* `README.md`: "Explore the API"'s tool-call bullet names `acp.StartToolCallStream` and `acp.ToolCallStream` and links the new
  section. `### Streaming a tool call`, between "Cancellation" and "Closing and limits": the snapshot rule, citing the MADR;
  the example between `<!-- toolstream: streamingAgent.Prompt -->` and `<!-- /toolstream -->`; and seven bullets: the output
  and its bound, the other fields and the owned ones, sending, finishing, the turn, the cost, and what the stream does not do.
* `docs_test.go`: `TestReadmeToolCallStreamIsTheExample`, the pattern of `TestReadmeQuickstartIsTheExample`.
* `docs/architecture.md`: `toolstream.go` in the hand-written files; the sentence on protocol state names the stream's; the
  prompt turn's passage gives the turn's `turnStreams` and the flush before the lock; the wait group's passage gives
  `goIfLive` and its anchor; the tests list names phase 2's tests and gains `toolstream_test.go`; the tree gains
  `internal/tailbuf/`.
* `doc.go`: unchanged. It names no tool-call constructor, so the step's condition is not met.

**Found during the phase.** The step put `ExampleStartToolCallStream`'s body in the README. That body is the client's half:
pipes, `Initialize`, `Prompt`, and printing. The block is the agent's `Prompt` instead, `streamingAgent.Prompt` in
`example_toolstream_test.go`, which is the code an agent writes; the test reads it with the same `exampleBody`, which finds the
one `Prompt` of that file, and its marker says so.

**Tests.** `go test -count=1 -run '^TestReadme' .`: exit 0.

**Plant,** `tools/plants.py`: the README block's `output :=` line changed. `docs_test.go:216: README.md's tool-call stream
example is not streamingAgent.Prompt's body.`; "1/1 as required".

**Pre-add.** `make pre-add-check FILES="README.md docs_test.go docs/architecture.md"`, the README not being docs-safe:
`make` exit 0, "1 file(s) gofmt-clean; both modules linted, vetted and tested (-race -shuffle=on, uncached), tidy,
govulncheck." `make check-records`, `make markdownlint` (52 files, 0 issues) and `make disclosure-check`: exit 0 each.

**Not done in this phase:** the close, phase 6's.

**Commit.** The owner committed the phase as `62f62aa`.

### Phase 6, the close (2026-11-10)

**Base.** `62f62aa`; `git status --short` was empty before the first edit.

**Deviation, 2026-11-10, the Windows gate.** `python3 tools/gates.py --all`, with both hosts, exited 1: 17 gates passed and
`windows` failed, in the root package alone: `TestToolCallStreamRoundTrip`, `TestToolCallStreamSendError`, and
`FuzzToolCallStream`'s three seeds. `python3 tools/remote.py pkg --host windows . -test.run '^TestToolCallStreamRoundTrip$'
-test.v` gave `toolstream_test.go:178: step 3: … locations[0]: ToolCallLocation: path: "\\x" is not an absolute path`. Phase 3's
test helper `absPath` built `string(os.PathSeparator) + name`, which is absolute on Unix and not on Windows, which needs a
volume; the SDK's check refuses it, as it must. The defect is the test's, from phase 3, not the stream's. The owner chose, of
two resolutions offered (fix the helper in this phase; stop and add a separate step first), to fix it here: `absPath` is
`filepath.Join(os.TempDir(), name)`, absolute on every platform. The phase's files gain `toolstream_test.go`. Skipping the
tests on Windows was not offered: it would hide the defect.

**Done.**

* `tools/consumer.py`: the docstring names the stream and the floor `v1.35.0`; its test template imports `io` and gains
  `TestToolCallStream`, an `acptest` pair, `StartToolCallStream`, `io.WriteString`, `Finish`, and a `ToolCallTimeline` fed the
  `MemoryClient`'s updates that must hold the call completed with the text. `WANT_RUN`, the program's output, is unchanged.
* `tools/test_tools.py`: `HelpersTest.test_consumer_streams_a_tool_call` requires the template's stream, write, `Finish` and
  timeline, and the docstring's floor. `tools/README.md`: the `consumer.py` row names the test and `v1.35.0`.
* `toolstream_test.go`: `absPath`, by the deviation above.
* `docs/reports/0052-REPORT-release-notes-v1-35-0.md`, new: the release text and its sources.

**Step 1, the consumer.**

* `make check-tools`: exit 0; 71 tool tests, 41 script tests, `check-ci-filters: 0 problem(s)`.
* Before the tag, by a scratch script: `consumer.py`'s program and test in a scratch module with a `replace` onto the working
  tree, `go test -run '^TestToolCallStream$'`: exit 0, PASS. The module was removed.
* Plant, by a scratch script in a copy of `tools/`: the template's `Finish` call removed; `test_consumer_streams_a_tool_call`
  failed, "FAILED (failures=1)", as required.
* `python3 tools/consumer.py v1.35.0` needs the tag on the proxy: it runs after the release, step 6.

**Step 2, the gates.**

* `python3 tools/gates.py --all` with both hosts, first run: exit 1, `windows` failed (the deviation above), the 17 others
  passed.
* After the fix: `python3 tools/remote.py pkg --host windows . -test.run '^(TestToolCallStreamRoundTrip|TestToolCallStreamSendError|FuzzToolCallStream)$' -test.v`:
  exit 0, three PASS. `go test -race -count=1 -run 'ToolCallStream|WrapUp' .`: exit 0.
* `python3 tools/gates.py --all` again: exit 0, "all gates passed", 18 of 18: `regen`, `precheck`, `newfiles`, `nojsonv2`,
  `winvet`, `records`, `mdlint`, `idscan`, `tools`, `lint`, `fix`, `tidy`, `api`, `race3`, `floor`, `platforms`, `windows`,
  `linux`. Its `precheck` is `make pre-add-check` over the whole tree, the fixed test included.
* `python3 tools/interop.py`, on the first run's tree: exit 0, "all pairs passed", the eight pairs against the official
  TypeScript and Python SDKs over Streamable HTTP and WebSocket. The fix after it touches a root test alone.

**Step 3, the API.** `make api`: exit 0, "against v1.34.2: 0 incompatible, 7 compatible", the seven additions step 3 names
(`ToolCallStream`'s nine methods within its type); "reported packages unstable, remote: 0 incompatible, not gated".

**Step 4, the release notes,** `docs/reports/0052-REPORT-release-notes-v1-35-0.md`, in the form of
`0050-REPORT-release-notes-v1-34-0.md`.

**Step 5, the records.** The Verification list, checked:

| Item | Held by |
| :--- | :--- |
| each phase's `make pre-add-check` | phases 1 to 5's records; phase 6's through the `precheck` gate |
| every new test seen to fail on its plant | each phase's plant table; phase 6's plant above |
| `go test -race -count=20 -run 'ToolCallStream\|WrapUp\|GoIfLive\|WriteLoopEndsLiveFirst' .` and the same over `./internal/tailbuf/ ./clienthost/` | run in this phase: exit 0 each, 24.2 s; 1.3 s and 70.0 s |
| `FuzzToolCallStream`, 60 s, no crasher | phase 3 |
| `make api`, exactly the additions | step 3 |
| `make check-records`, `make markdownlint`, `make disclosure-check`, `make check-tools` | each phase; this phase's below |
| the Linux and Windows hosts; interop | step 2 |
| no 10 MiB default outside the records | phase 1, step 2's search |

The MADR's Consequences were read against what was built; the one the MADR had not stated, the generator's check, is its
amendment of phase 3.

**Not done:** step 6, the tag and the release, waits for the owner's ask in that turn; `tools/consumer.py v1.35.0` and
`tools/publication.py` run after it. This PLAN stays `in-progress` until then.

**Commit.** The owner committed the phase as `644cfe4` and pushed it.

### Phase 6, the release (2026-11-10)

* **CI on `main`,** `python3 tools/watch_ci.py --branch main 644cfe4`: exit 0. Run 42870969917, CI, `success`, its six
  `validate` jobs (ubuntu-24.04, macos-15 and windows-2025, each at 1.26.10 and 1.27.3); run 42870970116, Docs, `success`.
* **The tag.** The owner pushed `v1.35.0`, annotated, naming `644cfe4` (`git ls-remote --tags origin`). Its CI,
  `python3 tools/watch_ci.py --branch v1.35.0 644cfe4`: exit 0, run 42872665778, `success`, the six `validate` jobs.
* **The release,** on the owner's ask in that turn. `python3 tools/release_body.py v1.35.0 644cfe4`: exit 0, "65 lines, 1
  links", its identifier scan "0 match(es)"; the body read in full before publishing. `gh release create v1.35.0 --verify-tag
  --title v1.35.0 --notes-file …`, the title as `v1.34.2`'s: published, not a draft or prerelease.
* **`python3 tools/publication.py v1.35.0 644cfe4 <body>`:** exit 0, "10 of 10 checks hold": the release exists, its body is
  the extracted text, it is marked latest, the tag is annotated and names `644cfe4`, which is on origin's `main`, CI passed on
  `main`'s head and on the tag, the proxy serves the `.info` at that commit, `go list -m @latest` gives `v1.35.0`, and
  `go mod download` of the tag passes with the checksum database on.
* **`python3 tools/consumer.py v1.35.0`:** exit 0, "as required". A module at the `go 1.26.10` floor fetched `v1.35.0` from
  the proxy; its program printed `WANT_RUN`; `go test -count=1 .` passed, `TestToolCallStream` included; it built for
  windows/amd64; its `go.mod` kept the `go` line and gained no toolchain line.

Every Verification item holds and the release is published: this PLAN is `complete`.
