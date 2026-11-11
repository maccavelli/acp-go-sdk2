# Release notes for v1.36.0, against v1.35.0

Observed 2026-11-11 on `c36fdfe` with phase 5's changes in the working tree, for
[0054-PLAN-adopt-acp-schema-1-31-0-and-client-system-prompts.md](../decisions/0054-PLAN-adopt-acp-schema-1-31-0-and-client-system-prompts.md),
phase 5, step 5. This report drafts the text of the GitHub release for `v1.36.0`, as `RELEASING.md` describes it. It decides
nothing. It is committed before the tag, so the tagged tree holds it. Phase 5 changes `tools/` and records, and no exported
API, so the API it tags is `c36fdfe`'s.

Each statement names its source:

- a record;
- a commit;
- or a command run in phase 5, whose output the PLAN's execution record holds.

Summary:

- 57 compatible additions and nothing incompatible. `make api` against `v1.35.0`: "0 incompatible, 57 compatible", 11 in the
  root package and 46 in package `unstable`, which is reported and not gated. So the release is a minor, as `RELEASING.md`
  says and the MADR's question 6 decided.
- The ACP schema pin moves from `1.25.0` to `1.31.0`
  ([0054-MADR-adopt-acp-schema-1-31-0-and-client-system-prompts.md](../decisions/0054-MADR-adopt-acp-schema-1-31-0-and-client-system-prompts.md),
  D1).
- It adds the client system prompt, stable since schema `1.30.0`, with its capability gates and its check (D4 to D6).
- One change on the wire, limited to session requests that carry `systemPrompt`.
- The `go` line moves to `1.26.11` and the toolchain to `go1.27.4`
  ([0055-MADR-pin-go-1-27-4-and-floor-1-26-11.md](../decisions/0055-MADR-pin-go-1-27-4-and-floor-1-26-11.md)). The
  requirements are unchanged.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.36.0

The ACP schema is now `1.31.0`, up from `1.25.0`. `make api` reports 57 compatible additions against `v1.35.0` and nothing
incompatible: 11 in the root package, all for the client system prompt, and 46 in package `unstable`, which is outside the
`v1` promise. No exported name changes or goes away
([0054-MADR-adopt-acp-schema-1-31-0-and-client-system-prompts.md](../decisions/0054-MADR-adopt-acp-schema-1-31-0-and-client-system-prompts.md)).

Two things to read before upgrading: the module now requires Go 1.26.11 ("Changed: Go 1.26.11 is the floor"), and a
session request that carries `systemPrompt` is now checked on both sides ("Added: the system prompt's gates").

#### Added: the client system prompt (D4)

A client can give an agent system-level instructions for a session, stable in ACP since schema `1.30.0`:

- `SystemPrompt{Mode, Content}` on `NewSessionRequest`, `LoadSessionRequest` and `ResumeSessionRequest`;
- `SystemPromptMode`, an open string type, with `SystemPromptModeAppend` and `SystemPromptModeOverride`;
- the agent's capability, `SessionCapabilities.SystemPrompt`, a `*SessionSystemPromptCapabilities`. `{}` offers `append`;
  its `Mode.Override` offers `override` too.

There is no constructor: the value is a two-field struct, and `acp.TextBlock` makes its content (D7). The README's "System
prompts" has an example for each side. (`894726f`, `c36fdfe`.)

#### Added: the system prompt's gates (D5, D6)

The SDK holds both sides to the schema's rules for the system prompt, as it does for every stable capability. A typed method
returns the error and writes nothing. A received request is answered `-32602`, and the agent's handler is not called, so no
session is created, loaded or resumed.

| What | Refused as |
| :--- | :--- |
| A `systemPrompt` the agent did not advertise | `*acp.CapabilityError`, `sessionCapabilities.systemPrompt`, field `systemPrompt` |
| `override` without `mode.override` | `*acp.CapabilityError`, `sessionCapabilities.systemPrompt.mode.override`, field `mode` |
| A mode the schema does not list, one beginning with `_` and a `null` or empty one included | `*acp.CapabilityError`, `sessionCapabilities.systemPrompt.mode.<mode>`, field `mode` |
| A content block that is not `text` | `*acp.CapabilityError`, `sessionCapabilities.systemPrompt.content.<type>`, field `content[i]` |
| Empty `content` | `*acp.ValidationError`, `acp.RuleMinItems`, field `content` |

- **What changes on the wire.** Only a `session/new`, `session/load` or `session/resume` that carries `systemPrompt`.
  `v1.35.0`'s types had no such field. An agent built on this release that does not advertise the capability answers such a
  request `-32602`. A request without the field is handled as before.
- **Before sending.** `Negotiation.AllowsValue(acp.AgentMethodSessionNew, req)` returns the error a send would, without
  sending, so a client can leave the prompt out or change its mode.
- **Opting in.** An agent that implements a mode of its own, or takes an empty prompt, accepts it with
  `acp.WithInboundValidation`, which is given the SDK's refusal.
- **Not checked.** The specification's re-send of the prompt on `session/load` and `session/resume`, and content beyond an
  agent's own limits or policy, which stays the agent's to refuse.
- **Not added.** The unstable `session/fork` carries no `SystemPrompt` in package `acp`: its request is frozen at its
  `1.25.0` shape.

(`06aa72c`, `894726f`.)

#### Changed: Go 1.26.11 is the floor

`go.mod`'s `go` line is `1.26.11`, up from `1.26.10`, and its `toolchain` line is `go1.27.4`, up from `go1.27.3`. A module
that requires this release needs Go 1.26.11 or newer. GO-2026-6859 (CVE-2026-97054), a race in `net/http`'s
`MaxBytesReader` that can crash an HTTP/1 server, is fixed in those two versions; package `remote` calls
`http.MaxBytesReader`. CI tests both
([0055-MADR-pin-go-1-27-4-and-floor-1-26-11.md](../decisions/0055-MADR-pin-go-1-27-4-and-floor-1-26-11.md)). The module's
requirements are unchanged. (`4d82028`.)

#### The schema pin: 1.31.0 (D1, D2)

- **The frozen names stay.** `schema/unstable-v1.json` is not rewritten, so the names package `acp` took from the unstable
  schema at `1.25.0` keep that shape, and the NES methods stay in `acp`, deprecated.
- **`x-deserialize-default-on-null`.** The schema now marks 14 stable arrays and objects, such as a session request's
  `mcpServers`, as reading `null` as empty. The generator reads the keyword, and refuses it on a node that is neither. The
  decoders already read `null` that way at all 14, so nothing changes on the wire; `TestDefaultOnNullLists` now holds them
  to it.
- **Doc comments.** 134 definitions' doc comments change wording and links, and nothing else.

(`0d8f211`, `894726f`.)

#### Added, outside the v1 promise: package `unstable`

Package `unstable` gains its first generated file, `unstable_gen.go`, with 46 names: the types and method-name constants
of the unstable methods `auth/status`, `auth/status_update`, `session/archive`, `session/unarchive`, `session/options`,
`session/set_title` and `session/status`, with their constructors and decoders, and `ToolCallContentSession`. Package
`acp` does not dispatch these methods: a received one is answered `-32601`, as any method `acp` does not declare. The names
may change or go in any release.

**Not in it: the proxy methods.** Schema `1.31.0`'s unstable `proxy/initialize` and `proxy/successor` have no type and no
constant in this release. The generator reads the schema's `proxyMethods` and emits nothing for them: their definitions use
forms it does not build, and no consumer needs proxy chains today. It is a recorded gap, in the MADR's "Amendment
2026-11-10: the proxy methods emit nothing". (`0d8f211`, `894726f`.)

#### For maintainers

- **The generator's table.** Four capability-gate kinds, `GatePresent`, `GateValue`, `GateUndeclaredValue` and
  `GateOnlyVariants`, and a validation list, `NonEmptyArrays`. The protocol table holds 36 capability rules, each quoting the
  schema sentence it follows. (`06aa72c`, `894726f`.)
- **Generated imports.** The emitter writes the standard library's imports first, then a blank line, then the others, as
  `goimports` formats them. `unstable_gen.go` is the first generated file that imports the module's own package.
  (`894726f`.)
- **A test fixed on Windows.** `TestGeneratedFilesUpToDate` compared a path built with the platform's separator with the
  generator's slash paths, and failed on Windows once a generated file was in a subdirectory. The generator was right.
  (`8cf69bf`.)
- **`tools/consumer.py`** sends, and has refused, a system prompt in its test, so its floor is `v1.36.0`.
  **`tools/consumer_probe.py`**, new, runs the consumer's program and test against a tree before the tag.
- **Tests.** Each rule has a test for send, receipt and `AllowsValue` that fails with its change undone in a copy of the
  tree; the PLAN's execution record lists each planted break and its failure.

## Sources not in the release text

- **Commits since `v1.35.0`** (`git log v1.35.0..c36fdfe`), thirteen:
  - `8d8642f` records 0052 as released in `v1.35.0`;
  - `a336669`, `bdbd841` and `108b344` close
    [0031-PLAN-verification-tools-in-repository.md](../decisions/0031-PLAN-verification-tools-in-repository.md): `tools/`
    and records, no Go file of the module;
  - `9491138` adds the 0053 report;
  - `130ea34` adds the 0054 MADR and PLAN;
  - `0d8f211`, phase 1 (D2, D3): the generator's loader and model, four engine cases;
  - `06aa72c`, phase 2 (D5, D6): the four gate kinds and the validation list, twelve engine cases;
  - `894726f`, phase 3 (D1, D4, D5, D6, D8): the pin, the schema files, rules R1 to R4 and the empty-content rule, the
    emitter's import groups, the regenerated files,
    `unstable/unstable_gen.go`, `system_prompt_test.go`, `default_on_null_test.go`, `testdata/encodings.golden`;
  - `3b46d36` adds the 0055 MADR and PLAN;
  - `8cf69bf`, 0054's deviation 6: `cmd/generate/main_test.go`;
  - `4d82028`, 0055: both `go.mod` files, `.github/workflows/ci.yml`, `README.md`, two test fixtures;
  - `c36fdfe`, phase 4: `README.md`, `docs/architecture.md`.
- **Go files changed.** `git diff --name-status v1.35.0 c36fdfe -- '*.go'` lists 31: three new (`system_prompt_test.go`,
  `default_on_null_test.go`, `unstable/unstable_gen.go`) and 28 changed. Of the root package's files that are not tests,
  only the generated `types_gen.go`, `agent_gen.go` and `client_gen.go` changed: no hand-written code of the module did.
- **Unchanged** (`git diff --stat v1.35.0 c36fdfe`, empty for these): `go.sum`, `cmd/generate/go.sum`, `schema/meta.json`,
  `schema/unstable-v1.json`, `agentproc/`, `acptest/`, `clienthost/`, `remote/` and `RELEASING.md`.
- **`make api`,** in phase 5's gate set: exit 0, "against v1.35.0: 0 incompatible, 57 compatible"; "reported packages
  unstable, remote: 0 incompatible, not gated". The 11 in the root package:
  - `NewSessionRequest.SystemPrompt`, `LoadSessionRequest.SystemPrompt`, `ResumeSessionRequest.SystemPrompt` and
    `SessionCapabilities.SystemPrompt`;
  - `SystemPrompt`, `SystemPromptMode`, `SystemPromptModeAppend` and `SystemPromptModeOverride`;
  - `SessionSystemPromptCapabilities`, `SessionSystemPromptModeCapabilities` and
    `SessionSystemPromptOverrideCapabilities`.

  The other 46 lines each begin "compatible: unstable:". None names a proxy method.
- **The gate set,** phase 5, step 2, with the Linux and Windows hosts at go1.27.4 and the floor gate at go1.26.11: see the
  PLAN's execution record, phase 5.
- **Interop,** phase 5, step 2: `python3 tools/interop.py`, exit 0, "all pairs passed", eight pairs against the official
  TypeScript and Python SDKs at their pinned revisions, over Streamable HTTP and WebSocket. Package `remote` did not change,
  so this is a regression check.
- **The consumers.** gobble-cli at `d6c10a5`, read as data and never written: its tree unpacked into a scratch directory,
  with a `replace` onto this tree, builds, vets and passes its tests with no other change. Its `go` line is `1.27.3`, above
  the new floor. It adopts the system prompt under its own records, as the MADR's "For gobble-cli" says. `tools/consumer.py
  v1.36.0` runs after the tag; `tools/consumer_probe.py` ran before it.
