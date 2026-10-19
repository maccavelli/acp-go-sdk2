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

### Amended while planning phase 2, 2026-10-13

* **Decision 6 changes: an undeclared elicitation mode is refused both ways.** Research for the phase 2 PLAN found that the current
  ACP elicitation page (upstream `agentclientprotocol/agent-client-protocol` at `e5bcca615c2560a7d707365c773ae768ae595e88`,
  `docs/protocol/v1/elicitation.mdx:166`) says "Requests using a mode the Client has not advertised produce JSON-RPC `-32602`
  (Invalid params)". A client can advertise only `form` and `url`, so the schema's `other` variant, a "Custom or future elicitation
  mode", is never advertised. Decision 6 accepted it on receipt. The owner decided on 2026-10-13: sending `other` is refused, as
  decided, and a received one is answered `-32602`, as every value gate is, with the `WithInboundValidation` hook able to accept it,
  so a proxy or an application that advertised a `_`-prefixed mode through `_meta` keeps working.

### Amended while executing phase 2, 2026-10-14

* **B21's fact changes: no answer is written after the connection's end.** The finding says a turn whose connection ended "is
  answered `cancelled`", and no peer receives it. That held at `e5e716e`: the answer, `{"stopReason":"cancelled"}`, was written.
  Since phase 2's B17 ([0019-PLAN-api-decisions.md](0019-PLAN-api-decisions.md), step 3), every send after the connection's end
  fails without writing, so the dispatch still settles the turn as `cancelled` and its answer is not written. Decision 10 is
  unchanged: the 0017 text is amended to what the code does, which is now this. Found and decided with the owner on 2026-10-14,
  writing the amendment.
* **Decision 12's version changes: phases 1 and 2 ship as `v1.14.0`.** A tag `v1.13.0`, pushed by accident at an unfinished
  commit of phase 2, was cached by the Go module proxy, which made it the module's `@latest`. The owner decided on 2026-10-14 to
  release phases 1 and 2 as `v1.14.0`, its `go.mod` retracting `v1.13.0`, in place of `v0.18.0`; nothing else in decision 12 changes.
  Recorded in [0020-MADR-retract-accidental-v1-tag.md](0020-MADR-retract-accidental-v1-tag.md).

### Amended while planning phase 3, 2026-10-14

* **Two findings join phase 3.** Re-running the probes of G2 to G19 on `c97e6d3` for the phase 3 PLAN found two latent defects the
  pass did not list. The owner decided on 2026-10-14 to add both to phase 3: G20 is refused, G21 is handled with G12. The committed
  schemas contain neither shape; their only non-string `const` values are the 8 integers of `ErrorCode`, on the integer-enum path.

  | Id | Severity | Finding | Evidence | Fix in |
  | :--- | :--- | :--- | :--- | :--- |
  | G20 | latent | A union variant with no `$ref` and no properties whose `const` is a number, a boolean, or a string in a union that also holds a non-string const is generated with the const dropped: the wrapper is typed by the variant's `type` (or is an empty struct when it has none), and decoding, sending and `Validate` accept any value of that type. | Probe on `c97e6d3`: `oneOf` of number consts 1.5 and 2.5 decoded `3.5` and `Validate` passed; boolean consts `true` and `false` decoded `false` as the `true` variant; a string const `"a"` beside an integer const accepted `"b"`. | gen |
  | G21 | latent | An integer bound is emitted through `int64`: a `uint64` `maximum` above `math.MaxInt64` saturates, so `maximum: 1e19` is emitted as `9223372036854775807` and refuses valid values; and a bound the Go type cannot reach (`uint8` with `minimum: 300`) is refused only by the type checker, in the generated file, without naming the property. | Probe on `c97e6d3`. | gen |

* **Facts that moved since `c77f25a`.** Each of G2 to G19 still reproduces on `c97e6d3`. G6's third re-read of `type` is gone; two
  remain. G19's list of untested error sites is replaced, in the phase 3 PLAN, by the list measured on `c97e6d3`. Decision 5 is
  unchanged: the field "whose schema text says explicit null is retained" is named in the protocol table with that schema sentence,
  and a guard test requires every property whose description says so to be named there, as
  [0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md](0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md) did for the patch
  fields; the one such property today is the unstable `McpError.data`.

### Amended while planning phase 4, 2026-10-16

Re-checking E5 to E16 and I12 on `0acc8e1` for the phase 4 PLAN found each still present, some wider, and one new tooling finding.
The owner decided on 2026-10-16, each as recommended:

* **E16 is split.** Its modernizing linters, and lint over generated code, move to phase 5 with the fixes for what they flag: enabled
  on `0acc8e1`, they report 126 issues at the root, most in generated code, and 28 in `cmd/generate`, which are phase 5's I9, I13 and
  I14. Phase 4 keeps E16's tool pin: govulncheck v1.9.0 in CI, which the pre-add check reads.
* **I12 covers every exported identifier.** 654 exported identifiers have no doc comment, where I12 named 191: also 316
  `UnmarshalJSON`, 71 `MarshalJSON` and 76 enum constants, whose values' schema descriptions the generator drops. Phase 4 documents
  all of them, and a test requires a doc on every exported declaration.
* **E5 settles gemini's flag.** The gemini CLI 0.64.0 marks `--experimental-acp` deprecated, "use --acp instead"; the example passes
  `--acp`. This answers the question "More Information" left unchecked.
* **E7 is decided:** the example clients advertise only what they implement; their terminal methods answer method-not-found, and a
  write outside the session's working directory is refused.
* **E17 joins phase 4, and `mise.lock` is removed.**

  | Id | Severity | Finding | Evidence | Fix in |
  | :--- | :--- | :--- | :--- | :--- |
  | E17 | ci | `mise.lock` is tracked with no `mise.toml`, so it configures nothing, and its pins contradict the real ones: golangci-lint 2.12.2 and Go 1.26.3, where CI, the `Makefile` and the pre-add check use v2.14.0 and `go.mod` 1.27.2. | Read, on `0acc8e1`. | tool |

* **Facts that moved since `c77f25a`.** Root coverage is still 88.8%; `SendRequestNoResult` is now partly run. E10 has a third fixed
  sleep (`connection_cancel_test.go:688`), and `TestConformanceConstructors` still skips four subtests. The examples give 50 lint
  issues, where E12 counted 55.

### Amended while assessing phase 5, 2026-10-18

The owner asked on 2026-10-18 for phase 5 to be re-assessed against the codebase, and for pending issues to be found.

**Where the program stands.** Phases 1 to 4 are complete:

| Phases | PLAN | Release |
| :--- | :--- | :--- |
| 1 and 2 | [0019-PLAN-behaviour-fixes.md](0019-PLAN-behaviour-fixes.md) and [0019-PLAN-api-decisions.md](0019-PLAN-api-decisions.md) | `v1.14.0` |
| 3 | [0019-PLAN-generator-hardening.md](0019-PLAN-generator-hardening.md) | `v1.15.0` |
| 4 | [0019-PLAN-examples-docs-tests-tooling.md](0019-PLAN-examples-docs-tests-tooling.md) | `v1.15.1` |

Phase 5 has no PLAN yet.

**How it was re-checked.** Everything was re-checked on `7c48c95`, read-only:

- a pattern count of each finding's code;
- `go fix -diff` in both modules;
- the lint probe of phase 4's planning, re-run on a scratch copy;
- the open GitHub issues and pull requests;
- the records' open items;
- the upstream schema releases.

#### Phase 5's findings on `7c48c95`

The table gives each finding's state, with the line numbers as of `7c48c95`; those the finding gives date from `c77f25a`.

| Id | On `7c48c95` |
| :--- | :--- |
| I1 | Holds: 88 union `MarshalJSON` sites still marshal, decode into a map, set the discriminator and marshal again (first at `types_gen.go:406`). No benchmark is committed (`func Benchmark` appears nowhere), so the pass's measurements are the only ones. |
| I2 | Holds: `canonicalJSONRPCIDKey` (`connection.go:771`) builds a decoder for every id (`:777`). |
| I3 | Holds: `validateOutbound` (`connection.go:165`) copies a value through `reflect.New` (`:176-177`) and asserts `ptr.Interface().(validator)` (`:178`); `checkResult` is at `:1922`. |
| I4 | Narrowed: `orDefault` (`types_gen.go:20580`) now returns a present value without decoding the default, but still decodes the default's string on every absent field. |
| I5 | Narrowed: the disconnect errors are built lazily now (`connection.go:1714`, `:1753`). The `fmt.Errorf` of the superseded cause still runs on every prompt under the default overlap policy (`agent_gen.go:149`). |
| I6 | Holds: 291 `objectKeys` calls in the generated decoders. It still needs design. |
| I7 | Holds: 7 sites (first at `types_gen.go:378`). |
| I9 | Narrowed. The examples' assertions are gone (phase 4). Left: 2 `errors.As` in hand-written code (`connection.go:1973`, `:1980`); 2 in the generator (`output/check.go:86`, `:147`); 31 in root tests; and 8 `err.(*json.UnmarshalTypeError)` in generated code (first at `types_gen.go:374`), the 8 `errorlint` issues below. |
| I10 | Holds, at `client_gen.go:713`, the same lines issue #7 names (B22 below). |
| I11 | Holds: wake goroutines with stop channels at `connection.go:1756` and `:1824`, and the end-of-connection broadcast at `:636`. The dead nil-cause branches are at `:1735` and `:1801`. Phase 4's step 6 named `:1802-1803` and `:1807` as never run, since a done context always has a cause. |
| I13 | Holds: 97 `_b, _e` and `return []byte{}, _e`; 46 `var a Alias; a = Alias(v)` (52 before); 26 `resp, err := …; return resp, err` (27); 8 `else { if`. |
| I14 | Holds: `fmt.Errorf` without a verb at `extensions.go:32` and 21 times in the generator (20 before); the view struct at `errors.go:52`; the index loop at `connection.go:917`. `go fix -diff` now proposes 5 changes at the root and 6 in the generator, where it proposed 6 and 4. They are `errors.AsType`, `strings.SplitSeq` and `strings.FieldsSeq`, `slices.Backward`, `reflect.Type.Fields`, `maps.Copy` and an embedded-literal simplification. |
| I15 | Holds: 3 `wg.Add`, 50 `context.Background()`, 1 `slog.NewTextHandler(io.Discard, …)` and 3 `fmt.Sprintf("%d", …)` in tests. Missing `t.Helper()` was not recounted. |
| I16 | Holds: the mutation tool has no signal handling, so its deferred `os.RemoveAll` (`cmd/generate/internal/mutate/main.go:112`) does not run on Ctrl-C. `exec.Command` without a context is at `:166`. |

**E16's linters.** The probe adds `modernize`, `intrange`, `copyloopvar`, `perfsprint` and `usestdlibvars`, and lints the generated
files. It finds the same counts for `darwin` and `windows`:

- **At the root:** 127 issues, one more than phase 4's planning counted. The new one is `perfsprint` in `example/agent/main.go`, which
  lint now covers. 20 are in hand-written code: `modernize` 13, `perfsprint` 6, `intrange` 1. 107 are in generated code, all from
  linters the repository already enables: `goconst` 57, `errcheck` 26, `errorlint` 8, `gocritic` 7, `gocognit` 5, `gocyclo` 2 and
  `unconvert` 2.
- **In `cmd/generate`:** 28 issues, all in hand-written code: `perfsprint` 22 and `modernize` 6.

Outcome item 5 names I9, I13 and I14 for these. But turning on lint over generated code brings in `goconst`, `gocognit`, `gocyclo`,
`errcheck`, `gocritic` and `unconvert`, which no finding names (question 15).

#### New findings

Three issues opened after `v1.14.0` and `v1.15.0` are defects in phases 1, 2 and 0017's gates. No record tracks them. Each still
reproduces on `7c48c95` by its quoted code, unchanged since its report.

| Id | Severity | Finding | Evidence | Fix in |
| :--- | :--- | :--- | :--- | :--- |
| B22 | low | `ClientSideConnection.Prompt` sends `session/cancel` whenever the call fails with an ended context, including when `sendRequest` refused to write the request (B9), so the agent receives a cancel for a turn it never got. Under `PromptOverlapConcurrent`, it can end another turn of the session. | Issue [#7](https://github.com/maccavelli/acp-go-sdk2/issues/7), reproduced on `v1.14.0`; the code is unchanged at `client_gen.go:710-715`. | gen |
| B23 | medium | An agent never answers a request received just before its input reaches end of file. `shutdownReceive` cancels the connection's context first (`connection.go:1354`). The handler then runs under an ended context, and its reply is dropped, because every send fails once the connection has ended (B17). A script that pipes requests into a stdio agent and closes its input gets no reply. | Issue [#8](https://github.com/maccavelli/acp-go-sdk2/issues/8): 0 of 100 runs answered on `v1.15.0` and `v1.14.0`. A downstream consumer serving ACP over stdio meets it. | rt |
| B24 | gap | `subagent_update`, `session_message` and `session_message_chunk` are not gated on `subagents`, though the unstable schema's `SubagentCapabilities` says "The client must advertise this capability before the agent sends subagent updates or session-directed messages". `SessionUpdate.gates` checks `session.notices` and `session.compaction` only. 0017 left `subagents` ungated because no gate was stated then; one is now. | Issue [#9](https://github.com/maccavelli/acp-go-sdk2/issues/9); `SessionUpdate.gates` in `types_gen.go`. | gen |
| E18 | low | The example clients print a tool call update's status with `%v` of its pointer (`updated: 0x…`). | Phase 4's step 4 record; named in the `v1.15.1` release notes. | ex |

#### Pending items outside the program

- **The schema.** ACP published `schema-v1.26.0` on 2026-10-16; this SDK pins `1.25.0` (`schema/version`). The stable `schema.json`
  and `meta.json` are byte-identical. The unstable schema removes 10 methods (`nes/*` and `document/*`) and 57 definitions, adds 5
  (a system-prompt capability), and changes 10.
  - Regenerated, 323 of the 2429 lines of the exported API listing would go, 11 methods among them, with fields of stable types such as
    `AgentCapabilities.Nes` and `ClientCapabilities.PositionEncodings`.
  - That is a removal from a `v1` module, which no record decides (question 16).
- **Pull request #1.** It is Dependabot's, opened 2026-10-08. It edits `.github/workflows/ci.yaml` and `release-notes.yaml`, which
  `9fb9ae5` deleted, and bumps `jdx/mise-action`, which nothing uses. Its `actions/checkout` v7.0.1 is already the pin in `ci.yml`. It
  is obsolete.
- **Windows symbolic links.** Whether the Windows host ran the example clients' symbolic-link check is not established (phase 4's
  step 4 record).

#### Questions for the owner

Each has a recommended answer.

14. **Where B22 to B24 and E18 go.** Recommended: a PLAN under this record, `0019-PLAN-issue-fixes.md`, executed before phase 5,
    because the program is ordered by severity and B23 is medium. Each fix gets a test seen to fail first, and they are released
    together after it lands, the version confirmed at the tag. B22 is fixed by cancelling only a written request, and I10's
    `context.WithoutCancel` lands with it at the same lines. B24 adds three variant gates to the protocol table, citing the
    schema's sentence.
    - B23 needs its own design. Recommended: at end of input, requests already received finish and their replies are written before
      the connection ends, with the wait bounded. The agent's own pending requests fail at once with the end-of-input cause. This is
      the default, and the release notes name it as a behaviour change, since `Done`'s doc describes today's behaviour.
    - What the Rust and TypeScript SDKs do at end of input is checked while planning.
    - Alternative: a `ConnectionOption`, off by default.
15. **Lint over generated code (E16).** Recommended:
    - every linter the configuration enables also runs over the generated files;
    - each finding is fixed in the generator: `errcheck`, `errorlint`, `gocritic`, `unconvert` and the modernizers;
    - except `goconst`, `gocognit` and `gocyclo`, excluded for `_gen.go` files with their reason in `.golangci.yml`: the repeated
      strings are schema literals, and the dispatch and decoders are long by construction.

    Alternative: fix every finding, `goconst`'s 57 included.
16. **Schema 1.26.0.** Recommended: a new record decides the bump and the policy for removing unstable exported API under `v1`. It is
    not part of this program, so phase 5 runs on schema `1.25.0`, and its benchmarks compare like with like.
17. **Pull request #1.** Recommended: the owner closes it as obsolete, naming `9fb9ae5`. No change to the tree.

#### Evaluated and decided with the owner, 2026-10-18

The owner asked for the pending issues to be evaluated and brought into scope, and for the latest schema not to be pinned yet.

**Probes.** Probe tests in a scratch copy of `7c48c95` reproduce each issue:

- **B22:** `Prompt` with an ended context writes `session/cancel` as its first line, where nothing should be written.
- **B23:** of 50 requests received just before end of input, none was answered, and all 50 handlers saw their context already ended.
- **B24, sending:** all three variants were sent to a client that advertised no `subagents`, with no `*CapabilityError`.
- **B24, receiving:** the client handed `session_message_chunk` to its `SessionUpdate`. An ungated notification sent after it shows the
  order, since notifications are handled one at a time.

**What each fix touches.**

- **B22:** `sendRequest` already writes nothing for an ended context (`connection.go:1569-1572`). The cancel in
  `ClientSideConnection.Prompt` does not know whether the request was written, so the fix needs that fact from `sendRequest`. A write
  that ended with the context may still have reached the peer, so a cancel stays right there.
- **B23:** a request's handler runs under the connection's context (`connection.go:1162`), which end of input cancels first
  (`shutdownReceive`, `:1354`). A notification's handler runs under `inboundCtx`, which is cancelled only after the queue drains or
  `notificationQueueDrainTimeout` passes (`:1369-1373`). End of input already drains notifications, then, but not requests.
- **B24:** three variant rules in the protocol table beside `notice` and the compaction variants
  (`cmd/generate/internal/model/acp.go:153-155`). `README.md:404-406`, which lists `subagents` among the unchecked features, changes
  with them.

**What the reference SDKs do at end of input.** Read, not run:

| SDK | Replies of handlers running | Own pending outbound requests | Bound |
| :--- | :--- | :--- | :--- |
| TypeScript, `agentclientprotocol/typescript-sdk` at `1726f28f`, `src/jsonrpc.ts` | Dropped, as here: the receive loop closes the connection (`:1226`, `:1241`), which aborts every handler (`:1165`, `:1172`); `sendWireMessage` refuses once aborted (`:1533-1534`). | Rejected at once (`:1168`). | None. |
| Rust, `agentclientprotocol/rust-sdk` at `d5e8bf4d` | Written: end of input is handled after the running handler returns, and the outgoing queue drains before output closes (`jsonrpc.rs:1775-1777`, `:2102-2105`). Work a handler moved to a spawned task is dropped. | Fail at once, with "Incoming transport closed" (`jsonrpc.rs:2363-2372`, `:3747`). | None. |
| Python, `agentclientprotocol/python-sdk` at `11f8a4c8` | Written by `Connection` (`connection.py:251`). Cancelled under `run_agent` (`core.py:73-76`), which closes the connection when its input ends. | Rejected at once (`connection.py:335-340`). | Reads only. |

**The Windows host.** The example clients' symbolic-link case does run there. `go test -v -run TestWriteTextFileStaysInCwd` over
the three example clients, on a Windows host reached over SSH, logged no "no symbolic link on this host" line, and all three passed.
Phase 4's open question is closed.

**Decisions.** The owner decided on 2026-10-18:

| Question | Answer |
| :--- | :--- |
| 14. Where B22 to B24 and E18 go | A PLAN under this record, `0019-PLAN-issue-fixes.md`, executed and released before phase 5, as recommended. I10 moves into it, since it changes B22's lines. |
| 14. B23 at end of input | **Drain by default, bounded**, as recommended and as the Rust SDK does. Requests already received keep running, and their replies are written. The connection's own pending requests fail at once, with an end-of-input cause that wraps `ErrPeerDisconnected`, so no exported API is added. `Done` closes after the drain. The wait is bounded, as the notification drain is, so a stuck handler cannot hold the connection open. It is a behaviour change: `Done`'s doc changes, and the release notes name it. |
| 15. Lint over generated code | As recommended: every enabled linter also runs over the generated files, and each finding is fixed in the generator. The exceptions are `goconst`, `gocognit` and `gocyclo`, excluded for `_gen.go` files with the reason in `.golangci.yml`. |
| 16. Schema 1.26.0 | **Not pinned yet.** The program stays on schema `1.25.0`, and no record decides the bump now. |
| 17. Pull request #1 | Closed by the agent at the owner's request, on 2026-10-18, with a comment naming `9fb9ae5`. |

### Amended while planning phase 5, 2026-10-18

The owner asked on 2026-10-18 for the next phase to be researched and this record drafted against the codebase as it is. The owner
called it "phase 6". It is this record's phase 5, idioms and performance, the only phase left. The issue fixes ran before it as their
own PLAN, [0019-PLAN-issue-fixes.md](0019-PLAN-issue-fixes.md), released as `v1.15.2`. The name "phase 5" is kept, so that the
decisions already recorded against it still apply.

**How it was researched.** Everything was re-measured on `f67c9c1`, read-only, with prototypes and benchmarks in scratch copies of the
tree:

- a pattern count of each finding's code;
- `go fix -diff` in both modules;
- the lint probe of phase 4's planning, which adds `modernize`, `intrange`, `copyloopvar`, `perfsprint` and `usestdlibvars` and lints
  the generated files;
- `thelper` on its own;
- `go doc` on `go1.27.2` for every API named below: `reflect.TypeAssert`, `context.AfterFunc`, `testing.B.Loop`, `errors.AsType`,
  `strings.SplitSeq`, `sync.OnceValue`, `sync.WaitGroup.Go`, `signal.NotifyContext`, `slices.Backward` and `testing/synctest`, all
  present.

Benchmarks are the median of six runs of `go test -bench -benchmem` on `go1.27.2`, `darwin/arm64`.

#### Phase 5's findings on `f67c9c1`

`connection.go` has moved since the assessment on `7c48c95`, with B22 and B23, so its lines are given anew.

| Id | On `f67c9c1` |
| :--- | :--- |
| I1 | Holds, and wider than written: all 88 union `MarshalJSON` arms marshal the variant, decode it into a map and marshal again, in 22 unions. 67 arms then set the discriminator. In each, the variant's struct carries the discriminator as a string field: 55 variants are plain structs and 12 have their own `MarshalJSON`. The other 21 arms, in unions with no discriminator (`AgentResponse`, `ClientResponse` and others), set nothing: the round trip only re-sorts the top-level keys. |
| I2 | Holds: `canonicalJSONRPCIDKey` (`connection.go:778`) builds a decoder for every id. Its key for a plain unsigned decimal integer without a leading zero, or `0`, is the integer's text, for every sample tried, up to the 20 digits of `18446744073709551615`. `-0`, `1e2` and `1.0` normalise to `0`, `100` and `1`, and `007` is refused. |
| I3 | Holds: `validateOutbound` (`connection.go:165`) copies a value through `reflect.New` and asserts `ptr.Interface().(validator)`, 2 allocations per call. It is called in `encodeParams` (`:1714-1718`), for the params of every request and notification sent, and in `checkResult` (`:2029`), for every handler's result. |
| I4 | Narrowed, and joined to a lint finding. `orDefault` is called 475 times; 455 pass no default and only decode the present value. Of the 20 with a default, 11 are `bool` with `"false"`, the zero value, and 8 are capability structs whose JSON defaults are made of `false` and `{}`. `ElicitationSchemaType`'s `"object"` is the one non-zero scalar default. Separately, 24 of the 26 `errcheck` issues in generated code are `json.Unmarshal([]byte("<default>"), &a.X)` calls that drop the error, for a missing or null field's default: 24 calls for 22 fields. The 20 fields with an `orDefault` default are among those 22; the direct decode also covers `ElicitationSchema.Properties` and `InitializeResponse.AuthMethods`. |
| I5 | Narrowed (`7c48c95`): only `agent_gen.go:149`'s `fmt.Errorf` of the superseded cause remains. It runs on every prompt under `PromptOverlapSupersede`, the default, being the zero value (`connection.go:410`): the `default:` branch that serves it makes the error each time (`agent_gen.go:148-149`). |
| I6 | Holds: 291 `objectKeys` calls. A union decodes the whole object into a map and folds its keys; the variant it picks then calls `objectKeys`, which decodes the same bytes into a map again, before decoding them a third time into its shadow struct. A `session/update` with a text chunk is parsed six times, three for the update and three for its `ContentBlock`, as the generated decoders read. |
| I7 | Holds, and is dead work: the seven sites decode the whole payload into `[]map[string]json.RawMessage` inside an `if` whose body is empty, all seven counted. The emitter writes the decode whether or not the union has an array variant (`arrayMatch`, `cmd/generate/internal/emit/types.go:769-777`). |
| I9 | Narrowed (`7c48c95`). Left: 2 `errors.As` in hand-written code (`connection.go:2079`, `:2086`), 2 in the generator (`output/check.go:86`, `:147`), 31 in root tests, and the 8 `err.(*json.UnmarshalTypeError)` assertions in generated code, which are the 8 `errorlint` issues. |
| I10 | Done by [0019-PLAN-issue-fixes.md](0019-PLAN-issue-fixes.md), step 3. |
| I11 | Holds: the end-of-connection broadcast goroutine of `start` (`connection.go:642`) and the stop-channel wake goroutines of `waitNotificationsUpTo` (`:1859`) and `waitForNotificationDrain` (`:1927`). The nil-cause branches after `<-ctx.Done()` are at `:1835`, in `waitForResponse`, and `:1904`, in `waitNotificationsUpTo`. Phase 4's step 6 found the second never run and did not measure the first; `go doc context.Cause` says it "returns a non-nil error explaining why c was canceled", so neither can run. |
| I12 | Done in phase 4. |
| I13 | Holds: 97 `_b, _e` with `return []byte{}, _e`; 46 `var a Alias; a = Alias(v)`; 26 `resp, err := …; return resp, err`; 8 `else { if`. `AgentSideConnection`'s generated methods name their receiver `a` on 4 and `c` on 21. |
| I14 | Holds: `fmt.Errorf` without a verb at `extensions.go:32` and 21 times in the generator; the view struct at `errors.go:52`; the index loop at `connection.go:924`. `go fix -diff` proposes 5 changes at the root (`connection.go`, `gates_test.go` 2, `instances_test.go` 2) and 6 in the generator (`mutate/main.go` 3, `mutate/sites.go`, `model/keywords_test.go`, `output/check.go`). |
| I15 | Holds: 3 `wg.Add`, 49 `context.Background()` and 3 `fmt.Sprintf("%d", …)` in tests; 1 `slog.NewTextHandler(io.Discard, …)`. `thelper` finds 10 issues at the root and 10 in the generator. |
| I16 | Holds: the mutation tool runs each mutant under `context.Background()` (`cmd/generate/internal/mutate/main.go:144`) and lists packages with `exec.Command` (`:166`). Ctrl-C ends it without its deferred `os.RemoveAll` of the work directory (`:112`): by default "a SIGHUP, SIGINT, or SIGTERM signal causes the program to exit" (`go doc os/signal`), which runs no deferred call. |
| E16 | The probe finds the same counts for `darwin` and `windows`. |

E16's counts, in detail:

- **At the root:** 128 issues.
  - Hand-written code: 20 (`modernize` 13, `perfsprint` 6, `intrange` 1).
  - Generated code: 108 (`goconst` 58, `errcheck` 26, `errorlint` 8, `gocritic` 7, `gocognit` 5, `gocyclo` 2, `unconvert` 2).
- **In `cmd/generate`:** 28, all in hand-written code (`perfsprint` 22, `modernize` 6).
- **What question 15's answer leaves to fix in generated code:** 43 issues, after excluding `goconst`, `gocognit` and `gocyclo`.
  - `errcheck`: 26 (I4's defaults, `Prompt`'s best-effort cancel, and one type assertion).
  - `errorlint`: 8 (I9).
  - `gocritic`: 7 (4 one-case `switch` statements, 3 chains of `append` calls).
  - `unconvert`: 2.

#### What it costs now, and what the prototypes save

Medians of six runs:

| Benchmark | `f67c9c1` | I1 prototype | I1 and I6 prototypes |
| :--- | :--- | :--- | :--- |
| `SessionUpdate` encode | 3,530 ns, 37 allocs | 1,059 ns, 9 allocs | — |
| `SessionUpdate` decode | 6,530 ns, 45 allocs | 6,532 ns, 45 allocs | 4,290 ns, 31 allocs |
| `SessionNotification` decode | 8,415 ns, 58 allocs | 8,470 ns, 58 allocs | 6,037 ns, 44 allocs |
| `session/update` sent and received over pipes | 34,432 ns, 115 allocs | 19,153 ns, 86 allocs | 15,948 ns, 73 allocs |
| a prompt round trip over pipes | 37,519 ns, 149 allocs | 36,490 ns, 135 allocs | — |
| the id key of `12345` | 542 ns, 10 allocs | — | — |
| `validateOutbound` on a value | 62 ns, 2 allocs | — | — |

**The I1 prototype** rewrote all 88 arms of `types_gen.go`:

- the 67 with a discriminator set it on a copy of the variant and marshal once;
- the 21 without one marshal the variant.

The root suite passed. Every schema instance of every registered type, 1248 in all, was decoded and encoded on `f67c9c1` and on the
prototype, and the two dumps were byte-identical. The dump holds the variants with their own `MarshalJSON`: three instances each of the `form` and `url` elicitations and of the `http` and `sse` servers. A discriminator altered in the
prototype changed 5 lines of the dump, so the comparison does see a difference.

**The I6 prototype** gave two variants, `SessionUpdateAgentMessageChunk` and `ContentBlockText`, an unexported `decodeKeys(b, m)`: the
decoder's body after `objectKeys`. Their unions call it with the map they have already parsed. The root suite passed.

A discriminated union folds its discriminator and every variant's `FitKeys`, the variant's other required keys
(`cmd/generate/internal/model/model.go:209-212`), so the keys a variant requires arrive folded. `decodeKeys` folds them again on the
shared map all the same: that changes nothing on a folded map, costs no parse, and keeps `decodeKeys` independent of its caller.

#### Design

Each item takes the approach that changes no exported API and no wire bytes. A test or a guard seen to fail first pins each, and a
benchmark measures each performance item.

- **D1 (I1): marshal once.** A union arm whose variant struct carries the discriminator as a string field marshals a copy with that
  field set: `v := *u.X; v.Field = "value"; return json.Marshal(v)`. An arm with no discriminator marshals the variant. The map round
  trip is no longer emitted. A variant without such a field, if the schema ever has one, keeps the map path, and an engine case pins
  that.
- **D2 (I6): decode once at each level.** Every generated object type with an `objectKeys` decoder gains an unexported `decodeKeys(b
  []byte, m map[string]json.RawMessage) error`. It folds the type's required keys on `m`, checks them, and decodes its fields;
  `UnmarshalJSON` becomes `objectKeys` and then `decodeKeys`. A union calls the chosen variant's `decodeKeys` with its own map. The
  third parse, into the shadow struct, stays: decoding field by field from the map would replace one parse with many small ones, and
  would re-implement `encoding/json`'s field matching, which `foldKeys` follows ("the value encoding/json gives a struct field of
  that name", `cmd/generate/internal/emit/decode.go:408-412`).

  Amended while drafting [0019-PLAN-idioms-performance.md](0019-PLAN-idioms-performance.md), 2026-10-18: except for a variant that
  keeps undeclared keys in `Extra`, of which there are six, all catch-alls. The union's map is folded with the union's key list, so
  it could spell a key `Extra` keeps differently from how it was received; those six decode from the bytes as before.

  Amended while executing [0019-PLAN-idioms-performance.md](0019-PLAN-idioms-performance.md), step 6, 2026-10-18: it can for three
  of the six. The unions of the other three fold only a key their catch-all declares, so no key `Extra` keeps is folded.
- **D3 (I4 and the default `errcheck` issues): defaults as Go values.**
  - The generator renders each schema default as a Go expression of its field's type when it emits the decoder: nothing at all for a
    default that is the type's zero value, a constant for a scalar, and a composite literal otherwise.
  - No default is decoded at run time, so no decode error is dropped.

    Amended while executing [0019-PLAN-idioms-performance.md](0019-PLAN-idioms-performance.md), step 4, 2026-10-18: besides a
    constant and a composite literal, a default is written as `new(…)` for a pointer field and as `json.RawMessage(…)` for a raw
    one, and a non-empty array or map default as a literal of its items. A default on a union field is refused.
  - `schema_gen_test.go` gains a table of each default's Go value beside its JSON, and a root test requires decoding the JSON to give
    the same value. A rendering that drifts from what decoding gives fails there.
- **D4 (I7):** the array match is emitted only for a union with an array variant.
- **D5 (I2):** a fast path in `canonicalJSONRPCIDKey` for a plain unsigned decimal integer without a leading zero, of at most 19
  digits: its key is its text. Everything else takes the decoder. A fuzz target requires the fast path and the decoder to give the
  same key and error for every input.
- **D6 (I3):**
  - The generated outbound methods and dispatch pass pointers to `validateOutbound`, so no value is copied.
  - For a value from the raw API, `reflect.TypeAssert[validator]` replaces `ptr.Interface().(validator)`.
- **D7 (I5):** the superseded cause is a package-level error, made once, still wrapping `ErrPromptCancelled`.
- **D8 (I11):**
  - The three wake and broadcast goroutines become `context.AfterFunc` registrations, stopped by their callers.
  - The two nil-cause branches are deleted: `context.Cause` returns a non-nil error once a context is canceled (`go doc context.Cause`).
- **D9 (I9, I13, I14), hygiene:**
  - `errors.AsType` everywhere `errors.As` or a type assertion on an error is used, the generated `UnmarshalTypeError` checks included.
  - The generated code drops its underscore names, the alias assignment, the `resp, err` copies and `else { if`.
  - `AgentSideConnection`'s methods take the receiver `a`.
  - `errors.New` where `fmt.Errorf` has no verb.
  - The view struct and the index loop go.
  - Each change `go fix -diff` proposes is applied.
  - The `gocritic` and `unconvert` findings are fixed in the emitter.
  - `Prompt`'s deliberate discard of its best-effort cancel's error is kept and marked, so `errcheck` sees it is meant.

    Amended while executing [0019-PLAN-idioms-performance.md](0019-PLAN-idioms-performance.md), step 3, 2026-10-18: the emitter
    cannot print a comment inside a function body, so the mark cannot be written. `Prompt` handles the error instead: a cancel
    that fails is logged at Debug on the connection's logger, as `connection.go` logs other best-effort work that fails. This is
    the one change of D9 that adds behaviour, a diagnostic. The wire bytes do not change.
- **D10 (I15):** `t.Context()`, `wg.Go`, `slog.DiscardHandler`, `strconv.Itoa` and `t.Helper()` where the tests need them.
- **D11 (I16):** the mutation tool runs under `signal.NotifyContext`, for `os.Interrupt` and, where the platform has it, `SIGTERM`. It
  passes that context to every subprocess, so an interrupt ends the running `go test` and the deferred clean-up runs.
- **D12 (E16), lint:**
  - `.golangci.yml` enables `modernize`, `intrange`, `copyloopvar`, `perfsprint` and `usestdlibvars`, and `thelper` (question 20).
  - It lints the generated files, excluding `goconst`, `gocognit` and `gocyclo` for `_gen.go` with the reason, as question 15 decided.
  - The lint over generated code runs in `make pre-add-check` and CI, as the rest does.
- **D13, benchmarks committed:**
  - A root `bench_test.go` holds the seven benchmarks above, written with `testing.B.Loop`, and `make bench` runs them.
  - CI does not run them: a benchmark measures, and has nothing to fail on.
  - Each PLAN step that changes a hot path records the medians of at least six runs, before and after.
  - No benchmark-comparison tool is added as a dependency.
- **D14, a guard on the encodings:**
  - A root test encodes every schema instance of every registered type and compares the bytes with a committed file under `testdata/`.
  - A `-update` flag rewrites the file when a change is meant.
  - D1, D2, D3 and D9 then cannot change a byte of the wire format unseen.

  Amended while executing [0019-PLAN-idioms-performance.md](0019-PLAN-idioms-performance.md), step 2, 2026-10-18: one value is not
  compared as received. The instances use one absolute path, the host's temporary directory, and 57 of the 1248 encodings carry it.
  The test writes it as `<instancePath>`, so the committed file carries no host's path and is the same on every host.

#### Questions for the owner

Each has a recommended answer.

18. **The encodings guard (D14).** Recommended: commit it. The conformance round trip (`conformance_test.go:282-302`) and the 33 JSON
    goldens (`json_parity_test.go:13-23`) compare decoded values, so a change of key order or of a number's spelling would pass them.
    A few fidelity tests compare the bytes of single cases (`fidelity_test.go:120`).
19. **Defaults (D3).** Recommended: Go values from the generator, with the generated equivalence test.

    Alternative: decode each default once, through `sync.OnceValue`, and copy it per use. That is smaller in the generator, but keeps a
    run-time decode, and needs a deep copy for any default with a map or slice.
20. **`thelper` (I15).** Recommended: enable it with D12's linters, so missing `t.Helper()` calls stay fixed.

    Alternative: fix the 20 now and leave the linter out.
21. **The release.** Recommended: a patch, `v1.15.3`. No exported API changes, the wire bytes are pinned by D14 if question 18
    adopts it, and the changes are performance and hygiene. The owner confirms at the tag.

#### Decided with the owner, 2026-10-18

The owner asked for every claim of this amendment to be proved before deciding. An audit against the tree followed.

- **Three claims were wrong** and are corrected above: I4's two sets of defaults nest rather than match; a discriminated union does
  fold every variant's required keys; and phase 4 measured only one of I11's nil-cause branches.
- **The others are now cited:** each to a measurement, a line of the tree, or `go doc`.
- **The guard's own check:** the encoding comparison of the I1 prototype was seen to fail on a planted discriminator.

Then the owner answered questions 18 to 21. One answer differs from the recommendation, and is marked.

| Question | Answer |
| :--- | :--- |
| 18. The encodings guard (D14) | Committed, as recommended. |
| 19. Defaults (D3) | Go values from the generator, with the generated equivalence test, as recommended. |
| 20. `thelper` | Enabled with D12's linters, as recommended, and its 20 findings fixed. |
| 21. The release | **Not the recommendation:** a minor release, `v1.16.0`, which the owner confirms at the tag. |

#### The scope of phase 5, as researched

I1 to I7, I9, I11, I13 to I16, and E16's linters with lint over generated code. I8 was done in phase 2, I10 in the issue fixes, and
I12 in phase 4. D1 to D14 are the designs, as questions 18 to 21 settled them. Phase 5 releases as `v1.16.0`. The schema stays at
`1.25.0` (question 16).

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
   Amended 2026-10-14: and G20 and G21 ("Amended while planning phase 3, 2026-10-14").
4. **Examples, documentation, tests and tooling**: E5 to E16, I12, with E11 through the template (decision 11).
   Amended 2026-10-16: E16's linters move to phase 5, I12 covers every exported identifier, and E17 joins ("Amended
   while planning phase 4, 2026-10-16").
5. **Idioms and performance**: I1 to I7, I9 to I11, I13 to I16, with benchmarks committed so the gains are measured, not asserted.
   Amended 2026-10-16: and E16's linters, with lint over generated code. Amended 2026-10-18: re-checked on `7c48c95` ("Amended while
   assessing phase 5, 2026-10-18"). I4, I5 and I9 have narrowed. Questions 14 to 17 are open: issues #7 to #9 as B22 to B24, E18, the
   scope of lint over generated code, and schema 1.26.0. Decided the same day ("Evaluated and decided with the owner, 2026-10-18"):
   before phase 5, `0019-PLAN-issue-fixes.md` takes B22 to B24, E18 and I10; phase 5 lints the generated code except for `goconst`,
   `gocognit` and `gocyclo`; and the schema stays at `1.25.0`. Amended 2026-10-18, again: researched on `f67c9c1`, with the designs
   D1 to D14 and questions 18 to 21 ("Amended while planning phase 5, 2026-10-18"). Decided the same day: the encodings guard, defaults
   as Go values, `thelper`, and a minor release, `v1.16.0`.

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
* After phase 1, the probes for B1 to B13 pass against the tree as committed tests. Amended 2026-10-13, phase 1 step 2: B1's
  original probe sends from the handler under a context unrelated to the handler's, the limit decision 2 accepts; it keeps failing,
  and `TestRequestFromNotificationHandler` covers the handler's context and one derived from it. Amended again 2026-10-13, phase 1
  step 3: one of B12's two probes passes only if a request sent without an id is run, which B12's remedy replaces with dropping and
  logging it; it keeps failing, and `TestMethodKindMismatch` and the other probe cover the finding. Amended again 2026-10-13, phase
  1 step 4: one of B4's two probes requires a close that ends in an error to answer the session's pending requests, which the
  PLAN's design does only on success; it keeps failing, and `TestCloseSessionAnswersPendingPermissions`,
  `TestFailedCloseLeavesPermissionsPending` and the other probe cover the finding.
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
  instance tests. Both were settled later. The flag is deprecated for `--acp` ("Amended while planning phase 4, 2026-10-16"). The four
  variants were not covered elsewhere, and now run with their empty unions filled
  ([0019-PLAN-examples-docs-tests-tooling.md](0019-PLAN-examples-docs-tests-tooling.md), step 6).
