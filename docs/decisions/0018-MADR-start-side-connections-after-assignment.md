---
status: accepted
date: 2026-10-13
decision-makers: Repository owner
---

# A side connection assigns its Connection before the Connection starts reading

## Context and Problem Statement

GitHub issue #3, "NewAgentSideConnection races its own constructor: a fast initialize can nil-deref in checkInbound (v0.16.0+)",
opened on 2026-10-13, reports that `NewAgentSideConnection` publishes the connection's reader before it assigns `asc.conn`. A downstream
consumer found it with `go test -race` while pinning `v0.17.0`, and saw the agent process panic once. This record checks the issue against
the code, reproduces it, and decides the remedy. Facts were read on `e21928c` (the `v0.17.0` tag) on 2026-10-13.

### What the issue reports, checked

* **The constructors assign after the reader starts.** `agent_gen.go:33` is `asc.conn = NewConnection(asc.handleWithExtensions,
  peerInput, peerOutput, opts...)` and `client_gen.go:27` is the same for `csc.conn`. `NewConnection` (`connection.go:417`) starts four
  goroutines before it returns (`connection.go:451-459`), among them `receive`, which reads `peerOutput` and dispatches to the handler. The
  handler can therefore run before the assignment, with no happens-before edge between the assignment and its read.
* **Every typed case of `handle` reads the field.** Each case of the generated `(*AgentSideConnection).handle` and
  `(*ClientSideConnection).handle` calls `a.conn.checkInbound` or `c.conn.checkInbound` first, and `checkResult` after;
  `checkInbound` reads `c.rewrite` on its receiver (`connection.go:1521`), so a nil receiver panics. A panic on the handler goroutine ends
  the process.
* **Affected versions.** The constructor has had this shape since before `v0.14.0` (hand-written in `agent.go` and `client.go` until
  `19e062d` generated them with no change of order). At `v0.15.0` no case of `handle` read `conn`; the reads arrived with `checkInbound` in
  `v0.16.0` ([0015-MADR-validate-absolute-protocol-paths.md](0015-MADR-validate-absolute-protocol-paths.md)). `v0.16.0` and `v0.17.0` are
  affected.
* **The client side is reachable too.** The issue calls the client side "not reached in practice". It is reached whenever an agent writes
  a request before `NewClientSideConnection` returns: the typed agent methods refuse to send before `initialize`, but `Connection()`,
  `SendRequest` and an agent not built on this SDK do not. Before `initialize` the client's case still calls `c.conn.checkInbound`, which is
  where the gate answers `-32601`, so the read happens first.

### Reproduction

A scratch copy of `e21928c`, outside the tree, with one test per side: the input already holds one line when the constructor is called
(`initialize` for the agent, `fs/read_text_file` for the client), the output is `io.Discard`, and each test builds 200 connections and
waits for `Done()`.

* `go test -race -count=3 -run TestRepro18Agent`: exit 1, a `DATA RACE` whose read is `agent_gen.go:214` (the `initialize` case) and whose
  write is `agent_gen.go:33`, then `panic: runtime error: invalid memory address or nil pointer dereference` at `agent_gen.go:214`.
* `go test -race -count=3 -run TestRepro18Client`: exit 1, the same pair at `client_gen.go:115` and `client_gen.go:27`, then the same panic
  at `client_gen.go:115`.
* `go test -count=20 -run TestRepro18Agent`, without the race detector: exit 0, 4000 constructions, no panic. The panic depends on
  scheduling; the race does not.

### What else holds the same pattern

* `NewConnection` is exported. A consumer whose handler reads a variable it assigns from `NewConnection`'s result has the same race in its
  own code, and nothing in `NewConnection`'s documentation says the handler may run before it returns.
* Every other field of either side is assigned before `NewConnection` is called, so the Go memory model's rule for the `go` statement
  already orders them before any read by the connection's goroutines. `conn` is the only field written after.

## Decision Drivers

* No panic and no data race when the peer writes at once, which is what editors and stdio clients do.
* The exported API and `NewConnection`'s behaviour for direct callers stay as they are, so the fix ships as a patch release.
* No cost on the hot path: nothing added to every dispatched message.
* The side connections are generated (0017's S1), so the fix lands in the generator and the regenerated output agrees with it.
* A test that fails on the old order, in CI.

## Considered Options

* C1. Build the `Connection` without starting it, assign it, then start it
* C2. Make the handler wait on a channel the constructor closes after the assignment
* C3. Keep `conn` in an `atomic.Pointer` and load it in `handle`
* C4. Have the `Connection` pass itself to the handler through the context
* C5. Export a two-phase API: `NewConnection` returns an unstarted connection, and `Start` starts it

## Decision Outcome

Chosen option: "C1. Build the `Connection` without starting it, assign it, then start it", because it removes the race at its cause,
costs nothing per message, and changes no exported identifier or behaviour.

1. `connection.go` splits `NewConnection` into an unexported `newConnection`, which builds the `*Connection` and starts nothing, and an
   unexported `(*Connection).start`, which starts its four goroutines. `NewConnection` is `newConnection` followed by `start`.
2. `NewConnection`'s documentation says the handler may be called before `NewConnection` returns, so a handler must not read a variable
   assigned from its result.
3. The generator emits each side's constructor as: assign every field, `x.conn = newConnection(...)`, then `x.conn.start()`, then
   `return x`. The `go` statements in `start` then follow the assignment, so the memory model orders it before every read by the
   connection's goroutines.
4. A root test builds each side with its input already waiting, many times, and waits for the answer each writes; under `-race` it fails
   on the old order.
5. The fix ships as `v0.17.1`, with release notes in a 0018 report, and issue #3 is closed with a reference to the release.

### Consequences

* Good, because the race and the panic are gone on both sides, with no change to the exported API (the API listing is unchanged).
* Good, because direct callers of `NewConnection` keep its behaviour and gain a documented warning about the same pattern in their code.
* Good, because nothing runs per message.
* Neutral, because the order now matters: a later field added to a side and assigned after `start` would bring the race back. The test
  covers `conn`, the only field the dispatch reads that is not set before construction; the generator's body keeps every assignment
  before `start`.
* Bad, because the test fails reliably only under `-race`: without it the old order passed 4000 constructions in the reproduction. CI runs
  `-race` on Linux (`.github/workflows/ci.yml`, "race detector") and `make pre-add-check` runs `go test -race`, so the regression is still
  caught before a merge.

### Confirmation

* The new test fails under `-race` on a scratch copy whose generated constructors keep the old order, and passes on the tree.
* `cd cmd/generate && go run .` regenerates `agent_gen.go` and `client_gen.go` with `newConnection`, then `start`; the generated-output
  check stays clean.
* The exported API listing against `v0.17.0` is unchanged.
* The issue's own reproduction (input waiting before the constructor, `go test -race -count=50`) passes on both sides, on macOS, Linux (CI)
  and Windows.

## Pros and Cons of the Options

### C1. Build, assign, start

* Good, because the order is fixed by construction: the goroutines do not exist until the field is set.
* Good, because it touches one runtime function and one generator function.
* Neutral, because it adds two unexported functions to `connection.go`.

### C2. A ready channel

* Good, because the constructor order would not matter.
* Bad, because every dispatched message waits on a channel receive.
* Bad, because the generated `handle` grows a wait in every case, or `handleWithExtensions` does, for a condition true after the
  first microseconds of the connection's life.

### C3. An atomic pointer

* Good, because it silences the race detector.
* Bad, because a load that comes first still sees nil, so the panic stays unless every case also waits or refuses on nil, which is C2.

### C4. The connection in the context

* Good, because the handler would never read the side's field.
* Bad, because every case would fetch its connection from a context value, a hidden coupling the reader of the generated code has to
  know about, to work around an order that C1 fixes directly.

### C5. A two-phase exported API

* Good, because direct callers could avoid the pattern too.
* Bad, because changing `NewConnection` to not start is a breaking change for every direct caller, and adding `Start` beside it widens the
  API for a need the documentation in outcome 2 covers.

## More Information

* Issue #3 proposes C1 in the same three steps.
* The constructors were generated by
  [0017-MADR-remediate-acp-v1-conformance-violations.md](0017-MADR-remediate-acp-v1-conformance-violations.md), S1, which moved them
  unchanged. The reads in `handle` came from [0015-MADR-validate-absolute-protocol-paths.md](0015-MADR-validate-absolute-protocol-paths.md).
  Both records are complete; this is a new defect found after release, so it takes its own number.
* The implementation is [0018-PLAN-start-side-connections-after-assignment.md](0018-PLAN-start-side-connections-after-assignment.md).

### Amended during implementation, 2026-10-13

* **The builder is `buildConnection`, not `newConnection`.** revive's `confusing-naming` rule refuses an unexported function that differs
  from `NewConnection` only by case. Outcomes 1 and 3 hold with that name; nothing else changes. Recorded in the PLAN's deviation entry
  of the same date.
