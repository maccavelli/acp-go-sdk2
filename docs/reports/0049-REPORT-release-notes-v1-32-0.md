# Release notes for v1.32.0, against v1.31.2

Observed 2026-11-04 on `a85a653`, for [0049-PLAN-surface-fixes.md](../decisions/0049-PLAN-surface-fixes.md), phase 6, step 4.
This report drafts the text of the GitHub release for `v1.32.0`, as `RELEASING.md` describes it. It decides nothing. It is
committed before the tag, so the tagged tree holds it. The commit that adds it changes records and nothing else, so the API it
tags is `a85a653`'s.

Each statement names its source:

- a record;
- a commit;
- or `make api`, run on `a85a653` with nothing uncommitted but this report.

Summary:

- 26 compatible additions and nothing incompatible. `make api` against `v1.31.2`: "0 incompatible, 26 compatible", so the
  release is a minor, as `RELEASING.md` says and the surface PLAN planned. The additions are `ErrUnadvertisedAuthMethod` and
  25 `Validate` methods (the PLAN's deviation 3).
- Nine fixes to the SDK surface, S1 to S9 of
  [0049-MADR-remediate-the-2026-11-01-debugging-pass.md](../decisions/0049-MADR-remediate-the-2026-11-01-debugging-pass.md).
- What a consumer may meet, in the PLAN's order:
  - values the SDK sent before and now refuses: an empty or unlisted closed enumeration, such as `PromptResponse{}`'s
    `stopReason`; an unadvertised `methodId`; a `boolean` `session/set_config_option`, an unstable method or variant, or a
    prompt block of a type the schema does not list, without its capability;
  - values read otherwise: a key-less variant's own name, an unknown `type` that fits `ValueId`, and an unknown closed value
    in a lenient field, read as absent;
  - an empty `content` or `locations` on a `tool_call_update` is now sent, and clears the collection;
  - the timelines copy deeply what they keep and what they return.
- The `go` lines, the requirements and the schema are unchanged.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.32.0

The ACP schema is still `1.25.0` (`git diff v1.31.2 -- schema/` is empty). `make api` reports 26 compatible additions against
`v1.31.2` and nothing incompatible. This release fixes the nine defects of the SDK surface that the 2026-11-01 debugging pass
found ([0049-MADR-remediate-the-2026-11-01-debugging-pass.md](../decisions/0049-MADR-remediate-the-2026-11-01-debugging-pass.md),
S1 to S9). Each fix has a test that fails without it.

Most of these fixes refuse something `v1.31.2` sent. Read "Changed: what the SDK refuses to send" before upgrading.

#### Changed: what the SDK refuses to send

Each refusal below sends nothing, and the typed method returns the error. `Validate` reports a closed enumeration's refusal,
and `Negotiation.AllowsValue` a gate's. What this SDK does when a peer sends the same value is in each item.

- **Closed enumerations (S6).** A field of a closed enumeration must hold one of its type's constants. The closed
  enumerations are `StopReason`, `PermissionOptionKind`, `PlanEntryPriority`, `PlanEntryStatus`, `ToolKind`, `ToolCallStatus`,
  `Role`, `StringFormat` and `ElicitationSchemaType`, and the unstable `NesDiagnosticSeverity`, `NesRejectReason`,
  `NesTriggerKind`, `PositionEncodingKind` and `TextDocumentSyncKind`. A required field is always checked, so `""` fails. An
  optional field is checked when it is set.
  - The failure is a `*acp.ValidationError` with `acp.RuleEnum`, quoting the value:
    `PromptResponse: stopReason: "" is not a StopReason`.
  - **An agent whose `Prompt` returns `PromptResponse{}` now answers `-32603`.** Set `StopReason`, for example
    `acp.StopReasonEndTurn`.
  - `Validate` checks the same, and 25 types that hold a closed enumeration and had nothing to check gain a `Validate`
    method. Among them are `PermissionOption`, `PlanEntry`, `Annotations`, the content types, and `AgentCapabilities` and
    `ClientCapabilities`.
  - What is received is not checked: a peer's unknown `stopReason` is returned as it arrived.
  - `acptest`'s `ScriptAgent` answers a `Turn` without a `StopReason` with `end_turn`, where it sent `""`.
  - (`cedb98d`.)
- **An unadvertised auth method (S8).** `Authenticate` with a `methodId` that names no auth method of the agent's
  `initialize` answer returns an error wrapping the new `acp.ErrUnadvertisedAuthMethod`. The schema says the id "Must be one
  of the methods advertised in the initialize response". Received, such a request is answered `-32602`.
  - An entry of a type this SDK does not know, kept in `AuthMethod.Unknown`, counts by its `"id"`.
  - A `terminal` method's id still returns `acp.ErrTerminalAuthMethod`, checked first.
  - (`6ad8759`.)
- **A prompt block of an unlisted type (S9).** A `session/prompt` block of a type the schema does not list, such as one kept
  in `ContentBlock.Unknown`, is refused with every capability advertised. The schema says "Other variants must be explicitly
  opted in to", and no capability opts one in.
  - The error is `*acp.CapabilityError{Method: "session/prompt", Capability: "promptCapabilities.<type>", Field:
    "prompt[i]", Variant: "<type>"}`.
  - Received, such a request is answered `-32602`. Text and resource-link blocks, the baseline, still need nothing.
  - (`6ad8759`.)
- **A boolean `session/set_config_option` (S5).** A client sends one of `type: "boolean"` only when it advertised its own
  `session.configOptions.boolean`, as the schema says. Received without it, it is answered `-32602`. A value-id request needs
  nothing. (`6ad8759`.)
- **The unstable capabilities (S3).** Each needs the capability the schema names for it:
  - `providers/list`, `providers/set` and `providers/disable` need the agent's `providers`;
  - `nes/start`, `nes/suggest`, `nes/accept`, `nes/reject` and `nes/close` need the agent's `nes`;
  - each `document/<event>` notification, `didOpen`, `didChange`, `didClose`, `didSave` and `didFocus`, needs the agent's
    `nes.events.document.<event>`;
  - the `plan_update` and `plan_removed` session updates need the client's `plan`;
  - an `nes/suggest` result's `jump`, `rename` and `searchAndReplace` suggestions need the client's `nes.jump`, `nes.rename`
    and `nes.searchAndReplace`. An `edit` needs none.

  A method or update sent without its capability returns a `*acp.CapabilityError`. Received, a request is answered `-32601`,
  and a notification, a `document/<event>` or a plan update, is dropped and logged. An agent whose `nes/suggest` result holds an unadvertised suggestion answers `-32603`, and a client
  that receives one returns the `*acp.CapabilityError`. (`6ad8759`.)

#### Changed: what the SDK reads

- **A key-less variant's own name (S7).** In a union without a catch-all, a discriminator equal to the name of the variant
  the schema means when the discriminator is absent now decodes into that variant. `McpServer`'s `"type":"stdio"` decodes into
  `Stdio`, and `AuthMethod`'s `"type":"agent"` decodes into `Agent`. Both were kept in `Unknown` before. A payload that names
  the variant and does not fit it is an error naming the variant. Any other unknown type still goes to `Unknown`. (`22d508e`.)
- **`SetSessionConfigOptionRequest`'s unknown `type` (S1).** An unknown `type` with a string `value` decodes as `ValueId`, as
  the schema's "Unknown `type` values with string payloads also gracefully deserialize into this variant" says. An agent's
  handler receives the session, the config id and the value. Any other payload stays in `Unknown`. (`22d508e`.)
- **An unknown closed value in a lenient field (S6).** A field the schema marks `x-deserialize-default-on-error` reads a
  closed enumeration's unknown value as absent. A `tool_call`'s unknown `kind` or `status` reads as `""`, a
  `tool_call_update`'s as nil, and `ElicitationSchema.type` takes its default, `object`. An array marked
  `x-deserialize-skip-invalid-items` drops an unknown item, such as an `audience` role. A required field that is not lenient,
  such as `stopReason`, keeps what it received. (`cedb98d`.)

#### Fixed: an empty `content` or `locations` clears the collection (S2)

The schema says a `tool_call_update`'s `content` and `locations` replace the collection, so an empty one clears it.
`ToolCallUpdate` omitted an empty slice, and a client could not be told to clear. Now:

- on a `ToolCallUpdate` or `SessionToolCallUpdate`, a nil slice is not sent and an empty one is sent as `[]`;
- `acp.WithUpdateContent` and `acp.WithUpdateLocations` send `[]` when given nil or empty, so passing either always replaces
  the collection.

Decoding is unchanged. (`e0dd5d2`.)

#### Fixed: the timelines share nothing with their callers (S4)

`ToolCallTimeline` and `CompactionTimeline` copied only the top of an entity. A caller that changed a content block, a diff,
a location's line or a nested `_meta` value, through the copy `Get` or `Apply` returned, changed the timeline. Both now keep
deep copies of what they take from an update and return deep copies. Changing an update after `Apply`, or an entity they
returned, never changes what they hold. (`6f17464`.)

#### For maintainers

- **Tests.** Each finding has a test that fails with its fix reverted in a copy of the tree. The surface PLAN's phase 6
  lists them by finding, and phases 1 to 5 record each planted break and its failure.
- **The generator** has three new protocol-table rules: `KeyLessFallbacks`, `EmptyClears`, and the gate kind
  `GateAdvertisedAuthMethod`. `GateUndeclaredVariant` now reads a union's raw `Unknown` through a property. Each rule quotes
  the schema sentence it applies, and `Build` refuses a rule whose sentence the schema does not state. The generator writes
  `unknownKind` beside `foldKeys`, and `helpers.go` loses its copy (deviation 4).
- **Validation.** The generated `validate` takes a `validation`: sending checks closed enumerations, and receiving does not.
  [0012-MADR-schema-fidelity-framework.md](../decisions/0012-MADR-schema-fidelity-framework.md)'s amendment of 2026-11-03
  records treatments 5, 6 and 7.
- **A deep copy,** `deepCopy`, unexported, in `deepcopy.go`.

## Sources not in the release text

- **Commits since `v1.31.2`** (`git log v1.31.2..a85a653`), eight:
  - `e98987a` records the runtime fixes as released in `v1.31.2`;
  - `df1eb2c` records the MADR's answers and designs and adds the surface PLAN;
  - `22d508e`, phase 1 (S1, S7): the generator's union decoder and model, `types_gen.go`, `union_keyless_test.go`, two engine
    cases, `README.md`, `docs/architecture.md`;
  - `e0dd5d2`, phase 2 (S2): the `EmptyClears` rule, `types_gen.go`, `helpers.go`, `helpers_test.go`, `README.md`,
    `docs/architecture.md`;
  - `cedb98d`, phase 3 (S6): the closed enumerations, `types_gen.go`, `connection.go`, `errors.go`, `acptest/script.go`,
    `enum_closed_test.go`, the 0012 amendment, `README.md`, `docs/architecture.md`;
  - `6ad8759`, phase 4 (S8, S9, S5, S3): the gates, `types_gen.go`, `client_gen.go`, `errors.go`, `helpers.go`,
    `gates_surface_test.go`, `README.md`, `docs/architecture.md`;
  - `6f17464`, phase 5 (S4): `deepcopy.go`, `tooltimeline.go`, `compaction.go`, `deepcopy_test.go`, `README.md`,
    `docs/architecture.md`;
  - `a85a653` records phase 6's gates and `make api`.
- **Go files changed.** `git diff --name-status v1.31.2 a85a653 -- '*.go'` lists 42:
  - the root's non-test files: `client_gen.go`, `compaction.go`, `connection.go`, `deepcopy.go` (new), `errors.go`,
    `helpers.go`, `tooltimeline.go`, `types_gen.go`;
  - `acptest/script.go` and its test;
  - the generator: 19 files of `cmd/generate`, 11 of them code and 8 tests, two of the tests new, `closed_test.go` and
    `keyless_test.go`;
  - the root's tests: 13, five new, `deepcopy_test.go`, `enum_closed_test.go`, `gates_surface_test.go`, `helpers_test.go` and
    `union_keyless_test.go`, and eight changed.
- **Unchanged** (`git diff --stat v1.31.2 a85a653`, empty for these):
  - `go.mod`, `go.sum` and `cmd/generate/go.mod`;
  - the schema, `.github/`, `RELEASING.md`, `tools/consumer.py`;
  - `agentproc/`, `clienthost/`, `remote/`, `internal/`, `unstable/`.
- **`make api` on `a85a653`,** this report its only uncommitted file: exit 0, "against v1.31.2: 0 incompatible, 26
  compatible"; "reported packages unstable, remote: 0 incompatible, not gated". The PLAN's phase 6, step 2, had the same on
  `6f17464`.
- **The gate set** on `6f17464`'s tree, the PLAN's phase 6 step 1: 18 of 18, with the Linux and Windows hosts.
- **CI on `a85a653`:** run `41836300472`, "completed success", six jobs (`validate` on Ubuntu, Windows and macOS at Go
  `1.27.3` and `1.26.10`); the Docs workflow's run `41836300433`, "completed success".
- **The consumer.** `tools/consumer.py` is unchanged: the PLAN's step 3.4 found its program sends only listed values.
  `consumer.py v1.32.0` runs after the tag.
