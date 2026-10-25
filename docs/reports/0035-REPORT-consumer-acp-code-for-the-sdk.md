# ACP code in the two consumers: what belongs in the SDK

Observed 2026-10-24 on `7581822`, at the owner's request. This report reads the ACP code that the SDK's two known consumers,
`magic-cli-remote` (at `f27b743f`) and `<cli-b>` (at `1d2d95e`), both on `v1.20.0`, wrote for themselves, and sorts it into what
would be better developed here and what stays with them. The owner's rule is that anything ACP-related that is idempotent belongs
in the SDK. The report decides nothing: each move it recommends needs a MADR in this repository, and where one would reopen a
decided record, it says which.

**Aligned with issue #11, 2026-10-24.** After this report was written, the owner opened issue #11, "Complete SessionUpdate
constructors, add Kind(), and a ClientRefusals embed for unadvertised methods", with sibling issues in both consumers. The issue
settles three items here, C1, A4 and B3, and it settles them more narrowly than the first draft proposed. Those items now follow the
issue and name it. [0036-MADR-session-update-constructors-kind-and-client-refusals.md](../decisions/0036-MADR-session-update-constructors-kind-and-client-refusals.md)
decides them.

`<cli-b>` stands for the second consumer, as in
[0025-REPORT-schema-1-26-0-adoption-assessment.md](0025-REPORT-schema-1-26-0-adoption-assessment.md). Its own extension methods and
`_meta` key carry its name, so this report calls them "its `_`-prefixed methods" and "its `_meta` key".

## Answer

**Yes, a fair amount, and most of it is small.** The two consumers' ACP code is about 39,000 lines: in `<cli-b>`, its client
(`acpclient/`, `acpclient/acptest/`) and its agent (`acpserver/`); in `magic-cli-remote`, its client (`internal/provider/acpagent/`).
Most of it is application policy and stays: model catalogs, the phone round trip, replay, prompt queues, tool-text rendering. What is
left is a set of protocol-generic pieces, and these are the ones that should come here:

- **Five are written in both consumers, differently, and one of each pair is wrong.** These are the paging of `session/list`, picking
  a permission option, classifying an error, naming a union's variant, and reading a `_meta` object. In each case one consumer has a
  defect the other avoids (A1–A5).
- **Three exist because of a rule this SDK enforces but gives no help with.** These are bare stdio MCP commands, the agent's offered
  auth methods, and the stubs every `Agent` and `Client` writes for methods it does not offer (B1–B3). Issue #11 settles the client
  half of B3 as `ClientRefusals`.
- **Eight are generic helpers that one consumer wrote and any consumer needs** (C1–C8). Two of them close gaps against a protocol
  MUST: truncating terminal output on a character boundary, and checking an elicitation answer against its schema. Issue #11 takes up
  C1, the update constructors.
- **A test harness:** an in-memory pair, a frame recorder, a fake client and a scripted agent, which both consumers and this
  repository's own tests each build by hand (D1). It is the largest piece, and the one that needs its own MADR first.
- **Four proposals would reopen a decided record.** A bounded `Wait` (0026), the code refuse answers (0021), close waiting for turns,
  and exit versus connection cause (E1–E4). The evidence is listed, and the decision is the owner's.

Nothing recommended needs a new dependency. Everything in A, B and C is additive under `v1` (`make api`).

| Id | What | Where it is now | SDK home | Size |
| :--- | :--- | :--- | :--- | :--- |
| A1 | page `session/list` to the end; read `updatedAt` | both clients | `ClientSideConnection` iterator, `SessionInfo` method | small |
| A2 | pick a permission option by kind | both clients | `helpers.go` | small |
| A3 | classify a `*RequestError` / `*CapabilityError` | both clients | `errors.go` | small |
| A4 | name the set variant of `SessionUpdate`; other unions later | both clients | `SessionUpdate.Kind()`, hand-written (issue #11) | small |
| A5 | read a namespaced `_meta` object, typed | both, 7 sites | `helpers.go` | small |
| B1 | resolve a bare stdio MCP command before the absolute-path rule | `<cli-b>` agent | opt-in inbound rewrite | small |
| B2 | auth methods filtered by `auth.terminal` | `<cli-b>` agent | `helpers.go` | small |
| B3 | `ClientRefusals`, an embed for the nine fs, terminal and elicitation methods; agent side later | `<cli-b>`, 4 places | hand-written (issue #11) | small |
| C1 | constructors for the eight stable `SessionUpdate` variants `helpers.go` lacks | `<cli-b>` agent, magic tests | `helpers.go` (issue #11) | small |
| C2 | select config option by category; grouped options flattened | magic client | `helpers.go` | small |
| C3 | elicitation form: field order and answer check | magic client | methods on `ElicitationSchema` | medium |
| C4 | client terminal host, char-boundary truncation | magic client | `agentproc` | medium |
| C5 | client fs read with `line`/`limit`, not-found as `-32002` | magic client | `helpers.go` or a small subpackage | small |
| C6 | pre-check a value against the negotiation | magic client | `Negotiation` method | small |
| C7 | split a tap's bytes into frames | magic client | `agentproc` | small |
| C8 | count of messages refused inbound | magic client | `Connection` method | small |
| D1 | in-memory test harness | `<cli-b>` `acptest`, magic fixtures, this repo's tests | new `acptest` package | large |
| E1–E4 | bounded `Wait`; refuse code; close waits for turns; exit vs connection cause | see E | reopen 0026, 0021 | — |

## How it was assessed

- **Reading.** Three read-only reviews read, in full, every non-test file of the scope: `<cli-b>` `acpclient/`, `acpclient/acptest/`,
  `exp/acpws/`, `internal/archtest/check.go` and `acpserver/`; and `magic-cli-remote` `internal/provider/acpagent/`,
  `internal/provider/acpcommon/` and `internal/wirecap/`. Each also read the consumer's records that explain why the code exists.
  Some files were only skimmed for the lines that touch ACP: in `<cli-b>`, `compact.go`, `autocompact.go`, `personality.go`,
  `rules.go` and `history.go` outside its replay; in `magic-cli-remote`, the grok-only `quota.go`, `xaiusage.go` and
  `subagents.go`. Test files were read only for harness code.
- **Checking.** Every claim this report rests on was checked directly against both trees and this one before it was written: the
  three copies of `handlersDone`; `<cli-b>` `ListSessions` ignoring `NextCursor`; magic's substring permission match, literal
  `-32000`, byte-boundary truncation and 500 ms exit preference; the seven `_meta` assertions; the hand-built `Type: "select"`; the
  variant-name switches; the `encoding/json/v2` imports; `McpServerStdio.command` in `AbsolutePaths` (`acp.go:152`); and the
  absence here of `Unimplemented*`, of builders for the other update variants, and of a bounded `Wait`.
- **Records here.** Each proposal was checked against the decision records in `docs/decisions/` for a prior decision. The two it
  found are under E.

Nothing in this repository, the consumers or `$HOME` was written but this report and its row.

## The test applied

The owner's rule does not define "idempotent", so this report reads it as three conditions, all of which must hold:

1. **Protocol-generic.** The specification or the schema fixes the behaviour, or a rule this SDK enforces does. Product, provider,
   UI and storage choices do not.
2. **Deterministic.** The same input gives the same result in every consumer. A tuning knob, such as a timeout or a chunk size,
   becomes a parameter, and the generic core moves.
3. **Self-contained.** It needs no consumer type and no new dependency.

Code that passes the test only in part is listed by its generic core; the policy around it stays.

## A. Written in both consumers

### A1. Paging `session/list`, and its timestamp

- **magic-cli-remote** `sessioncaps.go:64-106` follows `NextCursor` for up to 20 pages, and stops on a cursor that repeats.
- **`<cli-b>`** `acpclient/sessions.go:168-191` makes one call and never reads `NextCursor`. Against an agent that pages, it shows
  only the first page. Its own agent answers in one page, so its tests do not catch this.
- **Both** parse `SessionInfo.UpdatedAt` with `time.Parse(time.RFC3339, …)` (`sessions.go:183`, `sessioncaps.go:207-216`), and
  both drop a value that does not parse.

Proposed: `func (c *ClientSideConnection) AllSessions(ctx, req ListSessionsRequest) iter.Seq2[SessionInfo, error]`, which stops
on a cursor that repeats, and `func (s SessionInfo) Updated() (time.Time, bool)`. `iter` is in the standard library from Go 1.23,
below the floor.

### A2. Choosing a permission option

- **`<cli-b>`** `acpclient/conn.go:503-511` declines by kind: the first `reject_once`, else `reject_always`, else `cancelled`. Its
  fake client allows with the first `allow_once` or `allow_always`.
- **magic-cli-remote** `session.go:2593-2642` (`autoAllow`, `pickAllowOptionID`) matches by substring on the kind *or the name*:
  `strings.Contains(k, "allow") || strings.Contains(n, "allow") || … "approve"`. Failing that, it takes the first option.
  - An option named "Don't allow" or "Disallow", of kind `reject_once`, is auto-selected as an allow.
  - With no allow option at all, the first option is selected, whatever it is.
  - `PermissionOptionKind` is a closed enum in the schema, so matching on it alone is the correct rule.
  - The outcome is also built by hand (`Outcome: "selected"`), where `NewRequestPermissionOutcomeSelected` exists.

Proposed: `func PickPermissionOption(opts []PermissionOption, kinds ...PermissionOptionKind) (PermissionOption, bool)`, and
`func (r RequestPermissionResponse) Chosen() (PermissionOptionId, bool)` for the agent side, which reads the answer to its own
requests.

### A3. Classifying an error

- **magic-cli-remote** has three predicates:
  - `isMethodNotFound` (`sessioncaps.go:188-194`) accepts a peer's `-32601`, or a local `*CapabilityError` with no field and no
    variant;
  - `isAuthRequired` (`runtime.go:344-349`) compares `re.Code == -32000` as a literal, although the SDK has
    `ErrorCodeAuthenticationRequired`;
  - `isBenignPromptErr` (`session.go:2836-2859`) handles cancellation.
- **`<cli-b>`** matches `ErrorCodeAuthenticationRequired` at `acpclient/conn.go:329`. Its `sessionError` (`sessions.go:37-58`)
  reads a refusal's reason from `data.error`, which is this SDK's own convention for what it refuses.

Proposed, in `errors.go`:

- `IsMethodNotFound(err) bool`, which includes the method-level `*CapabilityError`;
- `IsAuthRequired(err) bool`;
- `IsCancelled(err) bool`;
- `func (e *RequestError) Reason() string`, which reads `data.error` and falls back to the message.

The string-matching fallbacks stay in the consumers.

### A4. Naming a union's variant

- **`<cli-b>`** `acpclient/update.go:304-318` (`blockText`) switches on `ContentBlock`'s pointer fields to name `image`, `audio`,
  `resource_link` and so on.
- **magic-cli-remote** does the same in `compaction.go:89` (`blockKind`). It also decodes the discriminator out of `Unknown` by
  hand, at `session.go:2076-2082` and in `plans.go:73` (`unknownPlanKind`), to name what it declines.

Issue #11 settles this for `SessionUpdate` alone, as a hand-written `func (u SessionUpdate) Kind() string`. It returns the set
variant's discriminator; for `Unknown`, the payload's `sessionUpdate`; and `""` for the zero value. That covers magic-cli-remote's
`session.go` case. Its `blockKind`, its `unknownPlanKind`, and `<cli-b>`'s `blockText` name `ContentBlock` and plan variants, so they
stay until a later issue extends `Kind()` to those unions. The first draft proposed a generated `Variant()` on every union. That was
broader, and a generator change; the issue chose the smaller step.

### A5. Reading a namespaced `_meta` object

- **`<cli-b>`** repeats `meta[key].(map[string]any)` seven times, at `acpclient/conn.go:336, 517`, `sessions.go:149` and
  `acpserver/config.go:405`, `question.go:161`, `grants.go:138`, `mentions.go:120`.
- **magic-cli-remote** `acpagent.go:1214-1232` round-trips through JSON to get a typed view: "The SDK decodes _meta as
  map[string]any, so the typed view is recovered by a JSON round trip".
  [0012-MADR-schema-fidelity-framework.md](../decisions/0012-MADR-schema-fidelity-framework.md) keeps `_meta` as `map[string]any`
  on purpose. This proposal leaves that type alone.

Proposed: `func MetaObject(meta map[string]any, key string) map[string]any` and `func DecodeMeta[T any](meta map[string]any, key
string) (T, bool, error)`, both on `encoding/json` v1.

## B. Code that exists because of a rule this SDK enforces

### B1. A bare stdio MCP command

The protocol table requires `McpServerStdio.command` to be absolute (`cmd/generate/internal/model/acp.go:152`). Real clients send
`"npx"`. `<cli-b>` `acpserver/rewrite.go:17-36` installs `resolveBareCommands` through `WithInboundRewrite` (`serve.go:44`). It
replaces a bare command with `exec.LookPath`'s answer on `session/new`, `load` and `resume`, and leaves what it cannot resolve
for the SDK to refuse `-32602`. It does not cover the unstable fork request, which carries the same list. Every agent behind such
a client needs this.

Proposed: `func ResolveStdioCommands(lookPath func(string) (string, error)) func(ctx context.Context, method string, params any)
error`. Taking `lookPath` as a parameter keeps `os/exec` out of the root package. A one-hook-per-connection option would also want
a small `ChainRewrites`.

### B2. Auth methods for a client without `auth.terminal`

This SDK refuses an `initialize` answer that offers a terminal auth method to a client that did not advertise `auth.terminal`.
`<cli-b>` `acpserver/auth.go:43-64` and `agent.go:394` filter the list to avoid that refusal. Proposed:
`func OfferedAuthMethods(caps ClientCapabilities, methods []AuthMethod) []AuthMethod`.

### B3. Stubs for the methods a side does not offer

`Agent` and `Client` require every method, and the gates already answer `-32601` for one the peer did not negotiate. Every
implementation still writes a stub for it:

- `<cli-b>` writes nine client stubs (`acpclient/conn.go:552-600`) and two agent stubs (`acpserver/agent.go:1179, 1245`). The
  comment on the second says "the SDK answers -32601 before this runs".
- `acptest/client.go` and `acptest/script.go` write their stubs again.

`<cli-b>`'s `0031-MADR-sdk-current-releases-and-adoption.md` lists "a default client for fs, terminal and elicitation" as
something this SDK lacks.

Issue #11 settles the client half, and settles the trade-off. It is `ClientRefusals`, a hand-written struct to embed:

- It answers `NewMethodNotFound` with the method's constant for exactly the nine methods a client may leave unadvertised:
  `ReadTextFile`, `WriteTextFile`, the five terminal methods and `CreateElicitation`. `CompleteElicitation` returns nil.
- It does not implement `SessionUpdate` or `RequestPermission`, so a client still has to write those, and the compiler says so.
- A full method-not-found `Client` was rejected: a forgotten method would compile and answer `-32601` without anyone having decided
  that.

The agent side is not in the issue. `<cli-b>`'s two agent stubs stay until a later one.

An outer type's own methods take precedence over promoted ones, so embedding `ClientRefusals` would not hide magic-cli-remote's
real fs and terminal methods. The issue's reason for magic not to embed it is a different one: a real method later renamed or
removed would be replaced by a refusal without a compile error. Recorded in the 0036 MADR.

## C. Generic helpers one consumer wrote

### C1. Builders for the remaining `SessionUpdate` variants

`helpers.go` builds the message, thought, plan and tool-call variants. The protocol table leaves `SessionUpdate` to it, so
`helpers_gen.go` writes none. `<cli-b>` builds the rest by hand:

- config options at `acpserver/commands.go:531`;
- session info at `:597`;
- available commands at `:728`;
- current mode at `modes.go:84-86`;
- usage at `prompt.go:312-320`.

Its `0016-MADR-acp-go-sdk2-v0.15.0-and-stable-surface.md`, decision D13, asks this SDK for them, and its
`0031-MADR-sdk-current-releases-and-adoption.md` records that no issue was filed.

Issue #11 settles the set. These are the eight stable variants of schema 1.25.0 that `helpers.go` does not build:

- `UpdateAvailableCommands`, `UpdateCurrentMode` and `UpdateConfigOptions`;
- `UpdateSessionTitle(Patch[string])`, for `session_info_update`;
- `UpdateUsage`;
- `UpdateNotice`, `UpdateCompaction`, `UpdateCompactionSummary` and `UpdateCompactionSummaryText`.

`schema/schema.json` lists `notice` and both compaction variants as stable, so the first draft was wrong to leave them out. The
unstable variants are a later issue, filed when a product advertises `plan` or `subagents`.

### C2. A config option by category, and grouped options

- magic-cli-remote `session_meta.go:73-80` (`selectOption`) finds the `Select` option of a category, such as model or thought
  level. It uses it in four places.
- `emitConfigOptions` (`session.go:2288-2292`) reads only `Ungrouped`, so an agent's grouped options are lost.
  magic-cli-remote's own 0208 records this.

Proposed: `func ConfigOptionByCategory(opts []SessionConfigOption, c SessionConfigOptionCategory) (*SessionConfigOptionSelect,
bool)`, and `func (o SessionConfigSelectOptions) All() []SessionConfigSelectOption`, which flattens the groups.

### C3. An elicitation form's fields and its answer

magic-cli-remote `elicitation.go` does two things with a form:

- It orders the fields: `PropertyOrder` first, which this SDK keeps by
  [0022-MADR-keep-elicitation-property-order-before-v1-15-0.md](../decisions/0022-MADR-keep-elicitation-property-order-before-v1-15-0.md),
  then by name, with the required ones marked.
- It checks the answer (`checkForm`, `:302-430`) against required keys, string length, `format`, `pattern`, the enum or `oneOf`,
  numeric bounds and item counts, and builds typed `ElicitationContentValue`s.

The client dispatch here passes the form to the handler and sends the answer back unchecked.

Proposed: `func (s ElicitationSchema) Fields() []ElicitationField` and `func (s ElicitationSchema) Check(content
map[string]ElicitationContentValue) error`, both stdlib (`regexp`, `net/mail`, `net/url`).

There is one risk. JSON Schema patterns are ECMA-262 and `regexp` is RE2, so a pattern RE2 cannot compile has to be reported, not
taken as a match. The consumer already handles this (`:232-237`).

### C4. A client terminal host

magic-cli-remote `terminal.go:20-302` implements the five `terminal/*` methods:

- a registry of terminals;
- output kept up to a byte limit, with `truncated`;
- the wait status mapped to an exit code or a signal;
- kill, release and `CloseAll`.

It violates one rule. Its buffer drops from the front at any byte (`:278`, "may split UTF-8; acceptable for tool output"), and the
schema says "The Client MUST ensure truncation happens at a character boundary" (`types_gen.go:4094`).

It also kills the process tree with code of its own, where `agentproc` already does that on every platform.

Proposed: a `TerminalHost` in `agentproc` that answers the five methods. A caller-supplied function builds the `*exec.Cmd`; the
consumer's shell-line routing for grok (`:305-330`) would be that function. It would reuse `agentproc`'s tree handling and its
existing Windows-only `golang.org/x/sys`.

This is the largest of the C items: one exported type and its options.

### C5. Client file reads and writes

magic-cli-remote `session.go:2648-2689` and `convert.go:67-80` read a file and apply the 1-based `line` and `limit`, and write
with `MkdirAll`. Its 0207 MADR (H2) records that a missing file goes out as `-32603`, where `NewResourceNotFound` (`-32002`)
exists.

Proposed: `ReadTextFileOS(req) (ReadTextFileResponse, error)` and `WriteTextFileOS(req)`. Both answer a missing file with
`-32002`. They would go in the root package, or in a small subpackage if the root should keep no `os` file access.

Confining paths to the session's roots (`pathWithinRoots`, `session.go:2740-2779`) is policy, and stays. `Roots()` already gives
the roots.

### C6. Checking a value against the negotiation before sending it

magic-cli-remote drops what the agent did not negotiate before it sends, because this SDK refuses the whole request otherwise
(`acpagent.go:863-866`). It drops MCP servers at `acpagent.go:635` and image blocks at `session.go:569`. The rules are the
generated `gates`, which are unexported.

Proposed: `func (n Negotiation) AllowsValue(method string, params any) error`, which exposes them.

### C7. A tap's bytes as frames

magic-cli-remote `wirecap.go:192-207` splits `agentproc`'s output tap into newline-terminated frames, and copies each one,
because a tap must not keep `p`.

Proposed: `agentproc.LineTap(func(frame []byte)) io.Writer`. The file layout and the redaction stay in the consumer.

### C8. Counting refused messages

magic-cli-remote counts, through `WithInboundValidation`, what the SDK refuses (`session.go:239-242, 639-645`).

Proposed: `func (c *Connection) RefusedMessages() uint64`, beside the existing count of dropped notifications.

## D. A test harness

Both consumers and this repository build in-memory ACP peers by hand:

- **`<cli-b>`** `acpclient/acptest`, about 1,300 lines:
  - `Connect` joins an agent and a client with two `io.Pipe`s;
  - a `Recorder` splits each direction into frames and writes a canonical transcript with golden files;
  - `Abs` makes fixture paths absolute on every host, because this SDK checks received paths with the host's rule;
  - a fake `Client` serves in-memory files and scripted terminals;
  - `ScriptAgent` plays `SessionUpdate`s per turn.
- **magic-cli-remote** has `wireagent_test.go` (a byte-level scripted peer) and `initfixture_test.go` (answers `initialize`).
- **This repository's** tests call `io.Pipe()` 122 times across 26 test files to build their peers.

Proposed: a new `acptest` package with `Connect`, the recorder, `Abs`, a fake client and a scripted agent. It needs its own MADR,
for three reasons:

- **It is a new exported package.**
- **It cannot be copied as it is.** `<cli-b>`'s recorder and script import `encoding/json/v2` and `jsontext`. These build by
  default at Go 1.27.3, but not at this module's floor, Go 1.26.10, or under CI's `GOEXPERIMENT=nojsonv2`. The canonical form
  needs a rewrite on v1.
- **`<cli-b>` would need to amend a rule.** Its architecture rule 2 (`internal/archtest/check.go:221-237`) lets only `acpserver`
  and `acpclient/...` import this module, test files included. Its 13 `internal/cli` test files that use `acptest` would need that
  rule amended, or a thin wrapper kept.

`<cli-b>`'s 0016 MADR D6 also proposes a `ValidateGolden`, which decodes each recorded frame into its type and calls `Validate`.
It belongs in this package.

## E. Proposals that would reopen a decided record

### E1. A bounded `Wait`

`<cli-b>` keeps three copies of `handlersDone`, which waits for `Wait` under a time limit: at `acpclient/conn.go:207`,
`acpclient/acptest/pair.go:65` and `acpserver/serve.go:71`. Each cites a handler that ignores its context.

[0026-MADR-connection-end-cause-and-wait.md](../decisions/0026-MADR-connection-end-cause-and-wait.md), question 4, decided on
`Wait() error`, unbounded. It considered `Wait(ctx)` and rejected it: "A caller that needs a bound runs it on a goroutine and
selects". The three copies are that pattern.

A `WaitContext` here would save the copies, but could not end the goroutine any more than they do. This report does not recommend
reopening 0026.

### E2. The code `PromptOverlapRefuse` answers

`<cli-b>` runs `PromptOverlapConcurrent` and refuses an overlap itself, with `-32600` (`acpserver/agent.go:760-777`). Two
differences stop it from using `PromptOverlapRefuse`:

- Refuse answers `-32603`, decided in
  [0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md](../decisions/0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md).
- A cancelled turn stops counting at its cancel, not when its handler returns. `<cli-b>` also counts its own extension methods
  as turns.

A configurable code would reopen 0021. The extension turns are the consumer's own policy.

### E3. `session/close` waiting for the cancelled turn

`<cli-b>` `acpserver/agent.go:1182-1213` waits up to 5 s (`closeWait`) for a turn that the SDK cancelled before `CloseSession`
ran, so that the turn's last entries are written. The SDK holds the turns, so it could offer
`func (c *AgentSideConnection) TurnsDone(id SessionId) <-chan struct{}`. That is a change in a generated path. No record decides
it either way.

### E4. The agent's exit versus the connection's end

magic-cli-remote `session.go:1304` waits a fixed 500 ms (`exitPreference`) on `Process.Done()`, so that the agent's exit status
wins over `conn.Err()`. Its 0207 PLAN found that the connection end was seen first in 28 of 30 runs. `agentproc`'s watcher knows
which came first.

`Process.Wait` reporting the connection's cause beside `Cmd.Wait`'s error would remove the guess. That amends
[0027-MADR-agent-process-helper.md](../decisions/0027-MADR-agent-process-helper.md).

## F. What stays in the consumers

These hold no generic core beyond the builders they already use:

- the translation of updates into each consumer's own event types;
- `<cli-b>`'s `_`-prefixed methods and its `_meta` key;
- magic-cli-remote's `_x.ai` handlers and grok fallbacks;
- model catalogs, prewarm and auth selection;
- the phone permission round trip, the timeouts and the auto mode;
- replay on `session/load`;
- the prompt queue, the stall watchdog and stderr scraping;
- tool-text rendering, cost, and the plan vocabulary;
- the mode sets and the model and thought config state machines;
- permission requests used as yes/no questions;
- `<cli-b>`'s 50 ms / 4 KiB chunk coalescer. The batching is a phone-legibility policy; its generic core, flushing before any other
  update, is too small to carry an API;
- the import rules of `internal/archtest`.

The following are generic in shape, but each consumer's policy differs. They are not recommended now:

- a session-state reducer, which no consumer has as one unit;
- routing updates by `SessionId`. `<cli-b>` attributes early updates by a heuristic, and magic-cli-remote drops every session but
  the live one;
- a session-bound sender that mirrors every client method;
- an extension-method mux. `<cli-b>`'s is on `encoding/json/v2`;
- `ServeAgent` and an in-process `agentproc`.

`<cli-b>`'s `exp/acpws/` is a declared, empty package for ACP over WebSocket and Unix sockets. It would fit here once it is built.
A WebSocket implementation needs a dependency, so it would need a MADR.

## G. Seen in passing: consumer fixes that need nothing new here

These are the consumers' own to fix. The SDK already has what they need:

- **`<cli-b>`:**
  - `acpserver/config.go:204, 229` writes `Type: "select"` by hand, where `NewSessionConfigOptionSelect` exists.
  - `ErrImagesUnsupported` (`acpclient/conn.go:313`) repeats the SDK's `promptCapabilities.image` gate.
  - `acpserver/agent.go:387` keeps its own copy of the client's capabilities beside `Negotiated()`.
- **magic-cli-remote:**
  - `runtime.go:346` compares the literal `-32000`.
  - Its permission outcomes and MCP server entries are built by hand, where constructors exist (its 0207 MADR H3).
  - `session.go:109-134, 1668-1685` and `xaiusage.go:14-18` justify code by `acp-go-sdk@v0.13.5` behaviour that is gone: a full
    queue now drops a notification under `OverflowDropNewest`, and `Usage` carries cache fields.
  - The A2 substring match is the one with user-visible risk.

## What would follow

If the owner accepts the direction, these are natural units, each one MADR and PLAN:

1. C1, A4 for `SessionUpdate`, and the client half of B3. These are issue #11:
   [0036-MADR-session-update-constructors-kind-and-client-refusals.md](../decisions/0036-MADR-session-update-constructors-kind-and-client-refusals.md)
   and its PLAN. Both consumer issues pin the tag that ships them.
2. A1–A3, A5, B1, B2, C2 and C8. These are additive helpers in the root package, need no generator change, and could ship as one
   minor release.
3. A4 for the other unions, and the agent half of B3, if wanted.
4. C3 and C6, the elicitation check and the exposed gates.
5. C4, C5 and C7, the client-side helpers in `agentproc` and for the file system.
6. D1, the `acptest` package.
7. E3 and E4, only if the owner reopens them.

Each consumer would then remove its copy in its own repository, under its own records.
