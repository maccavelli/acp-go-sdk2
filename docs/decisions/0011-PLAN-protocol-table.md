---
status: in-progress
date: 2026-10-10
associated-madr: "0011-MADR-harden-code-generator-engine.md"
---

# Implement phase 3 of the generator hardening: the protocol table

Associated MADR: [0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md)

## Goal

Every piece of ACP behaviour the generator adds beyond the schema lives in one Go table, `ACP` in `cmd/generate/internal/model/acp.go`. The model reads
it and refuses a schema whose shape contradicts it; the emitters read only the model and name no wire method, no protocol type, and no interface the
table owns. Apart from one new constant, `ProtocolMethodCancelRequest`, the generated files do not change. This is phase 3 of the MADR, as refined by
its amendment "phase 3 facts and decisions".

## Scope

In scope:

* `cmd/generate/internal/model`: a new `protocol.go` (the table's types and how the model applies them) and `acp.go` (the table); `methods.go`,
  `model.go`, `fields.go`; their tests
* `cmd/generate/internal/emit`: `dispatch.go`, `dispatch_helpers.go`, `types.go`, `constants.go`; their tests
* `cmd/generate/main.go` and `main_test.go`, which pass the table to the model
* `constants_gen.go` (step 5 only) and `connection.go` (step 5 only, the `$/cancel_request` literals)
* `docs/architecture.md`, and `README.md` if it names what changes

Out of scope:

* moving the capability-gated agent methods off `Agent` (the amendment's decision: recorded, not acted on);
* the discriminator derived from the first `const` property (F12): it is a schema heuristic in the model, not protocol behaviour, and stays;
* phases 4 to 6: `x-deserialize-*`, integer formats, typed maps, deeper `Validate`, rendering in memory, type-checking the output, engine tests
  beyond the ones this plan adds;
* `RequestId` decoding any object as `Null` (found in phase 2), and `cancelRequestParams` in `connection.go`;
* the goldens, `encoding/json/v2`, a tag, a push, an agent commit on `main`.

## Grounding

Read on `128a920`; the probes ran in scratch copies, and nothing in the tree changed.

* **T1. Protocol literals left in the generator.** `internal/model`: `bindingOf` (`methods.go:225`, `session/load`); `notificationMethodName`
  (`methods.go:210`, `session_update` → `SessionUpdate`; `methods.go:212`, `session_cancel` → `Cancel`); `n.declare("AgentLoader", ...)`
  (`methods.go:143`); `interfaceName` (`methods.go:33`); `helperSkip` (`model.go:226`); `fixedNames` (`model.go:229`); the `ToolCallUpdate` case of
  `validateOf` (`model.go:631`). `internal/emit`: `wireSessionCancel` and the unused `nameSessionUpdate` (`constants.go:18`, `:19`); `session/cancel`
  (`dispatch.go:80`), `session/prompt` (`dispatch.go:91`, `:168`), `Cancel`, `CancelNotification`, and `SessionId` (`dispatch.go:19`, `:171`,
  `:173`); `AgentLoader` and `loader` (`dispatch_helpers.go:42`, `types.go:98`, `:99`); `ClientTerminal` (`dispatch_helpers.go:56`, `types.go:111`,
  `:119`).
* **T2. Which special cases change the output.** A probe of the committed model listed every method's Go name beside the default rule (the request
  name without `Request`, the notification name without `Notification`): the only difference is `session/update` (`SessionUpdate` against
  `Session`). `session/cancel` gives `Cancel` either way. `session/load` is the only method bound to an interface other than its side's required or
  experimental one.
* **T3. Dead code.** `BindClientTerminal` is never assigned, so the `ClientTerminal` interface is never emitted. `isExtensionVariant` (`fields.go:392`)
  matches no variant: `types_gen.go` has no `type X = json.RawMessage`. `nameSessionUpdate` has no reader.
* **T4. The prompt turn, as generated.** `agent_gen.go`: the `session/prompt` case derives `reqCtx` with `context.WithCancel`, cancels and replaces any
  previous one in `a.sessionCancels[string(p.SessionId)]`, calls `Prompt`, then deletes the entry; the `session/cancel` case cancels and deletes it
  before calling `Cancel`. `client_gen.go`: `Prompt`, on an error with `ctx.Err() != nil`, sends
  `c.Cancel(context.Background(), CancelNotification{SessionId: params.SessionId})`. `sessionCancels` and `mu` are fields of the hand-written
  `AgentSideConnection` (`agent.go:15`, `:16`); `ClientSideConnection` has neither. `PromptRequest.SessionId` and `CancelNotification.SessionId` are
  both the required property `sessionId`, of type `SessionId`, a `string`.
* **T5. The specification behind T4.** Prompt turn: "When the Agent receives this notification, it SHOULD stop all language model requests and all
  tool call invocations" and "MUST respond to the original session/prompt request with the cancelled stop reason"; "Clients MAY cancel an ongoing
  prompt turn at any time by sending a session/cancel notification".
* **T6. `$/cancel_request`.** `meta.json` and `meta.unstable.json` list it under `protocolMethods` (`cancel_request`). `model.Protocol` holds it with
  `CancelRequestNotification` and an empty `Const`; nothing reads `model.Protocol`. `connection.go` has the literal at lines 527, 689, 693, 699, 853,
  854, and 882, and its own `cancelRequestParams` (`connection.go:50`). `connection_cancel_test.go` spells the literal 6 times, in 5 tests that send or expect it on the wire.
  The ACP cancellation page: the receiver "MAY cancel the corresponding request activity" and "MUST send one of these responses for the original
  request", a result or `-32800`.
* **T7. Capability gates.** A script resolved each path from `AgentCapabilities` or `ClientCapabilities` through `$ref`, `allOf`, and nullable
  `anyOf`, in both schema files: `loadSession` (boolean), `sessionCapabilities.list`, `.delete`, `.resume`, `.close` (objects), `auth.logout` (object);
  `sessionCapabilities.fork` in the unstable file only; on the client side `fs.readTextFile`, `fs.writeTextFile`, and `terminal`. Each description
  names its method (`terminal`: "all `terminal/*` methods"; `auth.logout`: "the logout method"). `providers`, `nes`, and `elicitation` name none.
  The session-setup page: Clients "MUST NOT attempt to call" `session/load`, `session/resume`, or `session/close` unless advertised.
* **T8. Hand-written helpers.** `helpers.go` builds `ContentBlock` (`TextBlock`, `ImageBlock`, ...), `ToolCallContent` (`ToolContent`, ...), and
  `SessionUpdate` values; `helpers_gen.go` has no `New<Union><Variant>` for those three.
* **T9. `ToolCallUpdate.Validate`.** It checks `toolCallId` and is exported; no code in the SDK calls it. It came with the original generator.
* **T10. Who builds the model.** `main.go` (`run`) and `main_test.go` (`TestGeneratedFilesUpToDate`) build it from the committed schema. The model
  tests and `TestRunRefusesBeforeWriting` and `TestRunGeneratesRecasedUnion` build it from synthetic schemas whose meta files hold none of the methods
  T1 names.
* **T11. Coverage.** `go test -cover ./internal/model`: 95.0%.

## Design

### The table

`cmd/generate/internal/model/protocol.go` declares the table's types; `acp.go` holds its one value, `ACP`:

```go
type Protocol struct {
	Interfaces         []Interface     // optional interfaces the methods may bind to
	Methods            []MethodRule    // at most one per side and wire
	HandWrittenHelpers []string        // unions whose constructors helpers.go writes
	Validate           []ValidateRule  // non-RPC types that get a Validate of one required string
}
type Interface struct{ Binding Binding; Name, Var string; Doc []string }
type MethodRule struct {
	Side, Wire        string
	Binding           Binding // zero: the side's required or experimental interface
	GoName            string  // empty: the default rule
	PromptTurn        bool    // inbound: runs under a context the canceller cancels; outbound: a failed call with a done context sends the canceller
	CancelsPromptTurn bool    // inbound: cancels the session's prompt-turn context first
	SessionKey        string  // the property naming the session, for the two above
	Capability        string  // dotted path from <Side>Capabilities; recorded, not emitted
}
type ValidateRule struct{ Type, Prop string }
```

`ACP` holds: the interface `AgentLoader` (`Var` `loader`, today's doc comment); the rules `session/load` (`BindAgentOptional`, capability
`loadSession`), `session/prompt` (`PromptTurn`, `sessionId`), `session/cancel` (`CancelsPromptTurn`, `sessionId`), `session/list`, `session/delete`,
`session/resume`, `session/close`, `session/fork` (their `sessionCapabilities.*`), `logout` (`auth.logout`), `session/update` (`GoName`
`SessionUpdate`), `fs/read_text_file`, `fs/write_text_file` (`fs.*`), the five `terminal/*` (`terminal`); the helpers `ContentBlock`,
`SessionUpdate`, `ToolCallContent`; the validate rule `ToolCallUpdate.toolCallId`.

### How the model applies it

`model.Input` gains `Protocol *Protocol`. Nil applies no rule; `main.go` and `TestGeneratedFilesUpToDate` pass `&model.ACP`. Each rule applies only
where the schema carries its subject: a rule whose method the meta files lack, or a helper or validate type the schema lacks, does nothing, so the
synthetic schemas of T10 build as today. A subject that is present with a shape the rule cannot use is refused, with an error that starts
`protocol table:` and names the rule:

* two rules for one side and wire, a side other than `agent` or `client`, or a binding no `Interfaces` entry declares or that belongs to the other side;
* `PromptTurn` or `CancelsPromptTurn` on the client side (only `AgentSideConnection` holds `sessionCancels`, T4);
* `PromptTurn` on a notification or on a request without a result; `CancelsPromptTurn` on a request;
* a `SessionKey` that is not a required property of the method's type, or whose Go type is not a string;
* a `PromptTurn` method present without exactly one `CancelsPromptTurn` method present on its side, or the two with different session field names or
  Go types;
* a `GoName` that is not an exported identifier (the existing interface collision check runs after);
* a `Capability` path that does not resolve from `AgentCapabilities` or `ClientCapabilities`;
* a hand-written-helpers type that is not a union, or a validate rule whose property is not a required string of an object type.

`Method` gains `PromptTurn`, `CancelsPromptTurn`, `SessionField` (the Go field of `SessionKey`), `Canceller` (the `CancelsPromptTurn` method, on a
`PromptTurn` method), and `Capability`. `Model` gains `Interfaces`: `Agent`, `AgentExperimental`, `Client`, and `ClientExperimental`, with their doc
comments, which are the SDK's shape and stay in `model.go`, plus each table interface that a present method binds to, ordered by `Binding`.
`BindAgentLoader` becomes `BindAgentOptional`; `BindClientTerminal` is deleted; `ValidateToolCall` becomes `ValidateListed`. A protocol method's
`Const` is `ProtocolMethod` and its exported meta key, declared like the others.

### What the emitters read

`dispatch.go` selects the prompt-turn case by `mi.PromptTurn`, the cancel by `mi.CancelsPromptTurn`, the outbound cancel by `mi.PromptTurn` and
`mi.Canceller` (its `GoName` and `Notif`), and the session key by `mi.SessionField`. `dispatch_helpers.go` asserts an optional binding to the interface
and variable `m.Interfaces` names. `types.go` emits the interfaces in `m.Interfaces` order with their docs. `constants.go` emits a `Protocol method names`
block when `m.Protocol` is not empty. The emitters name no wire method and no protocol type.

## Implementation Steps

1. Work on `main`. Each step writes its tests first and records their failure on the current tree; changes the generator; regenerates
   (`cd cmd/generate && env -u GOPATH -u GOMODCACHE go run .`); compares every generated file with the previous commit's by script; shows each new
   check failing on a planted breakage in a scratch copy; runs `make pre-add-check`, `make check-records`, and `make markdownlint`; records the output in
   this plan's execution record; and stops for the owner's commit. The goldens must not change.
2. **Dead code.** Test: a union with a title-only string variant titled `FooExt` and described "An extension payload." gives that variant the kind
   `InlineNamed` and type `string`. Delete `BindClientTerminal`, its `interfaceName` entry, and the `ClientTerminal` branches of `emit/types.go` and
   `emit/dispatch_helpers.go`; `isExtensionVariant`, `InlineRaw`, and the `InlineRaw` case of `emitInlineVariant`; `nameSessionUpdate`. Expected:
   every generated file byte-identical.
3. **The table in the model.** Tests: `TestProtocolTableAppliesToCommittedSchema` (every rule's subject is present; `session/load` is bound to
   `AgentLoader`; `session/update` is `SessionUpdate`; `session/prompt` has `PromptTurn`, `SessionField` `SessionId`, and `Canceller` `session/cancel`;
   the three helper unions have no `Helpers`; `ToolCallUpdate` has `ValidateListed`; each of the 14 capability paths resolves, and its leaf's
   description names the method, `terminal/*`, or "the logout method"); `TestProtocolTableRefuses`, one synthetic case per refusal of the design;
   `TestProtocolTableSkipsAbsentSubjects` (the full `ACP` table on a schema with none of its subjects builds, and no rule applies); and, with a nil
   table, `session/load` on `Agent` and `session/update` named `Session`. `TestBuildMethods` passes a table where it expects the `AgentLoader` binding
   and the name `SessionUpdate`; its assertions are kept. Add `protocol.go` and `acp.go`; remove the literals of T1 from `internal/model`; pass `&model.ACP`
   in `main.go` and `TestGeneratedFilesUpToDate`. The emitters change only for the renamed constants. Expected: every generated file byte-identical.
4. **The emitters read the model.** Test: an emit test builds a model from a synthetic schema and a synthetic table that names everything differently
   (prompt method `chat/send` with key `conversationId`, canceller `chat/stop`, optional interface `AgentStore` with variable `store`), renders both
   dispatch files and the interfaces, and requires those names and none of `session/prompt`, `SessionId`, `CancelNotification`, `AgentLoader`. Change
   `dispatch.go`, `dispatch_helpers.go`, `types.go`, and `constants.go` to the design. A script checks that no non-test file of `internal/emit` contains a
   wire name of the meta files or a name `ACP` holds. Expected: every generated file byte-identical.
5. **The protocol method constant.** Tests: `TestBuildMethods` expects the protocol method's `Const` `ProtocolMethodCancelRequest`; an emit test expects
   the `Protocol method names` block. Set the constant in the model, emit the block, and replace the seven literals of T6 in `connection.go` with
   `ProtocolMethodCancelRequest` (in log messages by concatenation); its comment at line 525 keeps the wire name. Expected: `constants_gen.go` gains
   the block and nothing else changes; `connection_cancel_test.go`, which spells the wire name itself, passes unchanged.
6. **Docs.** `docs/architecture.md` names the table, `Input.Protocol`, and what the table holds; `README.md` changes only if it names something that
   changed. The name check of phase 2 runs over both files.
7. **Review and close.** Run the verification below, record each result and the exported API change for the release notes, and set this plan to
   `complete`.

## Verification

* After steps 2, 3, and 4 every generated file is byte-identical to `128a920`'s; after step 5 only `constants_gen.go` differs, by the new block.
* No non-test source file of `cmd/generate` outside `acp.go` contains a wire name of the meta files or a type, interface, or Go name `ACP` holds,
  except the default interface names in `model.go`; a script lists every match.
* `TestProtocolTableAppliesToCommittedSchema` passes; each refusal case and each new test failed first; each rule of the table, removed in a scratch copy
  one at a time, fails a test.
* `go test -race ./...` passes at the root and in `cmd/generate`; `go build ./example/...` passes; `TestGeneratedFilesUpToDate` passes.
* `go test -cover ./internal/model` stays at or above 85%; `./internal/emit` is reported.
* The exported API of the root package differs from `128a920`'s by the constant `ProtocolMethodCancelRequest` alone.
* `make pre-add-check`, `make check-records`, and `make markdownlint` pass.

## Rollout and Rollback

Commits land on `main`, made by the owner, one per step. No tag; a push waits for the owner's ask. The next minor version's release notes list the
constant `ProtocolMethodCancelRequest`. No wire behaviour changes.

Rollback is `git revert` of the step commits, newest first; each step leaves the generator and the generated files in step.

## Execution record

### Step 2, dead code, 2026-10-10

The owner committed this plan as `0566b33` and said "proceed". Step 2 ran on `main` from that clean tree. The agent did not commit.

`TestTitleOnlyVariantTakesSchemaType` in `cmd/generate/internal/model/unions_test.go` builds a union whose first variant is titled `FooExt`, described
"An extension payload.", and typed `string`, and requires that variant to be `InlineNamed` with type `string`. Run before the change:

```text
unions_test.go:124: variant FooExt: &{Kind:1 Doc:[An extension payload.] Fields:[] Type: KeepExtra:false}, want InlineNamed string
```

Kind 1 was `InlineRaw`.

Deleted: `BindClientTerminal` and its `interfaceName` entry (`model/methods.go`); the `ClientTerminal` branches of `emitRPCInterfaces`
(`emit/types.go`) and `clientAssert` (`emit/dispatch_helpers.go`); `isExtensionVariant` and `InlineRaw` (`model/fields.go`) and the `InlineRaw` case of
`emitInlineVariant`; `nameSessionUpdate` (`emit/constants.go`). A title-only variant is now `InlineNamed` when its schema has a type and `InlineEmpty`
otherwise. `TestInlineVariants` expected `InlineRaw` for its untyped variant titled `ExtPayload`; by the amendment's decision that variant is now
`InlineEmpty`, like its untyped neighbour `bare`, and its expected kind says so. `grep -rn` over `cmd/generate` finds none of the four deleted names.

Regenerated: each of the five generated files compares equal, by `cmp`, to its copy from `0566b33`.

Each rule was seen to fail, with one planted breakage in `model/fields.go` per fresh scratch copy, then `go test ./...` in `cmd/generate`:

```text
a name heuristic comes back: exit=1 failing=['TestTitleOnlyVariantTakesSchemaType']
title-only variants lose their type: exit=1 failing=['TestGeneratedFilesUpToDate', 'TestInlineVariants', 'TestTitleOnlyVariantTakesSchemaType']
```

After the change every test in the root and in `cmd/generate` passes; `go build ./example/...` passes. Coverage: `internal/model` 94.9%,
`internal/emit` 37.4%.

Gates: `make pre-add-check` with the seven changed files named: `go-precheck: 7 file(s) clean (gofmt, golangci-lint, go vet, go test,
govulncheck).` Without names: `go-precheck: 63 file(s) clean (...)`. `make check-records` and `make markdownlint` exit 0.

Commit point: the agent stopped here. The owner commits step 2 on `main`.
