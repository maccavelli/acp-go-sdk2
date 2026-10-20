# Schema 1.26.0: what adopting it would bring, what it would cost, and whether to adopt it now

Observed 2026-10-20 on `2965021`, at the owner's request. This report assesses adopting ACP schema `1.26.0` in place of the pinned
`1.25.0` (`schema/version`), against the SDK and its two known consumers, `magic-cli-remote` and `<cli-b>`. It decides nothing:
the decision belongs to the record question 16 of
[0019-MADR-remediate-debugging-pass-findings.md](../decisions/0019-MADR-remediate-debugging-pass-findings.md) calls for, which a
`0025` MADR can be.

**Opinion: no, not now.** The stable protocol does not change; the one addition is an unstable Draft feature no consumer uses; and the
removal half breaks this `v1` module's API in 115 places, which Go's compatibility rule and this repository's records do not allow in
a `v1` minor. Adopt it when a decided policy for unstable API makes the removal lawful, or when a stable change or a consumer's need
makes the cost worth paying. The reasons and the conditions follow.

## How it was assessed

- **Upstream:** the GitHub releases of `schema-v1.26.0`, `schema-v1.25.0`, the Rust crate `v1.12.0` and `v1.11.0`, and the
  TypeScript SDK `v1.9.0` and `v2.0.0`; the three pull requests 1.26.0 holds; the docs at `schema-v1.26.0` (`docs/rfds/about.mdx`,
  `docs/rfds/updates.mdx`, `docs/rfds/client-system-prompt.mdx`, `docs/libraries/community.mdx`); `claude-agent-acp`'s source and
  history; `coder/acp-go-sdk`'s releases. All read through the GitHub API.
- **Go:** go.dev/ref/mod, go.dev/doc/modules/version-numbers, go.dev/blog/v2-go-modules and go.dev/blog/module-compatibility,
  downloaded and quoted.
- **This SDK:** the four schema files of `1.26.0`, downloaded and compared with the committed ones; the tree regenerated from them in
  a scratch clone; `scripts/check_api.py` (`apidiff`) against `v1.17.1` there.
- **Consumers:** the working trees of `magic-cli-remote` and `<cli-b>`, read: their `go.mod` requirement on this module, every
  identifier of it they name, and every line naming an identifier 1.26.0 removes.

Nothing in this repository, the consumers or `$HOME` was written but this report and its row.

`<cli-b>` stands for the second consumer, a CLI the owner named in the request, whose name this repository's identifier scan keeps
out of committed files.

## What 1.26.0 is

`schema-v1.26.0`, published 2026-10-16, a week after `1.25.0`. Its release notes, whole:

- Added: "*(unstable)* add client system prompt schema (#2361)"
- Fixed: "*(unstable)* remove next edit suggestions (#2266)"
- Other: "*(unstable)* clarify that a forked session is active (#2359)"

Compared file by file with the committed `1.25.0`:

| File | Change |
| :--- | :--- |
| `schema.json` | byte-identical |
| `meta.json` | byte-identical |
| `schema.unstable.json` | 277 definitions to 225: 57 removed, 5 added, 10 changed |
| `meta.unstable.json` | 10 agent methods removed, `document/didChange`, `didClose`, `didFocus`, `didOpen`, `didSave` and `nes/accept`, `close`, `reject`, `start`, `suggest`; none added |

- **Removed:** every Next Edit Suggestions (NES) definition, the document-event notifications that only NES used, and position-encoding
  negotiation (#2266: "Remove the NES-only document-event notifications and position-encoding negotiation … removing 57 definitions
  from each schema").
- **Added:** `SystemPrompt`, `SystemPromptMode` and three `SessionSystemPrompt*Capabilities`.
- **Changed:** `AgentCapabilities` and `ClientCapabilities` lose `nes` and `positionEncoding(s)`; `NewSessionRequest`,
  `LoadSessionRequest`, `ResumeSessionRequest` and `ForkSessionRequest` gain an optional `systemPrompt`; `SessionCapabilities` gains
  `systemPrompt`; the four envelope unions follow.

**What the stages mean upstream.** `docs/rfds/about.mdx`: in Draft, "Experimental implementation may begin, properly feature-gated
with the RFD name, to prove out the design before broader adoption"; "'completed' is the only state that can represent a 1-way door
(if there is a stability commitment involved)"; an RFD that landed in draft and is dropped moves to "To be removed … then they are
removed entirely". NES followed that path (`docs/rfds/updates.mdx`: "moved to 'To be removed' because the proposal was no longer being
pursued", then "removed from ACP v1 and v2"). The client system prompt is at Draft (moved 2026-10-12), and "The feature stays unstable
until the RFD reaches Preview."

## Benefits of adopting it

- **The client-provided system prompt.** A client can give an agent session-scoped instructions in `append` or `override` mode on
  `session/new`, `load`, `resume` and `fork`, and an agent can advertise it. Before it, the only route was a `_meta` key or an
  agent's own extension (the RFD's survey: `claude-agent-acp`'s `_meta.systemPrompt`, Goose's `_goose/unstable/session/system-prompt/set`).
  `claude-agent-acp` implements the unstable field already (`src/client-system-prompt.ts`, added 2026-10-16 with TypeScript SDK
  `1.9.0`).
- **Less dead code.** Regenerated, `types_gen.go` goes from 23,459 to 20,613 lines, `agent_gen.go` from 1,095 to 882, `client_gen.go`
  from 1,038 to 806, `helpers_gen.go` from 487 to 445 and `constants_gen.go` from 59 to 49: about 3,300 lines and 10 outbound methods
  for a feature upstream abandoned, which no agent will answer.
- **Staying current.** The schema the SDK pins matches upstream's latest `v1` again, and the next update starts from it. The removal
  must be absorbed at some update, so deferring it does not make it smaller.
- **The fork clarification** (#2359) changes only a description: a forked session is active at once.

The stable protocol gains nothing: `schema.json` and `meta.json` do not change.

## Costs and risks

**The API change.** Regenerated from `1.26.0`, `scripts/check_api.py` against `v1.17.1` reports **115 incompatible and 16 compatible
changes**, exit 1: the gate fails, as it must.

- **40 of the 115 name no `Unstable` identifier.** The generator merges an unstable definition into the stable-named Go type it
  replaces, so NES reached stable-named API: `AgentCapabilities.Nes`, `AgentCapabilities.PositionEncoding`,
  `ClientCapabilities.Nes`, `ClientCapabilities.PositionEncodings`, the ten `AgentMethodNes*` and `AgentMethodDocument*` constants,
  `NesCapabilities` and seventeen more `Nes*` types, `PositionEncodingKind` and `TextDocumentSyncKind` with their constants. A policy
  limited to `Unstable*` names would not cover them.
- **The 16 additions** include the same definitions twice, `SystemPrompt` and `UnstableSystemPrompt`, `SystemPromptMode` and
  `UnstableSystemPromptMode`, by the same merge: one reached through a stable request, one through the unstable `session/fork`.

**Go's rule and this repository's.** go.dev/ref/mod: "If an old package and a new package have the same import path, the new package
must be backwards compatible with the old package", and "a v1 version acts as a commitment to compatibility". go.dev/doc/modules/
version-numbers: a major version "Signals backward-incompatible public API changes"; only v0 "makes no stability or backward
compatibility guarantees". [0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md](../decisions/0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md)
holds this module to it after `v1.14.0`. Go has no exemption for experimental API inside a `v1` package; Go's own route is a separate
path, `v0` or a pre-release, or `x/exp`. So adopting 1.26.0 as it stands needs one of:

- a `/v2` module path, which every importer must change (below);
- a decided, published policy that exempts the unstable surface, which the 40 stable-named breaks make hard to state honestly;
- or keeping the removed declarations, deprecated, until a major, which the generator cannot do today: they are no longer in any
  schema it reads.

**The work in this repository** is small once the policy exists: 38 hand-written lines in 4 files name a removed identifier
(`acp_test.go` 23, `example/agent/main.go` 13, `union_wire_test.go` 1, `cmd/generate/internal/model/classes_test.go` 1). The
generator's own type check names the 27 root test files that would not compile until they change, and `example/agent` does not
build. `README.md:448`–`:449` names `nes` and `positionEncoding`. The generator needs no change: no `1.26.0` node carries
`deprecated`.

## Consumers

| Consumer | Requires | Files importing the SDK | Identifiers of the SDK it names | Names a removed identifier | Uses a system prompt over ACP |
| :--- | :--- | ---: | ---: | :--- | :--- |
| `acp-go-sdk2` itself (tests, examples) | — | — | — | 38 lines in 4 files | no |
| `magic-cli-remote` | `v1.15.0` | 47 | 180 | no: 0 lines | no |
| `<cli-b>` | `v1.16.3` | 76 | 212 | no: 0 lines | no |

- **Neither consumer would break** from the removal: neither names an `Unstable`, NES, document-event or position-encoding identifier.
  The breakage is to the module's promise, and to any importer this report does not know of.
- **Neither consumer would gain** from the addition today. `<cli-b>` builds its system prompt itself (`acpserver/sysprompt.go`) and
  takes none over ACP. `magic-cli-remote` has no system prompt handling, and launches `codex`, `grok`, `kilo` and `opencode`, not
  `claude-agent-acp`; whether those agents implement the unstable field was not checked.
- **Neither is on the latest release** (`v1.17.1`), so no change here reaches them before they upgrade. A `/v2` would make each change
  its import path in every one of those 123 files.

## How other SDKs took it

| SDK | What it did with 1.26.0 | How its unstable API is separated |
| :--- | :--- | :--- |
| Rust, `agent-client-protocol` (upstream) | crate `v1.12.0`, a minor, the same day | Cargo features: #2266 removed "the `unstable_nes` Cargo feature"; a user who never enabled it is not broken |
| TypeScript, `@agentclientprotocol/sdk` | `v1.9.0`, a minor (2026-10-16): "isolate NES behind an experimental entrypoint", then "update schemas to v1.26.0" | `v2.0.0`, a major three days later: "Default v1 and experimental v2 imports now expose only stable-feature schemas. Unstable v1 features require @agentclientprotocol/sdk/unstable" |
| Go, `coder/acp-go-sdk` | not adopted; latest `v0.13.5` (2026-06-02) tracks schema `0.13.5` | at `v0`, where Go allows breaking changes; it removed the never-stabilized `session/set_model` there |

The two SDKs that adopted at once had, or built, a boundary that keeps unstable API out of what a default user imports, and the
TypeScript one spent a major to build it. This SDK has no such boundary: its unstable definitions share the root package, and some
share stable-named types.

## Opinion

**No to early adoption.** For it: less dead code, a current pin, and an unstable feature one external agent implements. Against it:

- the stable protocol is unchanged, so nothing stable is gained;
- the system prompt is a Draft RFD that "stays unstable until the RFD reaches Preview", and no consumer uses it;
- the removal is 115 incompatible changes to a `v1` module, 40 of them to stable-named API, with no decided policy that allows it;
- a `/v2` spent on an abandoned draft feature costs every importer an import-path change for no gain to them.

**When it becomes yes,** any of:

- the record question 16 of 0019 calls for decides how unstable API is separated or removed under `v1`. The precedents above point at
  a boundary, such as an `unstable` package, which in Go is itself a breaking move and so belongs with a major;
- a later schema brings a stable change, or a consumer needs the system prompt, so that the update is worth its cost;
- ACP v2 (`schema-v2.0.0-alpha.9`, a pre-release, 2026-10-16) stabilises, and a `/v2` of this module can carry both that and the
  unstable boundary in one break.

**Meanwhile,** the SDK stays on `1.25.0`, whose stable surface `1.26.0` does not change. The NES API it carries is inert on the wire:
no agent will offer it. One step that is lawful within `v1` and would prepare users for the removal: mark the NES, document-event and
position-encoding API `Deprecated:` now, naming upstream's removal, so that a later major removes what users were warned about. The
generator cannot yet deprecate a node the schema does not mark; that would be a protocol-table rule of its own, for the record to
decide.
