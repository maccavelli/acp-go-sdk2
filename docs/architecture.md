# Architecture

How this repository is put together, as it is now. This file carries no
history and no rationale — when you want to know *why* something is this
way, the decision records under [decisions/](decisions/) hold the argument,
and [README.md](README.md) points at the ones people ask for most.

## What it is

This repository is the Go module `github.com/maccavelli/acp-go-sdk2`, package
`acp`, at the repository root, package `agentproc` in `agentproc/`, which
starts an agent as a child process and stops it, package `unstable` in
`unstable/`, the API the unstable schema adds beyond the frozen manifest (both
below), and package `acptest` in `acptest/`, which stands up a connection pair
for a test (0039-MADR-five-adopter-additions-on-schema-1-25-0.md, item 5):

- `Connect` joins an `acp.Agent` and an `acp.Client` over two `io.Pipe`s
  through the public constructors, and the test's cleanup closes and waits for
  both, failing the test on an end that is not a close.
- `Recorder` keeps each newline-ended frame, re-encoded by `encoding/json` v1,
  and forwards `Close` to what it wraps.
- `ScriptAgent` embeds `AgentRefusals` and plays scripted turns.
- `MemoryClient` embeds `ClientRefusals` and serves files and permission outcomes from memory.
- `Abs` makes a fixture path absolute on the host.

Its tests run under a goroutine-leak guard of their own, a copy of the root's,
and `make api` gates it with the root and `agentproc`.

- Hand-written: `connection.go` (JSON-RPC over a reader and a writer, and
  `ChainRewrites`), `errors.go` (the error types and constructors, and
  `IsMethodNotFound`, `IsAuthRequired`, `IsCancelled` and `Reason`),
  `extensions.go` (`_`-prefixed methods), `helpers.go` (the constructors of the
  three unions the protocol table names, `SessionUpdate.Kind`, `Ptr`, `Roots`,
  and the helpers of
  [0038-MADR-consumer-helpers-kind-and-agent-refusals.md](decisions/0038-MADR-consumer-helpers-kind-and-agent-refusals.md):
  `AllSessions`, `PickPermissionOption`, `MetaObject`, `DecodeMeta`,
  `ResolveStdioCommands`, `OfferedAuthMethods`, `ConfigOptionByCategory` and
  the methods beside them), `refusals.go` (`ClientRefusals` and
  `AgentRefusals`, which answer method-not-found for the `Client` and `Agent`
  methods a side may leave unadvertised), and `doc.go`. None of them holds
  protocol state. A helper that takes a side connection is a package-level
  function: every method of a side connection that takes a context is a
  protocol method, as `TestSideConnectionMethodsAreProtocolMethods` requires.
- Generated from `schema/`: `constants_gen.go`, `types_gen.go`, `agent_gen.go`
  and `client_gen.go` (the two connection types, `AgentSideConnection` and
  `ClientSideConnection`, with the state the protocol table's rules need:
  the agent's prompt turns, the client's pending permission requests, and
  on both the record of `initialize`; their dispatch, their outbound
  methods, and the gates each passes to `connection.go`. Each constructor
  assigns every field, the `Connection` from `buildConnection` included, and
  hands the agent or client its connection through `AgentConnAware` or
  `ClientConnAware`, declared in `connection.go`, when it implements them,
  before it calls `start`, so a message already waiting never reaches the
  dispatch with a field unset), and `helpers_gen.go`, which
  holds a constructor for every variant of every other union the schema
  documents, and the test file `schema_gen_test.go`, which lists every
  generated type, union, and method for the conformance test. They are never
  edited by hand.

`cmd/generate` is its own Go module with no requirements. `internal/load` reads
the schema files, `internal/model` resolves them into types, unions, and
protocol methods, `internal/emit` builds the files as `go/ast` and prints them
through `go/format` into memory, `internal/output` type-checks and writes them,
and `internal/util` holds naming helpers. `make version` downloads the schema
for the stamp in `schema/version` and runs the generator.

Every line of a generated comment, a schema description's included, is
written as a line comment, so no description can become code or a directive.

Every exported generated declaration has a doc comment
(0019-PLAN-examples-docs-tests-tooling.md, I12):

- a type, field or interface method has its schema description;
- `Agent` and `Client` say which side implements them and what serves them;
- an enum constant names its value, then gives the value's description;
- a codec or `Validate` names where its type's schema sits: a definition, by
  its own key; a union's variant; or a property's inline object;
- a typed outbound method says what it sends and to which side, then repeats
  the interface method's doc.

`TestExportedDeclarationsDocumented` parses the package's non-test files and
fails on each exported declaration without a doc comment.

The generator writes nothing until every file has rendered and the five files
type-check with `go/types` together with the hand-written files of the output
directory, at the language version of its `go.mod`: first the package as
built, the hand-written files the go command would build there with the
generated files that are not tests, then the package as tested, which adds
the hand-written in-package test files and `schema_gen_test.go`, so a name
both declare is refused as `go vet` would refuse it. Output that does not
type-check is refused with the first 10 errors, those in generated files
first. The output directory must exist; a missing one is refused, since the
generated code needs the hand-written runtime beside it. A file whose bytes
have not changed is left alone. Every other file is
staged in the output directory as `.<name>.*.tmp`, synced, and set to mode
`0644`; once all are staged, each is renamed into place.

The protocol behaviour the schema does not express lives in one table, `ACP`
in `cmd/generate/internal/model/acp.go`, which `main.go` passes to the model as
`Input.Protocol`. It holds:

- the optional `AgentLoader` interface, for `session/load`;
- the prompt turn: `AgentSideConnection` runs `session/prompt` under a
  per-turn context. `sessionCancels` holds each session's running turns, and
  `storeTurn` stores a new one as the connection's `WithPromptOverlapPolicy`
  has it: supersede, the default, cancels the stored turns with a cause that
  wraps `ErrPromptCancelled`; refuse answers the prompt `-32603` while a turn
  is stored, without calling the handler; concurrent adds it. An inbound
  `session/cancel` cancels the session's stored turns through `cancelTurns`,
  with `ErrPromptCancelled` as their cause. Every received request and
  notification has a receipt number from `connection.go`, and a cancel
  applies only to a turn received before it. Every cancel is also kept in
  `lateCancels` for its session, for a prompt received before it whose turn
  is not stored yet; `keepCancel` in `connection.go` first removes each kept
  receipt that no request in flight is older than, from the receipts it
  keeps of the requests in flight, and, under supersede, a prompt newer than
  the kept cancel removes it. `session/close` (the table's `ClosesSession`)
  runs the same cancel before its handler. When the handler returns, the case
  takes the lock, removes its own turn (`removeTurn`) and reads its cause
  once: a cancelled turn is answered `cancelled`, else the handler's answer
  stands.
  `ClientSideConnection.Prompt` sends `session/cancel` when its context ends
  with an error;
- the permission requests: the client's `session/request_permission` case
  registers each pending request by session in `pending`, and its
  `session/cancel`, and a `session/close` the agent answers with success,
  cancel them, so `answerUnlessCancelled` in
  `connection.go` answers each `cancelled` at once;
- `initialize`: its version and capabilities keys. Both sides keep the
  request and the answer as a `Negotiation` in an
  `atomic.Pointer[Negotiation]`, the JSON form of what was sent and received
  (`jsonCopy`), and `Negotiated` returns a copy of it; it is stored by the
  agent before its answer is
  written and by the client as the answer arrives, by the commit the
  generated `Initialize` gives `sendRequestCommit`, a zero version
  filled and another refused;
- the name `SessionUpdate` for the client's `session/update` handler;
- the unions whose constructors `helpers.go` writes;
- the capability that gates each of 15 methods, resolved to its Go fields
  (`CapabilitySteps`);
- `CapabilityRules`, the 19 properties and variants a capability gates,
  each with the specification sentence it follows: among them the rule that an
  `authenticate` must not name a terminal auth method, and the rule that an
  elicitation's catch-all variant, a mode the schema does not list, is set
  only under a capability no client can advertise, so it is always refused,
  naming `elicitation.` and the mode (`GateUndeclaredVariant`). `model/gates.go`
  resolves each on its definition and on each copy of it the merge made from
  the unstable schema under another name, and gives a `gates` method to every type
  that holds one, directly or through fields, members or variants; with no
  `initialize` rule, none applies;
- `AbsolutePaths`, the 17 properties the protocol requires to be absolute
  paths, each with the specification sentence it follows. A rule reaches every
  type that holds its definition's property, an inline variant built through
  `allOf` included, and `Validate` checks it;
- `PatchFields`, the 12 properties with patch semantics (omitted keeps the
  stored value, `null` clears it, a value replaces it), each with the
  specification sentence it follows. A rule reaches every type built from its
  definition, as `AbsolutePaths` does, and the model refuses one whose property
  is required, has a default, or lacks `x-deserialize-default-on-error`. The
  field's type becomes a `RefPatch` of its type without the pointer, emitted as
  `Patch[T]` and tagged `omitzero`;
- `NullRetained`, the optional raw properties whose explicit `null` the schema
  keeps apart from omission, each with the schema sentence it follows: the
  unstable `McpError.data`. A field built from one keeps `null` when decoded;
- `KeyOrders`, the typed maps whose key order the protocol gives meaning, each
  with the field that holds the order and the reason: `ElicitationSchema`'s
  `properties`, field `PropertyOrder`. The generated `UnmarshalJSON` reads the
  map's keys in order through `keyOrder`; `MarshalJSON` writes them in that
  order through `orderedObject` when an order is set, and as before when none
  is. The model refuses a rule on a property that is not a typed map of a
  struct, on a struct with flattened members or kept extra keys, or whose
  field the struct already has.

The model applies a rule only where the schema carries its subject, and refuses
a subject whose shape the rule cannot use with an error that starts
`protocol table:`. The emitters take every protocol name from the model.
`constants_gen.go` has a constant for each method of the meta files, among them
`ProtocolMethodCancelRequest`, which `connection.go` uses.

The SDK uses `encoding/json` v1 only. The generated `MarshalJSON` methods send a
union's `Unknown` payload back when no variant is set, fail on a union with
neither, naming it, and send a nil required array or map as `[]` or `{}`. A
union's generated `UnmarshalJSON` leaves the union unchanged on `null`
(`RequestId` sets its `Null` variant) and otherwise clears it first. A union
with a discriminator fails on one that is present and not a string, naming the
union and the key; decodes the variant a known value names; sends an unknown
value to the schema's catch-all variant (`Other`, which keeps undeclared keys
in `Extra`) or keeps it in `Unknown`; and decodes a payload without the
discriminator into the schema's key-less variant, else into the one variant
whose own required keys it carries (not those the union requires of every
variant), else into `Unknown`, or fails in a union with a catch-all. A union
without a discriminator tries required keys, array items, and then each
variant in turn. A variant that is a `$ref` takes its keys, discriminator
value, and shape from the definition it names. Decode errors name the union
and, for a variant, the discriminator value or the variant. A struct with a
required key, a default, a lenient field, flattened members, or an `Extra`
gets a generated `UnmarshalJSON`. A struct gets a generated
`MarshalJSON` only when it does more than encode its fields: a nilable array
or object default it writes in, a nil required array or map, an `Extra`, or
flattened members. A catch-all has an `Extra`, and so does an object whose
`additionalProperties` is `true` (`Type.KeepExtra`): it keeps the keys it does
not declare, after folding, and writes them back, a declared field winning.
The model refuses an `additionalProperties` the struct would not keep: a typed
one on an object with properties, one beside flattened members, on a union
with keys of its own, on an inline object property, or on an inline variant
other than the catch-all, and a property that would take the field `Extra`.

Generated decoders read a key as `encoding/json` reads a struct field's name:
under Unicode case folding, the last matching key winning (`foldKeys` in
`types_gen.go`), and the model refuses a schema in which two keys one decoder
reads differ only in case. An object with properties of its own and an `anyOf`
or `oneOf` of references is a struct with one pointer field per member, whose
keys share its JSON object: decoding takes the first member whose required
keys are present, and `Validate` requires exactly one. An `anyOf` or `oneOf`
whose members are all integers, at least one a constant, is a named integer
type with a constant per value, named by the member's title; any integer
decodes. A definition the model gives no Go type is an alias of
`json.RawMessage`.

A definition with properties and no `type` is an object, as a variant with
properties is: it gets its decoder, `Validate` and gates, and refuses a value
that is not an object, which plain JSON Schema would allow. The model refuses,
naming the definition, the shapes no Go type holds as the schema states them
([0019-PLAN-generator-hardening.md](decisions/0019-PLAN-generator-hardening.md)):
an `enum` with a value that is not a string; a type list of two types other
than `null`; a named array definition; a union variant with no title, `$ref`,
properties or null type; a `const` on a union variant that is not an object,
which no generated code would check (a `oneOf` of string consts, or of titled
integer consts, is an enum kind); a discriminator const that is not a string;
a property key a json struct tag cannot carry; a field, union wrapper field or
flattened member named `MarshalJSON`, `UnmarshalJSON` or `Validate`; a
`required` key no property declares; and a definition linked by `x-method` and
`x-side` to a method no meta file lists. `Roots`, written by hand on five
request types, is left to the type checker: a property named `roots` there
does not type-check.

Every keyword the schema loader accepts has one treatment, listed in
`cmd/generate/internal/model/keywords.go`; the loader refuses any other
keyword, and one written in another letter case, which `encoding/json` would
match, and the model refuses an integer or number `format` without a Go type.
`deprecated` is an annotation: the loader refuses it when it is `null`, and the
model refuses it on a node no Go declaration is built from, anything but a
definition, a property of an object definition, or a direct `anyOf` or `oneOf`
member of a definition, and on an enum's member without a `const`. Each Go
declaration built from a marked node gets a last paragraph, "Deprecated: the
ACP schema marks <place> deprecated.", then the protocol table's note naming
the replacement when it has one: a definition's type, and for a request or
notification its interface method and its side connection's; a property's
field and nested type; a variant's field, inline type and helper, or its
constant; a flattened member's field. A note on a node the schema does not
mark is refused.
A property's `default` is checked against its schema, through references,
union members, array items and object properties: its JSON kind, an integer's
range for its format, its `enum` and its `const`, so the generated decoding of
a default, which ignores the error, cannot fail.
A number takes the Go type of its `format`; a typed `additionalProperties`
gives a typed map; a value the model gives no Go type is `json.RawMessage`; and
an optional struct value carries no `omitempty`. A generated `UnmarshalJSON`
requires the required keys through `objectKeys`, decodes each
`x-deserialize-default-on-error` field through `orDefault` and each
`x-deserialize-skip-invalid-items` array through `skipInvalid`, empties
an optional raw value that is `null` through `absentIfNull`, so it is not
sent on, and reads a patch field through `patchOf`, or `patchItems` for an
array that skips invalid items: a missing key is absent, `null` is null, a
value that decodes is that value, and one that does not is absent;
`types_gen.go` emits each helper once, and `Patch[T]` with its constructors
and methods once, when a patch field exists. A null raw value is an absent
one, as ACP v1 defines it for `rawInput` and `rawOutput`; a required one, such
as `result`, keeps `null`, and so does an optional one the protocol table's
`NullRetained` names, a property whose schema says explicit null is retained
(the unstable `McpError.data`): the model refuses such a rule on a required or
non-raw property, or one whose description does not state its source, and a
model test requires every committed property that says so to be named.
`Validate` checks what the schema states: a
union has exactly one variant set, a string meets its `minLength`, a number
meets a bound its Go type does not enforce, and every nested value validates,
a patch's through the value it holds; an integer bound with a fraction is
rounded inward, a `uint64` one above the `int64` range is written as such, and
one the Go type cannot reach is refused;
and it checks the table's absolute paths. Each exported `Validate` calls an
unexported `validate(abs func(string) bool)` with the host's `filepath.IsAbs`,
and passes `abs` to the nested values; every check returns a
`*ValidationError` naming the type, the property and the rule.

`connection.go` applies that check in both directions. Before sending, the
params of a generated type, and a handler's result, are validated with
`isPortableAbs`, which accepts a path absolute in POSIX or Windows form;
nothing is written on failure, and a failing result is answered `-32603`.
On receipt, `checkInbound` runs on every decoded request, notification and
response: the `WithInboundRewrite` hook, then `validate(filepath.IsAbs)`
and the gate, then the `WithInboundValidation` hook with the validation
error and, only if it accepts that, the gate's. The generated dispatch
first refuses a method received as the other kind, a request sent without an
id or a notification sent with one, by the mark `connection.go` puts in the
handler's context; it then decodes the params, absent ones as `{}`
(`decodeParams`), and calls `checkInbound`; `sendRequest` calls it on a
decoded result. `extensions.go` calls it on a received `_` message, with its
params as a `*json.RawMessage` and the side's `inboundGate`, the side's `gate`
when the schema has `initialize`, before the message reaches
`HandleExtensionMethod`.

Gates are generated and passed in, never stored: `connection.go` holds no
record. `gateMessage` in `types_gen.go` lets `initialize` through, refuses
anything else before a record with `ErrNotInitialized`, then applies
`Negotiation.Allows`, the method gates, and the message's `gates`;
`gateResult` applies only `gates`, the method having been gated when the
request was. Each side's `gate` and `resultGate` load its record and call
them: every typed wrapper but `Initialize` calls `gate` before writing and
passes `resultGate` for the response; the dispatch passes `gate` to
`checkInbound` and `resultGate` to `checkResult`; and the `initialize`
paths check the answer against the record it is about to make.
`inboundError` answers `ErrNotInitialized` and a method gate's
`*CapabilityError` `-32601`, other refusals `-32602`. `Connection`,
`SendRequest[T]` and the outbound extension methods pass no gate.

The client records `initialize` on arrival and holds what comes before it
(0030-MADR-record-initialize-on-arrival.md). The generated `Initialize` stores
an `initializeAttempt` in `initializing` before it sends, and calls
`sendRequestCommit` with a commit and the attempt's `end`. `handleResponse`,
on the reader, runs the commit for a result before it hands the answer over:
`checkInbound`, hooks and gate included, the version check, and the record's
`Store`; then `settle`, which ends the attempt, for any answer. A caller that
gives up after the reader has claimed the answer waits for the commit
(`claimedAnswer`), so its return and the record agree. The client's dispatch
and `inboundGate` pass `heldGate`, its outbound methods `gate`: `heldGate`
calls `holdUntilInitialized`, which, while an attempt is in flight, makes a
message `gate` refuses only with `ErrNotInitialized` wait for the attempt's
end, or the connection's, and judges it again. The agent's side holds nothing.

`receive` reads one line at a time through a 128 KiB `bufio.Reader`
(`readBufferSize`), up to
`WithMaxMessageSize` (10 MiB by default), and classifies it: an overlong
line is discarded and answered `-32600`; a line `json.Valid` refuses,
`-32700`; JSON that is not an object, `-32600`; an object without `method`
that has `result` or `error`, or whose `id` names a call still pending, is a
response, matched to its pending call and never answered (one with neither
`result` nor `error` is logged as a malformed response); anything else is a
request (an `id` member, `null` included) or a
notification, and one that is not a valid Request object is answered
`-32600`, with its id when valid and `null` otherwise; `params` of `null` are
read as absent. A malformed line is logged by its fault, size and id, never its
content.

Each request runs on its own goroutine, its receipt kept in `inflightReceipts`
while it runs and its cancel in `inflight` by id, each request removing only
its own entry. With `WithMaxConcurrentRequests(n)`, `receive` takes a slot of
`requestSlots`, a channel of capacity `n`, for each request, and answers one it
finds no slot for `-32603` without starting it; a slot is given back when the
handler returns, before its answer is written. Notifications run one at a time from a queue, each handler's
context marked with its sequence number. A held request keeps its slot, and a
held notification the loop, so the queue behind it fills, until the
`initialize` in flight ends. A response is returned after the
notifications received before it are handled, unless the request was sent
under the context of a notification whose handler is still running: that wait
stops before it. A message's params are left out when they encode as `null`,
refused when they encode as neither an object nor an array, and a request
whose context has ended is not written.

One writer goroutine, started by `start`, writes every message, in the order
`sendMessage` hands them over through an unbuffered channel; a sender returns
when its context ends, even while a write is blocked, and a write that has
finished is reported in preference to the context. Once the connection's
context has ended, `sendMessage` writes nothing and returns `endError`:
`ErrConnectionClosed` when this side closed it, through `Close` or the
overflow policy (`closeWith`), and otherwise `ErrPeerDisconnected` wrapping how
the reader ended. `Close` runs once: it marks the connection closed, ends its
context with `ErrConnectionClosed`, and closes the reader and the writer that
implement `io.Closer`. A `*RequestError` the connection makes for a local
failure keeps that failure as its cause, which `Unwrap` returns; one a peer
answered has none.

At a clean end of input (`io.EOF`), `receive` drains before it shuts down: it
closes `inputDone`, so a request waiting in `waitForResponse` returns at once
unless its answer was read first, and `sendRequestSent` and
`SendRequestNoResult` refuse a new request without writing it; then it waits for
the requests already received, which `inboundRequests` counts until their
answers are written, at most `notificationQueueDrainTimeout` or until `Close`.
Only then does `shutdownReceive` end the connection's context, which cancels
the handlers still running. A read that fails otherwise shuts down at once
(0019-PLAN-issue-fixes.md, B23).

The first of `closeWith` and `shutdownReceive` records why the connection
ended, in `endErr`, before it ends the context; `Err` returns that record once
the context has ended, and a later end leaves it as it was. Every goroutine the
connection starts, its handlers' included, starts through the `Go` of its
`goroutines` wait group, and `Wait` waits for the context's end and then for that
group. The `context.AfterFunc` callbacks are not counted: one on a caller's
context can start after the count has reached zero
(0026-MADR-connection-end-cause-and-wait.md).

Checks that guard this:

- `TestGeneratedFilesUpToDate` in `cmd/generate` fails when the committed
  `*_gen.go` files differ from the generator's output, and
  `TestCheckAcceptsRepository` when that output does not type-check with the
  hand-written files. The tests of `internal/output` cover the check and the
  staged write.
- `TestProtocolTableAppliesToCommittedSchema` in `internal/model` fails when a
  rule of the protocol table no longer applies to the committed schema.
- `TestKeywordRegistryMatchesLoader` and `TestCommittedSchemaKeywordsRegistered`
  in `internal/model` fail when a keyword has no treatment.
  `fidelity_test.go` covers the types, decoding, and validation it describes.
- `raw_value_test.go` guards raw values: an absent or `null` optional one
  decodes to empty and is not sent, under both builds of `encoding/json`.
- `paths_test.go` covers the absolute-path rule: sending, receiving, a
  handler's result, the two hooks and their order, and `Roots`; the engine
  case `accept-absolute-paths` covers the generated checks.
- `connection_close_test.go` covers a send that returns on its context, `Close`,
  a send after either end, and the overflow close; `errors_test.go` the
  causes of local errors; `conn_aware_test.go` the hand-off before reading,
  on both sides; and `connection_cancel_test.go` the request limit.
- `cancel_test.go` covers prompt turns and permission requests on cancel
  and on `session/close`, cancels ordered across two prompts, and the bound
  on kept cancels,
  `jsonrpc_test.go` the answers to malformed input, the size limit and the
  reader's buffer,
  `negotiation_test.go` the record and the version, `initialize_test.go` the
  record on arrival, the hooks on the reader, and the hold, with the queue and
  the slots it keeps, and `gates_test.go`
  every method and value gate both ways, the undeclared elicitation mode,
  extension messages through the hooks and the gate, the hook's order, the
  raw paths, and `initialize`'s own check; the engine cases
  `accept-prompt-cancellation`, `accept-permission-cancellation`,
  `accept-initialize-negotiation`, `accept-capability-gates`,
  `accept-capability-undeclared-variant` and `accept-session-close-cancels`
  cover the generated code, and
  `accept-description-comment-markers` the generator's comments.
- `TestEngineCases` in `cmd/generate` runs the generator on each synthetic case
  in `testdata/engine/`: a case with `want.err` must be refused with those
  lines, one with `want.api` must give that exported API, and the deprecation
  notices of its `want.deprecated`, or none when it has none. `TestEngineBehaviour`
  compiles each case's output with its `behaviour_test.go` and runs it.
  `internal/enginetest` reads the cases and lists an API and its notices.
- The conformance test at the root, `conformance_test.go` over
  `schema_gen_test.go`, builds instances of every definition from the schema
  files (`instances_test.go`) and requires each generated type to decode,
  validate, and send them back, to refuse what the schema refuses, and each
  method to reach its peer and bring back its result or error.
- `FuzzRoundTrip` at the root and `FuzzBuild` in `cmd/generate` run their seeds
  in every `go test`; `make fuzz` fuzzes them and `FuzzExportedName` for
  `FUZZTIME`. A crasher is kept under the package's `testdata/fuzz/` as a seed.
- `make mutation-engine` and `make mutation-generated` run
  `cmd/generate/internal/mutate`: one operator changed at a time, compiled
  through `go test -overlay`, reported and never gating. The engine run skips
  `TestGeneratedFilesUpToDate`.
- `json_parity_test.go` compares encodings with the goldens in
  `testdata/json_golden/`. `json_v1_pin_test.go` pins HTML escaping and sorted map
  keys. `generated_output_test.go` covers the encoding rules above, and
  `union_wire_test.go` the union decoding rules.
- `deprecation_test.go` requires each deprecation notice in the root's
  hand-written files in the form staticcheck honours, a paragraph of its own
  starting "Deprecated: "; gocritic's `deprecatedComment` reports most other
  forms too, but not a notice in the middle of a line.
- `leak_test.go`'s `TestMain` fails the root package's run when the tests leave a
  goroutine leaked, by the standard `goroutineleak` profile, such as a
  connection's that no test closed.
- `connection_end_test.go` covers `Err` for each end and after a late `Close`,
  `Wait` with a reader `Close` cannot close and with a handler that ignores its
  context, and both on the side connections; its `TestGoStatementsCounted` fails
  on a `go` statement in the root package's non-test files.
- `make pre-add-check` runs `scripts/go-precheck.sh`: gofmt, golangci-lint for the
  host and for `GOOS=windows`, vet, tests with `-race`, `go mod tidy -diff`, and
  govulncheck, for both modules. golangci-lint covers the generated files too,
  except for `goconst`, `gocognit` and `gocyclo`, which `.golangci.yml` excludes
  for `_gen.go` files with the reason. `make check-records` and `make markdownlint`
  check the docs, and `make disclosure-check` the tracked text for a real
  user-home path. GitHub Actions runs `.github/workflows/ci.yml`, which also runs
  the root tests with `GOEXPERIMENT=nojsonv2` on Linux, and both modules' tests
  with `-race` on the Linux and Windows jobs.
- CI ignores a change to the docs-safe paths alone: `docs/**`, `AGENTS.md`,
  `RELEASING.md`, `tools/README.md` and the agent-rule files under `.claude/`,
  `.codex/`, `.grok/`, `.kilo/` and `.opencode/`. Those paths, and the workflow
  itself, run `.github/workflows/docs.yml`, which runs the same three targets
  as CI's documentation step. A mixed change runs both; a `v*` tag and the
  schedule run CI. The root `README.md` is not docs-safe, since `docs_test.go`
  reads it. `scripts/check_ci_filters.py`, in `make check-tools`, reads both
  workflows with a YAML reader of its own, for the subset they use, and fails
  when their paths, gates, checkout pin or permissions disagree, or when CI
  ignores a Markdown file that Go code names
  (0037-MADR-align-docs-ci-and-disclosure-gates.md).
- `make api` and CI's "API compatibility" step run `scripts/check_api.py`:
  `apidiff`, pinned there, compares the working tree's root package,
  `agentproc` and `acptest` with the newest `v1` tag's, a package missing from the tag
  against an empty one, taken with `git archive`, or with `BASE`. An incompatible
  change, such as a removed declaration or a method added to an interface,
  fails; the compatible ones are listed. `apidiff` exits 0 either way, so the
  script reads its report. It needs the tags, so CI's Linux job at the newest
  Go fetches every commit and tag. `unstable` is a reported package: its
  incompatible changes are listed under a heading of their own and do not fail
  (0039-PLAN-five-adopter-additions-on-schema-1-25-0.md, 2e).

The schema stamp lives in `schema/version`. The `go` line lives in `go.mod` and
`cmd/generate/go.mod`.

## The unstable boundary

[0039-MADR-five-adopter-additions-on-schema-1-25-0.md](decisions/0039-MADR-five-adopter-additions-on-schema-1-25-0.md),
item 1, keeps package `acp` on its `v1` promise while the unstable schema moves.

- **The merge and its provenance.** Without a manifest, `load.MergeStableAndUnstable`
  works in three ways:
  - it replaces a stable definition that a stable method reaches with its unstable version;
  - it adds a definition only the unstable schema has under its own name;
  - it copies an unstable-only method's subgraph under the `Unstable` prefix.

  `Provenance` names what came from the unstable schema: the definitions, the
  wires, each definition's origin, and, in `Changes`, the properties and union
  entries each replaced definition gains or changes. An entry is matched to
  the stable one by `UnionEntryKey`. The model's `Field.Unstable` and
  `Variant.Unstable` come from it.
- **The manifest.** `schema/unstable-v1.json` is the unstable-origin API `acp`
  exports, frozen once, from schema `1.25.0`, by
  `go run ./cmd/generate -write-unstable-manifest`. That command refuses an existing
  manifest, and refuses unless the manifest reproduces the merge. The manifest holds:
  - the methods;
  - the name `acp` gives each kept definition;
  - for each replaced definition, the properties and the union order to apply;
  - the fragments, as a `$defs` under `schema.unstable.json`'s keys.

  When the manifest is present, `load.MergeManifest` builds `acp` from the stable files and the
  manifest alone. A union keeps the frozen entry order, then the entries the stable schema
  adds. Each definition built from the manifest names it as its origin, so the
  root conformance tests read the frozen fragments, laid over
  `schema.json`'s (`instances_test.go`). `TestUnstableManifestComplete`
  requires the model and the manifest to name each other, and, while
  `schema/version` is the one the manifest was frozen from, the manifest to be
  what that schema's merge gives.
- **The deprecation source.** Every declaration built from the manifest carries the notice "Deprecated:
  this declaration comes from the unstable ACP schema; it stays under v1 for
  compatibility only (…)". That covers:
  - the types, inline types, constants and constructors;
  - the fields and variants the manifest adds to a stable type;
  - the frozen methods' interface methods, constants and outbound methods;
  - an interface every method of which is frozen.

  The mark is `model.Deprecation.Frozen`. It is applied after the schema's
  marks, which it never replaces.
- **Package `unstable`.** `load.NewUnstable` compares the live unstable files
  with the manifest. What they add goes to `unstable/unstable_gen.go`:
  - a new definition, under its schema key, with its constructors;
  - a new method's types and constant;
  - a new variant of a stable union, as its inline type and a
    `Decode<Union><Variant>` that decodes the `Unknown` `acp` keeps for it.

  A new field of a stable type is emitted nowhere. The package's model is
  `acp`'s with the additions, so a name that collides with `acp`'s is refused.
  Its file refers to `acp` through the root's import path, which `main.go` takes
  from the `go.mod` above `-out`, with `acp` when there is none. It uses `acp`'s
  exported API only:
  - a failed check is an `*acp.ValidationError` without the message only `acp` sets;
  - an `acp` value is checked with its `Validate`;
  - an `acp` variant is decoded with `json.Unmarshal`.

  `output.Check` type-checks the root, then `unstable` through an importer that serves the root
  just checked. `output.Write` writes each package's directory, and removes a generated file a
  package directory holds that the run did not render. At `1.25.0`, `unstable/` holds only
  `doc.go`.

## The agent process helper

`agentproc` (0027-MADR-agent-process-helper.md) wraps an `*exec.Cmd` the
caller configures. `Command.Start` refuses a `Cmd` already started or whose
`Stdin` or `Stdout` is set, then gives the agent two `os.Pipe`s it owns: with
`StdoutPipe`, `Cmd.Wait` would close the connection's reader once the agent
exits, before its last output is read. A zero `WaitDelay` becomes half the
grace, so a `Stderr` writer that a process the agent started still holds
neither keeps `Wait` waiting nor makes `Close` escalate before the agent's exit
is seen.

The agent's tree is per platform, in `tree_unix.go`, `tree_windows.go` and
`tree_other.go`. On Unix `prepare` sets `Setpgid`, unless `Setsid` is set, and
refuses a non-zero `Pgid`; on Linux and FreeBSD it also sets `Pdeathsig` to
SIGKILL when the caller set none (`deathsig_pdeathsig.go`; `deathsig_none.go`
elsewhere on Unix); `terminate` and `kill` signal the group, and `kill` the
agent itself too. On Windows `prepare` adds `CREATE_SUSPENDED` to the
caller's `CreationFlags`, refusing a command that has it already, and `attach`
puts the agent in a job object with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` and
`JOB_OBJECT_LIMIT_BREAKAWAY_OK`, through `os.Process.WithHandle`, then resumes
it (`resume`: a thread snapshot, `OpenThread`, `ResumeThread`), so nothing the
agent starts precedes the assignment
(0030-MADR-record-initialize-on-arrival.md, the Windows amendment); a process
started with `CREATE_BREAKAWAY_FROM_JOB` leaves the job. `attachDelay`, zero
in production, lets a test make the assignment late. There is no terminating
signal, and `kill` ends the job. Elsewhere, `js`, `wasip1` and `plan9`, only
the agent is killed. `golang.org/x/sys` is built for Windows alone.

`run` starts the agent and waits for it on one goroutine, locked to its OS
thread until the agent is reaped: Linux sends a `Pdeathsig` when the thread
that started the child ends, which may be long before the process does
(0029-MADR-unix-client-death-pdeathsig.md). Two goroutines tie the process to
the connection. The watcher, which `run` becomes once `Start` has connected,
alone calls `Cmd.Wait`, as os/exec requires; when it returns, it ends what is left of the
tree under the lock that marks the agent exited, after which nothing is
signalled, closes `Done`, gives the connection up to the grace to read to the
end of the agent's output, and closes it. The monitor stops the agent when the
connection ends first. `Close` runs, once, its steps: close the agent's input,
which the connection shares through `sharedClose`, so the two closes never
collide; wait up to the grace; on Unix SIGTERM and the grace again; kill; close
the connection. `Kill` closes `hurry` and runs the same steps: a closed
`hurry` ends each wait at once and skips SIGTERM, so a `Close` in progress
kills too. `Wait` waits for both goroutines and the connection, drains the
taps, and joins `ErrTerminated` or `ErrKilled` to `Cmd.Wait`'s error by the
step that ended the agent.

The taps are in `tap.go` (0030-MADR-record-initialize-on-arrival.md, the
amendment for issue #10). `streams` hands the connection, for an output tap, a
`tapReader` over the output pipe, and for an input tap, a `tapWriter` over the
shared input, which offer each read's or write's bytes to the tap's queue, and
starts the tap's goroutine. That goroutine
writes the queue to the caller's writer, ignoring its error. A copy that would
take the queue past `tapQueueBytes`, 32 MiB, detaches the tap, so what it
received is a prefix, and counts what it does not receive. `Wait` runs
`drainTaps` once: it ends each tap's stream and waits, up to the grace from one
timer, for its queue to be written, then abandons a tap still writing. With no
tap set, the connection gets the pipe and the input themselves, and no
goroutine starts.

Checks that guard it:

- `agentproc`'s tests run the test binary itself as the agent, in a mode an
  environment variable names (`child_test.go`): `TestStartRefuses`,
  `TestNoLostOutput`, `TestCloseLadder`, `TestStderrHeldByDescendant`,
  `TestCloseIdempotent`, `TestDefaultGrace`, `TestCallersCommandKept`,
  `TestDescendantsEnded`, `TestConnectionEndStopsAgent`, `TestKill`, `TestKillCutsCloseShort`,
  `TestKillAfterExit`, and the taps' `TestOutputTapVerbatim`, `TestInputTapSeesWrites`,
  `TestTapErrorIgnored`, `TestTapsUnset`, `TestBlockedTapDetached`, `TestDetachedTapIsPrefix` and
  `TestTapDrainBoundedByGrace`; on Unix `TestStartRefusesSharedGroup` and
  `TestEscapedDescendantDoesNotHoldWait`, `TestTapKeepsCloseEnding` and `TestClientDeathEndsAgentAtInputEnd`; on Linux and FreeBSD
  `TestStartFromExitingLockedGoroutine`, `TestDefaultPdeathsig`, `TestCallersPdeathsigKept` and
  `TestClientDeathEndsAgent`; on Windows `TestClientDeathEndsTree`, `TestDescendantInJobDespiteSlowAttach`,
  `TestStartRefusesSuspended`, `TestCallersCreationFlagsKept` and `TestBreakawayLeavesJob`.
- Its `TestMain` runs the goroutine-leak guard after the tests, as the root's
  does.
- `TestReadmeAgentprocIsTheExample` at the root requires the README's example
  to be `ExampleCommand`'s body.

## Verification tools

`tools/` (0031-MADR-verification-tools-in-repository.md) holds what a PLAN's
steps run beyond `make pre-add-check`, in Python and the standard library;
`tools/README.md` gives each tool's use. `scripts/` keeps what `make` and CI
call. `_common.py` gives every tool the repository's root from git, the test
hosts from `ACP_LINUX_HOST` and `ACP_WINDOWS_HOST`, the logs under
`.cache/tools/`, and the caches, Go's and the planted breaks' copies of the
tree, in the user's cache directory, outside the tree, where `make
markdownlint` and `scripts/check_records.py` would otherwise walk them.

- `gates.py` runs the gate set, one gate at a time, each exit status taken
  before its output is read; `floor.py` the tests at the `go` line;
  `platforms.py` `agentproc`'s lint and vet per platform; `remote.py` the tests
  on a test host, in a fresh directory removed after, and `leftovers.py` the
  Windows host's process list.
- `idscan.py` scans for identifiers with the push guard's deny list
  (`DISCLOSURE_DENY` or `~/.config/git/disclosure-deny`), and prints where a
  match is, never what it is; nothing in the tree names an identifier.
- `plants.py` runs planted breaks from a JSON list, each in its own copy.
- `release_body.py`, `publication.py`, `consumer.py` and `watch_ci.py` build a
  release's body from its report and check the release, its publication, a
  consumer at the floor, and a commit's CI run.
- `ci_jobs.py` reads CI's job history: every job's first attempt tallied, a
  failed job's step, seed and crash, and a variable's value in each step's
  environment, each log read by its job id, so from that job's own attempt.
- `find.py` and `procs.py` search files and processes.
- `make check-tools` compiles every tool and script, runs `tools/test_tools.py`,
  the tools' offline parts, and `scripts/`' tests, and runs
  `scripts/check_ci_filters.py`, in CI's records step too; what needs a host or
  the network each step runs, and its record says.

## Tree

```text
README.md                 repository entry; links here
RELEASING.md              schema bump and release steps
AGENTS.md                 agent instructions
Makefile                  version, test, lint, pre-add-check, fuzz, mutation, docs gates, check-tools
go.mod                    the SDK module
*.go                      package acp: hand-written and *_gen.go files
agentproc/                package agentproc: an agent as a child process, started and stopped
unstable/                 package unstable: doc.go, and unstable_gen.go when the unstable schema adds API
acptest/                  package acptest: a connection pair, a frame recorder, a scripted agent, a memory client
schema_gen_test.go        generated list of types, unions, and methods for tests
cmd/generate/             the generator, its own module
  internal/load/          schema loading
  internal/model/         the resolved schema and the protocol table
  internal/emit/          go/ast emission, rendered in memory
  internal/output/        type-check and staged write of the output
  internal/util/          naming helpers
  internal/enginetest/    engine case reader, API and notice listings, for tests
  internal/mutate/        the mutation tool
  testdata/engine/        synthetic engine cases, one txtar each
  testdata/fuzz/          FuzzBuild seeds kept from crashers
schema/                   schema files, the version stamp, and the frozen unstable-v1.json
example/                  agent, client, claude-code, gemini programs
testdata/json_golden/     JSON goldens
testdata/fuzz/            FuzzRoundTrip seeds kept from crashers
scripts/go-precheck.sh    the pre-add gate
scripts/check_records.py  record numbering and link check
scripts/check_disclosure.py  real user-home paths in tracked text
scripts/check_ci_filters.py  CI's and Docs' paths, gates and pins agree
scripts/check_api.py      the API gate: apidiff against the newest v1 tag
tools/                    the verification tools; tools/README.md
.github/workflows/ci.yml  CI
.github/workflows/docs.yml  the documentation gates, for docs-only changes
.markdownlint-cli2.jsonc  fleet Markdown style
.gitattributes            LF on every platform
docs/
  README.md               ToC and the "I want to…" matrix
  architecture.md         this file
  decisions/              MADR/PLAN pairs
  reports/                numbered observations
  guides/                 unnumbered how-to documents (not created yet)
```

## What is not here

`docs/guides/` does not exist. The first guide creates it. The Go floor and
the schema pin live in the decision records indexed from `docs/README.md`.
