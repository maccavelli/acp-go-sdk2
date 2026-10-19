---
status: accepted
date: 2026-10-19
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

#### Versioning, packaging and process (V)

| Id | Finding | Evidence |
| :--- | :--- | :--- |
| V1 | **No policy for the unstable API under v1.** 79 exported `Unstable*` identifiers come from `schema.unstable.json`. [0019-MADR-remediate-debugging-pass-findings.md](0019-MADR-remediate-debugging-pass-findings.md), question 16, left "the policy for removing unstable exported API under `v1`" to a record of its own, not yet written. | `go doc -short .` |
| V2 | **No automated API-compatibility check.** The API listing (`cmd/generate/internal/enginetest/api.go:104`) is run by hand for each release's notes. No CI job, Makefile target or `RELEASING.md` step compares the API with the last tag. | `ci.yml`, `Makefile`, `RELEASING.md` |
| V3 | **No deprecation mechanism.** There are 0 `Deprecated:` comments, and the generator has no path to write one. Nothing is deprecated today. | `grep` |
| V4 | **No `CHANGELOG`.** The release notes in `docs/reports/` and the GitHub releases serve as one. The owner's workspace policy lists `CHANGELOG.md` under "Do not add", as a "fleet-wide decision, not a per-repo default" (`~/.claude/skills/workspace-scaffolding/references/do-not-add.md`). `RELEASING.md` names neither the notes reports nor the API listing. | `RELEASING.md` |
| V5 | **One package, by design.** `acp` holds 2,830 hand-written and 24,885 generated lines, with no sub-package. Moving any part into one breaks the API within v1 unless the root keeps aliases. | line counts |

#### Found while assessing, belonging to another record

* **0023's leak guard can miss a leak in the last test it runs.** `waitForLeaks` returns as soon as the profile counts 0. If the only
  leaking test is the last to run, its goroutines may not have blocked yet. In a scratch copy, a planted leaking test, run alone 30
  times on go1.27.2, was caught 12 times and missed 18. Step 2's planted breaks each ran once, and so did not show it.
* This is a defect found in that PLAN's live run, so by `AGENTS.md` it amends
  [0023-PLAN-connection-lifetime-and-test-leaks.md](0023-PLAN-connection-lifetime-and-test-leaks.md), not this record. It is listed
  here because G5's fix touches the same function.

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
   * A package overview with a happy path in `doc.go`, a short Go quickstart in the README, and per-symbol `Example` functions with
     `// Output:`.
   * The option semantics.
   * Doc notes on the wire-schema types and on `ErrorCode` against `RequestError`.
   * `RELEASING.md` naming the release notes and the API listing.
   * Question 4 decides the generated type comments.
3. **Behaviour (A1 to A4, A14).** As questions 1, 3 and 5 decide.
4. **Process (V2, V3).** An API-compatibility gate in CI from the existing listing, and the generator's path to `// Deprecated:`, as
   questions 6 and 7 decide. Decided under question 6: the gate is `gorelease`, not the existing listing.

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
* **Related records:**
  * [0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md](0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md): its Go pin, superseded if
    this record is accepted;
  * [0003-MADR-install-fleet-agent-workspace.md](0003-MADR-install-fleet-agent-workspace.md): `SendRequest`'s order;
  * [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md): generated docs;
  * [0017-PLAN-remediate-acp-v1-conformance-violations.md](0017-PLAN-remediate-acp-v1-conformance-violations.md): the test seam;
  * [0019-PLAN-api-decisions.md](0019-PLAN-api-decisions.md): B17 and B18;
  * [0019-MADR-remediate-debugging-pass-findings.md](0019-MADR-remediate-debugging-pass-findings.md): question 16, the unstable API;
  * [0023-PLAN-connection-lifetime-and-test-leaks.md](0023-PLAN-connection-lifetime-and-test-leaks.md): the leak guard and its defect.
