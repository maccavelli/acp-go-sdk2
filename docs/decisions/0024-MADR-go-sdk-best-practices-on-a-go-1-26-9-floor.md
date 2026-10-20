---
status: accepted
date: 2026-10-20
decision-makers: Repository owner
---

# Bring the SDK in line with Go SDK best practices, on a Go 1.26.9 floor

## Context and Problem Statement

On 2026-10-19 the owner gave a list of best practices for designing a Go SDK. It synthesizes Effective Go, the Go Code Review
Comments, Google's Go Style Guide, the module compatibility rules and community practice, in ten areas:

1. API design
2. construction and configuration
3. context
4. errors
5. versioning
6. documentation
7. dependencies and packaging
8. concurrency
9. security
10. a pre-publish checklist

The owner asked for the codebase to be assessed against it, and for a record of how to improve its compliance. The owner also set the
language floor: **the `go` line becomes 1.26.9**.

This record holds the findings and the decisions they need. Each question had a recommended answer, and the owner decided them on
2026-10-19 (see "Decided with the owner, 2026-10-19").

### How the assessment was made

* **Code:** read on `ce49693`, `v1.16.1` plus its publication record, by two read-only audits and by direct reading. Every figure below
  comes from a script or command run on 2026-10-19, and is cited at `file:line`. Claims an audit made only by reading were run before
  being used.
* **The floor:** tested in a scratch copy of the tree. Both `go` lines were set to `1.26.9`, then checked with:
  * the installed go1.27.2's `go vet`, whose `stdversion` check reports standard-library API newer than the `go` line;
  * a real go1.26.9 toolchain, through `GOTOOLCHAIN=go1.26.9` with its module cache, build cache and `GOPATH` in the scratch directory.

  Nothing in the tree or in `$HOME` was written.
* **Section 9 of the list** is written for an authentication SDK and a self-update SDK. This SDK is neither. What carries over is
  assessed below (logs, errors, wire data, supply chain). The rest is not applicable.

### What holds already

| Area | Finding | Evidence |
| :--- | :--- | :--- |
| 1 | Constructors accept interfaces and return concrete types | `NewConnection(handler MethodHandler, peerInput io.Writer, peerOutput io.Reader, …) *Connection` (`connection.go:568`); `NewAgentSideConnection(agent Agent, …) *AgentSideConnection` (`agent_gen.go:28`) |
| 1 | No global mutable state, but one test seam (finding A9) | 11 package-level `var`s: 5 exported sentinels, 5 unexported sentinels, `promptReturned` |
| 2 | Required values are parameters; 22 functional options, each documented with its default | 9 `ConnectionOption`s, 6 `ToolCallStartOpt`s, 7 `ToolCallUpdateOpt`s |
| 3 | Every exported call that takes a context takes it first, but `SendRequest` (A12) | `connection.go:1654` |
| 3 | Contexts stored in structs are lifetime contexts only | `connection.go:323-332` |
| 4 | Errors wrap with `%w`; sentinels and typed errors exist | generated: 577 of 578 `fmt.Errorf` wrap; hand-written: 4 of 6, the other 2 have no error to wrap. Sentinels: `ErrPromptCancelled`, `ErrNotInitialized`, `ErrTerminalAuthMethod`, `ErrPeerDisconnected`, `ErrConnectionClosed`. Typed: `*RequestError`, `*ValidationError`, `*CapabilityError`, `*ProtocolVersionError` |
| 4 | Every `panic` guards an internal invariant | 10, all in `connection.go`, on the notification sequence counters |
| 4, 9 | A message's content is never logged | `connection.go:1186`: "A message's content is never logged: it may hold prompt text or file contents" (B15). Wrong-typed `EnvVariable` and `HttpHeader` values are not echoed in decode errors (probe) |
| 5 | Semantic versioning and the import compatibility rule | module path has no `/vN`; tags go up to `v1.16.1`; `v1.13.0` retracted ([0020-MADR-retract-accidental-v1-tag.md](0020-MADR-retract-accidental-v1-tag.md)). The API listing shows no removal since `v1.14.0` in any release's notes. "After `v1.14.0` an incompatible API change needs `/v2`" ([0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md](0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md)) |
| 5, 10 | Releases are tagged and have notes | 10 notes reports in `docs/reports/`; 10 GitHub releases; `v1.16.1` latest |
| 7 | No dependencies | no `require` line in either `go.mod`; `go list -m all` lists each module alone |
| 7 | High coverage of public behaviour | root 89.1% of statements: hand-written 92.1%, generated 88.8%. `cmd/generate` 95.6% with `-coverpkg=./...`. 43 table-driven tests at the root; 4 fuzz targets, 2 in each module |
| 7 | Easy to test | the transport is an `io.Reader` and an `io.Writer`; the logger is an option |
| 8 | No lock held across a write or a callback | one writer goroutine holds no lock (`connection.go:671-680`); `c.mu`, `notifyMu` and the side connections' locks cover only map, counter and cancel operations |
| 8 | Long operations are cancellable | every call takes a context; `$/cancel_request`, `session/cancel`, `session/close` and `Close` end handlers |
| 9 | Supply chain | CI actions pinned by SHA (`ci.yml:28`, `:32`); `permissions: contents: read`; `govulncheck@v1.9.0` in CI weekly and in `make pre-add-check` |
| 10 | Checklist | `gofmt -l .` and `goimports -l .` empty; `go vet ./...` and `go test ./...` pass in both modules; `make lint` 0 issues for both `GOOS` values; `go fix -diff` empty |

### Findings

#### The Go 1.26.9 floor (G)

| Id | Finding | Evidence |
| :--- | :--- | :--- |
| G1 | **The library code needs no change.** With both `go` lines at `1.26.9`, `go build ./...` passes on go1.26.9 in both modules. go1.27.2's `go vet` reports no API newer than 1.26 once G2 is fixed. `make lint` reports 0 issues for both `GOOS` values in both modules, and `go fix -diff` prints nothing in either. | scratch copy, both toolchains |
| G2 | **Five test literals use a Go 1.27 language feature.** `gates_test.go` (`:440`, `:444`, `:449`, `:454`, `:461`) sets the fields of the embedded `gatedMethod` directly in `valueGate` literals. go1.26.9: "unknown field wire in struct literal of type valueGate". go1.27.2's `vet` at the 1.26.9 line: "use of promoted field … requires go1.27 or later". `95d66e3` ([0019-PLAN-idioms-performance.md](0019-PLAN-idioms-performance.md), step 7) introduced them through `go fix`'s `embedlit` modernizer. Naming `gatedMethod:` in the five literals makes the root tests compile on go1.26.9. | scratch copy |
| G3 | `TestCheckAcceptsRepository` (`cmd/generate/main_test.go:315`) type-checks the root package with its tests, so it fails on go1.26.9 for G2's reason. | scratch copy |
| G4 | **Two test fixtures write `go 1.27.2` into their own `go.mod`** (`cmd/generate/internal/mutate/mutate_test.go:92`, `:287`). On go1.26.9 their `go test` refuses to run: "go.mod requires go >= 1.27.2". With the fixtures at `1.26.9`, every `cmd/generate` package passes on go1.26.9. | scratch copy |
| G5 | **The leak guard panics on go1.26.9.** On Go 1.26, the `goroutineleak` profile exists only with `GOEXPERIMENT=goroutineleakprofile` (go.dev/doc/go1.26). Without it, `pprof.Lookup` returns nil, and `waitForLeaks` (`leak_test.go:35`) dereferences it after the tests pass. With the experiment set, the guard works on go1.26.9: a planted leaking pair gives "goroutineleak profile: total 8", as on go1.27.2. On go1.27.2 that setting is an error: "go: unknown GOEXPERIMENT goroutineleakprofile". | scratch copy |
| G6 | **The JSON backend differs between the two versions by default.** go1.27.2 builds with `goexperiment.jsonv2`; go1.26.9 does not (`go list -f '{{join context.ToolTags " "}}' encoding/json`). The root suite, goldens included, passes on go1.26.9 with G2 fixed, so the wire bytes hold on both backends, as CI's `GOEXPERIMENT=nojsonv2` job already checks on 1.27. | scratch copy |
| G7 | **CI tests one Go version.** `actions/setup-go` reads `go-version-file: go.mod` (`ci.yml:32-34`), so at a `1.26.9` line CI would test the floor only, not 1.27. | `ci.yml` |
| G8 | **0002's reason holds at 1.26.9.** [0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md](0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md) pinned `go 1.27.2` because "Go 1.27.2 carries the security fixes that 1.27.1 does not". go.dev/doc/devel/release: go1.26.9, released 2026-10-08 like go1.27.2, "includes security fixes to the go command, and the crypto/tls, html/template, net/http, net/textproto, and os packages", the same list. Go 1.26 is one of the two supported releases. | release history |
| G9 | **The README does not state a minimum Go version.** `AGENTS.md` calls the `go` line the language floor. | `README.md` |

#### API, errors and logging (A)

| Id | Finding | Evidence |
| :--- | :--- | :--- |
| A1 | **A panicking handler, or a nil `Agent` or `Client`, ends the process.** Handlers run on the SDK's goroutines with no `recover` (0 in non-test code). The constructors accept a nil agent or client. Probes, each in a child process: a raw handler that panics exits 2 with "panic: handler bug". `NewAgentSideConnection(nil, …)` answering `initialize` exits 2 with a nil pointer dereference at `agent_gen.go:340`. No record or doc states either. | probe |
| A2 | **A handler's error text reaches the peer verbatim.** Any error that is not a `*RequestError` is answered `-32603` with `{"error": err.Error()}` (`errors.go:241`; `-32800` at `:239`). An error carrying a local path or a URL with a token is sent as it is. Neither the godoc of `Agent` and `Client` nor the README's "Errors on the wire" says so. | `errors.go:228-242` |
| A3 | **Two log lines carry raw peer bytes.** These are the exceptions to B15: the `$/cancel_request` `requestId` that cannot be canonicalised (`connection.go:1536`) and an inbound id that cannot (`:1265`), logged at Error as `string(…)`. The value can be any JSON up to the 10 MiB line limit. | `connection.go` |
| A4 | **Peer-caused faults log at Error** (`connection.go:1064`, `:1190`, `:1197`, `:1207`, `:1398`, `:1422`, `:1526`, `:1530`, `:1536`), with no rate limit. Every connection end logs at Info (`:1466`). The default logger is `slog.Default()`. | `connection.go` |
| A5 | **No concurrency contract in the docs.** `Connection`, `AgentSideConnection`, `ClientSideConnection`, `Agent` and `Client` say nothing on concurrent use. `go doc -all .` mentions goroutines only in option docs. Handlers do run concurrently, one goroutine per request with notifications in order; only `docs/architecture.md` and `MethodHandler`'s doc say so. | `go doc -all .` |
| A6 | **The errors a method returns are not documented on it.** Of 50 exported error-returning functions and methods, only `Close` names its errors. The contract lives on the sentinels and in the README. | audit script |
| A7 | **370 of the 378 exported types in `types_gen.go` have a doc comment that does not start with the type's name.** The comment is the schema's `description`, as [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md) maps it. 41 constants in `constants_gen.go` rely on a group comment. The hand-written files all comply. | go/ast count |
| A8 | **No Go quickstart.** `doc.go` is five lines, with no happy path. The README's first Go block is at line 199, a `Patch` fragment. There are three `Example` functions, all package-level, two with `// Output:`, and none per symbol. | `doc.go`, `README.md` |
| A9 | **One mutable package-level test seam.** `var promptReturned func()` (`agent_gen.go:128`) is read in every prompt turn and set only by `cancel_test.go:165`. It came from [0017-PLAN-remediate-acp-v1-conformance-violations.md](0017-PLAN-remediate-acp-v1-conformance-violations.md). | `agent_gen.go:125-128` |
| A10 | **Option semantics are partly undocumented:** that a repeated option's last value wins; that `n <= 0` leaves the queue and message-size options unchanged but resets `WithMaxConcurrentRequests`; that a nil option panics. | `connection.go:393-456`, `:602` |
| A11 | **Wire-schema types beside the runtime's own model.** The ten JSON-RPC envelope types (`AgentRequest` … `ClientError`), `Error` and `ErrorCode` (an `int32`, where `RequestError.Code` is an `int`) and `CancelRequestNotification` are exported. The runtime uses none of them. Removing them breaks the API within v1. | `types_gen.go:205`, `:5892`, `:1374` |
| A12 | **`SendRequest` takes `ctx` second.** This is recorded: "Moving `ctx` first would be a breaking change for every caller" ([0003-MADR-install-fleet-agent-workspace.md](0003-MADR-install-fleet-agent-workspace.md)). | `connection.go:1654` |
| A13 | **Concurrent inbound handlers are unbounded by default.** A peer can start any number of handler goroutines unless the application sets `WithMaxConcurrentRequests`. This is recorded ([0019-PLAN-api-decisions.md](0019-PLAN-api-decisions.md), B18). | `connection.go:443-449` |
| A14 | **Validation messages quote the value.** 55 `ValidationError` sites put the path or discriminator they refuse into the message, e.g. `McpServerStdio: command: %q is not an absolute path`. The message reaches the peer and Error-level logs. A secret placed in `command` would follow it. | `types_gen.go` |

Annotated 2026-10-19 by [0024-PLAN-behaviour-fixes.md](0024-PLAN-behaviour-fixes.md), step 1, which re-read the code on
`12dd249`. Three findings above are corrected:

* **A3.** Three lines, not two, log an id that could not be canonicalised as `string(…)`. The response id's line, now
  `connection.go:1506`, is the third. The inbound and response ids have passed `validID`, so they are JSON strings or numbers; the
  `$/cancel_request` `requestId` can be any JSON value.
* **A4.** Eleven lines, not nine, log a fault of the peer's message at Error: the nine listed, now `connection.go:1074`, `:1200`,
  `:1207`, `:1217`, `:1408`, `:1432`, `:1536`, `:1540` and `:1546`, and the inbound and response id lines, `:1275` and `:1506`. A
  notification handler's error, `:1591`, is the peer's fault when the dispatch refused the notification.
* **A14.** 24 of the 55 `ValidationError` sites quote the value they refuse: 18 absolute-path checks and 6 excluded-value checks.
  55 is every site; the other 31 do not quote it.

#### Versioning, packaging and process (V)

| Id | Finding | Evidence |
| :--- | :--- | :--- |
| V1 | **No policy for the unstable API under v1.** 79 exported `Unstable*` identifiers come from `schema.unstable.json`. [0019-MADR-remediate-debugging-pass-findings.md](0019-MADR-remediate-debugging-pass-findings.md), question 16, left "the policy for removing unstable exported API under `v1`" to a record of its own, not yet written. | `go doc -short .` |
| V2 | **No automated API-compatibility check.** The API listing (`cmd/generate/internal/enginetest/api.go:104`) is run by hand for each release's notes. No CI job, Makefile target or `RELEASING.md` step compares the API with the last tag. | `ci.yml`, `Makefile`, `RELEASING.md` |
| V3 | **No deprecation mechanism.** There are 0 `Deprecated:` comments, and the generator has no path to write one. Nothing is deprecated today. | `grep` |
| V4 | **No `CHANGELOG`.** The release notes in `docs/reports/` and the GitHub releases serve as one. The owner's workspace policy lists `CHANGELOG.md` under "Do not add", as a "fleet-wide decision, not a per-repo default" (`~/.claude/skills/workspace-scaffolding/references/do-not-add.md`). `RELEASING.md` names neither the notes reports nor the API listing. | `RELEASING.md` |
| V5 | **One package, by design.** `acp` holds 2,830 hand-written and 24,885 generated lines, with no sub-package. Moving any part into one breaks the API within v1 unless the root keeps aliases. | line counts |

Annotated 2026-10-20 by "Phase 4 scoped" below, R11: V2's evidence is corrected. The listing each release's notes cite is the API listing
program of 0011's plans, run by hand and kept outside the tree. `enginetest.API` (`cmd/generate/internal/enginetest/api.go:104`) is
another listing, in the tree, that the engine cases compare.

#### Found while assessing, belonging to another record

* **0023's leak guard can miss a leak in the last test it runs.** `waitForLeaks` returns as soon as the profile counts 0. If the only
  leaking test is the last to run, its goroutines may not have blocked yet. In a scratch copy, a planted leaking test, run alone 30
  times on go1.27.2, was caught 12 times and missed 18. Step 2's planted breaks each ran once, and so did not show it.
* This is a defect found in that PLAN's live run, so by `AGENTS.md` it amends
  [0023-PLAN-connection-lifetime-and-test-leaks.md](0023-PLAN-connection-lifetime-and-test-leaks.md), not this record. It is listed
  here because G5's fix touches the same function.

Annotated 2026-10-20: resolved in that record. Its step 6 (`805d997`) gave `waitForLeaks` a settle window, `settleWindow`
(`leak_test.go:37`), and shipped in `v1.16.2`.

## Decision Drivers

* **The owner's floor:** `go 1.26.9`, with both supported Go releases, 1.26 and 1.27, tested.
* **Compatibility within v1:** only additive API changes. A behaviour change is named in the release notes.
* **No new dependency** without a record naming it (`AGENTS.md`, "Dependencies").
* **Records already decided** stand unless the owner reopens them: 0003 (`SendRequest`), 0012 (generated docs), 0017 (the test seam),
  0019 B17 and B18 (`Close`, the request bound), and the fleet policy on `CHANGELOG.md`.
* **Documentation fixes are cheap and safe.** They change no bytes and no API, and they close most of the gaps.

## Considered Options

* **P1.** One program under this record, in phases: the floor first, then documentation, then the behaviour fixes, then the process
  gates, each phase its own PLAN under this number, its own commit set and its own release.
* **P2.** Documentation only: the floor and every doc gap, leaving A1 to A4 and V2 as they are.
* **P3.** A `/v2` cleanup that also removes A11's types, fixes A12's order, and moves the unstable API.

## Decision Outcome

Chosen option: "P1", decided by the owner on 2026-10-19.

* It applies the floor the owner set, and the fixes the practices call for, without breaking v1.
* P2 leaves the crash on a nil argument (A1) and the raw peer bytes in logs (A3) in place.
* P3's gains are mostly cosmetic within an SDK whose callers are on v1. A12 and A11 are recorded or harmless, so a major version is not
  justified by them.

The phases, each subject to the questions below:

1. **The floor (G1 to G9).**
   * Both `go` lines become `1.26.9`.
     Amended while executing [0024-PLAN-go-1-26-9-floor.md](0024-PLAN-go-1-26-9-floor.md), before step 2, 2026-10-19: both
     modules also gain `toolchain go1.27.2`, the owner's preferred version. It applies only where this module is the main
     module. Consumers still need 1.26.9.
   * The five literals of G2 name `gatedMethod:`. The two fixtures of G4 write the floor.
   * CI runs a matrix of the floor, `1.26.9`, and the latest stable release. It keeps `GOEXPERIMENT=nojsonv2` on the latest and sets
     `GOEXPERIMENT=goroutineleakprofile` on the floor. Decided under question 8: the newest version is pinned, `1.27.2`, and a CI
     step checks that the matrix's floor equals the `go` line.
   * The leak guard handles an absent profile (question 2).
   * The README states the minimum Go version.
   * 0002's `go 1.27.2` is superseded by this record. Its schema decision stands.
2. **Documentation (A5 to A8, A10, A11, V4).**
   * A concurrency contract on the five main types.
   * One generated paragraph per typed method naming the errors it returns.
     Amended while executing [0024-PLAN-documentation.md](0024-PLAN-documentation.md), during step 2, 2026-10-19: the four
     extension methods, `CallExtension` and `NotifyExtension` of each side, gain a hand-written paragraph naming theirs. They send
     before initialize, so it does not name `ErrNotInitialized`.
   * A package overview with a happy path in `doc.go`, a short Go quickstart in the README, and per-symbol `Example` functions with
     `// Output:`.
   * The option semantics.
   * Doc notes on the wire-schema types and on `ErrorCode` against `RequestError`.
   * `RELEASING.md` naming the release notes and the API listing.
   * Question 4 decides the generated type comments.
3. **Behaviour (A1 to A4, A14).** As questions 1, 3 and 5 decide.
4. **Process (V2, V3).** An API-compatibility gate in CI from the existing listing, and the generator's path to `// Deprecated:`, as
   questions 6 and 7 decide. Decided under question 6: the gate is `gorelease`, not the existing listing.
   Scoped 2026-10-20 in "Phase 4 scoped" below, with questions 9 to 13, which the owner decided the same day.

V1, the unstable API's policy, stays with the record question 16 of 0019 calls for, which a schema `1.26.0` update will need. A9, A12
and A13 stay as recorded.

### Consequences

* Good, because consumers on Go 1.26, a supported release, can use the SDK. Today `go 1.27.2` excludes them, and nothing in the
  library's code needs 1.27 (G1).
* Good, because CI then tests both supported Go releases and both JSON backends by default, not one.
* Good, because documentation closes most gaps with no API or wire change.
* Good, because a nil argument fails at the call that passed it, not on a later goroutine.
* Neutral, because developers on Go 1.26 must set `GOEXPERIMENT=goroutineleakprofile` for the leak guard to run, or see it reported as
  skipped, as question 2 decides.
* Bad, because the floor commits the project to two Go versions in CI until 1.26 leaves support, when the floor moves with a record.
* Bad, because modernizers that need Go 1.27, such as `embedlit`, can no longer be applied. `go fix` and `modernize` follow the `go`
  line, so they stop proposing them.

### Confirmation

Each phase's PLAN names its checks, each seen to fail on a planted break in a scratch copy. At least:

* go1.26.9 and the latest release both pass `go build`, `go vet` and `go test ./...` in both modules, in CI.
* go1.27.2's `vet` reports no API newer than the floor.
* The leak guard runs on both versions in CI, and is seen to fail on a planted leak on each.
* The API listing against the last tag is unchanged by every phase.
* `make lint` reports 0 issues and `go fix -diff` prints nothing, at the new line.

#### Questions for the owner

Each has a recommended answer.

1. **Nil arguments and handler panics (A1).**
   * Recommended:
     * The three constructors, `NewConnection`, `NewAgentSideConnection` and `NewClientSideConnection`, panic at once, naming the
       argument, on a nil agent, client, reader or writer. `http.Handle` does the same for a nil handler: `ServeMux.register` panics
       with "http: nil handler". `NewConnection` keeps answering `-32601` for a nil handler, documented.
     * A panicking handler keeps ending the process, as any goroutine's unrecovered panic does, and the docs of `Agent`, `Client` and
       `MethodHandler` say so.
   * Alternative: recover handler panics in the SDK, answer `-32603` and log them, as `net/http` does for `ServeHTTP`. That hides bugs
     and changes behaviour for every application.
2. **The leak guard without the profile (G5).**
   * Recommended: when `pprof.Lookup("goroutineleak")` is nil, the guard fails the run with the instruction to set
     `GOEXPERIMENT=goroutineleakprofile` on Go 1.26. The Makefile's test targets set it when `go env GOVERSION` is 1.26, and so does
     CI's floor job.
   * Alternative: the guard prints that it was skipped and passes. That is quieter, and it lets a leak through on 1.26 unnoticed.
3. **A handler's error text on the wire (A2).**
   * Recommended: document it on `Agent`, `Client` and in the README's "Errors on the wire". An application that must not leak a
     message returns a `*RequestError` of its own.
   * Alternatives:
     * an additive option that maps the error to the answer's data;
     * a generic message by default, which changes the wire behaviour.
4. **Generated type comments (A7).**
   * Recommended: the generator writes `// <Name> is …` before the schema's description, from the definition's kind, so each comment
     starts with its name. This amends 0012's mapping, which keeps the description verbatim after it.
   * Alternative: keep the description alone, as 0012 decided, and leave the `revive` convention unmet in generated code.
5. **Logs (A3, A4, A14).**
   * Recommended:
     * A3's two lines log the value's size, as B15 does for messages.
     * Faults caused by the peer log at Warn, and each connection's end at Debug.
     * A14's messages stay, documented as quoting the value they refuse.
   * Alternative: keep the levels as they are, and fix A3 only.
6. **An API-compatibility gate (V2).**
   * Recommended: a CI job and a `make api` target. They run the existing listing, `enginetest.API`, on the last tag and on the tree,
     and fail on a removed line unless the PLAN of a major version allows it. No dependency is added.
   * Alternatives: `golang.org/x/exp/cmd/apidiff` or `golang.org/x/exp/cmd/gorelease`. Each is a dependency needing a record.
7. **Deprecation (V3).**
   * Recommended: the generator writes `// Deprecated:` when the schema marks a definition deprecated, with an engine case. It is
     decided now, so it is in place before an upstream deprecation needs it.
   * Alternative: decide it with V1's record.
8. **The CI matrix (G7).**
   * Recommended: `1.26.9` and the latest stable release, through `go-version` in a matrix rather than `go-version-file`.
   * Alternative: `go-version-file` plus `check-latest`, which tests one version.

#### Decided with the owner, 2026-10-19

| Question | Answer |
| :--- | :--- |
| 1. Nil arguments and handler panics | As recommended: the three constructors panic on a nil agent, client, reader or writer; a panicking handler keeps ending the process, documented. |
| 2. The leak guard without the profile | As recommended: the guard fails with the instruction to set `GOEXPERIMENT=goroutineleakprofile`; the Makefile and CI's floor job set it on Go 1.26. |
| 3. A handler's error text on the wire | As recommended: documented. |
| 4. Generated type comments | As recommended: the generator writes `// <Name> is …` before the description. This amends 0012's mapping. |
| 5. Logs | As recommended: A3's lines log the size; peer-caused faults at Warn; a connection's end at Debug; A14 documented. |
| 6. An API-compatibility gate | **Not the recommendation:** `gorelease`, from `golang.org/x/exp`, pinned to a pseudo-version, in CI and a `make api` target, on the root module. |
| 7. Deprecation | As recommended: the generator path to `// Deprecated:` now, with an engine case. |
| 8. The CI matrix | **Refined from the recommendation:** `1.26.9` and a pinned `1.27.2`, with a step that fails when the matrix's floor differs from the `go` line. |

On question 8, the owner asked for the matrix's pros and cons before deciding:

* For it: it tests both supported Go releases, so a construct like G2 fails CI at once, and each version's default JSON backend.
* Against it:
  * about twice the CI minutes;
  * a floating "latest" can turn CI red with no change here;
  * the floor in two places can drift.

Pinning `1.27.2` and checking the floor against the `go` line answer the second and third.

On question 6, the two tools were run on 2026-10-19 on scratch clones, at `golang.org/x/exp@v0.0.0-20261014143641-f0251eb4aa3e`, the
module's latest; it has no tagged version.

* **The tree against `v1.16.1`:** `gorelease -base=v1.16.1` exited 0 and suggested `v1.16.2`; `apidiff -m -incompatible` printed
  nothing.
* **With `ExtRequest` removed, committed in a scratch clone:**
  * `gorelease` printed "ExtRequest: removed" and "Incompatible changes were detected", and exited 1;
  * `apidiff -m -incompatible` printed "- ExtRequest: removed" and **exited 0**, so a gate on it would have to parse its output.
* **What gorelease needs:** a clean working tree ("repo … has uncommitted changes" otherwise) and the base version from the module
  proxy.

Both run as `go run <path>@<version>`, as `govulncheck` does, so neither changes a `go.mod`. This record names `gorelease` as a tool
dependency, as `AGENTS.md`, "Dependencies", requires. It describes itself as "an experimental tool".

#### Phase 4 scoped, 2026-10-20 (decided the same day)

The owner asked on 2026-10-20 for phase 4 to be scoped and specified, every claim proven. This section holds the findings, the
specification they lead to, and the questions they raise. It changes no decision above until the owner answers questions 9 to 13.

**How it was assessed.** The code was read on `b463216`, the `v1.17.0` publication record. Every probe ran in scratch clones of the
committed history or scratch copies of the tree, with the module cache, build cache and `GOPATH` in the scratch directory, and
`GOPROXY` the default with the checksum database on. Nothing in the tree or in `$HOME` was written. The upstream schemas, protocol
docs and `schemars_derive` source were downloaded into scratch and only read.

##### The gate, `gorelease` (V2)

| Id | Finding | Evidence |
| :--- | :--- | :--- |
| R1 | **The pin is still the latest.** `golang.org/x/exp@latest` is `v0.0.0-20261014143641-f0251eb4aa3e` (2026-10-14T14:36:41Z), the pseudo-version probed for question 6. The module has no tagged version. Its `go` line is `1.26.0`, so it builds on the floor. It requires `golang.org/x/mod v0.42.0`, `golang.org/x/tools v0.52.0`, `github.com/google/go-cmp v0.6.0` and `golang.org/x/tools/go/packages/packagestest`. The pinned commit is itself an `apidiff` change, the engine `gorelease` uses ("apidiff: require exported old types to match same-named type in new"). `cmd/gorelease` was last changed on 2026-08-24 (`e88cd73687aa`). | `go list -m -json`, `go mod download -json`, the GitHub commits API |
| R2 | **Run with no flags, it is the gate.** For a commit after `v1.17.0`: no API change exits 0, "Suggested version: v1.17.1". An exported function added exits 0, listing "ProbeAdded: added" under "compatible changes", "Suggested version: v1.18.0". A method removed exits 1, listing "(*ClientSideConnection).UnstableDidCloseDocument: removed" under "incompatible changes", "Incompatible changes were detected". A method added to `Agent` exits 1, "Agent.ProbeMethod: added" under "incompatible changes": an interface that gains a method breaks its implementers. | scratch clones, each change committed |
| R3 | **The base comes from the module proxy.** "Inferred base version: v1.17.0" in a full clone, in a clone like CI's checkout (`fetch-depth: 1`, no tags, `ci.yml:43`), and under go1.26.9. Given `-base=` an older tag and no `-version`, it exits 1: "Can only suggest a release version when compared against the most recent version of this major: v1.17.0". The gate therefore passes no `-base`. | scratch clones |
| R4 | **At a tag, it compares the tag with itself.** Checked out at `v1.17.0`, full or as a tag push fetches it, it infers base `v1.17.0` and exits 0. A tag-push run of the gate is vacuous; the runs on `main` and on pull requests carry it. | scratch clones |
| R5 | **`-version` checks a proposed tag.** After an addition, `-version=v1.17.1` exits 1: "There are compatible changes, but the minor version is not incremented over the base version (v1.17.0)"; `v1.18.0` exits 0. With no API change it accepts `v1.17.1`, `v1.18.0`, `v1.17.2` and `v1.17.1-rc.1`. It refuses `v2.0.0`, "The module path does not end with the major version suffix /v2", and an existing tag, "version v1.17.0 already exists". With the `go` line raised to `1.27.2` it suggests `v1.18.0` but accepts `-version=v1.17.1`, so it does not guard the floor; CI's step "the floor is the go line" does. | scratch clones |
| R6 | **It needs a clean tree.** A modified tracked file and an untracked file each exit 1: "repo … has uncommitted changes". An ignored directory (`.gocache`) does not. It cannot run inside `make pre-add-check`, which runs before anything is staged; it runs on a commit. | scratch clone |
| R7 | **It needs the network.** With `GOPROXY=off` and the module cache warm, `go run <pin>` exits 1: "loading deprecation for golang.org/x/exp: module lookup disabled by GOPROXY=off". | scratch clone |
| R8 | **The generator's module has nothing to compare.** No tag has the prefix `cmd/generate/`, and the proxy lists no version of `github.com/maccavelli/acp-go-sdk2/cmd/generate`. There, `gorelease` prints "Inferred base version: none" and suggests `cmd/generate/v0.1.0`. Question 6's "on the root module" stands. | `git tag -l`, `go list -m -versions`, scratch clone |
| R9 | **On this module's history it agrees with the listing.** Replayed at each of the eight releases from `v1.15.0` to `v1.17.0` against the one before: 0 incompatible changes in each. `v1.15.0` has 3 compatible changes, `ElicitationSchema.PropertyOrder`, `UnstableMcpError.Extra` and `UnstableMcpError.MarshalJSON`, the three lines its notes record from the listing ([0019-REPORT-release-notes-v1-15-0.md](../reports/0019-REPORT-release-notes-v1-15-0.md)). The other seven have none, as their notes say. Each run exited 1 for R3's reason; the count is read from its sections. | scratch clones at each tag |
| R10 | **A schema `1.26.0` update would fail the gate.** Regenerating from upstream's `schema-v1.26.0` stops first at the generator's type check: hand-written tests use unstable types it removes ("acp_test.go:145:54: undefined: UnstableDidChangeDocumentNotification", "and 20 more"), and so does `example/agent`. With the 27 test files the type check named removed from the scratch clone, `gorelease` exits 1 with 132 incompatible changes and 16 compatible. 40 of the 132 name no `Unstable` identifier, among them `AgentCapabilities.Nes`, `ClientCapabilities.PositionEncodings`, `AgentMethodNesStart`, `NesCapabilities` and `PositionEncodingKind`. A rule for "`Unstable*` identifiers" would not cover them. | scratch clone, upstream release files |
| R11 | **The listing the release notes cite lives outside the tree.** Each of the eight notes reports from `v1.15.0` to `v1.17.0` names "the 0013 report's listing program", run by hand on `git archive` of the last tag. On 2026-10-20 its scratch copy had been emptied, and was recovered from the session's record ([0024-PLAN-behaviour-fixes.md](0024-PLAN-behaviour-fixes.md), step 6). `enginetest.API` (`cmd/generate/internal/enginetest/api.go:104`) is in the tree and serves the engine cases. | `RELEASING.md:103`, the notes reports |
| R12 | **`RELEASING.md` shows a lightweight tag.** `RELEASING.md:85` reads `git tag vX.Y.Z`. Every tag from `v1.16.0` to `v1.17.0` is annotated (`git cat-file -t` prints `tag`). | `RELEASING.md`, `git cat-file` |

##### Deprecation (V3)

| Id | Finding | Evidence |
| :--- | :--- | :--- |
| D1 | **No published schema marks anything deprecated.** In each of the four files of the committed `1.25.0`, upstream `1.26.0` and upstream `2.0.0-alpha.9`: 0 `deprecated` keys, and 0 descriptions that mention deprecation. | a walk of every JSON value |
| D2 | **Upstream deprecates in prose.** At `schema-v1.26.0`: `docs/protocol/v1/initialization.mdx:261`, "Note: This transport has been deprecated by the MCP spec." (SSE); `docs/protocol/v1/session-setup.mdx:488`, "This transport was deprecated by the MCP spec."; `docs/rfds/session-config-options.mdx:233`, "We would also likely move session modes to be `@deprecated` in favor of this approach"; `docs/protocol/v2/migration.mdx:795`, "The deprecated HTTP+SSE transport (`"type": "sse"`) is **removed**". The schema's `McpServerSse` says only "SSE transport configuration for MCP." | the GitHub contents API |
| D3 | **Upstream's schema writer would emit the keyword.** `Cargo.lock` at `schema-v1.26.0` locks `schemars` and `schemars_derive` `1.2.2`. In `schemars_derive-1.2.2`, `src/attr/mod.rs:79` sets `deprecated` from a `#[deprecated]` attribute, and `:213`–`:215` insert `"deprecated": true`. The mutator is applied to containers (`src/schema_exprs.rs:113`, `:160`), fields (`:201`) and enum variants (`:339`, `:366`, `:461`, `:600`): a definition, a property, or a union variant. | crate source |
| D4 | **JSON Schema defines it as an annotation.** 2020-12 validation §9.3: "The value of this keyword MUST be a boolean." "If "deprecated" has a value of boolean true, it indicates that applications SHOULD refrain from usage of the declared property. It MAY mean the property is going to be removed in the future." "Omitting this keyword has the same behavior as a value of false." | json-schema.org |
| D5 | **The generator refuses it today.** With `"deprecated": true` added in a scratch copy, `go run .` exits 1 for each placement: on `McpServerSse`, "definition "McpServerSse": json: unknown field "deprecated""; on `McpCapabilities.sse`, "definition "McpCapabilities": …"; on `McpServer`'s `sse` variant, "definition "McpServer": …". The loader decodes strictly (`cmd/generate/internal/load/load.go:95`, `decodeStrict`), so an upstream deprecation stops `make version` until the generator models the keyword. | scratch copies |
| D6 | **Modelling a keyword has three parts.** A field of `load.Definition`; a treatment in the registry (`cmd/generate/internal/model/keywords.go`), which `TestKeywordRegistryMatchesLoader` keeps equal to the loader's fields; and a row of the keyword table of [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md). | `keywords.go`, `keywords_test.go` |
| D7 | **Descriptions reach 13 kinds of doc.** The model calls `docLines` on a schema description 13 times: definitions (`model.go:424`), enum and constant values (`:518`, `:561`, `:570`), nested structs (`:612`), inline variants (`:960`), flattened members (`:1052`), fields (`fields.go:529`, `:578`), inline shapes (`:596`, `:615`, `:617`) and methods, from their request definition (`methods.go:213`). A helper constructor's doc (`helperDoc`, `cmd/generate/internal/emit/helpers.go:61`) is built from the variant's discriminator, not its description, so a variant's deprecation must reach it as a flag, not as text. | `grep` |
| D8 | **Engine cases compare no doc comment.** A case holds `want.api`, the exported API as `enginetest.API` lists it, or `want.err` (`cmd/generate/engine_test.go:72`–`:73`). A deprecation case needs a section of its own. | `engine_test.go` |
| D9 | **Go's convention is a paragraph.** go.dev/doc/comment: "Paragraphs starting with Deprecated: are treated as deprecation notices. … Deprecation notices are followed by some information about the deprecation, and a recommendation on what to use instead, if applicable. The paragraph does not have to be the last paragraph in the doc comment." The Go wiki adds: "In general, if an API still works and does not have significant downsides, we do not formally mark it as deprecated". | go.dev |
| D10 | **staticcheck honours only that form.** Five functions in a scratch copy, called from another package: SA1019 reported the notice as its own paragraph ("Deprecated: use X." after a blank comment line), and none of "Deprecated:" with no blank line before it, "Deprecated:" mid-line, "deprecated:" and "Deprecated," (1 issue). | scratch copy, `golangci-lint` v2.14.0, the repository's config |
| D11 | **It reports uses from other packages only.** With notices on the generated `McpServerSse`, `McpServer.Sse`, `McpCapabilities.Sse` and `NewMcpServerSse`, `golangci-lint` with the repository's config, run at the root as `make lint` runs it, reports 0 issues for both `GOOS` values: the generated code and the root's tests use them inside the package. A package of the module that uses them gets three SA1019 reports, one each for the type, the field and the constructor. The examples are packages of their own, so a deprecation of something they use must change them in the same commit. The three doc tests (`TestExportedDeclarationsDocumented`, `TestExportedTypeDocsStartWithName`, `TestTypedMethodDocsNameErrors`) pass with the notices, and `go doc McpServerSse` shows the paragraph last. | scratch copy |
| D12 | **A plain `bool` field accepts `null`.** Decoded strictly into a struct with `Deprecated bool`, `"yes"` and `1` are refused ("json: cannot unmarshal string into Go struct field definition.deprecated of type bool", and "number"), but `null` decodes as `false` with no error. The same on go1.27.2's default backend and with `GOEXPERIMENT=nojsonv2`. 2020-12 says the value MUST be a boolean (D4). | scratch program, both backends |
| D13 | **Nothing hand-written is deprecated, and two records expect it may be.** `git grep -i deprecated -- '*.go' '*.txtar'` finds nothing (exit 1). [0022-PLAN-keep-elicitation-property-order-before-v1-15-0.md](0022-PLAN-keep-elicitation-property-order-before-v1-15-0.md): "After the tag, `PropertyOrder` stays within `v1`; it can be deprecated, not removed." [0021-PLAN-resolve-issues-4-and-5-before-v1-14-0.md](0021-PLAN-resolve-issues-4-and-5-before-v1-14-0.md): "step 3's option can be deprecated but not removed." | `git grep`, records |

Annotated 2026-10-20, during step 5 of [0024-PLAN-api-gate-and-deprecation.md](0024-PLAN-api-gate-and-deprecation.md): of D10's
four ignored forms, the repository's lint already reports three, through gocritic's `deprecatedComment`: no blank line before
it, lower case, and a comma. The mid-line form passes it. Question 13's answer stands, by the owner's decision the same day.

##### Specification, as recommended

**The gate (V2), as question 6 decided it:**

* **CI.** One step, "API compatibility (gorelease)", in the `validate` job, `if: runner.os == 'Linux' && !matrix.floor`, as the
  lint step is. It runs `go run golang.org/x/exp/cmd/gorelease@v0.0.0-20261014143641-f0251eb4aa3e` with no flags, from the root
  (R2, R3). It sits directly after `go mod download` (`ci.yml:55`), so no step before it has written the tree (R6). It runs on every
  push to `main`, every pull request and the weekly schedule; on a tag push it passes trivially (R4).
* **`make api`.** `GORELEASE_PIN` is read from `ci.yml`, as `GOVULNCHECK_PIN` is (`Makefile:27`), so the pin has one home. The target
  fails when the pin is missing, runs the gate with no flags, and adds `-version=$(TAG)` when `TAG` is set. `TAG`, not `VERSION`,
  because `make release VERSION=` already names a schema version. It is not part of `make pre-add-check` (R6). Offline it fails with
  R7's message.
* **`RELEASING.md`.**
  * Before tagging, `make api TAG=vX.Y.Z` exits 0 on the commit to be tagged.
  * Each notes report records `make api`'s compatible and incompatible changes and its suggested version, in place of the listing
    program outside the tree (R11). `enginetest.API` stays for the engine cases.
  * The tag command becomes `git tag -a vX.Y.Z -m vX.Y.Z`, the practice since `v1.16.0` (R12).
* **No override.** A removal or an interface method added fails CI. What a record decides for it is question 12.

**Deprecation (V3), as question 7 decided it, with the scope of questions 9 to 11:**

* **The keyword.** `load.Definition` gains `Deprecated bool` (`json:"deprecated"`), the registry gives it `TreatAnnotation`, and
  0012's table is annotated with the row (D6). A value that is not `true` or `false` is refused, `null` included, which a
  plain `bool` would accept (D12); `false` is the same as absent (D4).
* **Where a notice goes.** Each Go declaration made from a deprecated node carries one:

  | Schema node | Go declarations |
  | :--- | :--- |
  | a definition | its type; for a request or notification definition, the method of its interface and of its side's connection, whose docs come from it (D7) |
  | a property | its struct field; for an inline object property, its nested type too |
  | a union variant | the union's field, the variant's inline type, and its helper constructor; for a constant variant, the constant |

  Amended 2026-10-20, during step 4 of [0024-PLAN-api-gate-and-deprecation.md](0024-PLAN-api-gate-and-deprecation.md), by the
  owner: a direct `anyOf` or `oneOf` member that becomes a flattened member marks that member's field; an enum kind's member with
  no `const`, from which no declaration is built, is refused.

* **The notice.** A paragraph of its own, after a blank comment line (D10), as question 10 decides its text.
* **No change today.** The committed schema marks nothing (D1), so every generated file stays byte-identical and the API listing
  unchanged; `TestGeneratedFilesUpToDate` holds that.
* **Tests.**
  * The engine harness gains an optional `want.deprecated` section: every generated declaration with a notice, and its text, compared
    exactly, so a missing or an extra notice fails (D8).
  * Engine cases: a definition, a property, a variant, a constant variant and a method's definition, each deprecated; `deprecated:
    false`; and a string and a `null`, each refused, naming the definition and the keyword.
  * At the root, a test that every comment line of a non-test file whose text starts with "deprecated", in any case, is a notice in
    D10's form, so a hand-written one that tools would ignore fails (question 13).
* **Release.** Phase 4 changes no exported API and no behaviour, so `gorelease` would suggest a patch, `v1.17.1`. The owner confirms
  it at the tag.

##### Questions for the owner

9. **Where `deprecated` is honoured.** Question 7 says "when the schema marks a definition deprecated".
   * Recommended: every node `schemars` marks, a definition, a property or a union variant (D3), with the declarations of the table
     above.
   * Alternative: definitions only. A deprecated property or variant is then still refused, and stops `make version` (D5).
10. **The notice's text.** The schema carries a boolean, not a reason or a replacement (D4), where Go's convention asks for "a
    recommendation on what to use instead, if applicable" (D9).
    * Recommended: a fixed sentence naming the node, such as "Deprecated: the ACP schema marks the "sse" variant of the McpServer
      schema definition deprecated.", followed by an optional note from the model naming the replacement. A note for a node the schema
      does not mark is refused, as the model's other notes are.
    * Alternative A: a note required for every deprecated node, the generator refusing one without it. Every upstream deprecation then
      waits for a person to write it.
    * Alternative B: the fixed sentence only.
11. **Methods.** A deprecated request or notification definition is the method's own deprecation upstream.
    * Recommended: its interface method and its side connection's method carry the notice too (table above). Callers in other
      packages are then warned (D11); implementers are not, as SA1019 reports uses, not implementations.
    * Alternative: types only.
12. **What the gate does with an intended break.** R10 shows the next schema update makes 132 incompatible changes, 40 of them to
    names without `Unstable`, and R2 that a new stable method on `Agent` is incompatible too.
    * Recommended: phase 4 adds no override. A change `gorelease` calls incompatible fails CI until a record decides it: the record
      that question 16 of [0019-MADR-remediate-debugging-pass-findings.md](0019-MADR-remediate-debugging-pass-findings.md) calls
      for, for schema `1.26.0`, or a `/v2`. A new stable method can avoid the break by binding to an optional interface, as
      `session/load` binds to `AgentLoader`.
    * Alternative: a committed list of expected incompatible lines that CI compares with `gorelease`'s output. That parses its output,
      the cost question 6 held against `apidiff`.
13. **Hand-written notices.**
    * Recommended: the root test of the specification, so a notice in a form tools ignore (D10) fails the build.
    * Alternative: review alone.

##### Decided with the owner, 2026-10-20

| Question | Answer |
| :--- | :--- |
| 9. Where `deprecated` is honoured | As recommended: a definition, a property or a union variant, with the declarations of the specification's table. This widens question 7, which named definitions. |
| 10. The notice's text | As recommended: the fixed sentence naming the node, then an optional model note naming the replacement; a note for a node the schema does not mark is refused. |
| 11. Methods | As recommended: a deprecated request or notification definition also marks its interface method and its side connection's method. |
| 12. An intended break | As recommended: no override. A change `gorelease` calls incompatible fails CI until a record decides it. |
| 13. Hand-written notices | As recommended: the root test. |
| `RELEASING.md`'s tag command | As recommended: phase 4 changes it to `git tag -a vX.Y.Z -m vX.Y.Z` (R12). |

The specification above stands as written, and [0024-PLAN-api-gate-and-deprecation.md](0024-PLAN-api-gate-and-deprecation.md) implements it.

Corrected 2026-10-20, while drafting that PLAN: question 10's "as the model's other notes are" is not what the code does. A doc
note on a definition the schema lacks applies nothing at generation (`DocNote`, `cmd/generate/internal/model/protocol.go:65`),
and `checkCommittedDocNotes` (`cmd/generate/internal/model/patch_test.go:273`) requires each to name a committed definition. A
deprecation note is refused at generation, as decided, which is stricter.

Corrected the same day: D7's "methods, from their request definition (`methods.go:213`)" is narrower than the code. A method's
doc is the description of its notification, request or response definition, the first that has one (`methods.go:210`-`:216`).
Question 11's answer is unchanged: a method's notice comes from its request or notification definition being marked.

##### Consequences, if the recommendations are accepted

* Good, because an incompatible change fails CI at the push, not at the next release's hand-run listing, and the comparison is
  reproducible from a pin rather than a program outside the tree (R11).
* Good, because an upstream deprecation no longer stops `make version` (D5) and reaches callers as SA1019 (D11).
* Neutral, because `make api` needs a clean tree and the network (R6, R7), so it runs after a commit, as `make push-guard` does.
* Neutral, because a tag-push run of the gate checks nothing (R4).
* Bad, because the schema `1.26.0` update cannot land until its record decides the removals (R10), which until now nothing enforced.
* Bad, because the tool is experimental and untagged (R1); a pin update is a reviewed change to `ci.yml`.

##### Confirmation

The phase 4 PLAN names its checks. Each is seen to fail on a planted break in a scratch clone or copy, at least:

* the gate step, and `make api`, exiting 1 on a committed removal and on a method added to `Agent`, and 0 on a committed addition;
* `make api TAG=v1.17.1` exiting 1 on a committed addition;
* each engine case failing when its notice, or its refusal, is removed from the generator;
* the root notice test failing on each of D10's four ignored forms;
* `actionlint` and `make check-records` clean, and the API listing against `v1.17.0` unchanged.

## Pros and Cons of the Options

### P1. A phased program under this record

* Good, because each phase is small, reviewable and released on its own, as 0019's phases were.
* Good, because the floor lands first, so every later phase is tested on both Go versions.
* Neutral, because it is several PLANs and releases over time.
* Bad, because behaviour changes in phase 3 (A1's constructor panic, A4's log levels) must each be named in a release's notes.

### P2. Documentation only

* Good, because it changes no behaviour at all.
* Bad, because it leaves A1, A3 and V2 open. A3 contradicts B15's own rule.

### P3. A `/v2` cleanup

* Good, because it could remove A11's types, put `ctx` first in `SendRequest`, and move the unstable API out of the root.
* Bad, because every caller must change its import path for gains that are mostly cosmetic.
* Bad, because 0003 kept `SendRequest`'s order to spare every caller a break, and 0021 requires `/v2` for any incompatible change,
  so the cleanup needs a new module path.

## More Information

* **Not applicable from the list (section 9):** credential sources, token refresh, signed updates, atomic replacement and asset
  selection. This SDK neither authenticates to a provider nor updates itself. ACP's `authenticate` carries only a method id
  (`AuthenticateRequest`, `types_gen.go:883-892`).
* **Measurements outside the tree:** the floor's scratch copy, the probes for A1, the leak-guard repetition and the coverage profiles
  were all made under the session's scratch directory, like this repository's other measurements.
* **The PLANs:** [0024-PLAN-go-1-26-9-floor.md](0024-PLAN-go-1-26-9-floor.md) implements phase 1. Phases 2 to 4 each get a
  PLAN of their own under this number.

  Annotated 2026-10-20:

  * phase 1, [0024-PLAN-go-1-26-9-floor.md](0024-PLAN-go-1-26-9-floor.md): `complete`, released as `v1.16.2`;
  * phase 2, [0024-PLAN-documentation.md](0024-PLAN-documentation.md): `complete`, released as `v1.16.3`;
  * phase 3, [0024-PLAN-behaviour-fixes.md](0024-PLAN-behaviour-fixes.md): `complete`, released as `v1.17.0`;
  * phase 4, [0024-PLAN-api-gate-and-deprecation.md](0024-PLAN-api-gate-and-deprecation.md): `proposed`.

  Annotated again 2026-10-20: phase 4's PLAN is `complete`, closed by its step 7. The four phases of this program are complete.

* **Related records:**
  * [0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md](0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md): its Go pin, superseded if
    this record is accepted (annotated 2026-10-20: accepted 2026-10-19, and superseded since step 2 of
    [0024-PLAN-go-1-26-9-floor.md](0024-PLAN-go-1-26-9-floor.md), as 0002's own annotation records);
  * [0003-MADR-install-fleet-agent-workspace.md](0003-MADR-install-fleet-agent-workspace.md): `SendRequest`'s order;
  * [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md): generated docs;
  * [0017-PLAN-remediate-acp-v1-conformance-violations.md](0017-PLAN-remediate-acp-v1-conformance-violations.md): the test seam;
  * [0019-PLAN-api-decisions.md](0019-PLAN-api-decisions.md): B17 and B18;
  * [0019-MADR-remediate-debugging-pass-findings.md](0019-MADR-remediate-debugging-pass-findings.md): question 16, the unstable API;
  * [0023-PLAN-connection-lifetime-and-test-leaks.md](0023-PLAN-connection-lifetime-and-test-leaks.md): the leak guard and its defect.
