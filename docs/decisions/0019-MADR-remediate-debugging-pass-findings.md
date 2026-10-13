---
status: accepted
date: 2026-10-13
decision-makers: Repository owner
---

# Remediate the defects, gaps and idiom drift the 2026-10-13 debugging pass found, in phases ordered by severity

## Context and Problem Statement

The owner asked on 2026-10-13 for a debugging pass across the codebase: bugs, gaps, missing or incomplete wiring, and adherence to
idiomatic, modern Go 1.27.2. This record holds what the pass found and proposes how to remediate it. It decides nothing yet; every
finding below is evidence, and the owner's review settles the program and the open questions in "Decisions the owner is asked to make".

### How the pass was run

* **Commit:** `c77f25a` (the `v0.17.1` tag), on 2026-10-13, with `go1.27.2`.
* **Areas**, each examined separately: the runtime (`connection.go`); the SDK surface outside it (`agent_gen.go`, `client_gen.go`,
  `types_gen.go`, `helpers*.go`, `errors.go`, `extensions.go`) against `schema/`; the generator (`cmd/generate`); idioms and
  performance in both modules; and the examples, documentation, tests, CI and tooling.
* **Method:** reading; probe tests that fail when a defect is present, run in scratch copies of the tree, never in it; `go fix -diff`
  in both modules; `golangci-lint` v2.14.0 with a scratch configuration that adds `modernize`, `intrange`, `copyloopvar`, `perfsprint`,
  `usestdlibvars`, `exhaustive`, `errorlint`, `prealloc`, `contextcheck`, `sloglint`, `thelper`, `tparallel` and `paralleltest` and
  lints the generated files and the examples; a coverage profile; new benchmarks of the wire paths; running the example programs
  against each other; and probe schemas fed to the generator.
* **Evidence labels:** **Probe** means a probe test or probe schema reproduced the defect in a scratch copy. **Read** means the defect
  follows from the quoted code without a run. **Measured** means a benchmark in a scratch copy. Every finding at medium severity or
  above, and every finding a decision rests on, was re-read against the tree by the author of this record.
* **The tree was not changed.** `git status --porcelain` was empty after the pass. The probes, probe schemas and benchmarks live in
  the scratch copies and are not part of this record; the PLANs that follow turn the ones worth keeping into committed tests, each
  seen to fail before its fix.

### What holds

* `go test -race ./...` passes in both modules, `go vet` is clean in both, and `go mod tidy -diff` is clean in both.
* Regenerating gives output byte-identical to the committed files, and 30 in-process build-and-render runs gave identical bytes:
  every map that reaches output is sorted. A refused schema leaves the output directory unchanged, and files are staged and renamed
  as [0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md) phase 5 decided.
* Every method in `schema/meta.json` and `schema/meta.unstable.json` is wired on both sides, inbound and outbound, as a request or a
  notification matching the schema's unions. The 15 method gates and 18 capability rules read the right side's record in both
  directions.
* The constructors assign before reading starts
  ([0018-MADR-start-side-connections-after-assignment.md](0018-MADR-start-side-connections-after-assignment.md)); response matching,
  id canonicalisation, the notification barrier, shutdown, the size limit and the JSON-RPC classification match
  [0017-MADR-remediate-acp-v1-conformance-violations.md](0017-MADR-remediate-acp-v1-conformance-violations.md).
* The code is already largely modern: `errors.AsType` in `toReqErr`, `new(expr)` in `Ptr`, `atomic.Pointer` and `atomic.Uint64`,
  `context.WithCancelCause`, `for range n`, `t.Context()`, `t.Chdir`, `wg.Go`. `go fix -diff` proposes 6 changes at the root and 4 in
  the generator, none in generated files (which it skips).
* `encoding/json` v1 is the only JSON API used
  ([0007-MADR-self-contained-acp-sdk.md](0007-MADR-self-contained-acp-sdk.md) holds).

### Findings

Severity: **high** (a user of the SDK meets it in ordinary use), **medium** (wrong behaviour on a reachable path), **low** (wrong
behaviour on a narrow or misuse path), **gap** (behaviour the specification, the schema or the records call for, missing), **latent**
(the generator mishandles a schema shape the committed schema does not contain), **doc**, **test**, **ci**, **idiom**, **perf**.
"Fix in" names where the change lives: **gen** (the generator, then regenerate), **rt** (hand-written runtime files), **ex**
(examples), **docs**, **tool** (scripts, Makefile, CI, lint configuration).

#### Runtime and SDK behaviour

| Id | Severity | Finding | Evidence | Fix in |
| :--- | :--- | :--- | :--- | :--- |
| B1 | high | A request sent from inside a notification handler on the same connection waits for that handler to finish: a deadlock until its context ends, forever without a deadline. `handleResponse` takes `lastEnqueuedNotificationSeq` as the watermark (`connection.go:1088`), which counts the notification whose handler is running; `sendRequest` then waits for `completedNotificationSeq` to reach it (`:1222`), which happens only after that handler returns. A client whose `SessionUpdate` calls `ReadTextFile`, `CallExtension` or `SendRequest`, or an agent whose `Cancel` or extension notification handler sends a request, hangs. | Probe: `CallExtension` from `SessionUpdate` returned after 2.001 s with "context deadline exceeded", against a peer that answers at once. Read: the watermark and the wait. | rt |
| B2 | high | An `Agent` has no race-free way to reach its `AgentSideConnection`, and the examples and the README teach the racy way: build with `NewAgentSideConnection`, then `SetAgentConnection(asc)` (`example/agent/main.go:419-421`, `example_agent_test.go:126-129`). Reading starts inside the constructor, so a handler that reads the field races the assignment; `NewConnection`'s own documentation now forbids exactly this pattern (0018, outcome 2). `example/agent/main.go:153` names an `acp.AgentConnAware` interface that does not exist. The same holds for a `Client` and `ClientSideConnection`. | Probe: the `Example_agent` pattern with input already waiting, under `-race`: `DATA RACE`, read in `Initialize` via `agent_gen.go:218`, write in `SetAgentConnection`. Read: no such interface in the tree. | rt, gen, ex, docs |
| B3 | medium | A request whose `params` member is omitted is refused `-32602` "unexpected end of JSON input", even when every property of its params is optional (`logout`, `session/list`). JSON-RPC 2.0 says params "MAY be omitted". Every dispatch case decodes `json.Unmarshal(params, &p)` with `params` nil. | Probe: `{"jsonrpc":"2.0","id":1,"method":"logout"}` answered `-32602`; no handler ran. | gen or rt |
| B4 | medium | `session/close` is not treated as `session/cancel`. The schema's `CloseSessionRequest`: "the agent **must** cancel any ongoing work related to the session (treat it as if `session/cancel` was called)". The agent's close case (`agent_gen.go:451-463`) cancels no prompt turn, and `ClientSideConnection.CloseSession` does not cancel the session's pending permission requests as `Cancel` does. | Probe: the turn was answered `end_turn` after `session/close`; a pending permission request stayed open after `CloseSession`. Read: `schema/schema.json:5299`. | gen |
| B5 | medium | A cancel can miss a prompt received before it when two prompts for one session are in flight. A cancel that finds a stored turn cancels it and records nothing (`agent_gen.go:438-440`), so a prompt received earlier but stored later runs uncancelled; and a kept late cancel is consumed by the first of two unstored prompts (`:565`). This contradicts the README ("A prompt and a cancel are ordered as they were received") and the 0017 MADR's amendment. | Probe: two cases, each with turn 2 (receipt 2, before the cancel's receipt 3) not cancelled. Read: the cancel and prompt cases. | gen |
| B6 | gap | `session/fork`'s MCP servers are not gated on `mcpCapabilities`, though the README says an `http`, `sse` or `acp` server needs them. In the unstable schema the field is `[]UnstableMcpServer`, and the three rules name only `McpServer` (`cmd/generate/internal/model/acp.go:109-112`), so `UnstableForkSessionRequest.gates` checks only `additionalDirectories` (`types_gen.go:20170-20179`). | Probe: `session/new` with an http server refused, `session/fork` with the same server sent and handled. Read: the gates method. | gen |
| B7 | low | `lateCancels` grows without bound. A `session/cancel` that finds no running turn records the session's receipt; only a later `session/prompt` for that session removes it (`agent_gen.go:442-444`, `:565`). The SDK's own client produces this on every abandoned `Prompt` (the `$/cancel_request` ends the turn first), and a peer can grow it at will. | Probe: 1000 entries after 1000 cancels, unchanged after `session/close` of each. | gen |
| B8 | low | A handler that returns a typed-nil `*RequestError` in a non-nil `error` is answered as success with `"result": null`, skipping the result's validation and gates: `errors.AsType` matches the nil pointer and `toReqErr` returns it (`errors.go:199`). | Probe: `session/new` handler, answered `null`. | rt |
| B9 | low | `SendRequest`, `SendRequestNoResult` and the typed methods write the request even when the context has already ended, then send `$/cancel_request` for it (`connection.go:1200-1218`, `:1464-1478`); the peer may run a side-effecting method the caller had abandoned. `SendNotification` checks first. | Probe: both lines on the wire, the caller given `-32800`. | rt |
| B10 | low | Params that marshal to JSON `null` (a nil map, a nil pointer, `json.RawMessage("null")`) are sent as `"params":null`, which this SDK's own receiver answers `-32600` (`connection.go:1255-1264`, `:1587-1596`, against `requestFault`). Reachable through the raw API and the extension methods only. | Probe: `CallExtension(ctx, "_x", map[string]any(nil))` answered `-32600`. | rt |
| B11 | low | Two in-flight inbound requests with the same id: when the first finishes, its deferred `delete(c.inflight, idKey)` removes the second's entry, so `$/cancel_request` no longer reaches it (`connection.go:898-910`). The peer breaks JSON-RPC by reusing the id. | Probe: the second request not cancelled. | rt |
| B12 | low | The dispatch does not know whether a method is a request or a notification. A request-only method sent without an id (`session/prompt`) runs on the single notification goroutine, its answer discarded, and blocks every later notification, the `session/cancel` that would end it included; a notification-only method sent with an id is answered `"result": null`. | Probe: the prompt sent as a notification held the queue 1.5 s; `session/cancel` with id 9 answered `null`. | gen, rt |
| B13 | low | `Negotiated()` returns a shallow copy whose pointer fields and slices are the live record's, and the record shares pointers with the application's own values (`agent_gen.go:82-88`, `:242-245`; `client_gen.go:76-82`, `:320-323`). Writing through the copy changes what the gates read, and can race with them. | Probe: a gate that passed refused after writing through the copy. | gen |
| B14 | low | Inbound `_`-prefixed messages bypass both inbound hooks and the `initialize` gate (`extensions.go:37-67`), while `WithInboundRewrite`'s documentation says it "sees every request, notification and response" (`connection.go:382-383`) and the README says a request before `initialize` is answered `-32601`. | Probe: the hooks saw `session/new` and not `_x/ping`. | rt, docs |
| B15 | low | Raw received messages are logged in full at Error level when malformed (`connection.go:849`, `:856`, `:866`, `:975`, `:999`, `"raw", string(line)`). A malformed message can carry prompt text or file contents, which then reach the application's logs. | Read. | rt |
| B16 | gap | Writes ignore the caller's context. `sendMessage` holds `writeMu` across a blocking `Write` (`connection.go:1170-1182`), so a peer that stops reading blocks `SendRequest` past its deadline and every other writer, answers included. | Probe: a 200 ms deadline still blocked after 1.5 s. | rt |
| B17 | gap | There is no local `Close`. The connection ends only when its reader ends; under `OverflowCloseConnection` it stops reading and marks itself done but keeps writing, and `SendNotification` still succeeds after `Done()`. An application has no SDK call that stops the goroutines. | Read: no `Close` and no `io.Closer` handling in the tree. | rt, gen |
| B18 | gap | Inbound requests are unbounded: each gets a goroutine and an `inflight` entry with no limit (`connection.go:903-914`), where notifications are bounded by `WithMaxQueuedNotifications`. | Read. | rt |
| B19 | idiom | Local failures come back as `*RequestError` and lose their Go identity: a local cancel is `-32800`, identical to a peer's; a deadline or a disconnect is `-32603`, identical to a peer's internal error. `RequestError` has no `Unwrap`, so `errors.Is(err, context.DeadlineExceeded)` is false. Existing tests pin the codes. | Read; probe output of B9. | rt |
| B20 | gap | An elicitation in an undeclared mode (the `Other` catch-all) passes the gate even with no elicitation advertised; ACP: "Agents MUST NOT request a mode the Client has not advertised". | Probe: mode `telepathy` sent with nothing advertised. | gen |
| B21 | doc | A turn whose connection ended is answered `cancelled`; the 0017 MADR's outcome text excludes "the connection's end". No peer receives that answer. | Probe; Read: the 0017 MADR, line 282. | docs |

#### Generator

All are latent except G2 and G3, which already shape `UnstableMcpError`.

| Id | Severity | Finding | Evidence | Fix in |
| :--- | :--- | :--- | :--- | :--- |
| G1 | medium (latent) | A description line that starts with `//` or `/*` is emitted unchanged (`cmd/generate/internal/emit/ast.go:228-229`), so a schema description can inject code or a `//go:generate` or `//line` directive into the generated files. No emitter needs the pass-through. | Probe: a description carrying `/* x */ func Injected() int { return 42 }` and a `//go:generate` line; the generated package compiled and `Injected()` ran. | gen |
| G2 | gap | `additionalProperties` on an object that has properties is ignored, so extra keys are dropped. Live: the unstable `McpError` has `"additionalProperties": true` (`schema/schema.unstable.json:8995`); `UnstableMcpError` keeps no extra keys. | Probe: `{"code":1,"vendor":"x"}` round-trips as `{"code":1}`. | gen |
| G3 | gap | `UnstableMcpError.Data` turns an explicit `null` into absent (`types_gen.go:16778`), while the schema says "explicit null is retained" (`schema/schema.unstable.json:8991`). The null-is-absent rule of [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md) rests on ACP v1's rule for `rawInput` and `rawOutput`. | Read. | gen, docs |
| G4 | latent | Non-string `enum` values become a Go `string` type, so schema-valid input fails to decode; a `null` value would become the constant `"<nil>"`. | Probe: `{"type":"integer","enum":[1,2]}` became `type Level string`; `{"level":1}` failed. | gen |
| G5 | latent | A type list with two non-null types keeps one (two functions pick different ones) and a required field becomes a pointer. | Probe: `["string","integer"]` became `*int64`. | gen |
| G6 | latent | An object definition without `"type":"object"` becomes `json.RawMessage`; with flattened members it is a struct but loses `Validate` and gates, because three checks re-read `type`. | Probe: two schemas. | gen |
| G7 | latent | A named array definition becomes `json.RawMessage`, accepting any JSON. | Probe. | gen |
| G8 | latent | A union variant with no title, `$ref` or object shape is dropped silently (`model.go:811-815`), so its values fail to decode. | Probe: `anyOf: [{"type":"string"}, …]`, a string refused. | gen |
| G9 | latent | A property key a struct tag cannot carry (a comma, a quote) produces the wrong wire key. | Probe: `"a,omitempty"` encoded as `"a"`. | gen |
| G10 | latent | A schema `default` is never checked against its field's type, and the generated code discards the decode error (24 committed call sites, all with valid defaults today). | Probe: `uint8` with `default: 300` accepted, decoded as 0. | gen |
| G11 | latent | A `required` key the object does not declare is not enforced. | Probe. | gen |
| G12 | latent | A fractional `minimum` or `maximum` on an integer is truncated, not rounded inward. | Probe: `minimum: 1.5` accepted `1`. | gen |
| G13 | latent | The loader accepts schema keywords in any letter case (`encoding/json` matches fields case-insensitively), so `"Required"` is honoured instead of refused. A test guards the committed schema only. | Probe. | gen |
| G14 | idiom | Non-string discriminator constants and a property that collides with a generated method name (`Validate`) are refused only incidentally, by an unrelated check or by the type checker, without naming the definition. | Probe: three schemas. | gen |
| G15 | gap | `output.Check` does not type-check `schema_gen_test.go` against the hand-written test files, so a name collision passes the generator and fails `go vet`. | Probe. | gen |
| G16 | low | `os.MkdirAll` in `cmd/generate/main.go:65` never takes effect (the check before it fails on a missing directory), and that failure reads as hundreds of type errors. | Probe. | gen |
| G17 | idiom | Dead code and stale text: unused constants in `emit/constants.go:10-15`; `Facts.Format`, `Facts.Values` and `Facts.AbsolutePath` read only by tests, with a half-stale doc; `util/util.go:32` still names Jennifer; an impossible `resp == nil` in `model/methods.go:184`. | Read. | gen |
| G18 | latent | A definition linked by `x-method` and `x-side` to a method no meta file lists is ignored silently. | Probe. | gen |
| G19 | test | Error sites no test runs: `model/gates.go:108`, `:149`, `:189`; `model/protocol.go:505`; `output/check.go:40`, `:48`, `:56`; `output/write.go:89`; and four probably unreachable defensive sites. 0011's D6 requires each to be run by a test or named with its reason. | Coverage profile. | gen |

#### Examples, documentation, tests, CI and tooling

| Id | Severity | Finding | Evidence | Fix in |
| :--- | :--- | :--- | :--- | :--- |
| E1 | medium | The permission prompt of `example/client`, `example/claude-code` and `example/gemini` spins forever at stdin EOF: `line, _ := reader.ReadString('\n')`, then `continue` on an empty line (`example/client/main.go:32-47` and the two copies). | Probe: 5,961,377 prompts printed in 10 s with stdin `/dev/null`. | ex |
| E2 | medium | In the REPL examples `:cancel` cannot cancel a running turn (one goroutine both blocks in `Prompt` and reads stdin), and the permission handler's second reader on stdin loses lines the scanner buffered. | Probe: the turn ran to its end; a piped "1" was swallowed, then the prompt spun. | ex |
| E3 | low | The example agent ignores Ctrl-C in stdio mode (it waits only on `<-asc.Done()`, `example/agent/main.go:424`) and lists `os.Kill`, which cannot be caught (`:397`). | Probe: still running 3 s after SIGINT. | ex |
| E4 | low | The example agent sends POSIX literal paths (`/project/README.md`), which a Windows receiver refuses as not absolute, so the example pair cannot complete a turn on Windows; CI only builds the examples. | Read. | ex |
| E5 | low | `example/claude-code/main.go:177` launches `@zed-industries/claude-code-acp`, which npm reports renamed to `@agentclientprotocol/claude-agent-acp`. Whether gemini's `--experimental-acp` flag is current was not checked. | `npm view … deprecated`. | ex |
| E6 | gap | The example agent ignores the context the SDK gives `Prompt` and keeps its own cancel map (`example/agent/main.go:181-209`), teaching the pattern 0017 made unnecessary; its turn does not stop on disconnect. A permission outcome of `cancelled` ends the turn `end_turn`. | Read; behaviour checked with a raw driver. | ex |
| E7 | gap | The example clients advertise `terminal: true` with stub methods, and the fs example writes any absolute path the agent names without asking. Under the v0.17 gates, advertising tells the agent it may call. | Read. | ex |
| E8 | test | The three `Example` functions only compile: no `// Output:`; they are `package acp`, drop every error, and call the test helper `testAbsPath`, which a reader of the documentation cannot call; `Example_gemini` never stops its child. | Read. | ex |
| E9 | test | Documented behaviour with no test: `SendRequestNoResult` (0%), `(*AgentSideConnection).NotifyExtension` (0%), the awaited branch of `answerUnlessCancelled`, context cancellation inside `waitNotificationsUpTo` (54.8%), malformed `$/cancel_request` params (64.7%), `handleInbound` with a nil handler or a result that fails to marshal, and the id-canonicalisation fallbacks. Root coverage is 88.8%. | Coverage profile. | rt |
| E10 | test | Fixed sleeps before assertions (`acp_test.go:766`, `:1066`) can flake on a slow `-race` runner; `testing/synctest` makes them deterministic (shown in a scratch copy: 20 of 20 under `-race`, in no wall time). `TestConformanceConstructors` skips four variants on every run (`conformance_test.go:1126`). | Measured; Read. | rt |
| E11 | ci | `scripts/check_records.py` does not enforce what `AGENTS.md` says it validates: a PLAN without a MADR, a record in the wrong directory and a reused number only warn; a misnamed record is ignored; a link to a missing anchor passes; and `--next` returns the lowest gap (`next_number`, `:77-82`), which contradicts "never reuse a number". The script is copied from the workspace template, so the fix starts there. | Probe in a scratch copy. | tool |
| E12 | ci | The examples are excluded from lint (`.golangci.yml`, `^example/`) and only built in CI. Linted, they give 55 issues, the dropped error behind E1 among them; no step runs the example pair, which would have caught E1, E3 and E4. | Probe: the scratch lint run. | tool, ex |
| E13 | doc | `make release` ends with "then tag v$(VERSION)", where `VERSION` is the schema number, so it suggests tagging `v1.25.0`; RELEASING.md says the module tag is not the schema tag (`Makefile:177`, usage hint at `:167`). | Read. | tool |
| E14 | doc | `docs/architecture.md` says "an object with `result` or `error` is a response"; the code also requires no `method` and treats an id-only object matching a pending call as a malformed response (`connection.go:860`). README's "Running the examples" says the client connects to a running agent; with no arguments it starts one, from a source checkout only, and it ignores `StopReason`. | Read. | docs |
| E15 | ci | `scripts/go-precheck.sh` reports govulncheck in its success line when it was skipped, by `GO_PRECHECK_SKIP_VULN=1` or by an offline heuristic that matches any output containing "proxy" or "timeout". | Read. | tool |
| E16 | ci | The lint configuration enables none of the modernizing linters (`modernize`, `intrange`, `perfsprint`, `usestdlibvars`, `errorlint` on generated code) and excludes generated files, so the drift in the idiom findings is invisible to `make pre-add-check`. CI pins govulncheck v1.8.0; v1.9.0 is published. | Probe: the scratch lint run. | tool |

#### Idioms and performance

Benchmarks on the wire paths (no benchmark existed): a client-to-agent prompt round trip over `io.Pipe` costs 30.6 µs and 145
allocations; `SessionUpdate` send 4.59 µs and 42; its receive 8.3 µs and 58. The round trip's CPU goes mostly to goroutine handoff,
so codec cost matters most on the streaming `session/update` path. Every API named below was checked with `go doc` on `go1.27.2`.

| Id | Severity | Finding | Evidence | Fix in |
| :--- | :--- | :--- | :--- | :--- |
| I1 | perf | Union `MarshalJSON` marshals the variant, decodes it into a map, sets the discriminator and marshals again (`types_gen.go:2590-2601`, 88 sites). Where the variant struct carries the discriminator as a string field, setting it and marshalling once gives the same bytes. | Measured: `SessionUpdate` send 4.59 µs and 42 allocations to about 1.8 µs and 14; a byte-identity test passed, under `nojsonv2` too. | gen |
| I2 | perf | `canonicalJSONRPCIDKey` builds a decoder for every id (`connection.go:521-557`), on every inbound request and response. A fast path for plain integers, the form this SDK sends, returns the same key. | Measured: 497 ns and 10 allocations to 14 ns and 1; agreed with the slow path on more than 6,000 inputs. | rt |
| I3 | perf | The outbound methods and handler results pass values into `validateOutbound` and `checkResult`, which box and copy them by reflection; pointers give the same bytes. `ptr.Interface().(validator)` can be `reflect.TypeAssert[validator](ptr)`. | Measured: `validateOutbound` 2 allocations to 0. | gen, rt |
| I4 | perf | `orDefault` allocates both its values on every call, absent fields included (`types_gen.go:19724-19735`). | Measured: `session/update` decode 58 to 48 allocations. | gen |
| I5 | perf | Allocations on every success: the disconnect error built eagerly in `waitForResponse` and `waitNotificationsUpTo` (`connection.go:1326`, `:1363`), and `fmt.Errorf` for the superseded cause on every prompt (`agent_gen.go:553`). | Measured. | rt, gen |
| I6 | perf | Union decoding parses the payload two to three times (the union's map, then `objectKeys` in the variant, then the shadow struct); `objectKeys` is 38.7% of decode allocations. Needs design against the fold-key and required-key rules. | Measured. | gen |
| I7 | perf | Seven union fallbacks decode the whole payload into `[]map[string]json.RawMessage` and discard it (`types_gen.go:374-376`). | Read. | gen |
| I8 | perf | Seven identity `MarshalJSON` methods only copy into an alias (`types_gen.go:113` and six more); removing them changes those types' method sets. | Read. | gen |
| I9 | idiom | `errors.As` where `errors.AsType` fits: the three sites `go fix` names, `connection.go:1574-1575`, the 8 generated `err.(*json.UnmarshalTypeError)` assertions (not robust to wrapping), and 9 `err.(*acp.RequestError)` assertions in the examples, which miss wrapped errors such as `connection.go:1236`'s. | `go fix -diff`; Read. | rt, gen, ex |
| I10 | idiom | The best-effort cancel in `ClientSideConnection.Prompt` uses `context.Background()` (`client_gen.go:466-470`); `context.WithoutCancel(ctx)` keeps the caller's values. | Read. | gen |
| I11 | idiom | Hand-rolled wake goroutines with stop channels where `context.AfterFunc` fits (`connection.go:469-474`, `:1364-1382`, `:1425-1443`); dead nil-cause branches after `<-ctx.Done()` (`:1344-1351`, `:1401-1408`). | Read; `go doc context.AfterFunc`, `context.Cause`. | rt |
| I12 | idiom | Undocumented exported API: `type Agent` and `type Client`, the 40 typed outbound methods, the 147 `Validate` methods, `RequestIdNull`, `(*RequestError).Error`; the generator has the method docs (`mi.Doc`) and does not emit them. | Read. | gen, rt |
| I13 | idiom | Generated-code hygiene: `return []byte{}, _e` with underscore names (97 sites), `var a Alias; a = Alias(v)` (52), `resp, err := …; return resp, err` (27), receiver `a` in `handle` and `c` elsewhere, `else { if … }`. | Read. | gen |
| I14 | idiom | Hand-written cleanups: `fmt.Errorf` with no verbs (`extensions.go:25` and 20 in the generator), a redundant copy (`connection.go:1285`), a struct that duplicates `RequestError` (`errors.go:25-31`), parameter copies into goroutines that are not loop variables, a manual index loop (`connection.go:667`), and the `go fix` list (`strings.SplitSeq`, `strings.FieldsSeq`, `slices.Backward`, `maps.Copy`, `reflect.Type.Fields`, embedded-literal simplification). | `go fix -diff`; Read. | rt, gen |
| I15 | idiom | Test idioms: `wg.Go` and `t.Context()` where tests still use `wg.Add` and `context.Background()`, `slog.DiscardHandler`, missing `t.Helper()`, `strconv.Itoa` for `fmt.Sprintf("%d")`. | Read. | rt, gen |
| I16 | idiom | The mutation tool ignores Ctrl-C, skipping the cleanup of its work directory (`cmd/generate/internal/mutate/main.go:112`, `:144`, `:166`); `signal.NotifyContext` and `exec.CommandContext` fit. | Read. | gen |

### Decisions the owner is asked to make

Each has a recommended answer, used by the outcome below unless the owner chooses otherwise.

1. **How an `Agent` or `Client` receives its side connection (B2).** Recommended: an optional interface on the agent or client value
   (`SetAgentConnection(*AgentSideConnection)` / `SetClientConnection(*ClientSideConnection)`) that the generated constructor calls
   after assigning `conn` and before `start`, so the value has its connection before any handler can run. No exported signature
   changes, and the examples' existing setter becomes correct as written. Alternatives: an accessor from the handler's context, or a
   constructor that takes a factory.
2. **The notification-barrier deadlock (B1).** Recommended: `processNotifications` puts the running notification's sequence number in
   the handler's context, and a request sent under that context caps its barrier at the number before it. Alternative: document that
   notification handlers must not send requests on their connection.
3. **Error identity (B19).** Recommended: keep the codes and add `Unwrap` to a local-cause field of `RequestError`, so
   `errors.Is(err, context.Canceled)`, `context.DeadlineExceeded` and a new `ErrPeerDisconnected` hold.
4. **Close, write deadlines and request limits (B16, B17, B18).** Recommended: add `Close` to `Connection` and both side connections
   (cancel, close the reader and writer when they implement `io.Closer`, fail sends after `Done()`), use it on overflow; add
   `WithMaxConcurrentRequests`, unlimited by default; document that a stalled peer blocks writes, and revisit a writer goroutine only if
   `Close` does not suffice.
5. **The unstable `McpError` (G2, G3).** Recommended: keep extra keys where `additionalProperties` is `true`, and exempt from
   null-is-absent a field whose schema text says explicit null is retained, amending
   [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md).
6. **The undeclared elicitation mode (B20).** Recommended: refuse sending the `Other` variant; accept it on receipt.
7. **Extension messages and the hooks (B14).** Recommended: keep the bypass, as for outbound extension calls, and document it in the
   hook documentation and the README.
8. **Raw message logging (B15).** Recommended: log the fault, the size and the id, never the raw content.
9. **Identity `MarshalJSON` (I8).** Recommended: remove them in the minor release.
10. **The connection's end and `cancelled` (B21).** Recommended: amend the 0017 text to what the code does.
11. **The records script (E11).** Recommended: fix the workspace template's `check_records.py` first, then copy it, so the copy stays
    byte-identical to its template.
12. **Versions.** Recommended: phase 1 ships as `v0.17.2` (fixes, no exported change); phase 2 as `v0.18.0`, since it adds API
    and changes error identity; phases 3 to 5 ride the next release after each lands.

### Decided with the owner, 2026-10-13

The owner answered each question on 2026-10-13. Three answers differ from the recommendation, and are marked.

| Decision | Answer |
| :--- | :--- |
| Program | P1, five phases, as recommended. |
| 1. Connection hand-off (B2) | The optional setter, as recommended: the generated constructor calls `SetAgentConnection(*AgentSideConnection)` or `SetClientConnection(*ClientSideConnection)` on the value, when it has one, after assigning `conn` and before `start`. |
| 2. Barrier deadlock (B1) | The cap through the handler's context, as recommended. |
| 3. Error identity (B19) | Keep the codes and unwrap a local cause, with `ErrPeerDisconnected`, as recommended. |
| 4. Lifecycle (B16, B17, B18) | **Not the recommendation:** `Close`, `WithMaxConcurrentRequests` (unlimited by default) **and a dedicated writer goroutine**, so every send honours its context and `Done()`. B16 is fixed, not documented. |
| 5. `McpError` (G2, G3) | Keep extra keys where `additionalProperties` is `true`, and exempt from null-is-absent a field whose schema text says explicit null is retained, amending 0012, as recommended. |
| 6. Undeclared elicitation mode (B20) | Refuse sending `Other`, accept it on receipt, as recommended. |
| 7. Extension messages (B14) | **Not the recommendation:** run both inbound hooks on `_`-prefixed messages, with their raw params, **and gate them**: an extension request received before `initialize` is answered `-32601` and an extension notification dropped, like a typed one. This changes behaviour for a peer that sends extension requests before `initialize`. |
| 8. Raw logging (B15) | No raw content at any level: the fault, the size and the id, as recommended. |
| 9. Identity `MarshalJSON` (I8) | Removed, with the other exported changes, as recommended. |
| 10. Connection's end (B21) | Amend the 0017 text, as recommended. |
| 11. Records script (E11) | The workspace template first, then a byte-identical copy, as recommended. |
| 12. Versions | **Not the recommendation:** phases 1 and 2 ship together as `v0.18.0`; nothing is released after phase 1 alone. Phases 3 to 5 ride the release after each lands. |

### Amended while planning phase 1, 2026-10-13

13. **A received `"params": null`.** Research for the phase 1 PLAN found that the sources disagree. JSON-RPC 2.0 says params, if
    present, "MUST be provided as a Structured value", while ACP schema 1.25.0 declares `params` in each of its four envelopes
    (`AgentRequest`, `ClientRequest`, `AgentNotification`, `ClientNotification`) as the params union or `{"type":"null"}`, and requires
    only `id` and `method` (`schema/schema.json:242`, `:4625`). Since
    [0017-MADR-remediate-acp-v1-conformance-violations.md](0017-MADR-remediate-acp-v1-conformance-violations.md), J1, a received
    `"params": null` has been answered `-32600`. The owner decided on 2026-10-13: **a received `"params": null` is treated exactly as
    omitted params**, as ACP's schema allows, and the SDK still never sends it (B10). This amends 0017's J1 for that one case. It is
    in phase 1, with B3.

## Decision Drivers

* An SDK user must not meet a deadlock, a lost cancel or a refused valid request in ordinary use (B1 to B5).
* The specification's MUSTs and the records' promises hold: `session/close` cancels, the README's gate list is the gates.
* Each fix is small enough to review and has a test that fails before it, as every phase of
  [0017-PLAN-remediate-acp-v1-conformance-violations.md](0017-PLAN-remediate-acp-v1-conformance-violations.md) did.
* Exported changes are grouped into one minor release, with their decisions recorded first.
* The generator stays the single source of the generated files; no fix edits a `*_gen.go` file by hand.
* Idiom and performance work does not delay correctness work, and stays inside
  [0006-MADR-emit-go-with-go-ast.md](0006-MADR-emit-go-with-go-ast.md) and
  [0007-MADR-self-contained-acp-sdk.md](0007-MADR-self-contained-acp-sdk.md).

## Considered Options

* P1. One program under this record, in five phases ordered by severity, each with its own PLAN
* P2. One release that fixes everything at once
* P3. Fix the confirmed behaviour defects only, and record the rest as known limitations
* P4. A separate MADR for each area

## Decision Outcome

Chosen option: "P1. One program under this record, in five phases ordered by severity, each with its own PLAN", because it fixes
the defects users meet first, keeps every exported change in one reviewed minor release, and lets each phase be approved, tested and
reverted on its own.

The phases, each a `0019-PLAN-<scope>.md` beside this record, written and approved one at a time, with the answers recorded in
"Decided with the owner, 2026-10-13":

1. **Behaviour fixes, no exported change**: B1 (decision 2), B3, B4, B5, B6, B7, B8, B9, B10, B11, B12, B13, B15 (decision 8), G1,
   and the example fixes E1 to E4. Each with a test seen to fail first. Not released on its own (decision 12).
2. **API and behaviour decisions**: B2's hand-off (decision 1) with its example and README change, B14 (decision 7), B16 to B18
   with the writer goroutine (decision 4), B19 (decision 3), B20 (decision 6), I8 (decision 9), B21 (decision 10). Phases 1 and 2
   ship together as `v0.18.0` (decision 12).
3. **Generator hardening**: G2 and G3 (decision 5), G4 to G19, each latent shape refused or handled, with an engine case each.
4. **Examples, documentation, tests and tooling**: E5 to E16, I12, with E11 through the template (decision 11).
5. **Idioms and performance**: I1 to I7, I9 to I11, I13 to I16, with benchmarks committed so the gains are measured, not asserted.

Phases 3 to 5 ride the release after each lands (decision 12).

### Consequences

* Good, because the high and medium defects are fixed first, and every exported change is decided and recorded before the minor
  release that carries them both.
* Good, because consumers adapt once, to `v0.18.0`, for the fixes and the API changes together.
* Bad, because the deadlock and lost-cancel fixes are not released until phase 2 lands; a consumer who needs them earlier pins the
  phase 1 commit.
* Bad, because decision 7 refuses extension requests received before `initialize`, a behaviour change the `v0.18.0` release notes
  must name, and decision 4's writer goroutine changes the write path every message takes.
* Good, because each finding becomes a committed test or engine case, so its regression is caught.
* Neutral, because the latent generator findings change no committed output until the schema grows such a shape; they are hardening,
  not repair.
* Bad, because five phases mean five PLANs and several releases; the program runs over weeks, not days.
* Bad, because decision 1 and decision 4 add exported API that must then be kept.

### Confirmation

* Each phase's PLAN lists its findings by id, and its execution record shows, for each, the test or engine case that failed before
  the fix and passes after it.
* After phase 1, the probes for B1 to B13 pass against the tree as committed tests.
* After phase 5, `go fix -diff ./...` reports nothing in either module, and the lint configuration of E16 is clean.
* The owner settled the program and the decisions on 2026-10-13, and this record is `accepted`; a decision changed later is recorded
  here as an amendment before the phase that uses it.

## Pros and Cons of the Options

### P1. A phased program under this record

* Good, because ordering by severity ships the deadlock and the lost cancels first.
* Good, because each phase is small enough to review, and a regression points at one phase.
* Neutral, because it needs one PLAN per phase.
* Bad, because the idiom work waits behind the rest.

### P2. One release

* Good, because there is one round of review and one release.
* Bad, because the deadlock fix waits for the slowest finding.
* Bad, because a change set that touches the runtime, the generator, every generated file and the examples is hard to review, and a
  regression is hard to place.

### P3. Behaviour defects only

* Good, because it is the least work.
* Bad, because G1 (a schema description can inject code) and the gaps the README promises stay open.
* Bad, because the idiom drift grows unseen while the lint configuration cannot see it.

### P4. A MADR per area

* Good, because each record is shorter.
* Bad, because the findings cross areas (B2 is runtime, generator, examples and docs; B12 is generator and runtime), and five
  records would cite each other for one program.

## More Information

* The pass was asked for on 2026-10-13, after `v0.17.1`
  ([0018-MADR-start-side-connections-after-assignment.md](0018-MADR-start-side-connections-after-assignment.md)).
* Duplicates were merged: B7 was reported by two areas, B12 and B14 by two each, and E8 with an idiom finding.
* Not findings, by decision: the client's `Prompt` returning at once when its context ends (0017); the initialisms (`Url`, `Uri`,
  `Id`) kept for API stability; `encoding/json` v1 (0007); nojsonv2 and `-race` on Linux only (0011, 0018).
* Not checked: whether gemini's `--experimental-acp` flag is current; the four conformance variants E10 names may be covered by the
  instance tests.
