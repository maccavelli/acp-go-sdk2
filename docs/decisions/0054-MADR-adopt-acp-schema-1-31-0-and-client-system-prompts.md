---
status: accepted
date: 2026-11-10
decision-makers: the repository owner
consulted: "this tree at `108b344` (`v1.35.0-4-g108b344`); the `schema-v1.25.0`, `schema-v1.30.0` and `schema-v1.31.0` release assets; the specification repository at `f3ed0fc` (main) and `0849001` (tag `schema-v1.31.0`); the TypeScript SDK `v2.5.0`/`v2.6.0`, the Rust SDK `v5.1.0` and the Python SDK `main`; claude-agent-acp, codex-acp, gemini-cli and goose, read as data; gobble-cli at `d6c10a5`, read as data; a regeneration from `1.31.0` in a scratch clone"
informed: gobble-cli, magic-cli-remote
---

# Adopt ACP schema 1.31.0, and gate and check the client system prompt as the SDK gates every other capability

## Context and Problem Statement

The SDK is pinned to ACP schema `1.25.0`
([0004-MADR-pin-acp-schema-1-25-0.md](0004-MADR-pin-acp-schema-1-25-0.md)). Upstream has since released stable schemas
`1.26.0` to `1.31.0`. [0053-REPORT-acp-1-25-0-completeness-parity-and-adherence.md](../reports/0053-REPORT-acp-1-25-0-completeness-parity-and-adherence.md)
recorded that and left it as background.
[0025-REPORT-schema-1-26-0-adoption-assessment.md](../reports/0025-REPORT-schema-1-26-0-adoption-assessment.md) advised
against adopting `1.26.0`, and named the conditions under which the answer becomes yes:

* a decided policy for unstable API under `v1`;
* a later schema with a stable change;
* a consumer that needs the system prompt.

All three now hold:

* [0039-MADR-five-adopter-additions-on-schema-1-25-0.md](0039-MADR-five-adopter-additions-on-schema-1-25-0.md) built the
  policy: package `unstable` and the frozen manifest `schema/unstable-v1.json`.
* `schema-v1.30.0` stabilized the client system prompt.
* The owner wants system prompt support for gobble-cli.

The owner asked for a researched, grounded decision record for adopting `1.31.0` with that support. This record decides
the pin and how the SDK treats the system prompt. Its PLAN is not written yet.

### Findings

Each finding is marked **Read** (text or code read at the cited place) or **Probe** (a run whose output is quoted). The
probes ran in the session scratchpad, against this tree or a scratch clone of it regenerated from `1.31.0`. Nothing in
this tree was changed by them.

#### Upstream

* **F1. One stable feature since the pin.** Read and Probe. The release notes of `1.26.0` to `1.31.0` list one stable
  addition: "*(schema)* stabilize client system prompt (#2765)", in `1.30.0`. A structural diff of the stable assets,
  `1.25.0` against `1.31.0`, finds:
  * `meta.json` byte-identical (SHA-256 `061edb6e…`): no new stable method;
  * `schema.json`: 178 to 183 definitions. It adds `SystemPrompt`, `SystemPromptMode`, `SessionSystemPromptCapabilities`,
    `SessionSystemPromptModeCapabilities` and `SessionSystemPromptOverrideCapabilities`, and a `systemPrompt` property
    on `NewSessionRequest`, `LoadSessionRequest`, `ResumeSessionRequest` and `SessionCapabilities`;
  * `x-deserialize-default-on-null`, a keyword new since `1.27.0` ("mark DefaultOnNull lists with an x-deserialize
    hint", #2569), on 14 stable properties: `args` and `env` of `AuthMethodTerminal`, `CreateTerminalRequest` and
    `McpServerStdio`; `headers` of `McpServerHttp` and `McpServerSse`; `additionalDirectories` and `mcpServers` of the
    three session requests;
  * 134 definitions whose description or title alone changed: wire names in place of Rust identifiers (#2520), versioned
    documentation links (#2525), and a plain-language pass (#2730).

  The stable `schema.json` of `1.30.0` and of `1.31.0` are byte-identical (SHA-256
  `e747a7bec3c02708b70885bf3cccac5c72fa421390dd7e85758fd61e33b1ae21`). What `1.31.0` adds over `1.30.0` is unstable only.
* **F2. The unstable half.** Probe. `meta.unstable.json` drops the ten NES and document-event methods, as `1.26.0` did
  (0025). It adds `auth/status`, `session/archive`, `session/unarchive`, `session/options`, `session/set_title`,
  `session/status` and the notification `auth/status_update`. It also gains a new top-level key, `proxyMethods`
  (`proxy/initialize`, `proxy/successor`, from the proxy-chains RFD in #2737). `schema.unstable.json` goes from 277 to 257
  definitions.
* **F3. The rules of the system prompt.** Read. The v1 book, `docs/protocol/v1/session-setup.mdx` L364-422 (identical at
  `f3ed0fc` and at the `schema-v1.31.0` tag):
  * L403: "Clients **MUST** only send `systemPrompt` when the Agent advertises `sessionCapabilities.systemPrompt`, and
    **MUST** only use `override` mode when the Agent also advertises `sessionCapabilities.systemPrompt.mode.override`."
  * L394: "`mode` values beginning with `_` are reserved for implementation-specific modes, and other unknown values for
    future ACP modes. Clients **MUST** only use a mode the Agent advertised. An Agent that receives a mode it does not
    support **MUST** return an error rather than treating it as `append` or `override`."
  * L397: "only `text` content blocks are supported for now: Agents **MUST** accept them, and Clients **MUST NOT** send
    other content block types until a future capability allows them."
  * L398: "Clients **MUST NOT** send an empty `content` array. Agents **MAY** reject one with `-32602` without creating,
    loading, or resuming the session."
  * L400: on load and resume, clients **SHOULD** re-send the prompt they created the session with. An agent given a
    different one **SHOULD** apply it, and **MAY** defer the change until it next rebuilds its prompt.
  * L401: an agent that cannot apply a supplied prompt "**MUST** return an error and **MUST NOT** create, load, or resume
    the session". `-32602` is the RECOMMENDED error. `docs/protocol/v1/error.mdx` L49 repeats this.
* **F4. The schema's shape.** Read. In `1.31.0`:
  * `SystemPrompt` requires `mode` and `content`;
  * `content` is an array of `ContentBlock`, with no `minItems`, so the empty-array rule is prose only;
  * `SystemPromptMode` is open: `append`, `override`, and a plain-string branch titled "other";
  * the three request properties are `SystemPrompt` or `null`, without `x-deserialize-default-on-error`;
  * `SessionCapabilities.systemPrompt` and the two nested capability properties carry `x-deserialize-default-on-error`.

  #2361's body names the split: "Strict request field, lenient capabilities … a malformed value fails the request with
  `-32602` instead of creating a session without the prompt. This covers a missing or `null` `mode`, a missing
  `content`, a malformed content block."
* **F5. Why it is gated.** Read. The RFD, `docs/rfds/client-system-prompt.mdx` (status Completed), L154: "Without a
  capability, an 'optional' field is a de facto breaking change: existing agents would silently ignore it … The
  capability lets agents opt in and then be held to the behavior, and gives clients reliable feature detection". L157:
  `override` "changes the safety posture — so it is double opt-in: agent advertises, client explicitly requests". L307
  keeps the baseline at text "to keep the implementation floor low". The status field was replaced by a JSON-RPC error
  on 2026-05-21, in RFD PR #1237.
* **F6. The other SDKs carry the types and leave the gate to the application.** Read.
  * The TypeScript SDK exports the types from `v2.5.0` (2026-11-09; `v2.4.0` does not). A doc comment
    (`src/v2/acp.ts` L26-30) tells the application to check the capability. Its zod `content` has no `.min(1)`.
  * The Rust SDK carries them from `v5.1.0`. Its `md/protocol.md` L31 reads "After checking the agent's advertised
    capability"; malformed values fail decoding with `-32602`.
  * The Python SDK is pinned to `schema-v1.23.0` and has no support.
* **F7. Two agents advertise it, and no client is listed.** Read.
  * claude-agent-acp advertises `append` and `override` from `v0.92.0` (2026-10-16). It refuses other modes and non-text
    blocks with `-32602`, and accepts empty content.
  * codex-acp advertises `append` from `v2.3.0` (2026-10-26). It refuses empty content, non-text blocks and other modes
    with `-32602`.
  * gemini-cli (`e5d88fb`) and goose (`b7a5014`) do not advertise it. Goose has its own
    `_goose/unstable/session/system-prompt/set`.
  * #2765's body: "No Client implementation is listed in the RFD yet."

#### This SDK

* **F8. The generator refuses `1.31.0` as downloaded.** Probe. `go run .` in `cmd/generate` stops with
  `parse schema.json: definition "AuthMethodTerminal": json: unknown field "x-deserialize-default-on-null"`. With that
  keyword removed from the copies, it stops with `parse meta.unstable.json: json: unknown field "proxyMethods"`. Both
  refusals are by design:
  * `load.Definition.UnmarshalJSON` decodes strictly: "A keyword Definition does not model is an error, so a schema
    update that adds one fails here rather than being ignored" (`cmd/generate/internal/load/load.go:99-101`, `:115`).
  * Every keyword it models has a treatment in `model/keywords.go:28-62`, the table of
    [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md).
  * `load.Meta` declares `agentMethods`, `clientMethods` and `protocolMethods` only (`load.go:19-25`).
* **F9. Regenerated, nothing breaks.** Probe. With both keys removed from the scratch copies:
  * Generation succeeds, and `go build ./...` and `go vet ./...` pass.
  * `scripts/check_api.py v1.35.0` reports "0 incompatible, 57 compatible": 11 additions in `acp` (the five types, the
    two mode constants and the four fields) and 46 in `unstable`.
  * The NES names stay in `acp`, built from the manifest as 0039 designed, so 0025's 115 incompatible changes do not
    occur.
  * `unstable/unstable_gen.go` is generated for the first time: 1,765 lines, 28 types and 85 functions, with the proxy
    methods left out.
  * `UnstableForkSessionRequest`, frozen by the manifest, gains no `SystemPrompt`. The unstable `configOptions` on the
    three stable requests is emitted nowhere, as 0039 decided for a new unstable field on a stable type.
* **F10. The tests that move.** Probe, in the scratch clone.
  * The root module fails only `TestEncodingsUnchanged`. Rewritten with `-update`, `testdata/encodings.golden` gains 21
    lines, one for each instance of the five new types, and changes 6. The changed lines are two instances each of the
    new-, load- and resume-session requests, which now carry a `systemPrompt`. Nothing existing is removed.
  * The generator module fails two tests:
    * `TestGeneratedFilesUpToDate` (`cmd/generate/main_test.go:26-33` lists six files; the run renders
      `unstable/unstable_gen.go` too).
    * `TestCapabilitySourcesQuoteTheSchema`: the three NES rules (`model/acp.go:235-237`) quote sentences that
      `schema.unstable.json` no longer holds. The test reads only `schema.json` and `schema.unstable.json`
      (`model/sources_test.go:49`); the sentences are still in the manifest.
* **F11. The null-list keyword changes nothing on the wire today.** Probe. Against this tree, each of the 14 properties
  of F1 set to `null` decodes without error and validates. It re-encodes as `[]` when required and is omitted when
  optional: for example `{"cwd":"/a","mcpServers":[]}` from `"mcpServers":null`. That is the behaviour the keyword asks
  for.
* **F12. A system prompt is dropped today.** Probe. This tree decodes a `1.30.0` `session/new` holding a `systemPrompt`
  without error, and re-encodes it without the key. It does the same with `SessionCapabilities.systemPrompt`. A gobble
  agent cannot see a client's prompt or advertise the capability. No error results, because a conforming client sends
  nothing unadvertised.
* **F13. Regenerated, the types arrive with no rules.** Probe, in the scratch clone.
  * `NewSessionRequest`, `LoadSessionRequest` and `ResumeSessionRequest` gain
    `SystemPrompt *SystemPrompt \`json:"systemPrompt,omitempty"\``.
  * `SessionCapabilities` gains `SystemPrompt *SessionSystemPromptCapabilities`.
  * `SystemPromptMode` is a `string` type with the constants `SystemPromptModeAppend` and `SystemPromptModeOverride`.

  Without protocol-table rules:
  * `Negotiation{}.AllowsValue("session/new", …)` returns nil for an `override` prompt.
  * A missing `mode`, a missing `content` and a malformed text block are refused on decode.
  * `"mode": null` decodes as `""` and re-encodes as `"mode":""`.
  * Empty `content`, the mode `_vendor`, an `image` block and an unknown block type all decode and validate.
* **F14. The gate machinery has no kind for three of the rules.** Read. `GateKind` (`model/protocol.go:135-161`) has
  `GateNonEmpty` (a non-empty array), `GateVariant` (a union variant), `GateNotTerminalAuthMethod`,
  `GateUndeclaredVariant` (a union's catch-all) and `GateAdvertisedAuthMethod`. None gates:
  * an optional object being present;
  * a string property holding one value;
  * a union holding any variant but the listed ones.

  `gateCheck` resolves each kind (`model/gates.go:305-337`).
* **F15. How refusals are answered, and the way round them.** Read.
  * On receipt, a `*CapabilityError` that names a field or variant is answered `-32602`; one naming neither is answered
    `-32601` (`connection.go:3329-3341`). A `ValidationError` is answered `-32602`.
  * `WithInboundValidation` lets an application accept a message the SDK refused, or refuse one it accepted
    (0015 H1).
  * `Negotiation.AllowsValue` (`helpers.go:1069`) reports, before sending, the error a gate would return.
  * Validate already has `RuleMinItems` (used by the elicitation checks of
    [0040-MADR-check-elicitation-forms-answers-and-urls.md](0040-MADR-check-elicitation-forms-answers-and-urls.md)).
    The loader does not model the `minItems` keyword.
* **F16. The precedent for an unlisted value.** Read. `GateUndeclaredVariant` refuses an elicitation mode and a prompt
  block type that the schema does not list. The protocol table says "A mode the schema does not list is one no
  capability advertises" (`model/acp.go:217-219`, from 0019 decision 6). The empty `prompt` array on `session/prompt`,
  which F5's RFD cites as the model for the empty-content rule, is not refused by this tree (Probe: `Validate` returns
  nil).
* **F17. Two documents name the old state.** Read. `docs/architecture.md:927` says the manifest was frozen from `1.25.0`
  (true after the bump too). `:978` says "At `1.25.0`, `unstable/` holds only `doc.go`". `RELEASING.md:79-80`: "A schema
  bump never rewrites `schema/unstable-v1.json`".

#### gobble-cli (at `d6c10a5`, read as data; its working tree had uncommitted changes, so lines are HEAD's)

* **F18. gobble is the agent.** Read.
  * It requires this module at `v1.35.0`, with no `replace` (`go.mod:14`).
  * `gobble acp` serves `*acpserver.Agent` with `acp.ServeAgent` (`acpserver/serve.go:45`). Its in-process client
    (`acpclient/conn.go:150`, `agentproc.InProcess`) drives only that agent; it launches no other agent.
  * The external clients its records name are mcremote and magic-cli-remote.
* **F19. It has a system prompt, and no ACP path to it.** Read.
  * `prompt/sections.go:136-163` assembles named sections, `preamble` to `cwd`.
  * `--system-prompt` and `--append-system-prompt` (`internal/cli/flags.go:55-56`) set `PromptOptions.System` and
    `.Append`, a Go struct handed to the in-process agent. `gobble acp` has neither flag (`internal/cli/acp.go:22-27`).
  * The prompt is frozen per session as a baseline. Later changes are patches sent as messages, so the cached prefix
    holds (`acpserver/sysprompt.go:256-296`).
  * It advertises its session capabilities in `Capabilities()` (`acpserver/agent.go:384` at HEAD).
* **F20. The new fields break nothing in gobble.** Read. Every literal of the three requests and of
  `SessionCapabilities` is keyed, and none of them is compared with `==`. Its goldens hold the `initialize` answer and
  the `session/new` params; omitted pointer fields leave them unchanged.

## Decision Drivers

* The owner wants gobble-cli to support client system prompts, and gobble is an ACP agent (F18).
* The `v1` compatibility promise
  ([0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md](0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md)), which
  `scripts/check_api.py` gates.
* Every capability rule the specification states is enforced on send and on receipt
  ([0017-MADR-remediate-acp-v1-conformance-violations.md](0017-MADR-remediate-acp-v1-conformance-violations.md), G1).
  Today every stable capability of `1.25.0` is gated (0053).
* Unstable additions go to package `unstable`, and a frozen `acp` name never goes away under `v1` (0039).
* Every schema keyword has a decided treatment, and an unknown one stops the generator (0012; F8).
* The previous pins took the latest stable schema (0004).

## Considered Options

* **O1.** Pin `1.31.0`. Model the two new upstream keys. Gate and check the system prompt in the protocol table on send
  and on receipt.
* **O2.** O1's rules, pinned to `1.30.0`.
* **O3.** Pin `1.31.0` with the types only, and leave the rules to applications, as the TypeScript and Rust SDKs do.
* **O4.** Stay on `1.25.0`, and carry a system prompt between gobble and its clients in `_meta` or an extension method.
* **O5.** Wait for ACP v2, and take the system prompt with a `/v2` module.

## Decision Outcome

Chosen option: "O1", accepted by the owner on 2026-11-10, because it gives gobble the protocol's own system prompt with the same guarantees every other
capability has. It changes no exported name: 0 incompatible changes (F9). It also uses the unstable boundary 0039
built for this case. Of the other options:

* O2 has the same stable schema (F1) on an older pin.
* O3 would make the system prompt the only stable capability the SDK does not gate.
* O4 is gobble-only and not interoperable.
* O5 waits on a draft with no date.

### D1. The pin

`schema/version` becomes `1.31.0`. `make version` downloads the four assets of `schema-v1.31.0`, whose SHA-256 are:

| File | SHA-256 |
| :--- | :--- |
| `meta.json` | `061edb6efa8fb2aa2792459a86ec7268de5fe665bba48b2ffe7939df01481f88` |
| `meta.unstable.json` | `99f1b10aca5c1cf516bc1eb330e980e2fafd4029249bbe4c8557119ae1800ffc` |
| `schema.json` | `e747a7bec3c02708b70885bf3cccac5c72fa421390dd7e85758fd61e33b1ae21` |
| `schema.unstable.json` | `0aa22eba8becd67c67e6da0336b1761dc8684d2d12ba26baedeff1fd3ce257ea` |

The files are committed as downloaded, with no edit. The two keys of F8 are handled in the generator (D2, D3), not by
changing the schema. `schema/unstable-v1.json` is not rewritten (`RELEASING.md:79-80`). The frozen `acp` names keep their
`1.25.0` shape, and the NES methods stay in `acp`, deprecated.

This record supersedes [0004-MADR-pin-acp-schema-1-25-0.md](0004-MADR-pin-acp-schema-1-25-0.md)'s schema stamp, as 0004
superseded 0002's. 0004's other content stands.

### D2. `x-deserialize-default-on-null`

The loader models the keyword (`load.Definition`), and the keyword table gives it `TreatDecode`: a `null` value decodes
as the property's empty value. F11 shows the generated decoders already do this at all 14 stable properties, so the
generated code may not change. The model records the keyword on `Field` beside `DefaultOnError` and `SkipInvalidItems`
(`model/fields.go:113-115`), so the behaviour is decided rather than incidental.

Following the check for `x-deserialize-skip-invalid-items` (`model/schema.go:476-477`), the generator refuses the keyword
on a node that is not an array or an object. A conformance case decodes `null` at each property that carries the keyword
and requires the empty value, so a change to the decoders cannot drop the behaviour unseen.

### D3. `proxyMethods`

`load.Meta` declares `proxyMethods`. `proxy/initialize` and `proxy/successor` are unstable-only methods, so their types
and method-name constants go to package `unstable`, as 0039 decided for every new unstable method. `acp`'s dispatch
answers a received one `-32601`, as it does any unstable method `acp` does not declare.

Proxy methods belong to neither side, unlike every method the generator has placed so far. If the generator cannot
place them on that rule as it stands, the PLAN stops and this record is amended. They are not dropped silently.

*Amended 2026-11-10, in the PLAN's phase 1: the generator cannot place them. The proxy methods are modeled and emit
nothing; see "Amendment 2026-11-10: the proxy methods emit nothing" below.*

### D4. The types

The types are generated as F13 shows. No hand-written code changes for them. `SystemPromptMode` stays an open `string`
type, as the schema makes it, so a received unknown mode is kept and can be forwarded (RFD L257). `session/fork`'s
`systemPrompt` stays out of `acp` (F9): fork is unstable, and its frozen request is 0039's to keep.

### D5. Four capability rules

The protocol table (`model/acp.go`) gains four rules, all `Side: sideAgent`: the capabilities are the agent's, read from
its `initialize` answer. Each quotes the schema sentence it follows, which `TestCapabilitySourcesQuoteTheSchema`
requires. Four kinds are added to `GateKind` and to `gateCheck`, with emission in
`cmd/generate/internal/emit/gates.go`: the three F14 names, and an undeclared-value kind. The undeclared-value kind is
the open-string-enum counterpart of `GateUndeclaredVariant`, which reads only unions. *Corrected 2026-11-10 while the
PLAN was written: this sentence first said "the three new kinds of F14", but the table below has always named four.*

| Rule | Definition, property | Kind | Capability | Source (schema text) |
| :--- | :--- | :--- | :--- | :--- |
| R1 | `NewSessionRequest`, `LoadSessionRequest`, `ResumeSessionRequest`; `systemPrompt` | new: present | `sessionCapabilities.systemPrompt` | `SystemPrompt`: "Clients MUST only send this when the Agent advertises `sessionCapabilities.systemPrompt`." |
| R2 | `SystemPrompt`; `mode` = `override` | new: value | `sessionCapabilities.systemPrompt.mode.override` | `SystemPromptMode` `override`: "MUST only use it when the Agent advertises `sessionCapabilities.systemPrompt.mode.override`." |
| R3 | `SystemPrompt`; `mode` not `append` or `override` | new: undeclared value | none; the refusal names `sessionCapabilities.systemPrompt.mode.<value>` | `SystemPromptMode`: "An Agent MUST return an error for a mode it does not support rather than treating it as `append` or `override`." |
| R4 | `SystemPrompt`; `content` holds a block other than `text` | new: only the listed variants | none; the refusal names `sessionCapabilities.systemPrompt.content.<type>` | `SystemPrompt.content`: "Clients MUST NOT send other content block types until a future capability allows them" |

Notes on the rules:

* **R1** fails first. Without the capability, the whole field is refused, and R2 to R4 are not reached.
* **R3** refuses every mode the schema does not list:
  * `""`, which a `"mode": null` decodes to (F13), so a `null` mode is refused as #2361 asks;
  * modes beginning with `_`. The schema gives an agent no way to advertise one, and L394 says "Clients **MUST** only
    use a mode the Agent advertised". This is the elicitation-mode rule of F16.
* **R4** refuses `image`, `audio`, `resource_link`, `resource` and any unknown block type. A future schema that adds a
  content capability adds its own rule, as `promptCapabilities` has.

Each rule runs where every gate runs:

* **On send** it returns a `*CapabilityError` naming the field, and writes nothing. `Negotiation.AllowsValue` reports
  the same error beforehand, so a client can leave the prompt out or change its mode.
* **On receipt**, the agent's handler is not called, and the request is answered `-32602`, the code F3's L401
  recommends.

So an agent that advertises `append` only is held to F3's agent MUSTs before its code runs: it errors, and creates no
session, for an `override` prompt, an unknown mode or a non-text block. An agent that implements a `_` mode accepts it
with `WithInboundValidation` (F15). Content beyond an agent's own limits or policy stays the agent's to refuse
(L401, `security.mdx` L51).

### D6. Empty content

The protocol table gains a validation rule: `SystemPrompt.content` must not be empty. Its source is the schema's
"Clients MUST NOT send an empty array." `Validate` refuses an empty array with `RuleMinItems`:

* on send, so the SDK does not write one;
* on receipt, answered `-32602`, which L398 allows ("Agents **MAY** reject one with `-32602`").

An agent that prefers to accept an empty array does so with `WithInboundValidation`. The generator gains a table list
for this rule, as it has `AbsolutePaths`. It does not model the schema's `minItems` keyword, which this schema does not
use here (F4).

This record does not extend the rule to `session/prompt`'s `prompt` array (F16). That would change behaviour a released
SDK already has, and needs a record of its own.

### D7. No new hand-written helpers

The generated types, the gates, `AllowsValue` and `WithInboundValidation` are the whole SDK surface for the system
prompt. A constructor such as `SystemPromptText(mode, text)` would save little: the value is a two-field struct and
`TextBlock` already exists. Joining a prompt's text on the agent side is likewise the application's. This follows
[0050-MADR-auth-method-ids-and-the-prompt-result-for-issue-14.md](0050-MADR-auth-method-ids-and-the-prompt-result-for-issue-14.md)
D8, which declined constructors that the wire checks make redundant.

### D8. The tests and documents that move

* **`testdata/encodings.golden`** is rewritten with `-update`. The diff must show only additions, and changed lines whose
  only change is a `systemPrompt` key (F10).
* **`cmd/generate/main_test.go`**: the list of generated files gains `unstable/unstable_gen.go`.
* **`TestCapabilitySourcesQuoteTheSchema`** also reads the descriptions in the manifest's `$defs`. A frozen rule quotes
  the schema its type is built from, and the NES sentences are there (F10).
* **Each new gate kind and the validation rule** gets tests for send, receipt and `AllowsValue`. Each test is seen to
  fail on a planted break in a copy, as this repository requires.
* **`docs/architecture.md`** loses ":978 At `1.25.0`, `unstable/` holds only `doc.go`". It names the new keyword and the
  proxy methods.
* **`README.md`** gains a paragraph on the system prompt: the capability, the gates and what an agent advertises.

### D9. The release

This is a minor release, `v1.36.0`. `scripts/check_api.py` against `v1.35.0` must report no incompatible change in a
gated package; F9 measured 0. The `unstable` package's additions are reported, not gated (0039). The release notes name
the pin, the system prompt and its gates, the new `unstable` API, and the keyword.

### Questions for the owner

1. **The pin.** Recommended: `1.31.0`, the latest stable, whose stable schema equals `1.30.0`'s (F1).
   Alternative: `1.30.0`, the release that stabilized the feature, with less unstable surface (no `session/options`).
2. **Unlisted modes (R3).** Recommended: refuse every mode but `append` and `override`, `_` modes included, on send and
   on receipt; an agent that supports a `_` mode accepts it with `WithInboundValidation`. Alternative: let `_` modes
   pass both ways, with an agent's own code refusing what it does not support.
3. **Empty content on receipt (D6).** Recommended: refuse it with `-32602`, as codex-acp does. Alternative: refuse it on
   send only, and let an agent accept it, as claude-agent-acp does.
4. **Proxy methods (D3).** Recommended: their types and constants go to `unstable`. Alternative: model the key and
   generate nothing for its methods, recorded as such.
5. **Helpers (D7).** Recommended: none. Alternative: a constructor and a text accessor for `SystemPrompt`.
6. **Release (D9).** Recommended: `v1.36.0` at the close of this record's PLAN.

#### Decided with the owner, 2026-11-10

Each question was put with three options, the recommended one first. The owner chose the recommendation for all six.
D1 to D9 stand as written.

| Question | Answer | The other options put |
| :--- | :--- | :--- |
| 1. The pin | `1.31.0` | `1.30.0`; the newest stable at PLAN time |
| 2. Unlisted modes | Refuse every mode but `append` and `override`, `_` modes included, on send and on receipt; `WithInboundValidation` opts an agent in | let `_` modes pass both ways; refuse on send only |
| 3. Empty content | Refuse on send and on receipt (`-32602`) | on send only; not at all |
| 4. Proxy methods | Their types and constants to `unstable`; if the generator cannot place a method of neither side, the PLAN stops and this record is amended. *Superseded 2026-11-10 by the amendment below: model the key and emit nothing.* | model the key and emit nothing; full proxy support with dispatch hooks |
| 5. Helpers | None | a constructor and a text accessor; a constructor only |
| 6. Release | `v1.36.0` at the close of the PLAN | two releases, the types ungated first; hold the tag until gobble-cli is ready |

### Consequences

* Good, because gobble can advertise `sessionCapabilities.systemPrompt` and receive a client's prompt typed, checked and
  refused before its handler when it breaks a rule the agent's own advertisement sets.
* Good, because an SDK client cannot send an unadvertised prompt, an unadvertised mode, a non-text block or an empty
  array. The SDK is stricter than the TypeScript and Rust SDKs here (F6), and as strict as it is for every other stable
  capability.
* Good, because no exported name changes or goes away (F9), and the frozen NES API stays as 0039 designed.
* Good, because the pin is current again, and the next bump starts from the latest stable schema.
* Neutral, because 46 unstable names appear in package `unstable`, outside the compatibility promise. No consumer uses
  them yet.
* Neutral, because 134 definitions' doc comments change wording and links, and nothing else.
* Bad, because the generator gains four gate kinds and a validation list, more of the table to maintain, for one
  feature.
* Bad, because an agent that wants an empty prompt or a `_` mode must say so with `WithInboundValidation`.
* Bad, because the upstream cadence (six minors in a month, F1) means this pin will be behind again soon.

#### For gobble-cli (informed; its records decide)

* Advertising `SessionCapabilities.SystemPrompt` in `Capabilities()` opts gobble in, `append` first.
  `Mode.Override` follows only if gobble implements a replace of its built-in instructions. Its present "replace" keeps
  the personality, addendum, context files, skills and `cwd` (F19). L396 lets workspace context stay; whether
  personality may stay is for gobble's record.
* A prompt that differs on `session/load` or `session/resume` "**SHOULD**" be applied and "**MAY**" be deferred until
  the prompt is rebuilt (L400). gobble's frozen baseline and its patch path (F19) are one way to meet that.
* gobble's in-process client is held to the same send gates.

### Confirmation

* `schema/version` reads `1.31.0`, and the four schema files hash to D1's values.
* `make version` succeeds on the downloaded files with no edit to them.
* `scripts/check_api.py v1.35.0` reports no incompatible change in a gated package.
* The root and generator test suites pass under `make pre-add-check`.
* Each new gate kind and the validation rule has tests seen to fail on a planted break, run in a copy.
* `AllowsValue` returns R1's error for a prompt without the capability, R2's for `override` without `mode.override`,
  and nil when both are advertised.
* A received `session/new` with an unadvertised prompt is answered `-32602`, and the agent's `NewSession` is not called.
* gobble-cli at `d6c10a5` builds and passes its tests against the release candidate with no change. This is a read-only
  check in a scratch copy.

## Pros and Cons of the Options

### O1. Pin 1.31.0, model the keys, gate and check the system prompt

* Good, because it meets every driver: gobble's need, `v1`, 0017 G1, 0039 and 0012.
* Good, because the receipt-side gates do an advertising agent's MUSTs before its handler runs.
* Neutral, because it takes `1.31.0`'s unstable additions with it, into `unstable`.
* Bad, because it is the most generator work of the options: four gate kinds, a validation list and two loader keys.

### O2. O1's rules on 1.30.0

* Good, because `1.30.0` is the release that stabilized the feature, and its stable schema equals `1.31.0`'s.
* Neutral, because it still needs D2 and D3. `x-deserialize-default-on-null` arrived in `1.27.0` and `proxyMethods` in
  `1.29.0`. Both are in the `1.30.0` assets: its `meta.unstable.json` holds the same two proxy methods (Probe).
* Bad, because it pins a release that is no longer the latest, against 0004's practice, for no stable difference.

### O3. Pin 1.31.0, types only

* Good, because it is the least work: D1 to D4 alone.
* Good, because it matches the TypeScript and Rust SDKs (F6).
* Bad, because it makes the system prompt the only stable capability the SDK does not gate. An SDK client could send
  an unadvertised `override`, and an SDK agent would receive one in its handler, against 0017 G1.
* Bad, because each agent would rewrite the checks claude-agent-acp and codex-acp each wrote (F7).

### O4. Stay on 1.25.0; a `_meta` key or an extension method

* Good, because nothing in this SDK changes.
* Bad, because only gobble and its own clients would understand it. claude-agent-acp's `_meta.systemPrompt` and goose's
  extension method (F7, 0025) are the fragmentation the RFD was written to end.
* Bad, because the pin stays six minors behind.

### O5. Wait for ACP v2 and a `/v2` module

* Good, because a major could carry other breaks at once.
* Bad, because v2 is a pre-release (`schema-v2.0.0-alpha.15`) with no date, and the feature is already stable in v1.
* Bad, because every importer would change its import path.

## More Information

### Sources

* Release assets: <https://github.com/agentclientprotocol/agent-client-protocol/releases/tag/schema-v1.31.0> and
  `schema-v1.30.0`, downloaded and hashed. Release notes of `schema-v1.26.0` to `schema-v1.31.0`.
* The v1 book, `docs/protocol/v1/session-setup.mdx` L364-422; `error.mdx` L45-49; `initialization.mdx` L299-305;
  `security.mdx` L51. All at `f3ed0fca2fd6d85ef7ae77cfd074cc8605e490dc`.
* The RFD `docs/rfds/client-system-prompt.mdx` (blob `f82c0c91`), L106-157, L243, L257, L295-307.
* Upstream PRs #2361, #2739, #2765 (their bodies; none has review comments), and #1237 for the error decision.
* The TypeScript SDK (`src/v2/acp.ts` L26-30, `src/v1/schema/zod.gen.ts` L2421-2428), the Rust SDK (`md/protocol.md`
  L12-44), the Python SDK (`schema/VERSION`).
* claude-agent-acp (`src/client-system-prompt.ts` L24-58), codex-acp (`src/ClientSystemPrompt.ts` L16-24), gemini-cli
  (`packages/cli/src/acp/acpRpcDispatcher.ts` L91-102), goose (`crates/goose/src/acp/server.rs` L1782-1797).

### Records

* [0004-MADR-pin-acp-schema-1-25-0.md](0004-MADR-pin-acp-schema-1-25-0.md), whose schema stamp this supersedes.
* [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md), the keyword table.
* [0015-MADR-validate-absolute-protocol-paths.md](0015-MADR-validate-absolute-protocol-paths.md), the validation hook.
* [0017-MADR-remediate-acp-v1-conformance-violations.md](0017-MADR-remediate-acp-v1-conformance-violations.md), G1.
* [0019-MADR-remediate-debugging-pass-findings.md](0019-MADR-remediate-debugging-pass-findings.md), decision 6 and
  question 16.
* [0025-REPORT-schema-1-26-0-adoption-assessment.md](../reports/0025-REPORT-schema-1-26-0-adoption-assessment.md),
  whose three conditions this record meets.
* [0039-MADR-five-adopter-additions-on-schema-1-25-0.md](0039-MADR-five-adopter-additions-on-schema-1-25-0.md), the
  unstable boundary.
* [0050-MADR-auth-method-ids-and-the-prompt-result-for-issue-14.md](0050-MADR-auth-method-ids-and-the-prompt-result-for-issue-14.md), D8.
* [0053-REPORT-acp-1-25-0-completeness-parity-and-adherence.md](../reports/0053-REPORT-acp-1-25-0-completeness-parity-and-adherence.md),
  the parity at the old pin.

### The probes

The scratch clone was made with `git clone --local` from `108b344`. The `1.31.0` assets were copied in, and the two
keys of F8 were removed from the copies only, to see past the refusals. It was then regenerated with
`cmd/generate`'s `go run .`, and `go build`, `go vet`, `go test` and `scripts/check_api.py v1.35.0` were run. A second
module, with a `replace` pointing to the clone, decoded and validated the system-prompt cases of F13. A third, pointing
to this tree, ran the null-list and dropped-prompt cases of F11 and F12. The numbers in F9 and F10 come from that clone:
the generator's real output once D2 and D3 are built will differ by the proxy methods' types in `unstable` and by the
gates of D5 and D6, and the PLAN measures it again.

### Next

A PLAN, `0054-PLAN-adopt-acp-schema-1-31-0-and-client-system-prompts.md`, once the owner accepts this record. The
questions were answered on 2026-11-10.

## Amendment 2026-11-10: the proxy methods emit nothing

Found in the PLAN's phase 1, by reading the generator before any edit. The MADR's question 4 answer, the proxy methods'
types and constants in `unstable`, cannot be built on the generator as it stands. Three of its rules refuse what the
`1.31.0` unstable schema holds:

* **Alias definitions.** `InitializeProxyRequest` and `InitializeProxyResponse` are each
  `{"allOf": [{"$ref": "#/$defs/InitializeRequest"}]}` (or `InitializeResponse`). `checkTypable` refuses `allOf` on a
  definition: "allOf on a definition is not supported" (`cmd/generate/internal/model/schema.go:357`-`:358`).
* **One wire, two roles.** `proxy/successor` links `ProxySuccessorRequest`, `ProxySuccessorResponse` and
  `ProxySuccessorNotification`: it wraps either a request or a notification. `buildMethod` refuses a wire with both a
  request and a notification (`model/methods.go:213`-`:214`). `ProxySuccessorResponse` has no type at all.
* **A side of its own.** `"x-side": "proxy"` takes no binding (`bindingOf`, `model/methods.go:268`-`:283`).

The owner chose, on 2026-11-10, Q4's alternative of the three put: **model the key and emit nothing**. The other two
were building the generator support (alias definitions, a dual-role wire, a binding-less side), and the constants alone.

* `load.Meta` declares `proxyMethods`, so `meta.unstable.json` loads. A stable `meta.json` that holds proxy methods is
  still refused, since the SDK dispatches only agent, client and protocol methods.
* The proxy methods are merged into no side. So no proxy wire is unstable-only to the merge, none of their definitions
  is reached, and package `unstable` holds no proxy name. That is what the probe of F9 measured with the key removed:
  46 additions in `unstable`.
* An engine case pins the omission. The release notes name it, so it is a recorded gap, not a silent drop.
* Building the proxy types needs its own record: alias definitions and a dual-role wire are new generator machinery.
  That record waits until a consumer needs proxy chains; none does today.

## Released in v1.36.0, 2026-11-11

D1 to D9 shipped in `v1.36.0`, the tag naming `3be7af6`, as question 6 decided, with D3 as its amendment has it. The
PLAN's execution record holds the CI runs, the publication's ten checks and the consumer at the floor. The release also
carries the Go pin of
[0055-MADR-pin-go-1-27-4-and-floor-1-26-11.md](0055-MADR-pin-go-1-27-4-and-floor-1-26-11.md), which the PLAN's
deviation 5 found. The PLAN's phase 5 checked this record's Consequences against what was built and found no
difference.
