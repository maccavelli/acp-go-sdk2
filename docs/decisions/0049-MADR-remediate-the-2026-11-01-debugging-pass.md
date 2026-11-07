---
status: accepted
date: 2026-11-07
decision-makers: the repository owner
consulted: "probe tests and scripts in scratch copies of `d55ba2a`, of `e98987a` for the re-assessment, of `277aca0` for the second, with a linux/386 run on the Linux test host, of `1fe3dd0` for the third, of `ff31967` for the fourth, the generator's, and of `73b0537` for the fifth, the generator's again; the official TypeScript (`08a0d30`) and Python (`9853d8b`) SDKs' clones in the interop cache; `schema/schema.json` and `schema/schema.unstable.json` at ACP 1.25.0; `go help test`; GitHub's workflow documentation"
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
*2026-11-04:* the surface findings are done too, in `v1.32.0`. "Re-assessment at `277aca0`" below checks the subpackage
findings and the documentation again, before the subpackage PLAN. *2026-11-07:* "Re-assessment of F-G at `73b0537`" checks
the generator findings again after G14's fix, before the generator PLAN.

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

### Re-assessment at `277aca0`, 2026-11-04

The owner asked, after `v1.32.0`, for the repository's documentation, decisions and plans to be checked against the code and
brought up to date, and then for the subpackage PLAN. The method was the first re-assessment's: each subpackage probe re-run in
a scratch copy of `277aca0` (`git ls-files -co --exclude-standard`, copied), never in the tree; three read-only audits, of the
records and the index, of `docs/architecture.md`, and of `README.md`, each claim checked against the code with `go doc`, the
file and a script; and the author's check of each finding the corrections below act on.

**The environment:**

* `277aca0` is `v1.32.0` (`9eeb57f`) plus the record of its release.
* Go `1.27.3` on macOS `arm64`; the 32-bit probe ran as a linux/386 binary on the Linux test host
  (`tools/remote.py pkg --host linux --goarch 386 --tree <copy>`), where `strconv.IntSize` is 32.
* The host's load average was 28 to 42 during the runs. The flaky test's figures were taken under a load the probe adds on
  purpose; no other conclusion rests on a timing test.

**What changed since `e98987a`:** in the subpackages, `acptest/script.go` and its test alone (`git diff --stat e98987a 277aca0
-- agentproc clienthost remote internal acptest`), `ScriptAgent`'s default stop reason, which no P finding cites. Every F-P
citation holds at `277aca0`, line for line.

**F-P: all seven reproduce at `277aca0`.** The decisive lines, from `go test -run TestProbe` in the copy:

* P1: "the agent's connection was ended (cause remote: client disconnected); the client read 2000 of 2000 lines"; "a live,
  reading client was disconnected while the agent was busy: cause remote: client disconnected (ErrDisconnected=true)". With
  `PROBE_KA_LONG=1`, a 10 s keepalive, both pass.
* P2: "POST with Host=attacker.example:56010 Origin=http://attacker.example:56010 -> 200 {…"protocolVersion":1}}"; the
  control, `Host` 127.0.0.1 with the same `Origin`, `403`.
* P3: "file 64 MiB; answer "x\n" (2 bytes); allocated 640 MiB".
* P4: "3 of at most 50000 ReadTextFile calls on a path refused as a FIFO were still blocked after 500ms".
* P5, **now a probe** (`TestProbe32BitLineLimit`, `TestProbe32BitMemoryClient`, linux/386): `clienthost` panics for a
  `line` of 2^31 + 1 ("slice bounds out of range [-2147483648:]"), a `line` of 2^32 - 1 ("[-2:]"), and a `limit` of 2^31
  or 2^32 - 1 ("[:2147483648] with capacity 4"); `acptest`'s `MemoryClient` panics for those and for a `line` of 2^31 too
  ("[2147483647:3]"). `GOOS=linux GOARCH=386 go vet` and `GOOS=linux GOARCH=arm go build` of both packages exit 0, so the
  code ships there.
* P6: a 100-byte frame under a 100-byte limit is delivered; a 101-byte one closes the socket, "status = StatusMessageTooBig
  … (close status 1009)"; the next write fails "use of closed network connection".
* P7: `-race -count=3`: "WARNING: DATA RACE" in 2 of 3 runs, at `linetap.go:41` and `:46` against `:54`, from
  `transcript.go:54`.
* `TestProbeOutputProperty` and `TestProbeDialContextNotBound` pass, as in the pass.

**The flaky test:** 100 runs each of the probe, `go1.27.3`, without `-race`:

* unloaded: killed 0, `p50` 0.48 ms, `max` 26.6 ms;
* 32 busy goroutines, `GOMAXPROCS=2`, grace 300 ms: killed 9, each "agentproc: agent killed: agentproc: agent terminated",
  step-2 latencies of the kills 314 to 391 ms;
* the same, grace 1.2 s: killed 0, `p99` and `max` 688.7 ms.

The margin is the test's, as found; the latency's tail under load is over half of 1.2 s.

**Two new findings,** P8 and P9 under F-P below, each by a probe in the copy.

**The documentation.** Each correction below was checked against the code by the author and is made in the commit that adds
this section, on the owner's ask:

* `README.md`:
  * "The unstable features whose capabilities the specification does not tie to a rule (`nes`, `plan`, `providers`,
    `positionEncoding`) are not checked": `v1.32.0` gates the first three (S3). Only `mcp/message` and `positionEncoding` are
    not.
  * "batches, answered `501`": only a POST is (`remote/httpserver.go:100-102`); over WebSocket a batch is a line, which the
    connection answers `-32600`, "not a JSON-RPC object" (`connection.go:1737`).
  * The lenient closed enumerations name `kind` alone: `status` and `ElicitationSchema.type`'s default `object` are read so
    too (`TestLenientClosedEnumsReadAbsent`).
  * A validation error quotes the value for an absolute path and an excluded value: a closed enumeration's is quoted too
    (`errors.go:137-138`).
  * `ServeAgent` at `ctx`'s end returns `context.Cause(ctx)` joined with `Shutdown`'s error (`go doc acp.ServeAgent`).
  * `Terminals` keeps output to `outputByteLimit` alone: also to `MaxOutputBytes`, 16 MiB by default
    (`clienthost/terminals.go:18-20`, `:61-64`).
  * `Handler.Close` "ends every connection at once": it starts each connection's ending ladder at once
    (`remote/handler.go:236-244`).
  * `ErrDisconnected` is also the cause when a WebSocket message is not written within `Grace` (`remote/ws.go:129`, `:166`,
    `:175`).
* `docs/architecture.md`:
  * `Roots` is "written by hand on five request types": four, and `SessionInfo` (`helpers.go:150-163`).
  * The fuzz targets omit `FuzzCanonicalIDKey` (`fuzz_test.go:115`, `Makefile:118`).
  * The server "routes a request … after `originAllowed` … for every method": a WebSocket upgrade goes to
    `websocket.Accept` and its own origin rule, and any other method but GET, POST and DELETE is answered `405`, before
    `originAllowed` (`remote/handler.go:141-168`).
  * `ReplayAgent` "is a serve function": its `Serve` method is (`acptest/replay.go:133`).
  * `readHook` is `Command`'s and `InProcess`'s (`agentproc/inprocess.go:63`), and `inProcessAgent.end` also ends
    `Serve`'s context (`:166`).
  * `sessionScoped` is `sessionScopedMethods`, and `connectionScoped` gives a `$/` method to the connection
    (`remote/route.go:97`, `:107`).
  * A stream must last 300 ms under a 100 ms `WriteTimeout`, not outlive it by 300 ms (`remote/httpserver_test.go:575-583`).
  * T10 and T13's tree, below.
* `RELEASING.md`: T8 and T10, below.

Left to the subpackage PLAN, whose phases change the behaviour the text describes: the README's keepalive and back-pressure
text (P1), the origin rule (P2), `Files`' reads (P3), the message limit (P6, P9), `Transcript.Close` (P7) and `Conn.Read`'s end
(P8).

**The records:**

* This record's Decision Outcome, its R2 note and its surface answers said the runtime and surface PLANs were in progress
  or proposed; they are complete, and each says so now.
* [0048-MADR-create-releases-in-ci.md](0048-MADR-create-releases-in-ci.md) named `v1.31.1` as the last hand release:
  `v1.31.2` and `v1.32.0` were too. Its PLAN's `--check` would count
  [0019-REPORT-release-notes-v0-18-0.md](../reports/0019-REPORT-release-notes-v0-18-0.md) as pending: that report is
  `v1.14.0`'s, its file name kept from before the renumbering, and `git tag -l 'v0.18*'` prints nothing. Both carry a dated
  amendment; 0048 stays `proposed`.
* `docs/README.md`'s 0048 and 0049 rows gain what the records say: the answers, the amendments, the halves done.
* `make check-records`: exit 0. 139 records and 139 rows with the subpackage PLAN's, every MADR's and PLAN's status
  matching its row.

### Re-assessment of F-T and F-C at `1fe3dd0`, 2026-11-04

The owner said "proceed" after `v1.33.0`, for the next PLAN of Q7 (a), the tooling's. The method was the earlier ones': each T
probe re-run in a scratch copy of `1fe3dd0` (`git archive`), never in the tree, every citation read at `1fe3dd0`, a coverage
profile of both modules for F-C, read-only `gh` queries for T2; one investigator, and the author's own check of each claim
the PLAN rests on. `git status --porcelain` was empty afterwards.

**What changed since `e98987a`:** `tools/`, `scripts/`, the `Makefile`, `.github/` and `.golangci.yml` are unchanged
(`git diff --stat e98987a 1fe3dd0`); `RELEASING.md` and `docs/architecture.md` changed in `f6bfdcf`'s corrections.

**F-T:** eleven reproduce, T8 is fixed, T10 and T13 are fixed in part. The decisive lines:

* T1: broken links planted in `tools/README.md` and `RELEASING.md` pass `check_records.py --check-all`, `check_disclosure` and
  `check_ci_filters --docs-safe`, exit 0 each; only `ReadmeTest` fails. `unnumbered_docs` (`scripts/check_records.py:221-229`)
  lists `README.md`, `docs/README.md`, `docs/architecture.md` and `docs/guides/` alone.
* T2: at each of the last four releases, main's run for the release commit was the Docs workflow's (`v1.31.1` `41339370946`,
  `v1.31.2` `41728264484`, `v1.32.0` `41840076826`, `v1.33.0` `41889919614`), and CI ran on that commit only as the tag's run.
  The author checked `v1.33.0`'s: `gh run list --commit 6e04da6… --branch main` gives the Docs run alone, and with
  `--workflow ci.yml` it gives `[]`. So **the fix as stated, a workflow filter, would fail every release whose commit is
  docs-only**, which is each of them. Corrections: `main_run` has a test, `test_main_run_of_a_released_commit`
  (`tools/test_tools.py:476-480`, `c39195a`), which tests its branch rule and not the workflow; and its `headBranch` filter
  does nothing, since `--branch main` already filters on the server (`tools/publication.py:32-37`, `:82-86`). "CI on the tag"
  (`:87-90`) has no workflow filter either.
* T3: live in the tree. `make markdownlint` lints 48 files, one more than a clean copy: `.cache/tools/release/
  release-body-v1.28.0.md`, which `release_body.py` writes by default and `.gitignore:18` ignores. `check_records.py` also
  walks the untracked `.tools/`. In a copy, a record under `.worktrees/` gives `--next` `0100` and "number 0047 is claimed by
  2 MADRs", and a bad `.cache/bad.md` fails markdownlint, "3 issues in 1 file".
* T4, wider than recorded: with `GOSUMDB=off` in the parent and a `GOENV` file, both tools' Go keeps `GOSUMDB=off`, and
  removing `GONOSUMDB` from the environment lets the file's value apply. A `GOPRIVATE` in the file also sets `GONOPROXY` for
  the module, so `go list -m @latest` and `go mod download` go direct and prove nothing about the proxy.
* T5: in a three-commit repository, an identifier added and removed inside `HEAD~2..HEAD` gives "0 match(es)", exit 0;
  `--range HEAD` an `IndexError`, and, new, `--range A...B` a `CalledProcessError`, each exit 1, its "match" status.
* T6: a misspelt file, a bare directory and, new, `files no_such_dir` each exit 1 with no output.
* T7: read; `gh run list --branch main --status cancelled` gives `[]`, so it has not happened yet.
* T9: `/usr/bin/make` here is GNU Make 3.81: `make -n -B schema/meta.json` gives `…/download/schema-v/meta.json`, where
  gmake 4.4.1 gives `schema-v1.25.0`; and `false | true` passes under 3.81, so `.SHELLFLAGS` is lost.
* T10: `Makefile:106` still says "compares the working tree's root package", and, new, `ci.yml:102`'s comment "The root
  package's exported API".
* T11: with an empty deny list, `release_body.py` exits 1 printing a blank line and its summary, no reason. T11b:
  `check_disclosure.py` exits 2 with no output outside a work tree.
* T12: `golangci-lint` with the four dead exclusions removed, and `third_party$` and `builtin$`, which match nothing either,
  gives "0 issues." for darwin and windows, as with them.
* T13: `make help` omits `all`, `check-records`, `check-tools`, `clean`, `disclosure-check`, `markdownlint`, `release` and
  `version`; `tools/test_tools.py` has no test of `platforms.py`.

**T14, new,** under F-T below.

**F-C,** by a coverage profile of both modules, `-coverpkg=./...`, go1.27.3:

* `agentproc/agentproc.go:286-294`, `abandon`: 0 of 3 blocks run; its callers are `:240` and `:244`, a failed close of the
  parent's pipe ends and a failed `proctree.Attach`.
* `agentproc/stderr.go:21-23` and `elicitation.go:286`, the two `Error` methods: 0 of 1 block each.
* `ConfigOptionUpdate.gates`: generated and dead, at `types_gen.go:25427-25434`, 0 of 4 blocks; `SessionConfigOptionUpdate.gates`
  (`:25703-25710`) runs. The type, `:2575`, is no field's, params' or result's; `unused` does not report the method, which
  satisfies the unexported `gater` interface.
* `deathsig_none.go`: counted run, as corrected. `cmd/generate`'s two `main` bodies and G13's `Model.Validates`
  (`model/validate.go:80`) are the generator PLAN's.

### Re-assessment of F-G at `ff31967`, 2026-11-07

The owner asked to "assess the generator again, this time look for bugs, gaps, missing wiring or incomplete functionality.
anything found will added to the 0049 madr." No generator PLAN had been written, so F-G had not been re-read since `e98987a`.

**The method:**

* Four investigators worked in parallel, one area each: the input and output edges (`main.go`, `load/`, `output/`), the model
  (`model/`), the emitter and the code it generates (`emit/`, the `*_gen.go` files), and the wiring and test harness across them.
* Each one worked in its own scratch copy of `ff31967`, never in the tree, with G1 to G13 given as known.
* Each finding came with a probe it ran and its output, or with exact citations where the code alone settles it.
* The author re-ran G14, G15 and G16 in fresh `git archive` copies of `ff31967`, and read the code each other finding cites.
  `git status --porcelain` was empty afterwards.
* Each probe ran on a scratch copy of the committed schemas or a small schema of its own, with the committed files untouched.

**What changed since `e98987a`:** `git diff --stat e98987a ff31967 -- cmd/generate/` lists 30 files. They are the surface
PLAN's gates, closed enums and key-less fallbacks, and 0050's accepted-ids function. `main.go`, `fuzz_test.go`, `load/`,
`output/` and `model/defaults.go` are unchanged.

**G1 to G13 stand.**

* The files that G1, G2, G5, G8, G9, G11 and G12 cite are unchanged since `e98987a`.
* The code G3, G4, G6, G7, G10 and G13 cite was read at `ff31967`. It is unchanged in substance, but some lines have moved:
  * G3: `markUnstableEntries` is `model.go:495-497`, not `:476-478`.
  * G4: `bounds` is `model/validate.go:311-361`, not `:63-67` and `:304-321`. `boundLit` is `emit/types.go:1466-1474`, not
    `:1306-1314`.
  * G6: the code is `model/fields.go:553-560`.
  * G7: `plainFields` and its comment are `model/fields.go:588-605`.
  * G10 and G13 still match their citations.

**New:** G14 to G36, under F-G below.

* Five are medium. Of these, only G14 is reached by the committed schemas today: a `null` item in an array the schema says to
  skip invalid items in makes the whole received message fail.
* G15 to G18 come into play at a schema update, or when someone edits the tree.
* The rest are low or info, and none is reached today.
* The investigators' ids map to these: A1 to G15, A2 to G16, A3 to G27, A4 to G28, A5 to G31, A6 to G32, A's CLI note to G33; B1
  to G17, B2 to G20, B3 to G21, B4 to G22, B5 to G23, B6 to G25, B7 to G26; C1 to G19, C2 to G14, C3 to G30, C4 to G24; D1 to
  G18, D2 to G29, D3 to G34, D4 to G35, D5 to G36.

**What holds,** as the investigators checked it:

* All 40 meta methods have a constant, a receiving dispatch case and a sender on the other side.
* Every capability the schema ties to a method or variant is gated. Every path-like string property is ruled.
* Gates run on send and on receipt, for requests, notifications and results.
* Key folding agrees with `encoding/json`'s last-match rule.
* Helper marking happens before emitting, and the generated output is deterministic.
* 67 of the 90 accepted engine cases have a behaviour test, each with a failure call. The other 23 are API-shape cases.

### Re-assessment of F-G at `73b0537`, 2026-11-07

The owner asked to "re-assess the codebase, changes have been made. ensure the 0049 madr as regards generator fixes is still
up-to-date and accurate", then for the generator PLAN.

**What changed since `ff31967`:** four commits, `f31f487` to `73b0537`, all G14's. `git diff --stat ff31967 73b0537 --
cmd/generate/ '*_gen.go' schema/ Makefile` lists five files:

* `emit/decode.go`, `emitSkipInvalid`;
* `model/schema.go`, `walkRefs` and the new `admitsNull`;
* two engine cases;
* `types_gen.go`'s `skipInvalid`.

No other generator file changed, and the schemas did not.

**The method:**

* A scratch script, `cites.py`, printed each file-and-line citation of F-G at `73b0537` with its enclosing declaration. The
  author read each one against its finding's claim.
* A scratch script, `reprobe.py`, ran probes on a `git archive` copy of `73b0537`:
  * ten small schemas as engine cases, each run alone with `-update`, so the generated API was written into the copy;
  * `-h`, a stray argument, and a stale `_gen.go`, with the copy's own generator binary.
* `verify.py` re-ran G15 and G16 in fresh copies.
* `git status --porcelain` was empty afterwards.
* A scratch script, `facts.py`, read both committed schemas for the facts the PLAN's designs rest on, listed below.

**Every finding stands.** Re-run at `73b0537`:

* G1: a stale `old_gen.go` that a hand-written file uses. The generator exits 0 and deletes it, and `go build` then fails
  "./uses_stale.go:3:9: undefined: staleHelper".
* G2: `Node.next`, a `$ref` to `Node` with `default: {}`. Exit 2, "fatal error: stack overflow".
* G8: a `meta.json` with no `version` is accepted.
* G11: `generate stray` exits 0 with no output.
* G15: " Reworded." appended to `ToolKind`'s first value in `schema.unstable.json` alone. Exit 2, a panic; the unchanged
  copy exits 0.
* G16: a hand-written `helpers_gen_test.go`. Exit 0, and the file is gone.
* G17: `p`, `q`, `r` and `n` are generated as `P *int32`, `Q *B`, `R *bool` and `N *string`.
* G21, G22 and G32: the enum-discriminated `Shape`, the all-const `anyOf` `E` (`type E string`), and `"X": null`, each
  accepted, exit 0.
* G23: the required `xs`, `["array","null"]`, is `Xs []string "json:\"xs\""`, with no pointer and no `omitempty`.
* G33: `generate -h` exits 1, "generate: flag: help requested".
* The code the other findings cite is unchanged since `ff31967`, where their probes ran. Their citations were re-read at
  `73b0537`.

**Citations corrected** in place, each marked "corrected 2026-11-07": G3, G4, G6, G7, G13, G18, G29 and G35. G8 gains the
manifest path that `main.go` takes. G14's citation is the code before its fix, as the finding describes the defect.

**New:** G37, under F-G. G14's refusal misses two ways in which an item admits `null`.

**Facts for the generator PLAN's designs,** read by `facts.py` over both committed schemas:

* **One property has a non-zero scalar default** (G6, Q22): `ElicitationSchema.type`, `"object"`. It is an `allOf` of
  `ElicitationSchemaType`, a `oneOf` of the single const `"object"`, and is generated as
  `Type ElicitationSchemaType "json:\"type,omitempty\""` (`types_gen.go:5929`). Its zero value, `""`, is not a member, so it
  is never a valid value to send.
* **No keyword stands beside a `$ref`** (G5 d).
* **No default sits below a nested object or an inline variant** (G7, Q23).
* **One `anyOf` has two non-null members** (G17, Q24). In the stable schema, the first member of `ClientNotification.params`
  is itself an `anyOf` of two. It is a member of `params`'s own two-member list with `null`, not a property's list, and
  `params` is generated as `json.RawMessage` (`types_gen.go:1857`).
* **Three one-value `enum`s have no `const`** (G21): `jsonrpc`, in the root's `anyOf`, outside `$defs`. None is a
  discriminator.
* **`Model.Validates` has three callers** (G13), all test files of package `model`: `edges_test.go:153`, `paths_test.go:87`
  and `unions_test.go:143`.
* **No tag names `cmd/generate`** (Q29). The generator module is not released on its own.

## Findings

Severity is the effect on a consumer or a maintainer: **high** loses data, hangs, or crashes a correct program; **medium** gives a
wrong result or breaks a documented contract on a reachable path; **low** is an edge, a gap, or a wrong word.

### F-R: the connection runtime

*Done 2026-11-03:* R1 to R10 are fixed in `v1.31.2` by [0049-PLAN-runtime-fixes.md](0049-PLAN-runtime-fixes.md), each with
the test its phase 4 table names; R2 as amended under "R1's and R2's designs". *R11, done 2026-11-05,* in `v1.33.1` by
the same PLAN's phases 5 and 6, per Q20 (a), with the tests its phase 6 table names.

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

**R11. ~~Unrated~~ Medium, rated 2026-11-05; ~~open~~ done 2026-11-05 in `v1.33.1`. `TestOverflowBlockHeldInitialize` fails intermittently with the connection ended.** *Added
2026-11-04,* by the tooling PLAN's phase 6, its deviation 3, on the owner's choice to record it as a finding rather than a
footnote. `connection_overflow_block_test.go:306-339`, a `synctest` test of 0044's D3: with a queue of 1 under
`OverflowBlock`, three extension notifications held for `initialize`'s answer, then the answer.

* **Seen twice:** in [0049-PLAN-surface-fixes.md](0049-PLAN-surface-fixes.md)'s phase 2, a pre-add check at load 43,
  `-shuffle=1793756062234892000`; and in [0049-PLAN-tooling-fixes.md](0049-PLAN-tooling-fixes.md)'s phase 6,
  `tools/gates.py`'s `race3`, `go test -race -count=3 -shuffle=on .` at load 24 to 26, `-shuffle=1793834057755576000`. Both
  times: "Initialize = {"code":-32603,"message":"Internal error","data":{"error":"connection ended before response"}}, want
  its result" (`:329`). Neither phase changed the root package's code.
* **Not reproduced:** 2000 runs each in the tree and in archives of `v1.31.1` and `e98987a` (2026-11-03); 200 runs alone
  under `-race`, and the package with the second seed, `-race -count=3` (2026-11-04).
* **What it suggests, unconfirmed:** a `synctest` test fails by order, not by time. "connection ended before response" is
  `Initialize`'s answer when the connection ends with the request pending, so the connection ended, or was seen to, before
  the answer was delivered. Whether the SDK's held-notification path or the test's harness orders it so is not known.
* **Severity** is left unrated until the cause is known: a defect in the SDK's path would end a correct client's
  `Initialize` (medium); one in the harness, none.
* Next: examined by an amendment of [0049-PLAN-runtime-fixes.md](0049-PLAN-runtime-fixes.md), on the owner's approval.

*Cause found 2026-11-04,* on the owner's "work on R11, track that down so we can resolve it", by reading and two probes in
`git archive` copies of `c69499d`, never in the tree:

* **The decision is the channel's, not the count's.** `queueNotification` (`connection.go:1875-1935`) admits a
  notification when the send to `notificationQueue` succeeds (`:1894-1900`), and otherwise meets the policy. The channel's
  capacity is `n`, or `2n` under `OverflowBlock` (`:924-927`), and the notification the loop will run next stays in it until
  `processNotifications` (`:2137-2138`) has taken it. So whether the peer's notification `n+1`, or `2n+1` on the read-on
  path, is refused depends on whether the loop goroutine has been scheduled to take the head, not on what the peer sent or
  what the handlers have finished. `WithMaxQueuedNotifications` says "how many notifications may wait in the queue while a
  notification handler runs" (`:594-595`), and `OverflowBlock` "reads on, to twice the queue's size" (`:756-758`).
* **R11 is that race on the read-on path.** `sendRequestCommit` counts a peer wait for as long as `initialize` is in flight
  (`:2562`), so `waitForRoom` does not pause (`:2181`); with a queue of 1 the channel holds 2; when the third notification
  arrives before the loop has taken the first, the `select` falls to `default` and the connection closes with
  `ErrNotificationQueueOverflow` (`:1926-1934`), and `Initialize` answers "connection ended before response" (`:2848`).
  `synctest` fixes time, not the order in which runnable goroutines run.
* **What is not affected:** `waitForRoom`'s pause counts by the sequence, received and not completed (`:2180`); the byte
  budget counts bytes at enqueue and lets them go as the handler returns (`:1896-1897`, `:2152-2153`). Neither depends on the
  dequeue.
* **Probe 1** (`p49/r11/probe1.py`): the loop sleeps 1 ms, in fake time, before each dequeue, so the reader queues all it
  can first. `TestOverflowBlockHeldInitialize` fails on its first run, "connection_overflow_block_test.go:329: Initialize =
  {"code":-32603,…"connection ended before response"}, want its result", the failure seen twice;
  `TestHeldNotificationsFillQueue`, which requires the close, passes, 20 of 20.
* **Probe 2** (`p49/r11/probe2_test.go.txt`): after `initialize`, two notifications to a client with a queue of 1 whose
  handler blocks, one to run and one to wait, as the option allows; 20 runs a line:

  | Policy | As the code is | The loop slow to dequeue |
  | --- | --- | --- |
  | `OverflowCloseConnection` | open, 2 handled | closed, 1 handled |
  | `OverflowDropNewest` | open, 2 handled | open, 1 handled: one dropped |
  | `OverflowBlock` | open, 2 handled | open, 2 handled: it pauses |

  So the same input from the peer is closed on, dropped, or handled, by scheduling, under the two refusing policies and on
  `OverflowBlock`'s read-on path. The race has stood since the queue was bounded with `select … default`; the read-on path,
  R11's, since 0044 (`v1.29.0`).
* **Severity, proposed: medium.** On a reachable path it breaks the documented limits: a correct client's connection
  closes, or a notification is dropped, at the limit's boundary, by an ordering the program does not control. It needs a
  burst that reaches the boundary before the loop takes its head, which is rare at the default 1024 and likely at a small
  queue.

**Q20. R11, the boundary's fix:**

* (a) **Admit by the count, and give the channel the room the count allows** (recommended). The policy acts when the
  notifications received and not completed, the one running or next to run included, would pass `n + 1`, or `2n + 1` on
  `OverflowBlock`'s read-on path; the channel's capacity is that, so a send the count admits never fails. The outcome then
  depends on what the peer sent and what the handlers finished, as the byte budget's does, and matches the documented "n
  wait while a handler runs". No API change; a refusal that came sometimes at the boundary no longer comes, and none comes
  earlier than now: a patch release. Its test needs a seam to hold the loop before a dequeue, an unexported hook, as
  phase 5 of [0049-PLAN-tooling-fixes.md](0049-PLAN-tooling-fixes.md) added `attachTree`.
* (b) **Admit by the count, with no room for the running one.** The policy acts past `n` received and not completed, and
  the docs change to say so. As deterministic and simpler, but every limit shrinks by one against what a promptly
  scheduled loop gives now: a behaviour change, a minor release whose notes lead with it.
* (c) **Document the boundary as approximate, and order the test.** The docs say a refusal at the limit may come one
  notification early; `TestOverflowBlockHeldInitialize` waits for the loop to take the first notification before sending
  the rest. Cheapest, but the scheduling dependence stays, by contract.

*Answered by the owner, 2026-11-05:* Q20 (a), and the severity medium, as proposed. R11 is fixed by phases 5 and 6 of
[0049-PLAN-runtime-fixes.md](0049-PLAN-runtime-fixes.md), amended the same day, in a patch release, `v1.33.1`. It changes
[0044-MADR-notification-queue-back-pressure.md](0044-MADR-notification-queue-back-pressure.md)'s D4 as to the channel's
capacity, `2n + 1` and `n + 1`, not `2n` and `n`; that record carries the amendment.

### F-S: the SDK surface against the schema

*Done 2026-11-04:* S1 to S9 are fixed in `v1.32.0` by [0049-PLAN-surface-fixes.md](0049-PLAN-surface-fixes.md), each with
the test its phase 6 table names, under its deviations 1 to 5 and the amendments of 2026-11-03 to Q8, Q9 and Consequences.

*Re-run 2026-11-03 at `e98987a`:* all nine reproduce ("Re-assessment" above).
[0049-PLAN-surface-fixes.md](0049-PLAN-surface-fixes.md) took them up.

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

**G1. Medium; done 2026-11-07 by `5e0761b`, for `v1.34.2`. `output.Check` checks a different tree from the one `Write` leaves.** `cmd/generate/internal/output/check.go:53-56`,
`:72-74`, `:183-215`; `write.go:62`, `:89-93`. `handWritten` takes the directory's Go files less the rendered names, so a stale
`*_gen.go` is checked as hand-written and then deleted by `Write`; and `Check` returns before `unstable/` when nothing unstable
renders, though `Write` removes `unstable_gen.go`. Read by the author at those lines.

* **Probe** (investigator): a stale `old_gen.go` used by a hand-written file: generator exit 0, then `go build` fails
  "undefined: staleHelper"; the same for `unstable/`; and the inverse, a stale file that collides, refused although `Write` would
  have deleted it.
* Fix: exclude the stale files from the checked set, and check `unstable/` whenever it holds hand-written files.

**G2. Medium; done 2026-11-07 by `1a8d09c`, for `v1.34.2`. A self-referential default crashes the generator.** `internal/model/defaults.go:236-262`: `structValue` recurses
through a property's default with no visited set. **Probe** (`recursive-default.txtar`, re-run): "gen rc=2", "fatal error: stack
overflow", in `renderer.structValue`. `FuzzBuild` found it in 25 s. Fix: refuse a recursive default, naming it.

**G3. Medium; done 2026-11-07 by `1a8d09c`, for `v1.34.2`. A property the manifest changed silently overrides the stable schema's own later definition of it.**
`internal/load/manifest.go:194-203`, `:222-228`; `model.go:495-505` (`markUnstableEntries`; corrected 2026-11-07, from
`:476-478`) drops the notice too. **Probe** (`stabilized.txtar`, re-run): the
stable `Shared.b` is `{"type":"integer"}`; the generator exits 0 and emits `B string`; decoding `{"b":5}` fails "cannot unmarshal
number into Go struct field Shared.b of type string". Fix: refuse when the stable definition now holds the key and differs.
*Amended 2026-11-07,* by [0049-PLAN-generator-fixes.md](0049-PLAN-generator-fixes.md)'s phase 2, deviation 2: "differs"
leaves out annotations and `default`. The committed manifest overrides `AgentCapabilities.mcpCapabilities` and
`InitializeResponse.agentCapabilities` on purpose, and only their defaults differ, by the unstable `"acp": false`.

**G4. Low; done 2026-11-07 by `ac41197`, for `v1.34.2`. Integer bounds pass through `float64`.** `load/load.go:64-65`, `model/validate.go:311-361` (`bounds`),
`emit/types.go:1466-1474` (`boundLit`: `uint64` of an out-of-range float, implementation-defined). Corrected 2026-11-07, from
`validate.go:63-67`, `:304-321` and `types.go:1306-1314`. **Probe** (investigator): a `uint64`
`minimum` of 2^64 emits `< 18446744073709551615` on `arm64` and `< 9223372036854775808` on `amd64`; an `int64` `maximum` of
2^53+1 is emitted as 2^53. No committed case is affected. Fix: read bounds as `json.Number`, refusing magnitudes at or above 2^53 as
`integerConst` does. *Amended 2026-11-07,* by [0049-PLAN-generator-fixes.md](0049-PLAN-generator-fixes.md)'s phase 4, deviation 1: an
integer type's bound is computed exactly with `math/big` instead, as 0019's G12 and G21 accept a fractional bound and a
`uint64` one above `math.MaxInt64`.

**G5. Low; done 2026-11-07 by `ac41197`, for `v1.34.2`. Shapes silently dropped or loosened, each exit 0.** (a) `not` and `unevaluatedProperties` outside a catch-all;
(b) `const` and `enum` on an object property; (c) bounds on array items and map values; (d) `$ref` siblings; (e) a definition
that is only a `$ref`, emitted as raw JSON; (f) an unknown `type` name, emitted as raw JSON; (g) duplicate keys, the last one
winning. **Probe** (investigator, one probe schema each); none occurs in the committed schemas (scripts `latent.py`,
`kwpaths.py`, `constprops.py`). Fix: refuse each with its path, as 0012 aims.

**G6. Low; done 2026-11-07 by `624edc1`, for `v1.34.2`. A non-zero scalar default with `omitempty` cannot send the zero value.** `model/fields.go:551-567` (corrected
2026-11-07): the default is read at `:551-560`, and `:561-567` tags `omitempty` on every optional field whose default is not a
nil-able array or object. **Probe**
(investigator): `{"n":0,"s":"","b":false}` with defaults `5`, `"d"`, `true` re-encodes without `s` and `b` (`n`, nullable,
is a pointer and is sent; corrected 2026-11-03), and the peer reads the defaults. The committed schemas have one non-zero scalar default, where `""` is invalid anyway. Fix: refuse, or emit a pointer.

**G7. Low; done 2026-11-07 by `ac41197`, for `v1.34.2`. Defaults on nested inline objects and inline-variant properties are not applied.** `model/fields.go:588-610`
(`plainFields`; corrected 2026-11-03, and 2026-11-07) says so in its comment at `:590`, while `checkDefaults`
(`model/fields.go:380`) accepts them. **Probe** (investigator):
"inner.flag=false (schema default true)". None in the committed schemas. Fix: refuse, or apply.

**G8. Low; done 2026-11-07 by `ac41197`, for `v1.34.2`. `meta.json` without `version` emits `ProtocolVersionNumber = 0`.** `load/load.go:18-24`, `merge.go:84`; and `load/manifest.go:116`, on the path `main.go` takes, copies `stableMeta.Version`
unchecked (added 2026-11-07). **Probe**
(`noversion.txtar`, re-run): "gen rc=0". Fix: require `version >= 1`.

**G9. Low; done 2026-11-07 by `5124800`, for `v1.34.2`. `Check` type-checks only the generator's own `GOOS` and `GOARCH`, the host's unless the environment sets them.**
`output/check.go:189-191`. **Probe** (investigator): a Windows-only file that collides passes on `darwin`, and `GOOS=windows go build` then fails. Fix: also check for `windows` and
`linux`.

**G10. Low; done 2026-11-07 by `5124800`, for `v1.34.2`. `TestGeneratedFilesUpToDate` misses a stale root `*_gen.go`.** `cmd/generate/main_test.go:38-68` (corrected
2026-11-03) checks `unstable/` only. **Probe** (investigator): with `extra_gen.go` at the root, the test passes. Fix: the same loop for the root.

**G11. Low; done 2026-11-07 by `78f72e9`, for `v1.34.2`. Stray positional arguments are ignored.** `main.go:37-39`. **Probe** (investigator): `gen /some/other/out stray` exits 0
and writes to the working directory's repository. Fix: refuse `fs.NArg() > 0`.

**G12. Low; done 2026-11-07 by `2d8b62f`, for `v1.34.2`. `FuzzBuild` starts almost only from refused seeds, and never type-checks.** `cmd/generate/fuzz_test.go:36` passes an
empty `Meta`, so since 0019's G18 nearly every seed is an unlisted method. **Probe** (investigator): of 187 seeds, "accepted 1,
refused as unlisted-method 116, refused otherwise 70". Fix: fuzz each case's schema with its own `meta.json`, and run `Check`.

**G13. Info; done 2026-11-07 by `e31707c`, for `v1.34.2`. `Model.Validates` is used only by tests.** `model/validate.go:80` (corrected 2026-11-07); `deadcode ./...` lists it. Fix: move it to a test
file, or keep it as a test helper by name.

*Added 2026-11-07,* by the fourth re-assessment at `ff31967`: G14 to G36. Paths are relative to `cmd/generate/internal/`
unless they start at the repository (`cmd/`, the root's files).

**G14. Medium; done 2026-11-07 by `7b0cb46`, released in `v1.34.1`. A `null` item survives `x-deserialize-skip-invalid-items`, and the whole received message is then refused.**
*Done* by [0049-PLAN-null-items-in-skip-invalid-arrays.md](0049-PLAN-null-items-in-skip-invalid-arrays.md), per Q21 (a):
`skipInvalid` drops a `null` item, and the generator refuses skip-invalid items that admit `null`. The tests the
PLAN's phase 1 names failed with the fix removed.

`emit/decode.go:423-443` (`emitSkipInvalid`, at `ff31967`; `:427-448` since the fix), in `types_gen.go` as `skipInvalid`, keeps an item when `json.Unmarshal(it, &v)`
returns nil. For `null`, `encoding/json` leaves a string, an open enum or a generated union unchanged and returns nil, so the
zero value is kept, not dropped. It then fails `Validate`, which `checkInbound` runs on every received message.

* **Probe** (re-run by the author): `SessionNotification` with a `tool_call` whose `content` is
  `[null, {"type":"content",…}]` decodes, then fails `Validate` with "SessionNotification: update: SessionUpdate: variant
  ToolCall: SessionUpdateToolCall: content[0]: ToolCallContent must have exactly one variant set". With `7` in place of
  `null`, the item is dropped and `Validate` is nil.
* **The investigator's probe:** `ListSessionsResponse`'s `additionalDirectories` of `[null, "/a"]` fails "`""` is not an
  absolute path", and an `InitializeResponse` whose `authMethods` is `[null]` fails "AuthMethod must have exactly one variant
  set".
* **Committed schemas: yes.** The skip-invalid fields of `ToolCallContent`, `AuthMethod`, `SessionConfigOption` and the
  absolute-path strings, and `ContentBlock` through `patchItems`, are all affected. A peer that sends a `null` item there gets
  `-32602`, or an error from `Initialize`, where the schema says to drop the item.
* **Fix:** skip an item whose raw value is `null`, unless the element type admits null. This changes the generated code, so
  it reaches consumers.

**G15. Medium; done 2026-11-07 by `1a8d09c`, for `v1.34.2`. A change to a stable enum in the live unstable schema, even to one value's description, crashes the
generator.** Stable enums are a `oneOf` of `{"const": …}` entries, which have no `UnionEntryKey`. `load/unstable.go:150-176`
(`newEntries`) then compares entries with `reflect.DeepEqual`, so any difference counts as a new variant, and
`load/unstable.go:81-92` records its index. The model builds the enum with no `Variants`, and `emit/unstable.go:74`
(`t.Variants[i]`) indexes past the end.

* **Probe** (re-run by the author): with " Reworded." appended to the description of `ToolKind`'s first value in
  `schema.unstable.json` alone, the generator exits 2 with "panic: runtime error: index out of range [0] with length 0", in
  `emit.unstableFile` at `emit/unstable.go:74`. The unchanged copy exits 0.
* **The investigator's probe:** adding a value `"browse"` to `ToolKind` in the unstable schema alone panics the same way, with
  "index out of range [10] with length 0".
* **Committed schemas:** no, since the live unstable schema still equals the 1.25.0 freeze. But many stable enums have this
  shape, so a schema update that touches one stops regeneration with a panic, before anything is written.
* **Fix:** key a const-only entry by its `const`, compare entries without their annotations, and refuse an index the model's
  type does not have, with an error instead of a panic.

**G16. Medium; done 2026-11-07 by `5e0761b`, for `v1.34.2`. `Write` deletes a hand-written file named `*_gen_test.go`, silently.** `output/write.go:99-124` (`staleFiles`)
lists every `*_gen.go` and `*_gen_test.go` in the root and in `unstable/` that the run did not render, and `:89-93` removes
them. `helpers_gen_test.go` is the usual Go name for a hand-written test of `helpers_gen.go`.

* **Probe** (re-run by the author): a hand-written `helpers_gen_test.go` (`package acp`, one empty test) at the root. The
  generator exits 0 with no output, and the file is gone afterwards. `Check` had accepted it as a hand-written test first.
* **How it differs from G1:** G1 is about stale generated files, and about checking a different tree from the one `Write`
  leaves. Here what is deleted is hand-written, uncommitted work, unrecoverably and with no message.
* **Committed tree:** no such file today.
* **Fix:** delete only files that carry the generated header (`// Code generated … DO NOT EDIT.`), and name each one removed.

**G17. Medium; done 2026-11-07 by `1a8d09c`, for `v1.34.2`. A property whose `anyOf` or `oneOf` has exactly two members is typed from one of them, the wrong one where both
are non-null.** `model/fields.go:331-360` (`optionalTypeRef`) loops over the two members and keeps the last non-null one
(`nonNull = e`). It also retypes a `const` member as a string (`nn.Type = schemaTypeString`), whatever the const's own type.

* **Probe** (investigator), two non-null members: `p: {anyOf: [string, int32]}`, `q: {anyOf: [$ref A, $ref B]}` and
  `r: {oneOf: [$ref A, boolean]}`. The generator exits 0 with `P *int32`, `Q *B` and `R *bool`. Decoding
  `{"p":"text","q":{"a":"x"},"r":{"a":"y"}}` fails "cannot unmarshal string into Go struct field Alias.p of type int32".
* **Probe** (investigator), a typed const: `{"anyOf":[{"type":"integer","format":"int32","const":5},{"type":"null"}]}`
  becomes `N *string`, and `{"n":5}` fails "cannot unmarshal number into Go struct field Thing.n of type string". A boolean
  const fails the same way.
* **Committed schemas:** no. A schema update with either shape would generate a field that refuses valid payloads, and the
  generator would exit 0.
* **Fix:** refuse an `anyOf`/`oneOf` with two non-null members, and type a const from its own type, or refuse a const that is
  not a string.

**G18. Medium; done 2026-11-07 by `1a8d09c`, for `v1.34.2`. The tests of the protocol table against the committed schemas build their model from the live unstable schema;
the generator builds from the frozen manifest.** `cmd/generate/internal/model/classes_test.go:44` (`committedInput`) calls
`load.MergeStableAndUnstable`. `cmd/generate/main.go:199-201` (corrected 2026-11-07) uses `load.MergeManifest` whenever `schema/unstable-v1.json`
exists, which it does. `committedInput` feeds `TestProtocolTableAppliesToCommittedSchema` (`model/protocol_test.go:442`), the
union-class tests (`model/classes_test.go:55`, `:118`, `:220`), `model/keywords_test.go:233` and `model/unstable_test.go:128`.

* **Probe** (investigator), the two models today: "types 280/280", "gates 45/45", "paths 17/17", "methods 40/40" and
  "patch 12/12", with nothing on one side only. They are the same model today.
* **Probe** (investigator), a false failure: `nes/close` dropped from the live unstable meta, with its two definitions
  unlinked. The table test fails "agent method "nes/close": not in the committed meta files" and "27 capability gates, want 28".
  Yet the generator exits 0 with byte-identical output.
* **Probe** (investigator), a false pass: `session/foo` added to the live unstable schema alone, with a table rule for it. The
  table test passes and the generator exits 0, but no generated file mentions the rule's `FooProbe`.
* **Consequence:** once the live unstable schema moves away from the manifest, the tests that guard the table check a model
  the SDK does not ship. A rule can pass its test and be enforced nowhere, since Build skips a rule whose method it does not
  find.
* **Fix:** build `committedInput` with the same function `main.go`'s `loadSchema` uses, shared.

**G19. Low; done 2026-11-07 by `08ad4c1`, for `v1.34.2`. `Validate` and the gates do not reach inside a union variant that is a named array, and nothing refuses that
shape.** `model/validate.go:160-163` adds only `InlineStruct` variants to the validation shapes, and `:110-112` sets
`vi.Validates` from `m.validates[vi.TypeName]`, which a named slice type such as `[]Option` never gets.
`emit/types.go:1237-1243` then emits no call for that variant. `nested()` (`model/validate.go:271-289`) refuses a shape that
`Validate` does not descend into for fields, not for variants. The gates have the same gap (`model/gates.go:479-481`,
`:505-509`; the refusal at `:145-151` loops over fields only).

* **Probe** (investigator): `Option.name` has `minLength: 1`, and `Options` is an `anyOf` of `[]Option` and `[]Group`. An
  `Option` in a direct slice field fails "Root: direct[0]: Option: name: shorter than minLength 1". The same `Option` inside the
  array variant gives `Validate = <nil>`.
* **Committed schemas:** not reached. `SessionConfigSelectOptions` has exactly this shape, but its item types have no check
  yet, so the first check added to `SessionConfigSelectOption` or `SessionConfigSelectGroup` would be skipped.
* **Fix:** descend into the items of `InlineNamed` slice and map variants, in `Validate` and in the gates, or refuse such a
  variant when its items validate or are gated.

**G20. Low; done 2026-11-07 by `08ad4c1`, for `v1.34.2`. A capability rule with a `Property` reaches only its definition's own fields, so it can apply and gate nothing.**
`model/gates.go:200-208` (`gateCheck`) searches `t.Fields`, and returns `false, nil` when the property is not there. The patch,
empty-clears, null-retained and absolute-path rules instead follow `f.prop` into every copy (`model/model.go:1455-1477`,
`:1562-1577`, `:1624-1639`; `model/validate.go:185`). `resolveKeyOrders` (`model/model.go:1660-1676`) also reads only `t.Fields`.
*Amended 2026-11-07,* by [0049-PLAN-generator-fixes.md](0049-PLAN-generator-fixes.md)'s phase 6, deviation 1: it does, but it
refuses such a rule, "a key-order rule needs a property of an object", as a scratch copy of `624edc1` showed. Only
`gateCheck` applied and gated nothing. *Amended 2026-11-07,* by that PLAN's phase 6, deviation 2: Q28 (a) refuses the
first probe's rule; the owner extended it to the second's, a rule whose property is also a field of another struct, which
it would gate in part.

* **Probe** (investigator): a `GateNonEmpty` rule on `LoadSessionRequest.dirs`, with `LoadSessionRequest` a union with an own
  `dirs`. Build returns nil, and no type is gated. The same happens for a definition copied into an inline variant by `allOf`,
  where `WrapD`'s `Dirs []string` is not gated.
* **Committed schemas:** not reached.
* **Fix:** resolve gates and key orders per field by `f.prop` across `Fields`, `NestedStructs` and inline variants, as the
  patch rule does, and refuse what cannot be gated.

**G21. Low; done 2026-11-07 by `624edc1`, for `v1.34.2`. A discriminator written as a one-value `enum` is not recognised, so every payload decodes into `Unknown`.**
`model/model.go:748-756` (`discriminatorValue`) and `model/schema.go:134-136` read only `Const`.

* **Probe** (investigator): `Shape.oneOf` variants with `"kind": {"type":"string","enum":["circle"]}` and `["square"]`, and a
  declared discriminator. The generator exits 0. `{"kind":"circle","r":1}` decodes as `{ShapeVariant1:<nil>
  ShapeVariant2:<nil> Unknown:{"kind":"circle","r":1}}`, and so does every other input.
* **How it differs from G5 (b):** G5 (b) is a constraint left unchecked; here the union's routing breaks.
* **Committed schemas:** not reached.
* **Fix:** treat a one-value enum as a const, or refuse an enum on a discriminator.

**G22. Low; done 2026-11-07 by `624edc1`, for `v1.34.2`. An `anyOf` whose members are all string consts is modelled as an open enum, so `Validate` accepts any string.**
`model/schema.go:167-194` (`isOpenStringEnum`) requires only that some member is a const, although its comment says the type
permits arbitrary strings. `model/validate.go:212-215` leaves open enums out of the closed-enum checks.

* **Probe** (investigator): "E kind 3 (KindOpenEnum=3) closed=false", then "O.E Closed=false validation=<nil>".
* **Committed schemas:** not reached. Each `anyOf` string enum in them, such as `CompactionStatus`, has a plain string
  member.
* **Fix:** require at least one non-const string member for an open enum, and type the rest as closed.

**G23. Low; done 2026-11-07 by `624edc1`, for `v1.34.2`. A required nullable collection turns a relayed `null` into `[]`.** `model/fields.go:580` and `:603` set `Fill`
from `Required` and `IsCollection()` and ignore `includesNull`.

* **Probe** (investigator): required `xs: {"type":["array","null"]}` becomes `Xs []string`. `{"xs":null}` decodes with no error
  and re-encodes as `{"xs":[]}`.
* **Committed schemas:** not reached.
* **Fix:** make a required nullable collection a pointer, or do not fill it when null is admitted.

**G24. Low; done 2026-11-07 by `624edc1`, for `v1.34.2`. A closed-enum array marked `x-deserialize-default-on-error` becomes nil, not its default, when an item is
unknown.** `emit/types.go:787-793` (`unknownAbsent`'s `RefSlice` case without skip-invalid-items) assigns `nil`, and its comment
says the array "takes its zero value". The pointer case, `:784-786`, also ignores `def`. The scalar case, `:796-800`, assigns
`def`.

* **Probe** (investigator): `kinds` is an array of `K {a, b}` with default `["a"]`. `{"kinds":5}` gives `[a]`, but
  `{"kinds":["zzz"]}` and `{"kinds":["b","zzz"]}` give `nil`. `MarshalJSON` then writes the default, so what was read and what is
  re-sent differ.
* **Committed schemas:** not reached. The two closed-enum arrays, `audience` and `positionEncodings`, skip invalid items.
* **Fix:** assign `def`, when there is one, in the slice and pointer cases.

**G25. Low; done 2026-11-07 by `08ad4c1`, for `v1.34.2`. A deprecated property copied into an inline variant has no `Deprecated:` notice.** `model/fields.go:599`
(`plainFields`) never sets `Deprecation`. `deprecationsOf` (`model/protocol.go:1013-1023`) and `applyFrozenDeprecations`
(`model/frozen.go:49-59`) also mark only `t.Fields`.

* **Probe** (investigator): `Base.old` is deprecated, and `U.oneOf[0]` is `allOf Base`. `want.deprecated` lists
  `field Base.Old` alone, and `UBase.Old` has no notice.
* **Committed schemas:** not reached.
* **Fix:** carry `deprecationOf(prop)` in `plainFields`, and mark copies by `f.prop`.

**G26. Low; done 2026-11-07 by `08ad4c1`, for `v1.34.2`. Some name collisions are refused only at `Check`, naming a generated file instead of the schema or the rule.**

* **Interface methods:** `model/methods.go:171-184` keeps one set of method names per binding.
  *Amended 2026-11-07,* by [0049-PLAN-generator-fixes.md](0049-PLAN-generator-fixes.md)'s phase 6: the probe's shape below
  does not collide at `624edc1`, as the live merge prefixes unstable definitions (`Foo` and `UnstableFoo`). The collision is
  reached with a method bound to the optional interface beside one on `Agent`.
  * **Probe** (investigator): a stable `FooRequest` and an unstable `FooNotification` both become `Foo`. Build and Render return
    nil, and `Check` fails "client_gen.go:132:32: method ClientSideConnection.Foo already declared".
  * A method named like a hand-written method of the connections (`Close`, `Wait`, `Err`) is not checked either.
* **A key order's `Field`:** it is checked with `checkExported` (`model/protocol.go:436`) and against the struct's fields
  (`model/model.go:1673`), but not against the generated methods or another key-order rule.
  * **Probe** (investigator): `Field: "MarshalJSON"` passes Build, and `Check` fails "types_gen.go:44:28: field and method with
    the same name MarshalJSON".
* **`Interface.Var`:** it is checked only for being non-empty (`model/protocol.go:373`), not as an identifier, nor against the
  dispatch's locals.
* **Committed table:** not reached.
* **Fix:** keep one set of outbound method names per side, with the connections' own methods reserved; apply `checkNotMethod`
  and a check across rules to a key order's `Field`; check `Var` with `token.IsIdentifier`.

**G27. Low; done 2026-11-07 by `5124800`, for `v1.34.2`. `Check` does not type-check the root's external test package, or the packages that import the root.**
`output/check.go:183-215` (`handWritten`) lists `GoFiles` and `TestGoFiles`, never `XTestGoFiles`. 17 root files are
`package acp_test`, and `acptest`, `agentproc`, `clienthost`, `remote` and `example/` are not checked at all.

* **Probe** (investigator): `ToolKind`'s `"search"` renamed to `"find"` in both schemas. The generator exits 0, then `go vet .`
  fails "./tooltimeline_test.go:48:35: undefined: acp.ToolKindSearch". The same reference in a `package acp` test file is
  refused by `Check`.
* **Committed tree:** not reached. CI's `go vet` and build catch it after the files are written.
* **Fix:** type-check `XTestGoFiles` against the in-memory root, as `servingImporter` does for `unstable/`.

**G28. Low; done 2026-11-07 by `5124800`, for `v1.34.2`. `Check` reads the language version from `<out>/go.mod` only.** `output/check.go:219-234` (`goVersion`) reads
`filepath.Join(dir, "go.mod")` and otherwise returns `""`, the newest version. `rootImportPath` (`cmd/generate/main.go:94-118`)
walks up to the enclosing `go.mod`.

* **Probe** (investigator): `go 1.21` in a parent `go.mod`, `-out` a subdirectory, and `for range 3 {}` in a hand-written file.
  The generator exits 0, then `go build` fails "cannot range over 3 … requires go1.22 or later". With the `go.mod` inside
  `-out`, `Check` refuses it with the same message.
* **Committed tree:** not reached, since the repository's `-out` is the module root.
* **Fix:** take the `go` line from the `go.mod` that `rootImportPath` finds.

**G29. Low; done 2026-11-07 by `2d8b62f`, for `v1.34.2`. Nothing requires a rule where the schema's wording asks for a capability gate, a method gate or an absolute path.**
Four rule kinds have a scan of the schema's descriptions that requires a rule wherever their wording appears: `patchWording`
and `retainedWording` (`model/patch_test.go:307`, `:349`), `replaceWording` (`:483`) and `fallbackWording`
(`model/keyless_test.go:93`). The capability, method-gate and absolute-path rules have only the converse check, that each rule
applies, and fixed counts (`model/protocol_test.go:483`, `model/gates_test.go:227`, corrected 2026-11-07).

* **Probe** (investigator): three sentences added to the schemas: "Only available if the Agent supports the
  `sessionCapabilities.list` capability." on `SetSessionModeRequest`; "Agents MUST only send this update when the Client
  advertised …" on `usage_update`; and "Must be an absolute path." on `CreateTerminalRequest.command`.
  `TestProtocolTableAppliesToCommittedSchema` passes. The control, "Set to null to clear." on an unruled property, fails as
  required.
* **Committed schemas:** every such sentence today has its rule: 7 stable and 8 unstable requests, and 3 `SessionUpdate`
  variants.
* **Fix:** the same wording scans for these three rule kinds.

**G30. Info; done 2026-11-07 by `e31707c`, for `v1.34.2`. Nine hand-written `Kind()` methods compile only because one gate reads a raw key.** `emit/gates.go:59-73`
(`markGateHelpers`) marks `unknownKind` only for a gate check with a `RawKey`, and `emit/decode.go:71-74` emits it only when
marked. Yet `helpers.go:342`, `:366`, `:383`, `:398`, `:413`, `:429`, `:469`, `:535` and `:554` call it. The one check with a
`RawKey` today is the prompt's undeclared-variant rule (`model/acp.go:232`).

* **Probe** (investigator): with that rule removed, the generator exits 1, "helpers.go:342:10: undefined: unknownKind", and
  eight more lines like it.
* **Fix:** emit `unknownKind` whenever a union has an `Unknown`.

**G31. Info; done 2026-11-07 by `78f72e9`, for `v1.34.2`. The manifest's nested objects match keys in any letter case, although `ParseManifest` decodes "strictly".**
`load/load.go:141-148` (`decodeStrict`) checks letter case on the top level only, and `ManifestMethods` and `ManifestChange`
(`load/manifest.go:45-60`) have no strict decoder.

* **Probe** (investigator): `"changes": {"Obj": {"PROPERTIES": ["b"]}}` is accepted, and `Obj` gains `B`. A top-level
  `"CHANGES"` is refused.
* **Fix:** a `decodeStrict` `UnmarshalJSON` on both, as `Discriminator` has.

**G32. Info; done 2026-11-07 by `ac41197`, for `v1.34.2`. A definition that is `null` is dropped silently, and a `$ref` to it reports the wrong cause.**
`load/load.go:213-216` keeps it as nil, and the model skips it.

* **Probe** (investigator): `{"$defs": {"X": null}}` exits 0 with no `type X`. A `$ref` to it fails "names no definition".
* **How it differs from G5:** G5 drops keywords and shapes; this drops a whole definition, by the load's explicit `null` case.
* **Fix:** refuse a `null` definition by name.

**G33. Info; done 2026-11-07 by `78f72e9`, for `v1.34.2`. `generate -h` exits 1.** `cmd/generate/main.go:37-39` returns `fs.Parse`'s error, `flag.ErrHelp` included, as a
failure. It prints "generate: flag: help requested". **Fix:** exit 0 on `flag.ErrHelp`.

**G34. Info; done 2026-11-07 by `e31707c`, for `v1.34.2`. `docs/architecture.md`'s list of what the protocol table holds omits `DocNotes`.** `docs/architecture.md:182-282`
lists every rule kind but `DocNotes`, which `model/acp.go` holds 10 of. **Fix:** one bullet.

**G35. Info; done 2026-11-07 by `2d8b62f`, for `v1.34.2`. Two rule kinds have no engine case.** Engine cases run with `&model.ACP` (`cmd/generate/main.go:61`, called by `engine_test.go:75`; corrected 2026-11-07), so they
reach
only the rule kinds ACP's table uses.

* `DeprecationNotes` is empty in ACP, so no engine case reaches it. In a sampled run of `internal/mutate`, the mutant
  `model/protocol.go:1026` `&&` to `||` survived. That line matches a note to a variant without a discriminator value.
* `EmptyClears` has no engine case. `TestEmptyClearsRules` and the committed-schema check cover it.
* **Fix:** a unit case for a variant without a discriminator value, and an `accept-empty-clears` engine case with a behaviour
  test.

**G36. Info; done 2026-11-07 by `2d8b62f`, for `v1.34.2`. The table test looks a method up by its wire name alone.** `model/protocol_test.go:72-79` (`methodByWire`)
returns the first method with that name, agent methods first. `mcp/message` is on both sides, so a later client-side rule for
it would be reported "not in the committed meta files". It can fail falsely but not pass falsely. **Fix:** look up by side and
wire.

*Added 2026-11-07,* by the fifth re-assessment at `73b0537`: G37.

**G37. Low; done 2026-11-07 by `ac41197`, for `v1.34.2`. G14's refusal misses an item that admits `null` through an `allOf`, or by having no type; `skipInvalid` then
drops a valid `null`.** `model/schema.go:393-408` (`admitsNull`) reads `type`, `anyOf`, `oneOf` and `$ref`, never `allOf`. It
returns false for an item with no `type`, which admits every JSON value. `skipInvalid` (`emit/decode.go:427-448`) drops every
`null` item.

* **Probe** (the author, `reprobe.py` at `73b0537`):
  * items `{"allOf": [{"$ref": "#/$defs/Maybe"}]}`, with `Maybe` an `anyOf` of `Item` and `null`: accepted, as `Xs []Maybe`,
    where `Maybe` has a `MaybeVariant2` for `null`;
  * items `{}`: accepted, as `Xs []json.RawMessage`.

  In both, a `null` item decodes, yet it is dropped. The control, items `{"type": ["string","null"], "enum": ["a", null]}`, is
  refused "L.properties.xs: x-deserialize-skip-invalid-items on items that admit null, which it would drop".
* **Whitespace is not a gap.** A scratch program decoded `[ null , "a" ]`, `[\n  null,\n  "a"\n]` and `[\tnull\t]` into
  `[]json.RawMessage`, by default and with `GOEXPERIMENT=nojsonv2`. Each `null` item came out as the four bytes `null`, which
  `string(it) == "null"` matches.
* **Committed schemas:** not reached. No skip-invalid array has such items
  ([0049-PLAN-null-items-in-skip-invalid-arrays.md](0049-PLAN-null-items-in-skip-invalid-arrays.md), "Facts it rests on").
* **Fix:** `admitsNull` also returns true in three cases:
  * an `allOf` whose members all admit `null`;
  * an `enum` that holds `null`, or a `const` of `null`;
  * an item with no `type`, `$ref`, `const`, `enum`, `anyOf`, `oneOf` or `allOf`.

### F-P: the subpackages

*Done 2026-11-04:* P1 to P9 are fixed in `v1.33.0` by [0049-PLAN-subpackage-fixes.md](0049-PLAN-subpackage-fixes.md),
each with the test its phase 6 table names, on the answers Q12 to Q14 (a) and Q15 (c), and `TestInProcessServeAgentStep2`'s
margin is widened, under its deviations: phase 5's, the back-pressure tests' keepalive; phase 6's, the FIFO tests'
`unix.Mkfifo`.

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
`GOARCH=arm go build` succeed, so the code ships there. Fix: compare in `uint64`, and clamp. *Probe 2026-11-04,* on
linux/386: the panics, under "Re-assessment at `277aca0`".

**P6. Low. Over WebSocket, one message over `MaxMessageSize` ends the connection.** `remote/handler.go:217`, `ws.go:39`
(`SetReadLimit`). Streamable HTTP refuses only that POST (`413`), and stdio answers `-32600` and reads on. **Probe**
(`TestProbeWSLimit`, re-run, a demonstration): a 101-byte frame under a 100-byte limit closes with "StatusMessageTooBig"; the next
frame fails "use of closed network connection". Fix: Q6.

**P7. Low. `Transcript.Close` races with a tap that `Wait` left running.** `agentproc/transcript.go:52-58` sets `lineTap.held`
without a lock (`linetap.go:54`) while the tap goroutine is still in `lineTap.Write` (`linetap.go:41`). **Probe**
(`TestProbeTranscriptCloseAfterAbandon`, re-run, `-race -count=3`): "WARNING: DATA RACE" in 2 of 3 runs, the read at `linetap.go:41`,
the previous write at `linetap.go:54` from `Transcript.Close`. Fix: guard `held` with the Transcript's mutex.

**P8. Low. Over Streamable HTTP, `Conn.Read` ends the agent's clean end with an error wrapping `io.EOF`, not `io.EOF`.**
`remote/httpclient.go:262-270`: the connection's stream ends with `c.fail(fmt.Errorf("remote: the connection's stream ended:
%w", err))`. `io.EOF`'s doc: "Read must return EOF itself, not an error wrapping EOF, because callers will test for EOF using
==", and `Conn.Read`'s doc says it "returns io.EOF after the agent's clean end" (`remote/client.go:49-51`). **Probe**
(`TestProbeConnReadEOF`, 2026-11-04, `-count=3`): a serve that answers `initialize`, reads one notification and returns;
reading to the end as `io.ReadAll` does, over WebSocket "<nil>", over Streamable HTTP "remote: the connection's stream ended:
EOF", `errors.Is(err, io.EOF)` true. So `io.ReadAll(conn)` fails at a clean end. The SDK's own connection reads it with
`errors.Is`, so its end is still clean. Fix: return `io.EOF` itself when the stream ends at its end.

**P9. Low. An agent's message over `MaxMessageSize` is dropped, and the request it answers gets no answer.**
`remote/ws.go:207-224` (`pump`, both profiles: `httpserver.go:354-363` uses it for Streamable HTTP) logs "a message from the
agent exceeds MaxMessageSize and was dropped" and writes nothing else. **Probe** (`TestProbeOverlongAnswerDropped`,
2026-11-04): a serve answers request 1 with a 444-byte line under a 200-byte limit, then sends a notification; the client
receives the notification alone, "request 1 got no answer of any kind". On stdio the reader of an over-long line answers its
sender `-32600` with `"id": null` (`connection.go:1597-1601`), so the agent at least learns of it. Fix: Q14.

### F-T: tooling, CI and documentation

*Done 2026-11-04:* T1 to T14 are fixed by [0049-PLAN-tooling-fixes.md](0049-PLAN-tooling-fixes.md), with no release of
their own, each with what its phase 6 table names, on the answers Q16 to Q19 (a): T7 and T10 are comments, read, under
its deviation 2, and T8 was corrected in `f6bfdcf`.

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
2026-11-03). **Probe** (investigator): "mdsh not found", `Error 1`. *Corrected 2026-11-04* in `RELEASING.md`'s
prerequisites ("Re-assessment at `277aca0`").

**T9. Low. macOS's `/usr/bin/make`, 3.81, computes an empty schema version silently.** `Makefile:12`, `:37` (corrected
2026-11-03); `RELEASING.md:10` documents 4.2. **Probe** (investigator): make 3.81 gives `V=[]` and would fetch `…/schema-v/meta.json`. Fix: a version guard.

**T10. Low. The docs name the wrong packages for `make api`.** `RELEASING.md:98-104`, `docs/architecture.md:751-760`
(corrected 2026-11-03), `Makefile:106` against `scripts/check_api.py:49`, `:54` (gated: root, `agentproc`, `acptest`, `clienthost`; reported: `unstable`,
`remote`). *Corrected 2026-11-04* in `RELEASING.md` and `docs/architecture.md`, which still named three gated packages and
one reported, the citation having been corrected on 2026-11-03 and not the text; `Makefile:106` is the tooling PLAN's.

**T11. Low. `release_body.py` hides why the identifier scan failed.** `tools/release_body.py:77-81` prints idscan's stdout only.
**Probe** (investigator): with no deny list, exit 1 with no reason.

**T12. Low. `.golangci.yml` has dead exclusions:** `:116` (`//sys`, which no file has), `:126` (`godot`, not enabled), `:131` and
`:144` (`examples$`; the directory is `example`).

**T13. Low. Small doc gaps:** `docs/architecture.md`'s tree omits `scripts/push_guard.py`, `testdata/elicitation/` and
`testdata/encodings.golden`; `make help` omits `check-records`, `check-tools`, `markdownlint`, `disclosure-check`, `version`,
`release` and `clean`; `tools/platforms.py` has no test. *The tree corrected 2026-11-04,* with `version` and
`scripts/test_check_*.py` too; `make help` and `platforms.py`'s test are the tooling PLAN's.

**T14. Low. `ci_jobs.py tally` counts every workflow's runs as CI's.** `tools/ci_jobs.py:183-191` lists main's runs with no
workflow filter. **Probe** (2026-11-04): of main's last 40 runs, 26 are the Docs workflow's and 14 CI's, so a tally of "CI's
job history" is mostly the docs gates'. Fix: list the CI workflow's runs, as T2's fix does.

### F-C: coverage and dead code

*Done 2026-11-04,* by [0049-PLAN-tooling-fixes.md](0049-PLAN-tooling-fixes.md): `abandon` and the two `Error` methods are
run by tests, and `ConfigOptionUpdate.gates` is no longer generated, with `accept-gates-unreachable`'s `allOf` route under
its phase 6's deviation 1. G13 and `cmd/generate`'s two `main` functions stay the generator PLAN's.

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
* The generator refuses what it cannot render faithfully, rather than exiting 0 (G1 to G9; G15 to G37 add to it,
  2026-11-07).
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
  exported API. *Complete 2026-11-03,* released in `v1.31.2`. *R11, added 2026-11-04, is open:* an amendment of that PLAN
  examines it, on the owner's approval. *Amended 2026-11-05:* its phases 5 and 6 fix R11 per Q20 (a), released as `v1.33.1`. *Complete 2026-11-05,* released in `v1.33.1`.
* `0049-PLAN-surface-fixes.md`: S1, S2, S6 per Q4, S7, S8, S9, S5, S4, and S3 per Q3. The new gates and stricter validation are
  behaviour changes, released as a minor version with notes that lead with them. *Written 2026-11-03,*
  [0049-PLAN-surface-fixes.md](0049-PLAN-surface-fixes.md). It groups the order into phases by mechanism: S1 with S7,
  S2, S6, then the gates S8, S9, S5 and S3, then S4. *Complete 2026-11-04,* released in `v1.32.0`.
* `0049-PLAN-subpackage-fixes.md`: P1, P3, P4, P7, P5, P2 per Q5, P6 per Q6, and `TestInProcessServeAgentStep2`'s margin.
  *Written 2026-11-04,* [0049-PLAN-subpackage-fixes.md](0049-PLAN-subpackage-fixes.md), with P8 and P9 of the second
  re-assessment and Q12 to Q15 below. It groups the order into phases by package: P1; P3, P4 and P5, one function; P7 and
  the margin; P2; P6, P9 and P8. *Complete 2026-11-04,* released in `v1.33.0`.
* `0049-PLAN-generator-fixes.md`: G2, G1, G3, then G4 to G12, each refusal with an engine case; G13. *Amended 2026-11-07,*
  by the fourth re-assessment: G15 to G36 join it. G14, which changes the generated code and so the root package's
  behaviour, is fixed alone per Q21 (a), in `v1.34.1`, by
  [0049-PLAN-null-items-in-skip-invalid-arrays.md](0049-PLAN-null-items-in-skip-invalid-arrays.md). G15 to G18 are medium and go before the low findings, then G19 to G29, then the info findings G30
  to G36. Each refusal comes with an engine case, and each changed behaviour with a test seen to fail first.
  *Written 2026-11-07,* [0049-PLAN-generator-fixes.md](0049-PLAN-generator-fixes.md), proposed, after the fifth
  re-assessment, with G37 and Q22 to Q29 below. Its ten phases keep this order:
  * the mediums, G16 and G1, then G2, G15, G17, G3 and G18;
  * the output edge's lows, G9, G27, G28 and G10;
  * the model's refusals, G4, G5, G7, G8, G32 and G37, then types and decoding, G6, G21, G22, G23 and G24;
  * validation, gates and rules, G19, G20, G25 and G26;
  * the input edge and the CLI, G11, G33 and G31;
  * the table's tests and the fuzzer, G29, G35, G36 and G12;
  * the info findings, G13, G30 and G34;
  * the close.
  *Done 2026-11-07,* phases 1 to 9, `5e0761b` to `e31707c`, under the deviations the PLAN records, with the MADR's
  G3, G4 and G20 amended and G26's probe corrected; the patch `v1.34.2` follows, per Q29 (b).
* `0049-PLAN-tooling-fixes.md`: T2 (with 0048), T1, T3, then T4 to T13, and F-C's dead `ConfigOptionUpdate.gates` through the
  generator. *Written 2026-11-04,* [0049-PLAN-tooling-fixes.md](0049-PLAN-tooling-fixes.md), proposed, with T14, F-C's two
  untested `Error` methods and `abandon`, and Q16 to Q19 below. It groups the order into phases by tool: T2, T14 and T7;
  T1 and T3; T4, T11, T5 and T6; T9, T10, T12 and T13; F-C. *Complete 2026-11-04,* with no release of its own, under
  three deviations of its close: the `allOf` engine case, T12's test, and R11 recorded. Its fixes shipped in `v1.33.1`.

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
`accepted`, and [0049-PLAN-runtime-fixes.md](0049-PLAN-runtime-fixes.md) is in progress (*complete 2026-11-03,*
released in `v1.31.2`). 0046's amendment of 2026-11-02 records
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
  * *Amended 2026-11-03,* in [0049-PLAN-surface-fixes.md](0049-PLAN-surface-fixes.md)'s phase 5, deviation 5. S4 named
    the copies the timelines return. `Apply` also keeps what it takes from an update shallowly (`tooltimeline.go:70-71,92,95`,
    `compaction.go:63,76,82` at `6ad8759`), so a caller that changes its update afterwards, through a pointer, changes the
    timeline: "tool call timeline state changed through the applied update", and the same for compactions
    (`TestProbeTimelinesIngestAliases`). The deep copy applies on the way in too, so a timeline shares nothing with what it
    is given or what it returns. Asked with two options, the owner chose this one. Not chosen: documenting that a caller
    must not change an update after `Apply`, and recording the aliasing as a new finding.
* Q10 (a): a new sentinel, `ErrUnadvertisedAuthMethod`. Not chosen: a `*CapabilityError`; a widened `ErrTerminalAuthMethod`.
* Q11 (a): a protocol-table rule, `EmptyClears`. Not chosen: `omitzero` on every nullable array; `Patch[[]T]`.

[0049-PLAN-surface-fixes.md](0049-PLAN-surface-fixes.md) is written on these answers. It stays `proposed` until the owner
approves it. (*2026-11-03:* the owner committed it as `df1eb2c` and said "proceed"; it is complete, released in `v1.32.0`.)

#### The subpackage PLAN's designs, proposed 2026-11-04

[0049-PLAN-subpackage-fixes.md](0049-PLAN-subpackage-fixes.md) needs four answers Q5 and Q6 do not settle. Each is put with
three options, the recommended one first. The PLAN is written on the recommended answers and changes if the owner chooses
otherwise.

* **Q12. P1, what keeps a WebSocket alive under back-pressure.** The cause is one fact of the library: "You must always read
  from the connection. Otherwise control frames will not be handled", and `Ping` "does not read from the connection but
  instead waits for a Reader call to read the pong" (coder/websocket `v1.8.15`, `conn.go:31-32`, `:218-220`). A frame is read
  in order, so a ping behind a frame nobody takes is not read either. The server's reader waits in the agent's pipe
  (`remote/ws.go:155`), so the server reads no pong while its agent is busy; the client's waits at 256 lines
  (`ws.go:79`, `frame.go:132`), so the client answers no ping while its application is slow.
  * **(a) The client's reader never waits on its application, up to a byte bound; the server does not count a pong missed
    while its own reader waits on the agent** (recommended).
    * The client's `lineQueue` is bounded in bytes, not lines: a new `DialOptions.MaxBuffered`, zero 64 MiB, as
      `HandlerOptions.MaxBuffered` is for Streamable HTTP. Below it the reader takes every frame, so pings are answered.
    * The server marks the time its reader waits to hand a frame to the agent. A ping that fails while it waited is not
      counted; the next is sent.
    * The server holds what it holds now, one frame, in its pipe; the client holds up to its bound. A client past the bound
      stops reading, and is disconnected after a missed pong, as Streamable HTTP ends with `ErrOverflow` past its own.
    * One option added, in `remote`, which `make api` reports and does not gate.
  * **(b) Both readers never wait, each up to `MaxBuffered`.** The server queues a client's frames for the agent up to
    `HandlerOptions.MaxBuffered`, too, and its keepalive is unchanged. One rule on both sides; but each connection may hold up
    to 64 MiB of a client's messages in the server, which a client chooses to send, where it holds one frame now.
  * **(c) A missed pong never disconnects.** Pings are still sent, so intermediaries see traffic; a dead peer is found by a
    write that waits past `Grace`, or by TCP keep-alive, which `net` enables at 15 s idle, 15 s between probes and 9 probes
    (`go doc net.KeepAliveConfig`), so about 150 s. The smallest change; a dead, idle client's serve runs about two minutes
    longer, and `KeepAlive`'s doc and the README's change meaning.
* **Q13. P2, the Host rule of Q5 (a).**
  * **(a) `HandlerOptions.AllowedHosts`, patterns for the `Host` header's name, port ignored, matched as `OriginPatterns`
    are; on a loopback listener, when it is nil, an IP literal or `localhost` (or a name under `.localhost`) passes and any
    other name is refused** (recommended).
    * The listener is read from `http.LocalAddrContextKey`, "the local address the connection arrived on" (`go doc
      net/http.LocalAddrContextKey`); a listener that is not loopback, or a request without the key, checks nothing unless
      `AllowedHosts` is set.
    * Every request is checked, the WebSocket upgrade included, before the origin rule, and refused `403`.
    * An IP literal passes because a rebinding needs a name: a page served from the IP is not cross-site to it.
  * **(b) The same, on every listener.** A server reached by a name must list it, whatever its address. A private-network
    address is covered too, which a rebinding also reaches; every deployment behind a domain name breaks until it is
    configured.
  * **(c) Check `Host` only on a request that carries `Origin`.** A program that sends no `Origin` is never refused. But a
    same-origin GET after a rebinding sends none: the Fetch standard ("append a request `Origin` header") adds `Origin` to
    a same-origin request only when its method is neither GET nor HEAD, so the event streams stay open to the page.
* **Q14. P6 and P9, a message over `MaxMessageSize`.** On stdio the side that reads it discards it, answers `-32600` with
  `"id": null`, and reads on (`connection.go:1597-1601`, `WithMaxMessageSize`'s doc).
  * **(a) Each transport reader does as stdio's: discard, answer `-32600` with `"id": null` and "message exceeds N bytes",
    read on** (recommended).
    * The server, for a client's frame, answers on the socket; the client `Conn`, for the server's frame, answers on the
      socket; the server's pump, for an agent's line (P9), answers on the agent's input, as the client's connection would.
    * The socket's own limit is disabled (`SetReadLimit(-1)`, "Set to -1 to disable", `read.go:96`), and each frame is read
      through `Reader` under the transport's bound, the rest drained without being held.
    * A POST over the limit keeps its `413`.
  * **(b) The server alone, for a client's frame,** as Q6 (a) reads literally. An over-long frame still closes the client's
    socket, and an agent's over-long answer is still dropped with no answer, each documented.
  * **(c) The frame is handed on, and the connection's own limit refuses it.** The fault reaches the observer as on stdio;
    but `MaxMessageSize` then bounds nothing the agent reads, and a frame crosses the pipe whole.
* **Q15. P5, how a 32-bit target stays tested.** No CI job, gate or host runs 32-bit code today (`tools/platforms.py:25`'s
  `VET` list and `ci.yml` name no 32-bit target).
  * **(a) A CI step that runs `clienthost` and `acptest`'s tests as linux/386 on the Ubuntu job,** besides the new test on
    every host (recommended). An amd64 runner runs a static 386 binary, as the Linux test host did. The cost: about a
    minute of CI per run, and `ci.yml`, which `make check-tools` checks.
  * **(b) A gate, `linux386`, in `tools/gates.py`,** running those tests on the Linux test host with
    `remote.py pkg --goarch 386`. No CI change; it runs only when a PLAN runs `--all`.
  * **(c) No 32-bit run.** The new test checks the arithmetic with values past `math.MaxInt32` on 64-bit hosts, where they
    cannot overflow; `vet` and `build` stay the only 32-bit checks.

##### Answered by the owner, 2026-11-04

Each question was put with its three options:

* Q12 (a), the recommended one: the client's reader bounded in bytes, `DialOptions.MaxBuffered`, and the server not counting
  a pong missed while its reader waits on the agent. Not chosen: both sides buffering; a missed pong never disconnecting.
* Q13 (a), the recommended one: `HandlerOptions.AllowedHosts`, the loopback default on a loopback listener only. Not chosen:
  the default on every listener; a check only when `Origin` is sent.
* Q14 (a), the recommended one: every transport reader discards an over-long message and answers `-32600` with `"id": null`,
  the agent's over-long answer included. Not chosen: the server alone; handing the frame to the connection.
* Q15 (c), not the recommended one: no 32-bit run. The new tests run on 64-bit hosts, where they check the arithmetic and
  cannot fail on the old code. Each is still seen to fail once, as the PLAN's rule requires: its plant runs as linux/386 on
  the Linux test host, by script, and the record quotes it. Not chosen: a CI step on linux/386; a `linux386` gate.

#### The tooling PLAN's designs, proposed 2026-11-04

[0049-PLAN-tooling-fixes.md](0049-PLAN-tooling-fixes.md) needs four answers the questions above did not settle. Each is put with
three options, the recommended one first. The PLAN is written on the recommended answers and changes if the owner chooses
otherwise.

* **Q16. T2, which run shows "CI on origin main's head".** The fix as stated, the CI workflow on `--branch main`, finds no run
  for a docs-only head, which every release commit is (the re-assessment at `1fe3dd0`).
  * **(a) The CI workflow's newest run on the head commit, from any ref; or, when the head has none, on its newest ancestor
    from which every later change is docs-safe** (recommended). The tag's run counts for a release commit, and a docs-only
    commit after it is judged by the code it did not change, by `check_ci_filters.py --docs-safe`'s own list, so the check
    holds whatever was pushed since. "CI on the tag" takes the same workflow filter. Cost: an ancestor walk and its tests.
  * **(b) The CI workflow's run on the head commit, from any ref, alone.** Simpler; it holds at a release, when main's head is
    the tag's commit, and fails once a docs-only commit follows the tag before `publication.py` runs.
  * **(c) Rename the check to what it reads:** "the workflow that ran on main's head passed", Docs or CI, and leave the Go to
    "CI on the tag". The smallest change; the check then says less, and T14 still needs its own filter.
* **Q17. T3, files git ignores.**
  * **(a) Both checks read what git tracks or would:** `check_records.py` lists files with `git ls-files -co
    --exclude-standard`, falling back to `rglob` outside a work tree, and `.markdownlint-cli2.jsonc` gains `"gitignore": true`
    (recommended). Probed: "Linting: 47 files … 0 issues" with the bad files present. Cost: the markdownlint file is the fleet's
    shared configuration, so the line goes upstream too, or a refresh drops it.
  * **(b) Name the directories:** `.cache`, `.worktrees`, `.tools` and `.gocache` in `SKIP_PARTS` and in markdownlint's
    ignores. No git call; it drifts from `.gitignore`, and still touches the fleet file.
  * **(c) `make markdownlint` passes markdownlint the files `git ls-files` lists,** leaving the fleet file alone, and
    `check_records.py` as (a). Cost: the Makefile must keep the fleet's MADR and PLAN exclusions itself, since files given on
    the command line bypass the configured globs.
* **Q18. T9, make 3.81.**
  * **(a) An order-only prerequisite on the targets that read `schema/version`:** `version`, `release` and the four schema
    downloads fail at once, "schema/version read as empty: GNU make 4.2 or newer is needed, this is 3.81" (recommended).
    Probed under 3.81: the guard fails, exit 2, and `make check-records` still passes there. The docs gates keep working
    under stock macOS make.
  * **(b) Read the stamp with `$(shell cat schema/version)`,** which 3.81 has. The cause goes; `.SHELLFLAGS` is still lost
    under 3.81, so a pipeline's failure can pass silently.
  * **(c) Refuse make 3.81 for every target,** by `MAKE_VERSION`. The surest, and it breaks `make check-records` and the other
    docs gates under the make macOS ships.
* **Q19. T2 and [0048-PLAN-create-releases-in-ci.md](0048-PLAN-create-releases-in-ci.md).** Both change `publication.py`'s CI
  checks (0048's D5 adds `--run-id`) and `release_body.py`'s scan (0048's D3 and T11).
  * **(a) This PLAN first; 0048's PLAN amended after to build on its `publication.py` and `release_body.py`** (recommended).
    T2 is medium and wrong at every release now; 0048 is `proposed` and not accepted.
  * **(b) 0048 first,** then this PLAN on top. 0048's `verify-release` would inherit T2's defect until this PLAN lands.
  * **(c) One PLAN:** this one takes 0048's three phases in. One review of the release tooling; a larger PLAN, and 0048's
    decision still needs the owner's acceptance first.

##### Answered by the owner, 2026-11-04

Each question was put with its three options; the owner chose the recommended one each time:

* Q16 (a): the CI workflow's run on main's head from any ref, or on its newest docs-safe ancestor. Not chosen: the head's run
  alone; renaming the check.
* Q17 (a): `git ls-files -co --exclude-standard` in `check_records.py`, and `"gitignore": true` in markdownlint's configuration.
  Not chosen: naming the directories; the Makefile passing the file list.
* Q18 (a): an order-only guard on the targets that read `schema/version`. Not chosen: `$(shell cat)`; refusing make 3.81 for
  every target.
* Q19 (a): this PLAN first, and 0048's PLAN amended after to build on it. Not chosen: 0048 first; one PLAN.

#### The generator re-assessment, proposed 2026-11-07

* **Q21. G14, a `null` item in a skip-invalid array.** It is the one finding of the fourth re-assessment that the committed
  schemas reach, and its fix changes `types_gen.go`.
  * **(a) A patch, `v1.34.1`, for G14 alone, ahead of the generator PLAN** (recommended). The fix is one emitted helper and a
    regeneration. It turns a refused message into the drop the schema asks for, and changes no exported API.
  * **(b) In the generator PLAN's own release.** One release for the generator's work. G14 stays live until that PLAN lands.
  * **(c) Document it, and leave the fix to the PLAN's last phase.** The least work now. A peer's `null` keeps failing the
    message until then.

##### Answered by the owner, 2026-11-07

* Q21 (a): "Q21, yes it will ship as a patch." G14 is fixed alone, in `v1.34.1`, by
  [0049-PLAN-null-items-in-skip-invalid-arrays.md](0049-PLAN-null-items-in-skip-invalid-arrays.md), ahead of the generator
  PLAN. Not chosen: in the generator PLAN's release; documented now and fixed last.

#### The generator PLAN's designs, proposed 2026-11-07

Where a finding's fix reads "refuse, or …", or leaves a detail open, [0049-PLAN-generator-fixes.md](0049-PLAN-generator-fixes.md)
needs the choice. The facts are the fifth re-assessment's.

* **Q22. G6, a non-zero scalar default on an optional field.** The committed schemas have one: `ElicitationSchema.type`,
  whose closed enumeration, `"object"` alone, excludes the zero value. The engine case `accept-default-scalar.txtar` accepts the
  shape on purpose, with no enumeration: optional `mode` `"auto"`, `n` `3` and `on` `true`, "filled when absent".
  * **(a) A pointer when the zero value is a valid value; the plain type when the property's closed enumeration or `const`
    excludes it** (recommended). The field then sends an explicit zero, as a nullable one does. `ElicitationSchema.Type`
    keeps its type, and nothing generated changes. `accept-default-scalar.txtar`'s three fields become pointers, so its
    `want.api` changes.
  * **(b) Refuse it, unless the zero value is excluded.** This is smaller. But `accept-default-scalar.txtar`'s three fields
    become refusals, and a schema with an optional `true` default is no longer generated.
  * **(c) A pointer always.** `ElicitationSchema.Type` would become `*ElicitationSchemaType`, an incompatible change to the
    exported API, which needs a `/v2`
    ([0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md](0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md)).
* **Q23. G7, a default below a nested object or an inline variant.** None is in the committed schemas.
  * **(a) Refuse it, naming the path** (recommended). It is what "refuse what it cannot render" asks, and nothing committed
    changes.
  * **(b) Apply it, as `objectFields` does.** This adds encode and decode paths that no committed schema uses.
  * **(c) Apply it in nested inline objects, and refuse it in inline variants.** A middle path, with two behaviours to
    document and test.
* **Q24. G17, a two-member `anyOf` or `oneOf`.**
  * **(a) Refuse a property's list of two non-null members. Type a `const` member from its own `type`, or from its JSON kind
    when it has none** (recommended). `ClientNotification.params` is unaffected: its own list has a `null` member, and its
    inner `anyOf` is a member, not a property's list.
  * **(b) Refuse both shapes**, the non-string `const` too.
  * **(c) Generate a property of two non-null members as `json.RawMessage`,** with the `const` typing of (a). It never
    refuses, but those fields lose their types.
* **Q25. G19, a union variant that is a named array.** `SessionConfigSelectOptions` has this shape. Its item types have no
  check yet.
  * **(a) Descend into the items of `InlineNamed` slice and map variants, in `Validate` and in the gates** (recommended). A
    check that a schema update adds to `SessionConfigSelectOption` is then enforced, not silently skipped, and the generated
    code does not change until one is added.
  * **(b) Refuse such a variant when its items validate or are gated.** This is smaller, but the first such schema update
    stops regeneration until the generator is changed.
  * **(c) Descend in `Validate`, and refuse in the gates.** The gates are rarer and reach the wire. It splits the work,
    with two rules for one shape.
* **Q26. G21, a discriminator written as a one-value `enum`.**
  * **(a) Read a one-value string `enum` as its `const`, in `discriminatorKey` and `discriminatorValue`** (recommended). The
    union then routes, and the change is two reads.
  * **(b) Refuse an `enum` on a discriminator.** A schema update written that way then stops regeneration.
  * **(c) Route every value of a discriminator's `enum`, one or many, to its variant.** The most general, with routing
    logic for a shape no schema uses.
* **Q27. G23, a required nullable collection.**
  * **(a) Do not fill it when it admits `null`** (recommended). A received `null` stays nil and is re-sent as `null`, and the
    field keeps its type.
  * **(b) Make it a pointer.** This changes a field's type for any such schema.
  * **(c) Refuse a required collection that admits `null`, naming the path.** The smallest, but such a schema update stops
    regeneration.
* **Q28. G20, a capability or key-order rule whose property the definition's own fields lack.**
  * **(a) Refuse the rule, naming it and the copies that hold the property** (recommended). ACP's table has none such, and a
    rule then never applies to nothing silently.
  * **(b) Resolve it by `f.prop` across `Fields`, `NestedStructs` and inline variants, as the patch rules do.** This is
    complete, but it needs more code, and no committed rule uses it.
  * **(c) Resolve capability rules across the copies, and refuse key-order rules.** The gates reach the wire, and key orders
    do not. It splits the rule kinds.
* **Q29. The release.** Each phase keeps the generated files byte-identical, which `tools/gates.py regen` checks.
  * **(a) No release of its own,** as with the tooling PLAN; the next release's notes name the PLAN (recommended). A phase
    that changes a generated file stops as a deviation.
  * **(b) A patch release at the close.** A tagged release for traceability, with nothing in it for a consumer.
  * **(c) A minor release at the close.** It signals the generator's new refusals to maintainers, but bumps the minor
    version with no change a consumer sees.

##### Answered by the owner, 2026-11-07

Each question was put with its three options, the recommended one first.

* Q22 (a): a pointer when the zero value is a valid value, and the plain type when the closed enumeration or `const`
  excludes it.
* Q23 (a): a default below a nested object or in an inline variant is refused, naming the path.
* Q24 (a): a property's list of two non-null members is refused, and a `const` member is typed from its own type.
* Q25 (a): `Validate` and the gates descend into the items of named array and map variants.
* Q26 (a): a one-value string `enum` on a discriminator is read as its `const`.
* Q27 (a): a required collection that admits `null` is not filled.
* Q28 (a): a capability or key-order rule whose property only copies hold is refused.
* Q29 (b): **a patch release at the close**, not the recommended "no release of its own". The generated files stay
  byte-identical, so the patch's notes state that consumers meet no change, and name the generator's.

### Consequences

* Good, because a deadlock, a crash, and four schema mismatches a peer can see are fixed, each with a test that would have caught
  it. *2026-11-04:* the deadlock and the mismatches are fixed, in `v1.31.2` and `v1.32.0`; the crash, P5, is the
  subpackage PLAN's, fixed in `v1.33.0`.
* Good, because a `null` item in a skip-invalid array is dropped as the schema says, not a cause to refuse the message
  (G14, added 2026-11-07).
* Good, because the generator refuses the schemas it now crashes on, renders wrongly or drops parts of (G2 to G8, and G15 to
  G37 since 2026-11-07), before a schema update meets one.
* Good, because the checks the records quote (`publication.py`, `check_records.py`, the docs-safe set) check what they claim.
* Bad, because the surface fixes refuse what the SDK sent before (an empty `stopReason`, an unadvertised `methodId`, an ungated
  boolean option): a consumer that relied on it fails at the call, which is why they go in a minor release with notes.
* Bad, because five PLANs are a long program; Q1 (c) is the shorter alternative.
* Bad, because nothing runs 32-bit code after the subpackage PLAN (Q15 c): a later change that converts a `uint32` back to
  `int` in `Files` or `MemoryClient` passes every check on a 64-bit host.
* Neutral, because none of the fixes needs an incompatible API change; Q5 (a) adds one option, and Q10 (a) one sentinel.
  *Amended 2026-11-03,* in [0049-PLAN-surface-fixes.md](0049-PLAN-surface-fixes.md)'s phase 3, deviation 3: S6's
  membership check gives 25 types that held a closed enumeration and checked nothing an exported `Validate`, compatible
  additions the owner accepted.
  *2026-11-04:* Q12 (a) adds a second option in `remote`, `DialOptions.MaxBuffered`. `make api` reports `remote`'s changes
  without gating them, and counts its compatible ones with the rest (`scripts/check_api.py:135-140`).

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
  [0043-MADR-remote-http-and-websocket-transport.md](0043-MADR-remote-http-and-websocket-transport.md) (P1, P2, P6, P8, P9),
  [0045-MADR-notification-queue-byte-budget.md](0045-MADR-notification-queue-byte-budget.md) (R5),
  [0046-MADR-sending-and-receiving-middleware.md](0046-MADR-sending-and-receiving-middleware.md) (R2, R8),
  [0047-MADR-pre-add-check-full-coverage.md](0047-MADR-pre-add-check-full-coverage.md) (T1),
  [0048-MADR-create-releases-in-ci.md](0048-MADR-create-releases-in-ci.md) (T2).
* Schema 1.26.0 stays deferred by the owner, and no finding concerns it.
