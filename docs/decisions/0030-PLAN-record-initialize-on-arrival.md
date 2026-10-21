---
status: proposed
date: 2026-10-21
associated-madr: "0030-MADR-record-initialize-on-arrival.md"
---

# Implement 0030: record a client's initialize when its answer arrives, and hold what comes before it

Associated MADR: [0030-MADR-record-initialize-on-arrival.md](0030-MADR-record-initialize-on-arrival.md)

## Goal

Implement the MADR's O1 with the hold, as the owner decided questions 1 to 4 on 2026-10-21:

* **The commit** (question 1): the reader records the client's `initialize`, from the answer it has just read, before it reads the next
  message; `Initialize` returns what was recorded, or the commit's error.
* **The hold** (question 2): while the client's `initialize` is in flight, a received message that the gate would refuse only for want
  of a record waits until `initialize` completes or fails, then is judged.
* **A caller that gave up** (question 3): nothing recorded; a held message is then refused.
* **The test** (question 4): 200 trials in one write.

`make api` must report no exported change. The release is the minor `v1.20.0`, with
[0029-PLAN-unix-client-death-pdeathsig.md](0029-PLAN-unix-client-death-pdeathsig.md)'s change; this PLAN writes its notes and the owner
tags it.

## Facts this plan rests on

Each was read on `f193910` on 2026-10-21; step 1 re-asserts them with a script. The MADR's I1 to I8 hold the probe.

| Id | Fact | Where |
| :--- | :--- | :--- |
| F1 | the agent stores its record in the handler, before its answer is written | `agent_gen.go:400` |
| F2 | the client stores its record after `sendRequest` returns, on the caller's goroutine | `client_gen.go:494`–`:523` |
| F3 | the generator emits the client's `Store` | `cmd/generate/internal/emit/dispatch.go:690`–`:693` |
| F4 | `pendingResponse` holds only its channel | `connection.go:130`–`:132` |
| F5 | `handleResponse` hands the response over and returns to reading | `connection.go:1577`–`:1604`, called at `:1508` |
| F6 | `sendRequestSent` decodes and gates the result on the caller's goroutine | `connection.go:1807`, `:1820`, `:1823` |
| F7 | `gateMessage` refuses all but `initialize` with no record | `types_gen.go:23107`–`:23113` |
| F8 | the client's gate and extension gate are one, generated | `client_gen.go:102`–`:110`; `extensions.go:53`–`:58` |

## Scope

* **`connection.go`:** `pendingResponse` gains `commit`; `handleResponse` runs it; an unexported variant of `sendRequestSent` takes it.
* **The generator,** `cmd/generate/internal/emit/dispatch.go`: the client's `Initialize` with a commit; the client's state for an
  `initialize` in flight; the client's gate that holds. Regenerated: `client_gen.go`. The engine cases' `want.api` should not change; the
  pins in `emit/dispatch_test.go` follow the emitted code.
* **Tests:** at the root, below; in the generator, the pins.
* **Docs:** the README's paragraph on the record (`README.md:382`–`:384`); `docs/architecture.md`'s gate paragraph (`:275`–`:286`).
* **The records:** this PLAN and its row; the release notes `docs/reports/0030-REPORT-release-notes-v1-20-0.md` and their row, covering
  0029 too.

## Design

### The commit

* `pendingResponse{ch, commit func(result json.RawMessage) error}`.
* `handleResponse`, for a pending request with a commit, whose response is a result: runs the commit before it hands the response over;
  the envelope carries the commit's error. An error response runs no commit.
* `sendRequestCommit[T]`, beside `sendRequestSent`, registers the commit with the pending request and returns the commit's error, when
  there is one, before decoding; otherwise it decodes as `sendRequestSent` does. The commit decodes the answer once more; it is one small
  message per connection.
* The emitted `Initialize`: its commit decodes the `InitializeResponse`, checks the version (`ProtocolVersionError`), applies the result
  gate, `jsonCopy`s the record and `Store`s it, exactly as today's code after `sendRequest`, then marks `initialize` complete.

### The hold

* The client gains, generated, an `initialize` in-flight state: a pointer to the current attempt, whose `done` channel closes once, when
  the attempt completes (the commit stored the record) or fails.
* `Initialize` starts an attempt before it sends; a deferred function fails it unless it completed: an error answer, the commit's error,
  the caller's context, the connection's end.
* The client's gate, when `gateMessage` would refuse only for want of a record (`errors.Is(err, ErrNotInitialized)`) and an attempt is
  in flight: waits for the attempt's `done` or the connection's `Done`, then judges again against the record. No attempt in flight:
  refuses at once, as today.
* The agent's side is unchanged.

## Tests

At the root, a fake agent over `io.Pipe`s, as the MADR's probe:

* **`TestInitializeRecordedOnArrival`:** the answer, an extension notification and an extension request in one write, 200 times; each
  notification handled and each request answered with a result.
* **`TestHeldUntilInitialized`:** the extension notification and request in one write *before* the answer; both handled once
  `Initialize` returns, 200 times.
* **`TestHeldRefusedWhenInitializeFails`:** the same two before an error answer; before an answer with another protocol version; with the
  caller's context ended first; with the connection closed first: each request answered `ErrNotInitialized`'s `-32601`, each notification
  not handled, and none held past `Initialize`'s return.
* **`TestNotHeldBeforeInitialize`:** a message before `Initialize` is called is refused at once, as today.
* **`TestAgentRecordsBeforeAnswering`:** a raw client sends `session/new` the moment it reads the answer; answered, 200 times (the MADR's
  I8).
* **The pins:** the emitted `Initialize` registers a commit; the emitted gate holds.

## Implementation Steps

Each step ends with its gates and an execution record. Agents stage; a commit or a push on `main` needs the owner's explicit permission in
that turn (`AGENTS.md`, "Commits"). Every new test or check is seen to fail on a planted break in a scratch copy, never in the tree, before
it is relied on.

1. **Records:** this PLAN and its row `in-progress`; the facts re-asserted by a script.
2. **The commit:** `connection.go`; the emitter's `Initialize`; regeneration; `TestInitializeRecordedOnArrival`,
   `TestAgentRecordsBeforeAnswering`; the pins.
3. **The hold:** the emitter's in-flight state and gate; regeneration; `TestHeldUntilInitialized`, `TestHeldRefusedWhenInitializeFails`,
   `TestNotHeldBeforeInitialize`; the pins.
4. **Docs:** the README and `docs/architecture.md`; each claim mapped to a test.
5. **Release notes:** `docs/reports/0030-REPORT-release-notes-v1-20-0.md` and its row, for 0029 and 0030, with `make api`'s report.
6. **Verification and close:** the gates; the Linux and Windows hosts and `make push-guard` before the push; after CI's six jobs pass,
   this PLAN and its row `complete`, and 0029's if it waits on the release; the owner tags `v1.20.0` and publishes its release; the
   publication checks and a consumer on the floor.

## Verification

Per step: phase 1's gates on go1.27.3, regeneration included; `make api`; the floor gate at go1.26.10; the root tests on the Linux and
Windows hosts.

Planted breaks, each in a scratch copy:

| For | Break | The check that must fail |
| :--- | :--- | :--- |
| the commit | the record made by the caller again, the commit removed | `TestInitializeRecordedOnArrival` |
| the commit | the commit run after the hand-over | `TestInitializeRecordedOnArrival` |
| the commit | the version check left out of the commit | `TestHeldRefusedWhenInitializeFails`, another version |
| the hold | the gate refusing at once, as today | `TestHeldUntilInitialized` |
| the hold | a failure not closing `done` | `TestHeldRefusedWhenInitializeFails`, by its bound |
| the hold | an attempt not failed when the caller gives up | `TestHeldRefusedWhenInitializeFails`, the caller's context |
| the hold | messages held with no attempt in flight | `TestNotHeldBeforeInitialize`, by its bound |
| the emitter | the pins' expected code changed | the pins |

Acceptance:

* every step's checks pass and its record is written; every claim the new docs make is mapped to a test;
* the tests pass on macOS, on the Linux and Windows hosts, and in CI's six jobs;
* `make lint` reports 0 issues, `go fix -diff` prints nothing, `go mod tidy -diff` is clean;
* `make api` reports no exported change.

## Rollout and Rollback

Each step is one commit, reverted by reverting it; steps 2 and 3 revert the generated code with the emitter.

For a consumer: a client no longer refuses a message its agent sends right after, or before, its `initialize` answer; what that answer
does not allow is still refused once the record exists. The release is the minor `v1.20.0`, which the owner confirms at the tag.
