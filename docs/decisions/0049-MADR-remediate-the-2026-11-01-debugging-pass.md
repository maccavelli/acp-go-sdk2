---
status: accepted
date: 2026-11-02
decision-makers: the repository owner
consulted: "probe tests and scripts in scratch copies of `d55ba2a`; `schema/schema.json` and `schema/schema.unstable.json` at ACP 1.25.0; `go help test`; GitHub's workflow documentation"
informed: gobble-cli, magic-cli-remote
---

# Remediate the defects, gaps and dead code the 2026-11-01 debugging pass found, in phases by area and severity

## Context and Problem Statement

The owner asked on 2026-11-01 for a deep debugging pass over this repository: bugs, gaps, missing wiring and functionality, and
dead code, written up as a facts-grounded record whose assertions are proven, for review. The last such pass was
[0019-MADR-remediate-debugging-pass-findings.md](0019-MADR-remediate-debugging-pass-findings.md), on `v0.17.1`; since then the SDK
has gained the side-connection helpers, `agentproc`, `clienthost`, `remote`, `acptest`, the notification queue's back-pressure
and byte budget, middleware and the observer.

This record holds what the pass found, and proposes how to remediate it. It decides nothing yet: every finding is evidence, and the
owner's answers to "Questions for the owner" settle the program.

### How the pass was run

* **Commit:** `d55ba2a`, the tree of `v1.31.1` plus records, with Go `1.27.3` on macOS `arm64`, and Go `1.26.10` where a finding
  concerns the floor.
* **Areas,** each examined by a separate investigator with no knowledge of the others' results:
  * the connection runtime: `connection.go`, `middleware.go`, `observer.go`, `serve.go`, `errors.go`, `extensions.go`;
  * the SDK surface against the schema: the generated `*_gen.go`, `helpers.go`, `refusals.go`, `elicitation.go`, the timelines;
  * the generator: `cmd/generate`;
  * the subpackages: `agentproc`, `clienthost`, `remote`, `acptest`, `internal/proctree`;
  * tooling, CI and documentation: `tools/`, `scripts/`, the `Makefile`, the workflows, and the claims of `README.md`,
    `docs/architecture.md`, `RELEASING.md`, `AGENTS.md` and `tools/README.md`.
* **Method:** reading; probe tests that fail because of a defect, run in scratch copies (`git archive d55ba2a`), never in the tree;
  probe schemas fed to a scratch build of the generator; fuzzing `FuzzBuild`; Python cross-checks of the schema against the Go
  side; a coverage profile of both modules (`go test -coverprofile -coverpkg=./...`); the `deadcode` tool; and timing
  distributions under a CPU-starved runner for the one flaky test.
* **Evidence labels:** **Probe** means a probe reproduced the defect in a scratch copy; its command and output are quoted.
  **Read** means the defect follows from the quoted code, where no probe could run (for example a 32-bit target). Each finding
  also says who confirmed it: **re-run** means the author of this record ran the investigator's probe again and saw the same
  failure; **read** means the author read the cited code and found it as described. Every finding at medium severity or above was
  re-run or read by the author.
* **The tree was not changed.** `git status --porcelain` was empty after the pass. The probes live in the scratch copies under
  the session's scratch directory, `p49/`, and are not part of this record; the PLANs that follow turn the ones worth keeping into
  committed tests, each seen to fail before its fix.
* **Load.** The subpackage investigator worked while the host's load average was 60 to 130. Failures of timing tests under that
  load were set aside, and are not findings; the clean copy passed every suite.

### What holds

Checked and found correct, with the method:

* **Runtime:** the sequence and watermark invariants under `OverflowDropNewest`'s rollback; `Err` non-nil exactly when `Done` is
  closed; the notification queue closed once; `goroutines.Go` called only from counted goroutines; `Shutdown`'s admission under
  `c.mu`; every `initialize` commit path ending `settled`; slots and bytes released on every refusal (Read, each at the lines the
  runtime investigator cited). The root suite passes under `-race` in the scratch copy.
* **Surface:** every method of `meta.json` and `meta.unstable.json` has a constant, a dispatch case and a typed sender; the zero
  value of every generated non-union type encodes every schema-required key (a script compared them: "missing 0"); the `unstable`
  package and its frozen manifest agree with `schema.unstable.json`; every reference to a gated definition has its gate; every
  "absolute path" sentence of both schemas has a rule; [0016-REPORT-acp-v1-conformance-audit.md](../reports/0016-REPORT-acp-v1-conformance-audit.md)'s
  fixes still hold (Probe, `TestProbeHolds`, passes).
* **Generator:** regenerating the committed tree is byte-identical; 85 `accept-*` cases give identical output three times each on
  `arm64` and `amd64`; every refusal case of 0019's generator phase still refuses; the frozen manifest cannot be rewritten, and
  rebuilds byte-identically; 6.5 million fuzz executions with a recursion guard found no other crash.
* **Subpackages:** terminal output truncation holds a 20 000-case random property test (suffix, limit, character boundary,
  truncated flag, unfinished characters held back); `Dial`'s context bounds only the handshake; the WebSocket read limit has no
  off-by-one; a cross-site `Origin` that does not match `Host` is refused `403`.
* **Tooling:** `make check-records`, `check-tools` and `disclosure-check` pass; `go fix -diff` fails on a planted `interface{}`;
  the examples build, test, and run as documented; `version` equals `schema/version`; `opencode.json` is the intended OpenCode
  pointer.
* **Coverage:** 86.7% of the root module's statements and 95.1% of `cmd/generate`'s. Outside the example programs' `main`
  code, the untested functions are the public entry points of generated union variants (59 `UnmarshalJSON`, 24 `Validate`), whose
  unions call the shared lower-case methods the tests do reach (`types_gen.go:15471-15515`), and five small functions listed under
  F-C below.

### The flaky `TestInProcessServeAgentStep2`

It failed `v1.31.1`'s tag CI once (run `41339573142`, attempt 1; [0046-PLAN-sending-and-receiving-middleware.md](0046-PLAN-sending-and-receiving-middleware.md),
phase 8, deviation 1). **It is a test margin, not a defect.** Close's step 2 succeeds only if a chain of goroutines all get the
CPU within `testGrace`, 300 ms: `ServeAgent`'s select, `Shutdown`, the prompt handler, the agent's writer, the client's reader,
`inProcessAgent.run`, and `watch` closing `done`. Nothing in the chain blocks on anything only the kill could release.

* Unloaded, 100 runs: step 2 returned within grace 100 times, `p50` 0.208 ms, `max` 10.8 ms.
* With 32 busy goroutines, `GOMAXPROCS=2`, `go1.26.10`, `-race`: 9 kills in 100 runs, each with the CI's message, "Close = agentproc:
  agent killed: agentproc: agent terminated, want ErrTerminated".
* Goroutine stacks taken when step 2 timed out: every goroutine of the chain `[runnable]`, none blocked.
* The same load, one run each: `testGrace` 1.2 s failed 0 of 100; 300 ms failed 13 of 100.

The fix is the test's: a larger `Grace` for this test, as `descendantGrace` does elsewhere, keeping the `ErrTerminated` assertion.

## Findings

Severity is the effect on a consumer or a maintainer: **high** loses data, hangs, or crashes a correct program; **medium** gives a
wrong result or breaks a documented contract on a reachable path; **low** is an edge, a gap, or a wrong word.

### F-R: the connection runtime

**R1. High. Two SDK peers can deadlock: the reader writes its refusals synchronously.** `connection.go:1699-1718`, `:1735-1744`
(refusals), `:1524`, `:1653-1670` (faults), `:1957-1963` (`answer` writes with `sendMessage(context.Background())` on the reader's
goroutine), `observer.go:126-128`.

* Two connections over `io.Pipe`, each with `WithMaxConcurrentRequests(1)` and one handler held, each send the other 8
  requests. Each reader refuses by writing; each write waits for the other reader, which is itself blocked writing. Both readers
  stay blocked until `Close`, and even the admitted requests, once released, never answer.
* The same holds for refusals past `WithMaxQueuedBytes` and during `Shutdown`, and for the fault answers. Over `io.Pipe`, as
  `acptest`'s pairs and `agentproc.InProcess` connect, it is immediate; over OS pipes the kernel's buffer, about 64 KiB, must fill
  first.
* It contradicts `WithMaxConcurrentRequests`'s "The reader never waits for a handler": the reader waits for its peer.
* **Probe** (`TestProbeReaderWriteDeadlock`, re-run): `limit=0: ok=16 … timed out=0`; `limit=1: ok=0 refused=0 timed out=16;
  readers blocked in answerObserved: 2`.
* Fix: hand the reader's answers to the writer without waiting.

**R2. Medium. A receiving middleware can rewrite `initialize`'s result, and the agent records the result from before it.**
`agent_gen.go:383-401` records, then returns to the chain; `middleware.go:25-48`; `connection.go:872-877`. The sending side
guards this case (0046 D4, `ErrSendingMiddleware`); the receiving side neither guards nor documents it.

* **Probe** (`TestProbeReceivingMiddlewareInitializeRecord`, re-run): "client's answer loadSession=true; client's record
  loadSession=true; agent's record loadSession=false"; the client's `LoadSession` is answered `-32601 … capability loadSession not
  advertised`.
* Fix: Q2.

**R3. Medium. The response barrier is skipped for a request sent under another connection's notification context.**
`connection.go:2766-2768` (`waitNotificationsUpTo`'s cap), `:2116-2124` (`inRunningNotification`); `messageMark` (`:97-100`) holds a
sequence number but not which connection made it. A bridge whose notification handler on `x` calls `SendRequest` on `y` gets an
answer before `y`'s earlier notification is handled, against `MethodHandler`'s guarantee.

* **Probe** (`TestProbeBarrierAcrossConnections`, re-run): under `context.Background()`, "handled when SendRequest returned:
  true"; under `x`'s notification context, "false".
* Fix: record the owning connection in the mark, and apply the cap only on that connection.

**R4. Medium. `ServeAgent` returns at its context's end while handlers still run, and does not say so.** `serve.go:47-56` uses
`Shutdown` alone, which waits for requests only (`connection.go:1113-1136`); the input's-end path, `serve.go:35-46`, waits for
every handler and wraps `ErrHandlersRunning`.

* **Probe** (`TestProbeServeAgentContextEndLeavesHandlers`, re-run): at the input's end, "ServeAgent = acp: the connection's
  handlers still run 5s after its end" (as documented); at the context's end, "ServeAgent = the test's signal; agent's Cancel
  still running when ServeAgent returned: true".
* Fix: after `Shutdown`, `WaitContext` under the same five-second bound, wrapping `ErrHandlersRunning`.

**R5. Medium. A byte budget above `MaxInt64/2` closes the connection on the second notification.** `connection.go:2066-2072`:
`limit *= 2` overflows to a negative number under `OverflowBlock`, the default. `WithMaxQueuedBytes(math.MaxInt)` is the natural
"no limit", since `n <= 0` is ignored.

* **Probe** (`TestProbeByteBudgetReadOnOverflow`, re-run): `1<<40` "open, QueuedBytes=86"; `MaxInt` "the connection closed on the
  second notification: acp: connection closed: notification queue overflow".
* Fix: a saturating double.

**R6. Low. `WithMaxQueuedNotifications(math.MaxInt)` panics, and large values allocate their whole queue at once.**
`connection.go:865-868` (`queueCap *= 2`), `:899` (`make(chan …, queueCap)`). **Probe** (`TestProbeQueueSizeDoubling`, re-run):
"panicked: makechan: size out of range". Fix: clamp, and document a maximum.

**R7. Low. `WithMaxMessageSize` is off by one for CRLF lines.** `connection.go:1501` trims `"\n"`, keeping the `\r`, where `:1511`
trims `"\r\n"`. **Probe** (`TestProbeMaxMessageSizeCRLF`, re-run): a 50-byte message under a limit of 50 is accepted with `\n` and
answered `-32600 "message exceeds 50 bytes"` with `\r\n`. Fix: measure after trimming both.

**R8. Low. Malformed params from a sending middleware are not reported as `ErrSendingMiddleware`.** `middleware.go:192-195`
checks only the first byte. **Probe** (`TestProbeSendingMiddlewareBadParams`, re-run): `{"a":`, `[1,` and whitespace fail with "json:
error calling MarshalJSON … unexpected end of JSON input", not wrapping the sentinel, as 0046 D10 says they should. Fix:
`json.Valid`.

**R9. Low. `$/cancel_request` is sent for a request that was never written.** `connection.go:2539-2550` sends the cancel whenever
the context ended, `sent` or not; 0019's B9 and B22 used `sent` for `session/cancel` only. **Probe**
(`TestProbeCancelForUnsentRequest`, re-run): the peer read `{"method":"$/cancel_request","params":{"requestId":2}}` for request 2,
which it never received, and the observer reported a cancel sent. Fix: send it only when `sent`.

**R10. Low. `Wait` can block after `Close` on a writer that cannot be interrupted, and the doc does not say so.**
`connection.go:956-966`, the doc at `:1035-1040`. **Probe** (`TestProbeWaitBlockedByWriter`, re-run): "WaitContext after Close =
context deadline exceeded". Fix: name the case in `Wait`'s and `Close`'s doc.

### F-S: the SDK surface against the schema

**S1. Medium. `session/set_config_option` with an unknown `type` and a string `value` decodes to `Unknown`, not `ValueId`.**
`types_gen.go:16339-16342`. The schema, `SetSessionConfigOptionRequest/anyOf/1`: "Unknown `type` values with string payloads also
gracefully deserialize into this variant." The agent's handler cannot read `sessionId`, `configId` or `value`; `Kind()` says
`"select"`. **Probe** (`TestProbeSetConfigUnknownTypeE2E`, re-run): "agent handler got ValueId=nil, Boolean=false, Kind()="select"".
A patched copy passed the probes and every existing test. Fix: in this union's decoder, a string `value` with an unknown `type`
goes to `ValueId`.

**S2. Medium. A `tool_call_update` cannot send `"content":[]` or `"locations":[]`, so an agent cannot clear them.**
`types_gen.go:14357`, `:14361`, `:18445`, `:18449` tag both `omitempty`; `helpers.go:908-921`. The schema: "Replace the content
collection", "Replace the locations collection"; the SDK's own receiving side treats `[]` as clear (`tooltimeline.go:45-47`).
**Probe** (`TestProbeToolCallUpdateEmptyContentOnWire`, re-run): `WithUpdateContent([])` sent `{"sessionUpdate":"tool_call_update",
"toolCallId":"t1"}`. Fix: encode a non-nil empty slice.

**S3. Low. The unstable capabilities are not gated.** `providers/*` against `agentCapabilities.providers`, `nes/*` against
`agentCapabilities.nes`, `document/*` against `nes.events.document.*`, `plan_update`/`plan_removed` against
`clientCapabilities.plan`, and NES suggestion kinds against the client's `nes.*` (`types_gen.go:23830-23911`, `:24156-24205`;
`cmd/generate/internal/model/acp.go:51-88`, `:162-189`). **Probe** (`TestProbeUnstableCapabilitiesNotGated`, re-run): each allowed
without its capability. Fix: Q3.

**S4. Low. `ToolCallTimeline` and `CompactionTimeline` return copies that share nested values.** `tooltimeline.go:172-180`,
`compaction.go:134-139` clone slices of pointer-variant unions shallowly; the doc says "as a copy". **Probe**
(`TestProbeToolTimelineCopyAliases`, `TestProbeCompactionTimelineCopyAliases`, re-run): "timeline state changed through a returned
copy". Fix: copy deeply, and make the `*Copies` tests mutate through a pointer.

**S5. Low. A boolean `session/set_config_option` is not gated on `session.configOptions.boolean`.** The schema,
`SessionConfigOptionsCapabilities/properties/boolean`: "the client may send `session/set_config_option` requests with `type:
"boolean"`" once advertised; `acp.go:187` gates only `SessionConfigOption`. **Probe** (`TestProbeBooleanSetConfigNotGated`, re-run):
"sent and handled without session.configOptions.boolean". Fix: a variant gate.

**S6. Medium. Closed enums are never checked, so the SDK sends schema-invalid values.** `StopReason`, `PermissionOptionKind`,
`PlanEntryPriority`, `PlanEntryStatus`, `ToolKind`, `ToolCallStatus` and `Role` are closed sets of string constants; the required
`stopReason`, `kind`, `priority` and `status` accept any string, `""` included (`types_gen.go:11409-11411`). An agent whose `Prompt`
returns `PromptResponse{}` answers `{"stopReason":""}`. On decode, a lenient field keeps an unknown value, where 0012's treatment 5
reads a value that fails as absent, and `x-deserialize-skip-invalid-items` keeps `"system"` in `audience`.

* **Probe** (re-run): `TestProbeClosedEnumsNotValidated`, "PromptResponse{} (stopReason "") validates; … PlanEntry with
  priority/status "" validates"; `TestProbeEmptyStopReasonSent`, "agent answered and client accepted stopReason """;
  `TestProbeClosedEnumLenientDecode`, "kind "browse"/status "queued" kept …; audience [user system] kept 2 items".
* Fix: Q4.

**S7. Low. Naming the key-less variant explicitly gives `Unknown`, while `Kind()` returns that variant's name.** `McpServer`
(`types_gen.go:8271-8274`) and `AuthMethod` (`:748-751`); `Kind()` at `helpers.go:439-469`. A handler that switches on `Kind()`
dereferences a nil `Stdio`, and the server skips `ResolveStdioCommands`. **Probe** (`TestProbeKeylessVariantNamedExplicitly`,
re-run): "McpServer {"type":"stdio",...}: Stdio=nil, Kind()=stdio". Fix: decode the key-less variant's own name into it.

**S8. Low. `authenticate` with a `methodId` the agent never advertised is sent and dispatched.** `types_gen.go:23947-23954`. The
schema: "Must be one of the methods advertised in the initialize response." **Probe** (`TestProbeAuthenticateUnadvertisedMethod`,
re-run): "AllowsValue nil, sent (err=<nil>), agent handler ran". Fix: check the id against the recorded `authMethods`.

**S9. Low. A prompt block of an undeclared type passes the prompt gate.** `types_gen.go:24079-24111`; `PromptCapabilities`: "Other
variants must be explicitly opted in to." The elicitation gate refuses the analogous mode (0019 decision 6). **Probe**
(`TestProbePromptUndeclaredBlockNotGated`, re-run): "prompt with an undeclared "video" block allowed". Fix: a
`GateUndeclaredVariant` rule.

### F-G: the generator

**G1. Medium. `output.Check` checks a different tree from the one `Write` leaves.** `cmd/generate/internal/output/check.go:53-56`,
`:72-74`, `:183-215`; `write.go:62`, `:89-93`. `handWritten` takes the directory's Go files less the rendered names, so a stale
`*_gen.go` is checked as hand-written and then deleted by `Write`; and `Check` returns before `unstable/` when nothing unstable
renders, though `Write` removes `unstable_gen.go`. Read by the author at those lines.

* **Probe** (investigator): a stale `old_gen.go` used by a hand-written file: generator exit 0, then `go build` fails
  "undefined: staleHelper"; the same for `unstable/`; and the inverse, a stale file that collides, refused although `Write` would
  have deleted it.
* Fix: exclude the stale files from the checked set, and check `unstable/` whenever it holds hand-written files.

**G2. Medium. A self-referential default crashes the generator.** `internal/model/defaults.go:236-262`: `structValue` recurses
through a property's default with no visited set. **Probe** (`recursive-default.txtar`, re-run): "gen rc=2", "fatal error: stack
overflow", in `renderer.structValue`. `FuzzBuild` found it in 25 s. Fix: refuse a recursive default, naming it.

**G3. Medium. A property the manifest changed silently overrides the stable schema's own later definition of it.**
`internal/load/manifest.go:194-203`, `:222-228`; `model.go:476-478` drops the notice too. **Probe** (`stabilized.txtar`, re-run): the
stable `Shared.b` is `{"type":"integer"}`; the generator exits 0 and emits `B string`; decoding `{"b":5}` fails "cannot unmarshal
number into Go struct field Shared.b of type string". Fix: refuse when the stable definition now holds the key and differs.

**G4. Low. Integer bounds pass through `float64`.** `load/load.go:64-65`, `model/validate.go:63-67`, `:304-321`,
`emit/types.go:1306-1314` (`uint64` of an out-of-range float, implementation-defined). **Probe** (investigator): a `uint64`
`minimum` of 2^64 emits `< 18446744073709551615` on `arm64` and `< 9223372036854775808` on `amd64`; an `int64` `maximum` of
2^53+1 is emitted as 2^53. No committed case is affected. Fix: read bounds as `json.Number`, refusing magnitudes at or above 2^53 as
`integerConst` does.

**G5. Low. Shapes silently dropped or loosened, each exit 0.** (a) `not` and `unevaluatedProperties` outside a catch-all;
(b) `const` and `enum` on an object property; (c) bounds on array items and map values; (d) `$ref` siblings; (e) a definition
that is only a `$ref`, emitted as raw JSON; (f) an unknown `type` name, emitted as raw JSON; (g) duplicate keys, the last one
winning. **Probe** (investigator, one probe schema each); none occurs in the committed schemas (scripts `latent.py`,
`kwpaths.py`, `constprops.py`). Fix: refuse each with its path, as 0012 aims.

**G6. Low. A non-zero scalar default with `omitempty` cannot send the zero value.** `model/fields.go:555-561`. **Probe**
(investigator): `{"n":0,"s":"","b":false}` with defaults `5`, `"d"`, `true` re-encodes without them, and the peer reads the
defaults. The committed schemas have one non-zero scalar default, where `""` is invalid anyway. Fix: refuse, or emit a pointer.

**G7. Low. Defaults on nested inline objects and inline-variant properties are not applied.** `model/fields.go:583-602` says so in a
comment, while `checkDefaults` accepts them. **Probe** (investigator): "inner.flag=false (schema default true)". None in the
committed schemas. Fix: refuse, or apply.

**G8. Low. `meta.json` without `version` emits `ProtocolVersionNumber = 0`.** `load/load.go:18-24`, `merge.go:84`. **Probe**
(`noversion.txtar`, re-run): "gen rc=0". Fix: require `version >= 1`.

**G9. Low. `Check` type-checks only the host's build configuration.** `output/check.go:189-191`. **Probe** (investigator): a
Windows-only file that collides passes on `darwin`, and `GOOS=windows go build` then fails. Fix: also check for `windows` and
`linux`.

**G10. Low. `TestGeneratedFilesUpToDate` misses a stale root `*_gen.go`.** `cmd/generate/main_test.go:38-70` checks `unstable/`
only. **Probe** (investigator): with `extra_gen.go` at the root, the test passes. Fix: the same loop for the root.

**G11. Low. Stray positional arguments are ignored.** `main.go:37-39`. **Probe** (investigator): `gen /some/other/out stray` exits 0
and writes to the working directory's repository. Fix: refuse `fs.NArg() > 0`.

**G12. Low. `FuzzBuild` starts almost only from refused seeds, and never type-checks.** `cmd/generate/fuzz_test.go:36` passes an
empty `Meta`, so since 0019's G18 nearly every seed is an unlisted method. **Probe** (investigator): of 187 seeds, "accepted 1,
refused as unlisted-method 116, refused otherwise 70". Fix: fuzz each case's schema with its own `meta.json`, and run `Check`.

**G13. Info. `Model.Validates` is used only by tests.** `model/validate.go:70`; `deadcode ./...` lists it. Fix: move it to a test
file, or keep it as a test helper by name.

### F-P: the subpackages

**P1. Medium. The WebSocket keepalive disconnects a live peer that is only applying back-pressure.** `remote/ws.go:141-157`,
`:178-202`, `:71-83`; `frame.go:129-141`. coder/websocket reads pongs only while a `Read` runs ("Ping must be called concurrently
with Reader"), and a reader blocked on a full pipe or queue does not read. A busy agent, or a slow client under `OverflowBlock`,
is disconnected with `ErrDisconnected`.

* **Probe** (`TestProbeWSSlowClientDisconnected`, `TestProbeWSBusyAgentDisconnected`, re-run): "the agent's connection was ended
  (cause remote: client disconnected); the client read 2000 of 2000 lines"; "a live, reading client was disconnected while the
  agent was busy". With a 10 s keepalive both pass.
* Fix: keep the socket reader running whatever the delivery does, or do not count a missed pong while it is blocked.

**P2. Medium. DNS rebinding passes the origin rule.** `remote/handler.go:177-199` compares `Origin`'s host with `Host`; after a
rebinding both are the attacker's name. **Probe** (`TestProbeDNSRebinding`, re-run): a handler on `127.0.0.1` answered a POST with
`Host` and `Origin` `attacker.example:PORT` with `200` and an `initialize` result; the real `Host` with a foreign `Origin`, `403`.
0043's amendment 7 chose the origin rule; this gap is not recorded there. Fix: Q5.

**P3. Medium. `clienthost.Files.ReadTextFile` allocates about ten times the file whatever `line` and `limit` say.**
`clienthost/files.go:54` (`os.ReadFile`), `:117` (`strings.SplitAfter`). **Probe** (`TestProbeReadOneLineOfBigFile`, re-run): "file 64
MiB; answer "x\n" (2 bytes); allocated 640 MiB". Fix: stream to `line`, stop at `limit`.

**P4. Low. The FIFO refusal can be raced, and the read then blocks forever.** `clienthost/files.go:48` stats, `:54` opens again; the
same in `WriteTextFile` (`:63`, `:74`). [0041-MADR-close-the-gaps-the-consumers-wait-on.md](0041-MADR-close-the-gaps-the-consumers-wait-on.md)
refuses non-regular files because "a FIFO would block it". **Probe** (`TestProbeFifoSwap`, re-run): "3 of at most 50000
ReadTextFile calls on a path refused as a FIFO were still blocked after 500ms". Fix: open first, non-blocking on Unix, then
`Stat` the handle.

**P5. Low. On 32-bit targets, a `line` or `limit` of 2^31 or more panics the client.** `clienthost/files.go:115`, `:125-126`;
`acptest/client.go:174`, `:183-185` convert `uint32` to `int`. **Read** (no 32-bit runner here); `GOARCH=386 go vet` and
`GOARCH=arm go build` succeed, so the code ships there. Fix: compare in `uint64`, and clamp.

**P6. Low. Over WebSocket, one message over `MaxMessageSize` ends the connection.** `remote/handler.go:217`, `ws.go:39`
(`SetReadLimit`). Streamable HTTP refuses only that POST (`413`), and stdio answers `-32600` and reads on. **Probe**
(`TestProbeWSLimit`, re-run, a demonstration): a 101-byte frame under a 100-byte limit closes with "StatusMessageTooBig"; the next
frame fails "use of closed network connection". Fix: Q6.

**P7. Low. `Transcript.Close` races with a tap that `Wait` left running.** `agentproc/transcript.go:52-58` sets `lineTap.held`
without a lock (`linetap.go:54`) while the tap goroutine is still in `lineTap.Write` (`linetap.go:41`). **Probe**
(`TestProbeTranscriptCloseAfterAbandon`, re-run, `-race -count=3`): "WARNING: DATA RACE" in 2 of 3 runs, the read at `linetap.go:41`,
the previous write at `linetap.go:54` from `Transcript.Close`. Fix: guard `held` with the Transcript's mutex.

### F-T: tooling, CI and documentation

**T1. Medium. `tools/README.md` is docs-safe, but only `make check-tools` checks its links, and nothing checks `RELEASING.md`'s.**
`ci.yml:15`, `docs.yml:13`; `tools/test_tools.py:593-601`; `docs.yml:48-53` runs neither. `check_ci_filters.py`'s "readers" rule
sees only Go string literals. **Probe** (investigator): links broken in both files pass `check-records`, `disclosure-check` and
the new pre-add check's docs-safe skip; only `ReadmeTest` fails. Fix: check those files' links in `check_records.py`, or run
`check-tools` in Docs.

**T2. Medium. `publication.py`'s "CI on origin main's head" is passed by the Docs workflow.** `tools/publication.py:32-37`, `:82-86`
filter by branch, not workflow. **Read**, and seen in this session: `v1.31.1`'s check passed on the Docs run `41339370946`, which
runs no Go (0046 PLAN, phase 8). Fix: filter on the CI workflow; this interacts with
[0048-MADR-create-releases-in-ci.md](0048-MADR-create-releases-in-ci.md)'s D5.

**T3. Medium. `check_records.py` and `make markdownlint` scan directories git ignores.** `scripts/check_records.py:50-66` walks with
`rglob`, skipping neither `.cache` nor `.worktrees`, which `.gitignore` lists (read by the author). **Probe** (investigator): a copy
of the 0047 pair under `.worktrees/` gives `--next` `0100` and "number 0047 is claimed by 2 MADRs"; a bad `.md` under `.cache/`
fails `make markdownlint`. Fix: list with `git ls-files -co --exclude-standard`, and ignore both in markdownlint's globs.

**T4. Low. "Checksum database on" is not enforced.** `publication.py:103-105`, `consumer.py:492-494` keep an inherited `GOSUMDB`,
and a `GOENV` file still applies; `GONOSUMCHECK` is not a Go variable. **Probe** (investigator): with `GOSUMDB=off` in the parent
and a `GOENV` setting `GONOSUMDB`, both tools' environments keep them. Fix: set `GOSUMDB`, clear the others, `GOENV=off`.

**T5. Low. `idscan.py --range A..B` scans the endpoints, not the commits.** `tools/idscan.py:82-87` uses `git diff --name-only`
(read by the author); a file whose identifier was added and removed inside the range is missed, and `--range HEAD` crashes with
`IndexError`, exit 1, its "match" status. **Probe** (investigator). Fix: scan each commit's files, and refuse a range without `..`.

**T6. Low. `find.py lines` reports a mistyped path or a directory as "not found".** `tools/find.py:34-36`. **Probe**
(investigator): a misspelt file and a bare directory each exit 1 with no output. Fix: exit 2 naming the path.

**T7. Low. CI's comment "main runs each push" is false.** `ci.yml:36-39`, `docs.yml:33-35`: with the default concurrency queue, a
third push while one runs cancels the pending second. **Read**, GitHub's workflow syntax. Fix: correct the comment, or put the
SHA in the group for `main`.

**T8. Low. `RELEASING.md`'s prerequisites omit `mdsh`,** which `make version` needs (`Makefile:200-203`). **Probe** (investigator):
"mdsh not found", `Error 1`.

**T9. Low. macOS's `/usr/bin/make`, 3.81, computes an empty schema version silently.** `Makefile:12`, `:38`; `RELEASING.md:10`
documents 4.2. **Probe** (investigator): make 3.81 gives `V=[]` and would fetch `…/schema-v/meta.json`. Fix: a version guard.

**T10. Low. The docs name the wrong packages for `make api`.** `RELEASING.md:98-104`, `docs/architecture.md:723-732`,
`Makefile:106` against `scripts/check_api.py:49`, `:54` (gated: root, `agentproc`, `acptest`, `clienthost`; reported: `unstable`,
`remote`).

**T11. Low. `release_body.py` hides why the identifier scan failed.** `tools/release_body.py:77-81` prints idscan's stdout only.
**Probe** (investigator): with no deny list, exit 1 with no reason.

**T12. Low. `.golangci.yml` has dead exclusions:** `:116` (`//sys`, which no file has), `:126` (`godot`, not enabled), `:131` and
`:144` (`examples$`; the directory is `example`).

**T13. Low. Small doc gaps:** `docs/architecture.md`'s tree omits `scripts/push_guard.py`, `testdata/elicitation/` and
`testdata/encodings.golden`; `make help` omits `check-records`, `check-tools`, `markdownlint`, `disclosure-check`, `version`,
`release` and `clean`; `tools/platforms.py` has no test.

### F-C: coverage and dead code

* `agentproc/agentproc.go:286`, `abandon`, Start's clean-up when it cannot connect an agent: never run by a test.
* `agentproc/stderr.go:21` and `elicitation.go:286`: two `Error()` methods never run.
* `ConfigOptionUpdate.gates` (`types_gen.go:23956`): generated for a schema definition no message uses; the
  `SessionConfigOptionUpdate` variant is the one sent. Dead.
* `internal/proctree/deathsig_none.go:10`: built only on platforms the tests do not run.
* `cmd/generate`: G13, and the two `main` functions.

## Decision Drivers

* A correct program must not hang, crash or lose data because of the SDK (R1, P1, P5).
* What the SDK sends must be what the schema allows, and what it reads, what the schema says (S1, S2, S6).
* A documented contract holds, or its documentation changes (R2, R4, R10, S4, T10).
* The generator refuses what it cannot render faithfully, rather than exiting 0 (G1 to G9).
* The checks the records rely on check what they claim (T1 to T5).
* No incompatible change to the exported API without a `/v2` ([0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md](0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md)).

## Considered Options

* **A. One program, five PLANs by area,** each phase ordered by severity, each fix with its probe turned into a committed test:
  runtime (R), surface (S), subpackages (P), generator (G), tooling (T).
* **B. One PLAN, ordered by severity across areas.**
* **C. Fix only high and medium findings now,** and record the low ones as accepted.
* **D. A separate MADR per finding.**

## Decision Outcome

Chosen option: **"A"**, accepted 2026-11-02, because the areas touch different files and different checks, so each PLAN can land and be
released on its own, while the severity order inside each keeps R1 first. The plans would be:

* `0049-PLAN-runtime-fixes.md`: R1, then R5, R9, R4, R3, R7, R8, R6, R10, and R2 per Q2. A patch release when it changes no
  exported API.
* `0049-PLAN-surface-fixes.md`: S1, S2, S6 per Q4, S7, S8, S9, S5, S4, and S3 per Q3. The new gates and stricter validation are
  behaviour changes, released as a minor version with notes that lead with them.
* `0049-PLAN-subpackage-fixes.md`: P1, P3, P4, P7, P5, P2 per Q5, P6 per Q6, and `TestInProcessServeAgentStep2`'s margin.
* `0049-PLAN-generator-fixes.md`: G2, G1, G3, then G4 to G12, each refusal with an engine case; G13.
* `0049-PLAN-tooling-fixes.md`: T2 (with 0048), T1, T3, then T4 to T13, and F-C's dead `ConfigOptionUpdate.gates` through the
  generator.

Each fix's test is its probe, kept, seen to fail before the fix and to pass after; the probes named above are the starting point.

### Questions for the owner

* **Q1. The program's shape:** (a) five PLANs by area (recommended); (b) one PLAN by severity; (c) high and medium only now.
* **Q2. R2, a receiving middleware and `initialize`:** (a) refuse a replaced or short-circuited `initialize` result, as the
  sending side does (recommended: one rule for both sides, and 0030's record stays the dispatch's); (b) record the result after
  the chain; (c) document the gap only.
* **Q3. S3, the unstable capabilities:** (a) gate them as the stable ones are (recommended: the schema says omitted means not
  advertised); (b) record a decision that unstable capabilities stay ungated; (c) gate only `plan_update` and `plan_removed`,
  which reach every client.
* **Q4. S6, closed enums:** (a) refuse on send: `Validate` checks membership, and `checkResult` refuses an empty `stopReason`;
  on receive, keep unknown values readable but treat lenient fields as the schema says (recommended); (b) refuse both ways; (c)
  refuse only an empty `stopReason`.
* **Q5. P2, DNS rebinding:** (a) a `HandlerOptions` Host allowlist, defaulting to the loopback names when the listener is
  loopback (recommended); (b) document that the host must check `Host`; (c) refuse any `Host` that is not an IP literal or
  `localhost` unless allowed.
* **Q6. P6, an over-long WebSocket message:** (a) discard it while reading on, and answer `-32600` as stdio does (recommended);
  (b) document the difference; (c) keep closing, and close on write too, for symmetry.
* **Q7. Order across PLANs:** (a) runtime first, then surface, subpackages, tooling, generator (recommended: R1 and P1 reach
  consumers today); (b) by whichever area holds the most medium findings.

#### Answered by the owner, 2026-11-01

The owner committed this record as `5c3344c` and said "committed, proceed". Each question was then put with three options; the
owner chose the recommended one each time:

* Q1 (a): five PLANs by area. Not chosen: one PLAN by severity; high and medium only.
* Q2 (a): refuse a replaced or short-circuited `initialize` result on the receiving side. Not chosen: record the result after
  the chain; document only.
* Q3 (a): gate the unstable capabilities as the stable ones are. Not chosen: record them as ungated; gate plan updates only.
* Q4 (a): strict on send, readable on receive, lenient fields as the schema says. Not chosen: strict both ways; only an empty
  `stopReason`.
* Q5 (a): a `HandlerOptions` Host allowlist, defaulting to the loopback names on a loopback listener. Not chosen: document only;
  refuse every Host that is not an IP literal or `localhost`.
* Q6 (a): discard an over-long WebSocket message and answer `-32600`. Not chosen: document the difference; close on write too.
* Q7 (a): runtime, then surface, subpackages, tooling, generator. Not chosen: tooling first; most medium findings first.

Each PLAN is written when its turn comes, against the tree as it then stands, so its line references hold; the first is
[0049-PLAN-runtime-fixes.md](0049-PLAN-runtime-fixes.md). The decision stays `proposed` until the owner accepts it with that PLAN.

#### R1's and R2's designs, decided 2026-11-01

* **R1, the bound.** The reader queues the answers it gives itself, to faults and refusals, for a separate goroutine to write,
  and never waits on the peer. Asked what bounds that queue, with three options, the owner chose the recommended one: past 1 024
  unwritten answers the connection closes, its error naming a peer that does not read. Not chosen: an unbounded queue; dropping
  past the cap with a log, which leaves a request unanswered.
  * A consequence for [0046-MADR-sending-and-receiving-middleware.md](0046-MADR-sending-and-receiving-middleware.md)'s D9 and its
    amendment of 2026-11-01: the observer for `ObservedFault` and `ObservedRefused` runs on that goroutine, after the answer is
    written, not on the reader. The order the amendment states holds; where the observer runs changes, and with it the reader no
    longer stalls while it runs for those kinds. The runtime PLAN amends 0046 to say so.
* **R2, the guard (Q2 a).** On the agent's side only, where the dispatch records `initialize`, the connection notes the result the
  dispatch returned; when the receiving chain returns another result, or returns one without the dispatch having run, the
  request is answered `-32603` and the reason logged at `Error`. A middleware that fails `initialize` with an error is allowed, as
  on the sending side. No exported identifier is added.

*2026-11-02:* the owner committed these answers and the runtime PLAN as `9f59ee2` and said "proceed". The decision is
`accepted`, and [0049-PLAN-runtime-fixes.md](0049-PLAN-runtime-fixes.md) is in progress. 0046's amendment of 2026-11-02 records
where faults and refusals are now observed.

### Consequences

* Good, because a deadlock, a crash, and four schema mismatches a peer can see are fixed, each with a test that would have caught
  it.
* Good, because the generator refuses the schemas it now crashes on, renders wrongly or drops parts of (G2 to G8), before a
  schema update meets one.
* Good, because the checks the records quote (`publication.py`, `check_records.py`, the docs-safe set) check what they claim.
* Bad, because the surface fixes refuse what the SDK sent before (an empty `stopReason`, an unadvertised `methodId`, an ungated
  boolean option): a consumer that relied on it fails at the call, which is why they go in a minor release with notes.
* Bad, because five PLANs are a long program; Q1 (c) is the shorter alternative.
* Neutral, because none of the fixes needs an incompatible API change; Q5 (a) adds one option.

### Confirmation

Each PLAN confirms its findings: the probe, committed as a test, fails at `d55ba2a`'s code and passes after the fix; the gate set
and CI pass; and each release's notes name what changed for a consumer.

## Pros and Cons of the Options

### A. Five PLANs by area

* Good, because each area lands and releases on its own, with its own checks.
* Bad, because the severity order holds only within an area.

### B. One PLAN by severity

* Good, because R1 and every medium finding land first, whatever the area.
* Bad, because one PLAN touching every area is long, and its phases mix unrelated files.

### C. High and medium only

* Good, because it is about a third of the work.
* Bad, because the low findings include a crash on 32-bit (P5) and a data race (P7).

### D. A MADR per finding

* Good, because each decision stands alone.
* Bad, because fifty records for one pass bury the program.

## More Information

* Prior passes and the records whose fixes these findings touch: [0016-REPORT-acp-v1-conformance-audit.md](../reports/0016-REPORT-acp-v1-conformance-audit.md),
  [0019-MADR-remediate-debugging-pass-findings.md](0019-MADR-remediate-debugging-pass-findings.md),
  [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md) (S6),
  [0041-MADR-close-the-gaps-the-consumers-wait-on.md](0041-MADR-close-the-gaps-the-consumers-wait-on.md) (S2, P3, P4),
  [0043-MADR-remote-http-and-websocket-transport.md](0043-MADR-remote-http-and-websocket-transport.md) (P1, P2, P6),
  [0045-MADR-notification-queue-byte-budget.md](0045-MADR-notification-queue-byte-budget.md) (R5),
  [0046-MADR-sending-and-receiving-middleware.md](0046-MADR-sending-and-receiving-middleware.md) (R2, R8),
  [0047-MADR-pre-add-check-full-coverage.md](0047-MADR-pre-add-check-full-coverage.md) (T1),
  [0048-MADR-create-releases-in-ci.md](0048-MADR-create-releases-in-ci.md) (T2).
* Schema 1.26.0 stays deferred by the owner, and no finding concerns it.
