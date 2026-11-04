---
status: in-progress
date: 2026-11-04
associated-madr: "0049-MADR-remediate-the-2026-11-01-debugging-pass.md"
---

# Implement 0049, the subpackages: P1 to P9

Associated MADR: [0049-MADR-remediate-the-2026-11-01-debugging-pass.md](0049-MADR-remediate-the-2026-11-01-debugging-pass.md)

## Goal

Fix the subpackage findings of the MADR's F-P section, P1 to P9, and widen the margin of the flaky
`TestInProcessServeAgentStep2`, as the MADR states them. The owner's answers settle two: Q5 (a), a `HandlerOptions` Host
allowlist defaulting to the loopback names on a loopback listener, for P2; and Q6 (a), discard an over-long WebSocket message
and answer `-32600`, for P6. The MADR's "The subpackage PLAN's designs, proposed 2026-11-04" puts the four details this PLAN
needs, Q12 to Q15, answered by the owner on 2026-11-04: Q12 (a), Q13 (a) and Q14 (a), the recommended ones, and Q15 (c),
no 32-bit run. Each fix lands with its finding's probe as a committed test, seen to fail on the code it replaces and to pass
after. This is the third of the MADR's five PLANs (Q1 a, Q7 a), and its fixes go out as a minor release, `v1.33.0`, with
notes that lead with the behaviour changes.

Base: `277aca0`, whose Go code is `v1.32.0`'s. Every file and line below is at that commit; the commit that adds this PLAN
changes records and documentation only. The MADR's "Re-assessment at `277aca0`" shows every P probe failing there.

## Scope

In scope:

* **`remote`:**
  * `ws.go`: `dialWebSocket` (`:32-47`), `readLoop` (`:71-83`), `runWebSocket` (`:118-135`), `readWebSocket` (`:141-157`),
    `pumpWebSocket` (`:163-176`), `keepAlive` (`:180-202`), `pump` (`:207-224`);
  * `frame.go`: `lineQueue` (`:104-178`) and `lineSplitter`'s over-long path (`:39-72`);
  * `handler.go`: `HandlerOptions` (`:27-60`), `resolveHandlerOptions` (`:109-130`), `ServeHTTP` (`:141-169`),
    `serveWebSocket` (`:209-225`), `serverConn` (`:299-313`);
  * `client.go`: `DialOptions` (`:16-33`), `Conn.Read`'s doc (`:49-51`), `resolveDialOptions` (`:88-107`);
  * `httpclient.go`: `newHTTPClient`'s queue (`:72`), the connection stream's end (`:262-270`);
  * `httpserver.go`: `route` (`:354-363`), for P9's answer;
  * a new `host.go` for the Host rule.
* **`clienthost`:** `files.go` whole (`ReadTextFile`, `WriteTextFile`, `regular`, `selectLines`), and two new files,
  `open_unix.go` and `open_other.go`, for the open that does not block.
* **`acptest`:** `client.go:161-188`, `MemoryClient.ReadTextFile`.
* **`agentproc`:** `linetap.go` (`lineTap`, `:29-56`), `transcript.go` (`Transcript`, `:25-85`), and
  `inprocess_test.go:379-417`, `TestInProcessServeAgentStep2`.
* **Tests:** one test per finding, under the names each step gives, in the packages they fix; and the existing tests that pin
  the old behaviour, each named in its step.
* **Docs:**
  * `README.md`: "Package `remote`" (`HandlerOptions`, `DialOptions`, the cause table, the host's part, "Not supported"),
    the back-pressure section's remote paragraph (`:945-947`), `clienthost.Files` (`:143-145`), and the Transcript paragraph
    (`:864-866`);
  * `docs/architecture.md`'s `remote`, `clienthost` and `agentproc` sections;
  * each changed exported identifier's godoc.
* **The release,** a minor `v1.33.0`, on the owner's ask.

Out of scope:

* the tooling and generator findings, each the MADR's own PLAN (Q7 a);
* `remote`'s other behaviour: Streamable HTTP's `413`, `MaxBuffered`'s `ErrOverflow`, the ladder, `Grace`;
* any change to a gated package's exported API (the root, `agentproc`, `acptest`, `clienthost`);
* a 32-bit run in CI, the gates or the hosts (Q15 c);
* the 1.26.0 schema.

## Design rules

1. **No incompatible change** ([0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md](0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md)).
   No exported identifier of a gated package changes or is removed, and none is added. `remote` gains two fields,
   `HandlerOptions.AllowedHosts` (Q13 a) and `DialOptions.MaxBuffered` (Q12 a). `remote` is not under the Go 1 promise
   (`remote/doc.go:7-10`), and `make api` reports its changes without gating them; it counts their compatible ones with the
   rest (`scripts/check_api.py:135-140`), so the release is a minor.
2. **Behave as stdio does.** Where a transport differs from a connection over a child's pipes, the fix moves it to the
   connection's rule: a reader that falls behind slows its peer and is not dropped (P1); an over-long message is discarded and
   answered `-32600` with `"id": null`, and reading goes on (P6, P9; `connection.go:1597-1601`); a clean end is `io.EOF` (P8).
3. **Bounds stay bounded.** Each new buffer has a byte bound and a default, and a peer past it meets a documented end, as
   `MaxBuffered` and `WithMaxQueuedBytes` do. A message is never held whole to be discarded: an over-long one is drained.
4. **No new blocking.** A write the reader or the pump makes on its own account never waits on the peer it is reading from,
   the rule R1 set for the connection's own answers
   ([0049-PLAN-runtime-fixes.md](0049-PLAN-runtime-fixes.md), phase 1).
5. **Errors keep their types.** `ErrDisconnected`, `ErrOverflow` and the refusals' texts stay as they are; a new refusal is a
   status and a fixed text, logged by its label, never by a message or an id (`HandlerOptions.Logger`'s rule,
   `handler.go:56-59`).
6. **Behaviour changes are documented where the call is,** in the godoc, the README and the release notes.

## Implementation Steps

Every phase ends staged for the owner, who commits. A phase begins only on the owner's word. Each new test is a probe from
the pass's scratch copy (`TestProbe…` under `p49/reverify-sub/src` and `p49/reassess-277aca0/src` in the session's
scratchpad) under a descriptive name. Each is seen to fail on a planted break in a scratch copy, never in the tree, with the
plant restoring the code the test guards: by `tools/plants.py` where it can run the test, and by a scratch script where the
test needs `-race` or linux/386, which `plants.py` does not pass (`tools/plants.py:83-90`). Every phase runs
`make pre-add-check` with its files, the package's tests with `-race -count=3`, `tools/remote.py tree --host linux`, and
`tools/remote.py pkg --host windows` for the package's new tests.

The phases follow the MADR's order, P1, P3, P4, P7, P5, P2, P6, grouped by package: P5 joins P3 and P4, since P3's rewrite of
`selectLines` removes the conversions P5 is about, and the order would otherwise rewrite the same lines twice. P8 and P9,
found on 2026-11-04, join P6, the message limit's phase.

### Phase 1: P1, the WebSocket keepalive under back-pressure (Q12 a)

1. **The client reads every frame, up to a byte bound.**
   * `lineQueue` (`frame.go:104-178`) counts bytes, not lines: `newLineQueue(limit int)`; `push` waits while the queued bytes
     plus the line's would pass `limit` and the queue is not empty, so one line larger than the bound is still taken alone.
     `queueLimit` goes.
   * `DialOptions.MaxBuffered`, new: "the most bytes of messages a connection holds for its reader before it stops reading
     the server. Zero or less is 64 MiB. It is at least `MaxMessageSize`, raised to it." `resolveDialOptions` sets it.
   * `dialWebSocket` and `newHTTPClient` pass it. Streamable HTTP's queue was 256 lines too (`httpclient.go:72`); its
     bound becomes the same bytes.
   * Below the bound, `readLoop` returns to `Read` at once, so the library reads the server's pings and answers them.
2. **The server does not count a pong missed while its reader waits on the agent.**
   * `serverConn` gains `waiting atomic.Int32` and `waited atomic.Bool`. `readWebSocket` (`ws.go:155`) adds 1 to
     `waiting` and sets `waited` before `sc.inW.Write`, and subtracts 1 after.
   * `keepAlive` clears `waited` before each ping. A ping that fails while `waiting` is above 0, or with `waited` set, is
     logged at `Debug`, "remote: WebSocket ping not counted: the agent is not reading", and the loop goes on; any other failure
     ends the connection as now.
   * A failed `Ping` leaves the socket open: on its context's end it returns "failed to wait for pong" and closes nothing
     (coder/websocket `v1.8.15`, `conn.go:232-257`), as the probe's 10 s run shows.
3. **`HandlerOptions.KeepAlive`'s doc** says a ping is not counted while the agent is not reading, and `DialOptions`'
   that a client past `MaxBuffered` stops reading, and so stops answering pings and is ended `ErrDisconnected`.
4. **Tests,** `remote/ws_test.go`:
   * `TestWebSocketSlowClientStaysConnected`: `TestProbeWSSlowClientDisconnected`, a 100 ms keepalive, a client that reads
     nothing for 1 s while the agent writes 2 000 lines; the serve's context must not end, and the client reads all 2 000.
   * `TestWebSocketBusyAgentStaysConnected`: `TestProbeWSBusyAgentDisconnected`, an agent that reads nothing for 1 s while a
     live client waits; the serve's context must not end.
   * `TestWebSocketClientPastMaxBuffered`: `DialOptions.MaxBuffered` of 4 KiB, a client that reads nothing, an agent that
     writes 1 MiB; the serve ends with `ErrDisconnected`, so the bound is enforced.
   * `TestWebSocketKeepAlive` (`:376`), a raw client that never reads, is unchanged and must still pass.
   * `remote/frame_test.go`'s `TestLineQueue` (`:87`) is changed to the byte bound: a push past it waits, one line past it
     alone is taken, and the lines after `end` are read. It tested the 256-line rule this step removes; the record names it.
5. **Docs:** `README.md`'s `DialOptions` and `KeepAlive` bullets, and the back-pressure section's remote paragraph
   (`:945-947`): over WebSocket, a paused reader no longer drops the connection below the bound. `docs/architecture.md`'s
   `lineQueue` and keepalive.
6. **Seen to fail,** by `tools/plants.py`:
   * `push` bounded by 256 lines again: `TestWebSocketSlowClientStaysConnected` fails;
   * `keepAlive` counting every failed ping: `TestWebSocketBusyAgentStaysConnected` fails;
   * `push` never waiting: `TestWebSocketClientPastMaxBuffered` fails.

### Phase 2: P3, P4 and P5, `clienthost.Files` and `MemoryClient` (Q15 c)

1. **Open first, then check the handle (P4).**
   * `open_unix.go` (`//go:build unix`): `openFile(path string, flag int, perm fs.FileMode)` opens with `syscall.O_NONBLOCK`
     added, so a FIFO's open returns at once: for reading it succeeds, for writing with no reader it fails `ENXIO`.
     `open_other.go`: `os.OpenFile` unchanged. A regular file ignores `O_NONBLOCK`.
   * `ReadTextFile` opens with `os.O_RDONLY`, then `f.Stat()`: not a regular file is refused with today's text, "clienthost:
     %s is not a regular file", and `ENXIO` is refused the same way. A missing file stays `acp.NewResourceNotFound`.
   * `WriteTextFile` keeps the directory check and `MkdirAll`, then opens with `os.O_WRONLY|os.O_CREATE`, mode `0o644`, and
     no `O_TRUNC`, checks the handle as above, then `f.Truncate(0)`, writes, and returns `Close`'s error. So nothing that is
     not a regular file is truncated or written.
   * `regular` keeps only the absolute-path check. The stat-then-open window (`files.go:86` against `:54` and `:74`) is gone.
2. **Read only what is asked (P3).**
   * With neither `line` nor `limit`, the file is read whole into a `strings.Builder` grown to its size: one copy.
   * Otherwise a `bufio.Reader` of 64 KiB skips `line - 1` lines and keeps at most `limit`, each with its end as the file has
     it, then stops. A line longer than the buffer is read in pieces by `ReadSlice` and its `bufio.ErrBufferFull`, so a skipped
     line is never held.
   * The rules of `Files`' doc hold: a line past the end gives no content, a last line without a newline is a line, `line` 0 is
     invalid params.
3. **Count in `uint64` (P5).** The skip and the limit compare `uint64` counts with `uint64(*line)` and `uint64(*limit)`;
   nothing converts a `uint32` to `int`. `MemoryClient.ReadTextFile` (`acptest/client.go:161-188`) selects its lines the
   same way.
4. **Tests:**
   * `clienthost/files_test.go`:
     * `TestReadTextFileBoundedAllocation`: `TestProbeReadOneLineOfBigFile`. A 64 MiB file of two-byte lines; line 1, limit
       1 allocates under 1 MiB (`runtime.MemStats.TotalAlloc`); line 1, no limit, under three times the file.
     * `TestReadTextFileLinesPast32Bits`: lines of 2^31, 2^31 + 1 and 2^32 - 1, and limits of 2^31 and 2^32 - 1, on a
       three-line file: no panic, `""` past the end and every line from `line` for a large limit. On a 64-bit host, where
       every check runs (Q15 c), it checks the arithmetic and cannot fail on the old code; on linux/386 it is the probe.
     * `TestReadTextFileLines` (`:19`) and `TestWriteTextFile` (`:85`) keep passing unchanged: the line rules and the writes.
   * `clienthost/files_unix_test.go` (`//go:build unix`):
     * `TestFilesFIFOSwap`: `TestProbeFifoSwap`, the path flipped between a regular file and a FIFO, up to 50 000 reads or 2 s;
       no read may still be blocked after 500 ms. The fixed code cannot block, so the test cannot fail falsely; the old code
       blocked in every run of the probe.
     * `TestFilesRefuseFIFO`: a FIFO with no writer is refused by `ReadTextFile` and `WriteTextFile` within 1 s, each with the
       "not a regular file" text. `TestFilesRefuseDevice` (`:134`) keeps the device case.
   * `acptest/acptest_test.go`: `TestMemoryClientLinesPast32Bits`, the same cases as `clienthost`'s.
5. **No 32-bit run is added (Q15 c).** The two new tests run once each as linux/386 on the Linux test host, on the fixed
   code (`tools/remote.py pkg --host linux --goarch 386 --tree <copy>`), and the record says what passed; that run and the
   plant's below are the phase's only 32-bit runs.
6. **Docs:** `Files`' godoc: what it reads, and that it refuses what is not a regular file at the open; `README.md`'s
   `clienthost.Files` (`:143-145`); `docs/architecture.md`'s `clienthost` section.
7. **Seen to fail:**
   * by `tools/plants.py`: `os.ReadFile` and `strings.SplitAfter` restored: `TestReadTextFileBoundedAllocation` fails;
     `regular`'s stat restored before the open, the open without `O_NONBLOCK`: `TestFilesFIFOSwap` fails;
   * by script, as linux/386 on the Linux test host, since on a 64-bit host the plant cannot fail: the `int` conversions
     restored in each package: `TestReadTextFileLinesPast32Bits` and `TestMemoryClientLinesPast32Bits` fail with the probe's
     panics.

### Phase 3: P7, `Transcript.Close`, and the step-2 margin

1. **`lineTap` guards what it holds.** `lineTap` gains a `sync.Mutex` around `held`, taken in `Write` to join and split and in
   `Close`, and released before each call of `fn`, which may take the Transcript's lock. The writer stays for one goroutine;
   the lock is for `Close` from another, which `LineTap`'s doc now allows.
2. **`Transcript.Close` ends the record.** `Transcript` gains `closed`, under `t.mu`. `Close` sets it after closing the two
   taps; `write` writes nothing once it is set. A write in progress holds `t.mu`, so `Close` returns after it, and nothing
   reaches `w` once `Close` has returned. The doc says so, and that `Close` may be called while a tap `Wait` abandoned is
   still writing.
3. **The margin.** `TestInProcessServeAgentStep2` runs with `Grace: slack`, 3 seconds, the suite's bound for a step on a busy
   runner (`agentproc_test.go:22-24`), as `descendantGrace` is `2 * slack` for its test. `within`'s bound becomes
   `3*slack+slack`. The `ErrTerminated` assertion stays. Under the probe's load, 1.2 s killed none of 100, with the latency's
   tail at 688.7 ms; 3 s is over four times that.
4. **Tests:**
   * `agentproc/transcript_test.go`: `TestTranscriptCloseAfterAbandon`, `TestProbeTranscriptCloseAfterAbandon` made exact: the
     agent writes two lines in two writes to a writer that blocks, so one is queued when `Wait`'s grace abandons the tap;
     `DroppedTapBytes` must be above 0, the precondition the probe could not check (the MADR's re-assessment at `e98987a`,
     P7); then `Close`, the writer released, and nothing more written to it within 100 ms. Under `-race` in CI, a race fails
     it.
   * `TestTranscriptLines` (`:39`) and `TestTranscriptWriteError` (`:79`) keep passing unchanged.
5. **Docs:** `Transcript.Close`'s and `LineTap`'s godoc; `README.md`'s Transcript paragraph (`:864-866`): `Close` once the
   agent is done, and nothing is written after it.
6. **Seen to fail,** by script, with `-race -count=3`: `lineTap`'s lock removed: `TestTranscriptCloseAfterAbandon` reports
   "DATA RACE"; `write` ignoring `closed`: the same test fails on the late write. The margin is a test's bound and has no
   plant: the probe is run at 3 s under the same load, 100 runs, and the record gives its kills and tail.

### Phase 4: P2, the Host rule (Q5 a, Q13 a)

1. **`HandlerOptions.AllowedHosts`,** new: "the host names a request's `Host` header may carry, as patterns matched as
   `OriginPatterns` are, the port ignored. Nil, on a listener whose address is loopback, allows an IP literal, `localhost`
   and a name under `.localhost`, and refuses any other name, as a DNS rebinding sends; nil elsewhere allows every name."
2. **`host.go`'s `hostAllowed(r *http.Request, patterns []string) bool`:**
   * the name is `r.Host` without its port (`net.SplitHostPort`, or the whole when there is none), its brackets dropped,
     lower-cased;
   * with `patterns`, it passes when one matches (`path.Match`, as `originAllowed` does, `handler.go:189-196`);
   * without, the listener is `r.Context().Value(http.LocalAddrContextKey)`: a `*net.TCPAddr` whose `IP.IsLoopback()` allows
     an IP literal (`netip.ParseAddr`), `localhost` and a name ending `.localhost`; any other listener, or none, allows all.
3. **`ServeHTTP`** checks it after the closed check and before the WebSocket branch, so the upgrade is checked too, and refuses
   `403` with "remote: the request's host is not allowed", logged as a refusal.
4. **Tests,** `remote/handler_test.go`:
   * `TestHandlerHostRule`: `TestProbeDNSRebinding`, on `httptest`'s loopback listener: `Host` `attacker.example:PORT`, with
     and without a matching `Origin`, refused `403` on POST, GET, DELETE and the WebSocket upgrade; `127.0.0.1`, `[::1]`,
     `localhost` and `agent.localhost`, each with its port, pass; `AllowedHosts: {"agent.example"}` lets that name pass and
     refuses `localhost`.
   * `TestHandlerHostRuleOffLoopback`: `ServeHTTP` called directly, with `LocalAddrContextKey` set to a non-loopback
     `*net.TCPAddr`: any name passes with `AllowedHosts` nil, and only a listed one with it set.
   * `TestHTTPServerOrigin` (`httpserver_test.go:589`) and `TestWebSocketOrigin` (`ws_test.go:287`) keep passing: their
     requests carry the listener's own `Host`.
5. **Docs:** `HandlerOptions`' godoc; `README.md`'s `HandlerOptions` bullets, `OriginPatterns`' caveat, and the host's part
   (`:1267-1276`): the rule, and that a server behind a name it reaches on loopback, such as a reverse proxy's, lists the
   name; `docs/architecture.md`'s server paragraph.
6. **Seen to fail,** by `tools/plants.py`: `hostAllowed` returning true: `TestHandlerHostRule` fails; the loopback test
   inverted: both tests fail; the check after the WebSocket branch: the upgrade case fails.

### Phase 5: P6, P9 and P8, a message's limit and a stream's end (Q6 a, Q14 a)

1. **A reader discards an over-long frame and answers it (P6).**
   * `serveWebSocket` and `dialWebSocket` call `c.SetReadLimit(-1)` ("Set to -1 to disable", coder/websocket `read.go:96`),
     in place of `MaxMessageSize` (`handler.go:217`, `ws.go:39`).
   * `readWebSocket` and `readLoop` read each frame through `c.Reader` into a buffer bounded at `MaxMessageSize + 1`. A frame
     over the limit is drained to its end with `io.Copy(io.Discard, r)`, never held, and answered on the socket with
     `{"jsonrpc":"2.0","id":null,"error":{"code":-32600,"message":"Invalid request","data":{"error":"message exceeds N
     bytes"}}}`, the connection's own text (`connection.go:1601`), under `Grace` on the server; logged by its size; and reading
     goes on.
   * A frame at the limit is taken as now. A POST over it keeps its `413`.
2. **The pump answers an agent's over-long line (P9).** `pump` (`ws.go:207-224`), for both profiles, answers the agent on its
   input with the same `-32600` line, as the client's connection would on stdio. The answer is written by a goroutine of its
   own, so the pump never waits on the agent's reader (design rule 4); while one answer waits, a later over-long line is
   logged and not answered, so at most one goroutine is held. It ends when the agent reads it or the input closes.
3. **A clean end is `io.EOF` (P8).** The connection stream's end (`httpclient.go:262-270`) ends the reader with `io.EOF`
   itself when the stream's error is `io.EOF`, and wraps any other as now. `Conn.Read`'s doc then holds for both profiles.
4. **Tests:**
   * `remote/ws_test.go`:
     * `TestWebSocketOverlongFrameAnswered`: `TestProbeWSLimit` made a test. A raw client sends a frame at the limit, one
       over it and one at the limit again; the serve reads the first and the third, the client receives the `-32600` answer
       with `"id": null`, and the socket stays open. Then the server's side: an agent writes a line at the client's limit, one
       over and one at; the client `Conn` delivers the first and the third, and the agent's input receives the `-32600`
       answer.
     * `TestWebSocketLargeMessage` (`:167`) required "one byte past MaxMessageSize to end the connection on the side that
       received it". It is changed to the new rule, its 64 KiB round trip kept; the record names it.
   * `remote/httpserver_test.go`: `TestHTTPServerOverlongAnswer`, `TestProbeOverlongAnswerDropped` over Streamable HTTP and
     WebSocket: the agent's over-long answer to request 1 is answered `-32600` on its input, and the next message still
     arrives.
   * `remote/httpclient_test.go`: `TestHTTPClientCleanEndIsEOF`: `TestProbeConnReadEOF`, `Read` after the agent's clean end
     returns `io.EOF` itself over both profiles, so `io.ReadAll` returns nil. `TestHTTPClientStreamEnds` (`:379`) keeps its
     unclean end's error.
5. **Docs:** `HandlerOptions.MaxMessageSize`' and `DialOptions.MaxMessageSize`' godoc, `Conn.Read`'s; `README.md`'s
   `MaxMessageSize` bullets and "Not supported"; `docs/architecture.md`'s WebSocket reader and pump.
6. **Seen to fail,** by `tools/plants.py`: the read limit restored: `TestWebSocketOverlongFrameAnswered` fails; the answer
   not written: the same test fails waiting for it; the pump's answer removed: `TestHTTPServerOverlongAnswer` fails; the
   wrapped `EOF` restored: `TestHTTPClientCleanEndIsEOF` fails.

### Phase 6: close, and the release on the owner's ask

1. `tools/gates.py --all`, with the hosts named per command. P1 to P9 and the margin are listed, each with the test that shows
   it, in a table.
2. `make api` against `v1.32.0`: 0 incompatible, and the compatible changes `remote`'s two fields alone, `AllowedHosts` and
   `MaxBuffered`.
3. After the owner's push, CI on the head.
4. On the owner's ask:
   * `docs/reports/0049-REPORT-release-notes-v1-33-0.md`, leading with the behaviour changes;
   * `tools/release_body.py`;
   * the owner's tag `v1.33.0` and release, or the job [0048-PLAN-create-releases-in-ci.md](0048-PLAN-create-releases-in-ci.md)
     adds, if it has landed;
   * `tools/publication.py`;
   * `tools/consumer.py v1.33.0`. Its program dials `remote` on loopback with the listener's own `Host`, which phase 4 allows,
     so it is unchanged unless a phase's run shows otherwise.
5. This PLAN and its row are marked `complete`, and the MADR's P findings done.

## Verification

* Each of P1 to P9 has a committed test that fails at `277aca0`'s code, by its plant, and passes after; P5's plant fails
  on linux/386, where alone it can.
* `make api`: two compatible additions, in `remote`, and nothing incompatible against `v1.32.0`.
* `go test -race ./...` passes in both modules.
* The behaviour changes are stated in the godoc, the README and the release notes:
  * a WebSocket that falls behind is no longer dropped, below `MaxBuffered`;
  * a loopback handler refuses a `Host` it does not know;
  * an over-long message is answered `-32600` on every transport;
  * `Files` reads only what it is asked, and refuses what is not a regular file at the open;
  * `Transcript.Close` ends the record;
  * Streamable HTTP's clean end is `io.EOF`.
* `tools/gates.py --all` and CI pass.

## Rollout and Rollback

* **Rollout.** A minor release, `v1.33.0`. What a consumer meets, in the notes' order:
  * a request to a handler on a loopback listener whose `Host` is a name other than `localhost` is refused `403`; a server
    reached so, through a proxy for one, sets `AllowedHosts`;
  * a WebSocket whose reader falls behind stays open, up to `MaxBuffered` on the client; the server holds what it held;
  * an over-long WebSocket message, from either side, and an agent's over-long answer, are answered `-32600` and the
    connection reads on, where it closed or dropped;
  * `Transcript.Close` writes nothing after it returns;
  * `Files` and `MemoryClient` take any `line` and `limit` on a 32-bit target, and `Files` reads one line of a large file
    without reading the file;
  * `remote.Conn.Read` ends with `io.EOF` itself at a Streamable HTTP agent's clean end.
* **Rollback.** Revert the phase's commit; each phase stands alone. A consumer that needs the old behaviour pins `v1.32.0`.

## Execution record

The owner answered Q12 to Q15 on 2026-11-04 (the MADR's "Answered by the owner, 2026-11-04"), committed this PLAN with the
documentation's corrections as `f6bfdcf`, and said "committed, proceed".

### Phase 1: P1, the WebSocket keepalive under back-pressure, 2026-11-04

Begun on the owner's "committed, proceed" after `f6bfdcf`. No deviation.

**What it did:**

* **`remote/frame.go`:** `lineQueue` counts the bytes it holds (`held`) against `limit`, from `newLineQueue(limit)`. `push`
  waits while `held` is above 0 and the line would take it past `limit`, so a line longer than the limit is taken once the
  queue is empty. `Read` takes `held` down by what it copies and wakes a waiting `push`. `queueLimit` is gone.
* **`remote/client.go`:** `DialOptions.MaxBuffered`, its doc as step 1.1 has it; `defaultMaxBuffered`, 64 MiB, which
  `resolveHandlerOptions` uses too; `resolveDialOptions` defaults it and raises it to `MaxMessageSize`.
* **`remote/ws.go` and `httpclient.go`:** both clients' queues take `MaxBuffered`. `readWebSocket` adds to `waiting` and
  sets `waited` around each write to the agent's input. `keepAlive` clears `waited` before each ping, and a failed ping with
  either set is logged at `Debug`, "remote: WebSocket ping not counted: the agent is not reading", and not counted.
* **`remote/handler.go`:** `serverConn`'s `waiting` and `waited`; `KeepAlive`'s doc. The Streamable HTTP inbox's 256, which
  used `queueLimit`, is `inboxLines`, with the doc the field had; its behaviour does not change.
* **The tests:**
  * `remote/ws_test.go`, beside `backPressureServer`, a handler with the probes' 100 ms keepalive and 500 ms grace:
    `TestWebSocketSlowClientStaysConnected`, `TestWebSocketBusyAgentStaysConnected` and
    `TestWebSocketClientPastMaxBuffered`, as step 1.4 has them. The last uses `MaxMessageSize` 1 024 and `MaxBuffered`
    4 096, since `MaxBuffered` is raised to `MaxMessageSize`.
  * `remote/frame_test.go`'s `TestLineQueue` is changed to the byte bound: a push past it waits through a read that does not
    make room and returns after one that does, and a line of twice the limit is taken alone and holds the next push until it
    is read. It tested the 256-line rule; its order, `end` and push-after-end cases are kept.
  * `TestWebSocketKeepAlive`, a raw client that never reads, passes unchanged.
* **Docs:** `README.md`'s back-pressure paragraph on `remote`, the `DialOptions` `MaxBuffered` bullet and `KeepAlive`'s;
  `docs/architecture.md`'s `lineQueue`, `readWebSocket` and `keepAlive`.

**Seen to fail,** by `tools/plants.py`, 3/3:

```text
FAILED AS REQUIRED: push bounded by 256 lines again (^TestWebSocketSlowClientStaysConnected$): exit 1: ws_test.go:453: a client slow to read was disconnected: cause remote: client disconnected
FAILED AS REQUIRED: keepAlive counts every failed ping (^TestWebSocketBusyAgentStaysConnected$): exit 1: ws_test.go:479: a live client was disconnected while its agent was busy: cause remote: client disconnected
FAILED AS REQUIRED: push never waits (^TestWebSocketClientPastMaxBuffered$): exit 1: ws_test.go:510: a client holding MaxBuffered and reading nothing was not ended
```

* `TestLineQueue`'s first rewrite failed on the fixed code, "the waiting push did not return within 5s of a read": a `Read`
  returns at most one line, so a 4-byte read took 2. It reads with `io.ReadFull` now.

**Checks:**

* `go test -race -count=3` of the four WebSocket tests and `TestLineQueue`: `ok`; `go test -race -count=3 ./remote/`: `ok`.
* `make api`: "against v1.32.0: 0 incompatible, 1 compatible", "remote: DialOptions.MaxBuffered: added".
* `make pre-add-check` with the phase's seven Go files, the README and `docs/architecture.md`, at load 27: exit 0, "7
  file(s) gofmt-clean; both modules linted, vetted and tested (-race -shuffle=on, uncached), tidy, govulncheck."
* `tools/remote.py tree --host linux`: both modules, exit 0.
* `tools/remote.py pkg --host windows ./remote -test.run 'TestWebSocket(SlowClientStaysConnected|BusyAgentStaysConnected|
  ClientPastMaxBuffered|KeepAlive)$|TestLineQueue' -test.count=3`: "PASS".
* `make check-records`, `make markdownlint`, `make disclosure-check`: exit 0 each. `tools/idscan.py --paths` over the phase's
  files: 0 matches.

Staged for the owner, who commits. Phase 2 begins on the owner's word.
