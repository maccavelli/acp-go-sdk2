---
status: accepted
date: 2026-11-03
decision-makers: the repository owner
consulted: "probe tests and scripts in scratch copies of `d55ba2a`, and of `e98987a` for the re-assessment; the official TypeScript (`08a0d30`) and Python (`9853d8b`) SDKs' clones in the interop cache; `schema/schema.json` and `schema/schema.unstable.json` at ACP 1.25.0; `go help test`; GitHub's workflow documentation"
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
owner's answers to "Questions for the owner" settle the program. *2026-11-03:* the owner answered on 2026-11-01 and accepted
the record on 2026-11-02; the runtime findings are done, and "Re-assessment at `e98987a`" below checks the rest against the tree.

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

### Re-assessment at `e98987a`, 2026-11-03

The owner asked, before the surface PLAN, for the record to be checked against the tree as it now stands. The method was the
pass's: each finding's probe re-run in a scratch copy of `e98987a` (`git archive`), never in the tree, and each citation read
at `e98987a`. Three investigators each took one part: the generator; the subpackages and tooling; and the official SDKs. The
author took the surface. `git status --porcelain` was empty afterwards.

**The environment:**

* `e98987a` is `v1.31.2`, released 2026-11-03, plus the record of its release.
* Go `1.27.3` on macOS `arm64`; the floor is `go 1.26.10` in both `go.mod` files.
* The schema is ACP `1.25.0`. The 1.26.0 schema stays deferred by the owner.
* The official SDKs, compared from the clones `tools/interop.py` keeps in its cache, at the revisions it pins:
  * TypeScript `08a0d30` (package 2.1.0) vendors schema `1.27.0`;
  * Python `9853d8b` vendors schema `1.23.0`.
  * The S definitions agree across 1.23, 1.25 and 1.27, except `NewSessionRequest.mcpServers`'s annotations, which no
    finding concerns.
* The host's load average was 5 to 93 during the runs. The flaky test's figures were taken under a load the probe adds on
  purpose; no other conclusion rests on a timing test.

**What changed since `d55ba2a`:**

* The runtime PLAN changed `connection.go`, `middleware.go`, `observer.go`, `serve.go`, `agent_gen.go` and
  `cmd/generate/internal/emit/dispatch.go`, with their tests.
* No S, G, P, T or F-C finding cites those lines, apart from F-R's.
* `types_gen.go`, `helpers.go`, the timelines, `cmd/generate/internal/model/` and every subpackage are byte-identical to
  `d55ba2a` (`git diff --stat d55ba2a e98987a`).

**F-R:** done in `v1.31.2` (above).

**F-S: all nine reproduce at `e98987a`.** The decisive lines, from `go test -run TestProbe` in the copy:

* S1: "agent handler got ValueId=nil, Boolean=false, Kind()="select"".
* S2: "WithUpdateContent([]) / WithUpdateLocations([]) sent {…"tool_call_update","toolCallId":"t1"}}".
* S3: "plan_removed allowed without clientCapabilities.plan; providers/list allowed without its agent capability; …".
* S4: "timeline state changed through a returned copy" for both timelines.
* S5: "boolean set_config_option sent and handled without session.configOptions.boolean".
* S6: "PromptResponse{} (stopReason "") validates"; "agent answered and client accepted stopReason """; "audience [user system]
  kept 2 items".
* S7: "McpServer {"type":"stdio",...}: Stdio=nil, Kind()=stdio; AuthMethod {"type":"agent",...}: Agent=nil".
* S8: "AllowsValue nil, sent (err=<nil>), agent handler ran".
* S9: "prompt with an undeclared "video" block allowed".
* `TestProbeHolds` passes.

Every S citation holds. Facts the surface PLAN rests on:

* S6's closed sets are, by the generator's own classes (`KindEnum`, `KindStringConstUnion`, not `KindOpenEnum`):
  * stable: `StopReason`, `PermissionOptionKind`, `PlanEntryPriority`, `PlanEntryStatus`, `ToolKind`, `ToolCallStatus`, `Role`,
    `StringFormat`, `ElicitationSchemaType`;
  * unstable: `NesDiagnosticSeverity`, `NesRejectReason`, `NesTriggerKind`, `PositionEncodingKind`, `TextDocumentSyncKind`.

  The finding names seven of the fourteen.
* S2's "Replace the … collection" is the wording of `ToolCallUpdate.content` and `.locations` alone. `SessionMessage.content`
  and `CompactionUpdate.summary` already carry patch rules.
* S1 and S7 are one mechanism. The unions with a key-less variant are `AuthMethod` (`agent`), `McpServer` (`stdio`) and
  `SetSessionConfigOptionRequest` (`value_id`), and two with a catch-all as well, `MultiSelectItems` (`titled`) and the
  unstable `IdleStateUpdate` (`none`). The generated decoder sends any present discriminator it does not name to `Unknown`,
  before the key-less variant is tried (`cmd/generate/internal/emit/types.go:962-968`).
* The generated `validate` already takes a parameter that tells sending from receiving, the rule for an absolute path
  (`connection.go:215-241`, `:3115`). The types a timeline entity reaches have no unexported field, and their only interface
  values are `_meta`'s.

**The official SDKs on S1 to S9.** "Probe" means the investigator ran it against the clone; "read" means the code only.

| | TypeScript `08a0d30` | Python `9853d8b` |
| :--- | :--- | :--- |
| S1 | Probe: unknown `type` with a string `value` decodes as the value-id variant. `type:"boolean"` with a string is also accepted there (zod's first match). | Probe: the same, by `type == "boolean"`. Any other `type` is the select request; a non-string `value` is `-32602`. |
| S2 | Probe: `[]` is sent and received as `[]`; nothing is normalised. | Probe: `[]` is sent; an explicit `null` cannot be. |
| S3, S5, S8 | Read: no capability or `authMethods` check, either way; the README leaves it to the application. | Read: none; unstable routes sit behind a local flag only. |
| S4 | Read: no timeline helpers in v1. | Read: `contrib/` deep-copies with `model_copy(deep=True)`. |
| S6 | Probe: a required closed value that is unknown is refused on receive (`-32602`); a lenient one is dropped to absent; no check on send. | Probe: the same on receive. On send, a required one is refused; a lenient one is dropped silently. |
| S7 | Probe: `"type":"stdio"` decodes as stdio, and an unknown `type` with stdio's keys does too. | Probe: the same. It also decodes a key-less `AuthMethod` as terminal, against the schema. |
| S9 | Read: no capability check; an unknown block type is refused on receive. | Probe: the same. |

* So on S1 and S7 both SDKs read the key-less variant as the schema's `anyOf` allows. On S6's lenient fields both drop what
  0012's treatment 5 also drops.
* On required closed values both refuse what Q4 (a) chose to keep readable on receive. That is recorded as a difference; the
  owner's answer stands.
* Neither SDK gates capabilities. This SDK has gated the stable ones since
  [0017-MADR-remediate-acp-v1-conformance-violations.md](0017-MADR-remediate-acp-v1-conformance-violations.md) (G1), and Q3
  (a) extends that to the unstable ones.

**F-G: all thirteen reproduce at `e98987a`,** each probe re-run, every citation held, with these corrections to the text below:

* **G3:** the probe's stable `Shared.b` is `{"type":"integer","format":"int32","description":"Stable b."}`. The emitted field
  keeps neither the stable description nor an unstable notice.
* **G4,** seen in passing: an `int64` `minimum` of 2^63 is not refused by the model. It fails later, at `Check`, with "…
  overflows int64", naming the generated file, not the schema path. The cause is the same `float64` path, and the same fix
  covers it.
* **G5 (d):** only some `$ref` siblings are dropped. One carrying `type` or a bound is refused ("minimum or maximum on B, not a
  number"), and one carrying other properties is dropped silently.
* **G6:** in the probe `n` is `["integer","null"]`, a pointer, so `0` is sent. Only `s` and `b` are dropped, and the peer reads
  their defaults.
* **G7:** the function is `model/fields.go:581-603`, its comment at `:583`; `checkDefaults` is at `model/fields.go:373`.
* **G9:** "the host's build configuration" is the generator's own `GOOS` and `GOARCH`: `build.Default` follows the
  environment. With `GOOS=windows` set, the probe's collision is refused.
* **G10:** `cmd/generate/main_test.go:38-68`, not `:38-70`.
* **The paths:** G2 to G9 name files relative to `cmd/generate/internal/`; G1 and G10 to G12 relative to the repository.

**F-P, F-T and F-C:** every finding holds at `e98987a`.

* `agentproc/`, `clienthost/`, `remote/`, `acptest/`, `internal/`, `tools/`, `scripts/`, the `Makefile`, `.github/`,
  `.golangci.yml`, `RELEASING.md` and `tools/README.md` are unchanged since `d55ba2a`.
* Each re-runnable probe reproduced, at a load average of 5 to 16.
* Corrections and additions to the text below:
  * **The flaky test:**
    * Re-run with 32 busy goroutines, `-test.cpu=2`, `go1.26.10` and `-race`. The probe killed 6 of 100 at 300 ms and 0 at
      1.2 s; the real test, in two runs of 50 per binary, failed 8 of 100 at 300 ms and 0 of 100 at 1.2 s. Unloaded:
      `p50` 0.83 ms, `max` 18.2 ms. The figures move with the host's load; the margin does not.
    * The chain has one more link since `v1.31.2`: `ServeAgent`'s wait for the notification handlers after `Shutdown`
      (`serve.go:57-61`, R4).
  * **P1:** the function at `remote/ws.go:178-202` starts at `:180`. With a 10 s keepalive both probes still pass.
  * **P4:** the stat itself is in `regular`, `clienthost/files.go:86`.
  * **P5:** on macOS the commands need `GOOS=linux`: `GOOS=linux GOARCH=386 go vet` and `GOOS=linux GOARCH=arm go build` exit 0,
    as does `GOOS=windows GOARCH=386 go build`. `GOARCH=386` alone fails "unsupported GOOS/GOARCH pair darwin/386".
  * **P6:** the 101-byte frame closes with status 1009, "StatusMessageTooBig".
  * **P7:**
    * The race is also write against write: `linetap.go:46` (`l.held = nil`) against `:54`. So the read is at `:41` or `:46`.
    * The probe's own precondition log, "the tap was not abandoned", is a false negative: `tap.abandon` counts
      `queued - inflight` (`tap.go:76`), and the probe's one write in flight is not counted. The committed test needs another
      check that the tap was abandoned.
  * **T2: it recurred at `v1.31.2`.** `69eff66` changed records only, so the push ran the Docs workflow alone on `main`.
    `tools/publication.py`'s "CI on origin main's head" passed on run `41728264484`, which `tools/watch_ci.py` had reported as
    the Docs workflow's ([0049-PLAN-runtime-fixes.md](0049-PLAN-runtime-fixes.md), phase 4).
    * "CI on the tag" (`publication.py:87-90`) has no workflow filter either. Once
      [0048-MADR-create-releases-in-ci.md](0048-MADR-create-releases-in-ci.md)'s tag workflow exists, its run could pass it.
    * `main_run` has no test.
  * **T3:** `0100` from `--next` needs a record numbered `0099` under `.worktrees/`. A copy of the 0047 pair alone gives "number
    0047 is claimed by 2 MADRs" and leaves `--next` at `0050`. A bad `.cache/bad.md` fails `make markdownlint`, "4 issues in 1
    file".
  * **T7:** `docs.yml` has the same concurrency block, without the comment.
  * **T8:** the check is at `Makefile:202-204`, not `:200-203`.
  * **T9:** `Makefile:37`, not `:38`. Under make 3.81, `.SHELLFLAGS` is a plain variable, so recipes also lose `-eu -o pipefail`.
  * **T10:** `docs/architecture.md:751-760`, not `:723-732`.
  * **T11,** the same class, new: `scripts/check_disclosure.py:88-90` exits 2 silently when `git ls-files` fails, as outside a
    work tree.
  * **T13:** the tree also omits `scripts/test_check_*.py`, which may be intended.
  * **F-C:**
    * `ConfigOptionUpdate.gates` is dead, but its definition is used. `SessionUpdate`'s `config_option_update` variant includes
      it by `allOf`, and the generator flattens that variant into `SessionConfigOptionUpdate` (`types_gen.go:15413`). So the
      call at `:24199-24203` reaches `(*SessionConfigOptionUpdate).gates` (`:24207`), never this one.
    * `internal/proctree/deathsig_none.go:10` does run. Its build tag, `unix && !linux && !freebsd`, includes macOS,
      `tree_unix.go:45` calls it, and the profile counts it run (`deathsig_none.go:10.48,10.48 0 1`). It shows 0% because it
      has no statement. It is not a gap.

**Not findings, noted:**

* `mcp/message`, in both directions, and the agent's `positionEncoding` are not gated. No sentence of the schema states a gate
  for `mcp/message`. For `positionEncoding`, the schema says the agent selects "from the client's supported encodings",
  which no check enforces. Neither was probed as a defect, and no PLAN takes them up unless the owner asks.
* `SessionInfo.additionalDirectories` says "When present, this is the complete ordered additional-root list". So an empty list
  and an absent one may differ in meaning, as S2's do. It is reported, not probed.

## Findings

Severity is the effect on a consumer or a maintainer: **high** loses data, hangs, or crashes a correct program; **medium** gives a
wrong result or breaks a documented contract on a reachable path; **low** is an edge, a gap, or a wrong word.

### F-R: the connection runtime

*Done 2026-11-03:* R1 to R10 are fixed in `v1.31.2` by [0049-PLAN-runtime-fixes.md](0049-PLAN-runtime-fixes.md), each with
the test its phase 4 table names; R2 as amended under "R1's and R2's designs".

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

*Re-run 2026-11-03 at `e98987a`:* all nine reproduce ("Re-assessment" above).
[0049-PLAN-surface-fixes.md](0049-PLAN-surface-fixes.md) takes them up.

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
(investigator): `{"n":0,"s":"","b":false}` with defaults `5`, `"d"`, `true` re-encodes without `s` and `b` (`n`, nullable,
is a pointer and is sent; corrected 2026-11-03), and the peer reads the defaults. The committed schemas have one non-zero scalar default, where `""` is invalid anyway. Fix: refuse, or emit a pointer.

**G7. Low. Defaults on nested inline objects and inline-variant properties are not applied.** `model/fields.go:581-603`
(corrected 2026-11-03) says so in a comment at `:583`, while `checkDefaults` accepts them. **Probe** (investigator):
"inner.flag=false (schema default true)". None in the committed schemas. Fix: refuse, or apply.

**G8. Low. `meta.json` without `version` emits `ProtocolVersionNumber = 0`.** `load/load.go:18-24`, `merge.go:84`. **Probe**
(`noversion.txtar`, re-run): "gen rc=0". Fix: require `version >= 1`.

**G9. Low. `Check` type-checks only the generator's own `GOOS` and `GOARCH`, the host's unless the environment sets them.**
`output/check.go:189-191`. **Probe** (investigator): a Windows-only file that collides passes on `darwin`, and `GOOS=windows go build` then fails. Fix: also check for `windows` and
`linux`.

**G10. Low. `TestGeneratedFilesUpToDate` misses a stale root `*_gen.go`.** `cmd/generate/main_test.go:38-68` (corrected
2026-11-03) checks `unstable/` only. **Probe** (investigator): with `extra_gen.go` at the root, the test passes. Fix: the same loop for the root.

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

**T8. Low. `RELEASING.md`'s prerequisites omit `mdsh`,** which `make version` needs (`Makefile:202-204`, corrected
2026-11-03). **Probe** (investigator): "mdsh not found", `Error 1`.

**T9. Low. macOS's `/usr/bin/make`, 3.81, computes an empty schema version silently.** `Makefile:12`, `:37` (corrected
2026-11-03); `RELEASING.md:10` documents 4.2. **Probe** (investigator): make 3.81 gives `V=[]` and would fetch `…/schema-v/meta.json`. Fix: a version guard.

**T10. Low. The docs name the wrong packages for `make api`.** `RELEASING.md:98-104`, `docs/architecture.md:751-760`
(corrected 2026-11-03), `Makefile:106` against `scripts/check_api.py:49`, `:54` (gated: root, `agentproc`, `acptest`, `clienthost`; reported: `unstable`,
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
* `ConfigOptionUpdate.gates` (`types_gen.go:23956`): dead. ~~generated for a schema definition no message uses~~ The definition
  is used, by `allOf` in `SessionUpdate`'s `config_option_update`, which the generator flattens into
  `SessionConfigOptionUpdate`, whose own `gates` is the one called (corrected 2026-11-03).
* ~~`internal/proctree/deathsig_none.go:10`: built only on platforms the tests do not run.~~ *Corrected 2026-11-03:* it is
  built on macOS, called by `tree_unix.go:45`, and counted run; it shows 0% because it has no statement. Not a gap.
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
  exported API. *Complete 2026-11-03,* released in `v1.31.2`.
* `0049-PLAN-surface-fixes.md`: S1, S2, S6 per Q4, S7, S8, S9, S5, S4, and S3 per Q3. The new gates and stricter validation are
  behaviour changes, released as a minor version with notes that lead with them. *Written 2026-11-03,*
  [0049-PLAN-surface-fixes.md](0049-PLAN-surface-fixes.md), proposed. It groups the order into phases by mechanism: S1 with S7,
  S2, S6, then the gates S8, S9, S5 and S3, then S4.
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
(It was accepted on 2026-11-02, below.)

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
  * *Amended 2026-11-03,* in [0049-PLAN-runtime-fixes.md](0049-PLAN-runtime-fixes.md)'s phase 3, deviation 1: "another
    result" is judged by the result's encoding, not by interface equality. The dispatch returns a `*InitializeResponse`, and a
    middleware that changes it in place keeps the pointer while the client receives what the agent's record does not hold.
    Asked with three options, the owner chose encoded bytes, the recommended one. Not chosen: interface equality, which misses
    the in-place change; identity first and the bytes for the same pointer, the same coverage with more code.

*2026-11-02:* the owner committed these answers and the runtime PLAN as `9f59ee2` and said "proceed". The decision is
`accepted`, and [0049-PLAN-runtime-fixes.md](0049-PLAN-runtime-fixes.md) is in progress. 0046's amendment of 2026-11-02 records
where faults and refusals are now observed.

#### The surface PLAN's designs, proposed 2026-11-03

[0049-PLAN-surface-fixes.md](0049-PLAN-surface-fixes.md) needs four answers the questions above did not settle. Each is put with
three options, the recommended one first. The PLAN is written on the recommended answers and changes if the owner chooses
otherwise.

* **Q8. S1 and S7, the key-less variant.**
  * **(a) One rule for every union with a key-less variant** (recommended).
    * A discriminator equal to the variant's own name, its schema `title`, decodes into it. The `Kind()` helpers already
      return that name for it.
    * In a union without a catch-all, a discriminator that names no variant decodes into the key-less variant when the payload
      has its required keys and decodes as it, and into `Unknown` otherwise.
    * A union with a catch-all keeps sending unknown values there.
    * This is what JSON Schema's `anyOf` allows, as the key-less variants declare no discriminator and no
      `additionalProperties`. The schema says it outright for `SetSessionConfigOptionRequest`, and both official SDKs decode
      so.
    * Its cost: a future `McpServer` type that carries stdio's keys, or a future `AuthMethod` type that carries `id` and `name`,
      decodes as `Stdio` or `Agent`, not `Unknown`.
  * **(b) The own name for every union, the fallback only where the schema states it.** A protocol-table rule quotes
    `SetSessionConfigOptionRequest`'s sentence, and `Build` checks it. `McpServer` and `AuthMethod` keep `Unknown` for future
    types, unlike the other SDKs.
  * **(c) Special cases.** The fallback in `SetSessionConfigOptionRequest`'s decoder alone, and the own name for `McpServer`
    and `AuthMethod` alone. This is the smallest diff, with no rule to keep the generator honest when a schema adds a key-less
    union.
* **Q9. S4, how the timelines copy.**
  * **(a) An unexported deep copy by `reflect`,** used by both timelines (recommended).
    * It covers any field the schema adds, without generated code.
    * Every field the timelines reach is exported, and the only interface values are `_meta`'s, which it copies by their
      dynamic kind.
    * Its cost is reflection on each `Get` and `Apply`, which these helpers do not call in a loop.
  * **(b) Generated `clone` methods** for every type a timeline reaches. No reflection, and the fastest. But it is a new emit
    pass and its tests, for a low finding, and `_meta`'s `any` values still need a runtime copy.
  * **(c) A JSON round trip.** The least code, but it changes `_meta` values' dynamic types (an `int` comes back a `float64`),
    and costs an encode and a decode per call.
* **Q10. S8, the error for an unadvertised `methodId`.**
  * **(a) A new sentinel, `ErrUnadvertisedAuthMethod`,** wrapped with the method and the id, as `ErrTerminalAuthMethod` is
    (recommended). One exported identifier, a compatible addition; `errors.Is` reads it.
  * **(b) A `*CapabilityError`** naming `authMethods`. Nothing exported is added, but `authMethods` is not a capability, and
    the error would say "capability authMethods not advertised".
  * **(c) `ErrTerminalAuthMethod`,** its doc widened. Nothing is added, but a terminal method and an unknown one are different
    refusals, and code that tells them apart today could no longer.
* **Q11. S2, how an empty collection is sent.**
  * **(a) A protocol-table rule, `EmptyClears`,** for the two properties whose description says "Replace the … collection"
    (recommended).
    * Their fields are tagged `omitzero`, so nil is omitted and empty is sent as `[]`, and `Build` checks the quoted sentence.
    * The option helpers send `[]` for nil.
    * No Go type changes.
  * **(b) `omitzero` on every optional array** whose schema type admits `null`. This is wider. It sends `[]` where a caller
    set an empty slice for fields whose empty and absent mean the same, and changes the wire for the gated
    `additionalDirectories`.
  * **(c) `Patch[[]T]` for the two fields,** as the patch rules give. It is exact, but it changes two exported fields' types:
    an incompatible change, which needs a `/v2`.

##### Answered by the owner, 2026-11-03

Each question was put with its three options; the owner chose the recommended one each time:

* Q8 (a): one rule for every union with a key-less variant. Not chosen: the own name with a table rule for the fallback;
  special cases.
  * *Amended 2026-11-03,* in [0049-PLAN-surface-fixes.md](0049-PLAN-surface-fixes.md)'s phase 1, deviation 1. In a union
    with a catch-all, a key-less variant's title is not a wire value. `IdleStateUpdate`'s `none` means "`stopReason` is
    omitted or `null`", and that union gives unknown values to its catch-all. So both rules apply only to a union without a
    catch-all.
    * The own name decodes into the key-less variant, strictly: a payload that does not decode is an error.
    * Any other unknown discriminator falls back to the key-less variant when the payload fits.

    Asked with three options, the owner chose this one. Not chosen: the fallback alone; Q8 (a) as first written.
  * *Amended again 2026-11-03,* deviation 2.
    * The general fallback reverses
      [0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md)'s F1 for `AuthMethod` and
      `McpServer`, which Q8 did not name: "a discriminator value that names no variant is never matched by key or by trial".
      Three tests pin F1.
    * So the fallback applies only where the schema states it, by a sourced protocol-table rule:
      `SetSessionConfigOptionRequest`, which fixes S1.
    * The strict own name fixes S7.

    F1 holds everywhere else. Asked with three options, the owner chose this one. Not chosen: superseding F1; a hand special
    case for S1.
* Q9 (a): an unexported deep copy by `reflect`. Not chosen: generated `clone` methods; a JSON round trip.
* Q10 (a): a new sentinel, `ErrUnadvertisedAuthMethod`. Not chosen: a `*CapabilityError`; a widened `ErrTerminalAuthMethod`.
* Q11 (a): a protocol-table rule, `EmptyClears`. Not chosen: `omitzero` on every nullable array; `Patch[[]T]`.

[0049-PLAN-surface-fixes.md](0049-PLAN-surface-fixes.md) is written on these answers. It stays `proposed` until the owner
approves it.

### Consequences

* Good, because a deadlock, a crash, and four schema mismatches a peer can see are fixed, each with a test that would have caught
  it.
* Good, because the generator refuses the schemas it now crashes on, renders wrongly or drops parts of (G2 to G8), before a
  schema update meets one.
* Good, because the checks the records quote (`publication.py`, `check_records.py`, the docs-safe set) check what they claim.
* Bad, because the surface fixes refuse what the SDK sent before (an empty `stopReason`, an unadvertised `methodId`, an ungated
  boolean option): a consumer that relied on it fails at the call, which is why they go in a minor release with notes.
* Bad, because five PLANs are a long program; Q1 (c) is the shorter alternative.
* Neutral, because none of the fixes needs an incompatible API change; Q5 (a) adds one option, and Q10 (a) one sentinel.
  *Amended 2026-11-03,* in [0049-PLAN-surface-fixes.md](0049-PLAN-surface-fixes.md)'s phase 3, deviation 3: S6's
  membership check gives 25 types that held a closed enumeration and checked nothing an exported `Validate`, compatible
  additions the owner accepted.

### Confirmation

Each PLAN confirms its findings: the probe, committed as a test, fails at its PLAN's base (`d55ba2a`'s code, or the
later base a PLAN names) and passes after the fix; the gate set and CI pass; and each release's notes name what changed for a consumer.

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
