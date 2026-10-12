---
status: accepted
date: 2026-10-11
decision-makers: Repository owner
---

# Generate the ACP schema faithfully through a keyword registry

## Context and Problem Statement

[0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md) restructured the generator around a resolved schema model and
named phase 4, schema fidelity: honour `x-deserialize-default-on-error` and `x-deserialize-skip-invalid-items`, give integer fields the Go type of
their declared format, type `additionalProperties`, and make `Validate` descend and check bounds. Phases 1 to 3 are complete. Phase 1 already records
every fact phase 4 needs on each property (`model.Facts`: format, bounds, value type, the two keywords); nothing reads them yet.

The owner asked for an assessment against the ACP specification and documentation, JSON Schema practice, and idiomatic Go, and for the result to be a
framework: one rule for how every schema keyword reaches the generated code, rather than phase 4's list of point fixes. This record makes that
decision. It refines phase 4 of `0011-MADR-harden-code-generator-engine.md` and replaces its one-line scope.

The facts below were measured on `08821e9` against schema `1.25.0` (`schema/schema.json`, 178 definitions; `schema/schema.unstable.json`, 277), on
the merged model the generator builds, and by probes of `encoding/json` in Go 1.27.2 in scratch copies. Nothing in the tree changed.

### What the ACP sources say

* **The schema is generated from Rust.** "The schema files in the `/schema` directory are generated from the Rust code in the `src` directory"
  (agent-client-protocol `CONTRIBUTING.md`). No ACP page calls `schema.json` normative.
* **`x-deserialize-default-on-error` is Rust's `DefaultOnError`.** Every occurrence sits on a field declared
  `#[serde_as(deserialize_as = "DefaultOnError<...>")]`, and the schema generator's tests require the pairing. serde_with documents it: "Deserialize
  value and return `Default` on error … Instead of erroring, it simply deserializes the `Default` variant of the type", and "Missing entries still cause
  errors". On a malformed value the field takes its type's default; a missing key is a separate matter, governed by `#[serde(default)]`. The one field
  with a custom default agrees with its type's default. The official Python SDK uses the field's declared default instead.
* **`x-deserialize-skip-invalid-items` is `VecSkipError`.** "Deserialize a sequence into `Vec<T>`, skipping elements which fail to deserialize"; ACP
  passes no inspector, so skipped items are dropped silently and nothing is logged.
* **When to be lenient.** ACP's `AGENTS.md`: "Use `DefaultOnError`/`VecSkipError` only when the fallback means less … If the receiver acts on the
  value …, keep it strict so a request fails with `-32602` instead of silently doing something else."
* **Unknown values.** The v2 extensibility page: "When storing, replaying, proxying, or forwarding data, implementations SHOULD preserve unknown values
  and any raw tagged-union payloads"; the v2 migration checklist: "Strict where it counts. Reject malformed payloads for known discriminator values".
  The v1 extensibility page reserves `_`-prefixed method names and says custom root fields "MUST NOT" be added. No ACP page says "MUST ignore unknown
  fields"; the Rust crate does not deny them, and the schemas carry `additionalProperties: true` about 125 times and `false` never.
* **The keywords are recent.** The changelog entry for 0.13.5 (2026-06-01) reads "Experiment annotating lenient deserialize opportunities"; the
  Rust-only tolerance dates from 0.12.0. ACP also publishes v2 schemas; this SDK pins v1 at `1.25.0`.

### What JSON Schema and OpenAPI say

* **`format` is an annotation by default.** JSON Schema 2020-12 validation §7.2.1: "The value of format MUST be collected as an annotation", and an
  assertion mode "MUST be disabled by default". §7.2.3: "An implementation MUST NOT fail to collect unknown formats as annotations."
* **The integer formats are not JSON Schema formats.** 2020-12 §7.3 defines none of `int32`, `int64`, `uint16`, `uint32`, `uint64`, `double`. OpenAPI
  3.1 defines `int32`, `int64`, `float`, `double`; the OpenAPI format registry lists `uint8` to `uint64`. schemars, which writes ACP's schema, emits
  them from the Rust types (`u64` → `"uint64"` with `"minimum": 0`; `u16` → `"uint16"` with `"minimum": 0, "maximum": 65535`).
* **`default` is an annotation.** §9.2: "This keyword can be used to supply a default JSON value associated with a particular schema." Filling it in
  is the generator's choice, made by phase 1 and kept here.
* **Unknown keywords.** 2020-12 core §4.3.1: "Unknown keywords SHOULD be treated as annotations." The generator's loader goes further and refuses a
  keyword it does not model, so a new keyword cannot be ignored silently.

### What the generated code does today

* **S1. Integers.** 46 generated fields with an integer format (`int32` 1, `int64` 12, `uint32` 16, `uint64` 17) and 12 named types
  (`ProtocolVersion` `uint16`, `RequestIdNumber` and `ElicitationContentValueInteger` `int64`, the nine `ErrorCode…` variants `int32`) are `int` or
  `*int`: signed, and 32 bits wide on 32-bit platforms. In Go 1.27.2 a sized type checks the range on decode: `-1` into `uint64`, `4294967296` into
  `uint32`, and `65536` into `uint16` each fail with an `UnmarshalTypeError`. `double` is already `float64`.
* **S2. Typed maps.** 6 generated fields whose `additionalProperties` names a type are `map[string]any`: `env` of `AuthMethod` and
  `AuthMethodTerminal` and `headers` of `UnstableSetProviderRequest` (`string`), `content` of `CreateElicitationResponse` and
  `ElicitationAcceptAction` (`ElicitationContentValue`), `properties` of `ElicitationSchema` (`ElicitationPropertySchema`).
* **S3. Arbitrary JSON.** 18 fields are `any`: `rawInput` and `rawOutput` (on `ToolCall`, `ToolCallUpdate`, and two `SessionUpdate` variants), the
  `params` and `result` of the six envelope types (`AgentRequest` … `ClientResponse`), `Error.data`, `UnstableMcpError.data`,
  `UnstableMessageMcpResponse.result`, and the null-typed `stopReason` of `IdleStateUpdate`'s `None` variant. Decoding into `any` makes every number a
  `float64`: `9007199254740993` comes back as `9.007199254740992e+15`.
* **S4. The deserialization keywords.** 528 generated fields carry `x-deserialize-default-on-error` (13 of them required, 21 with a schema `default`)
  and 35 carry `x-deserialize-skip-invalid-items`; none is honoured, so `InitializeResponse` with `"agentCapabilities":"garbage"` fails to decode
  (0011's F4). Every required field with default-on-error is an array that also skips invalid items.
* **S5. Required keys.** 438 required keys across 214 structs are not checked on decode: a missing key decodes to the Go zero value. `Validate` then
  catches only required strings (`""`) and arrays (`nil`), 39 checks in all, which is stricter than the schema for a present empty string
  (`{"sessionId":""}` is schema-valid and rejected with `-32602`) and blind to a missing number, boolean, or object.
* **S6. Validation depth.** 99 types have `Validate`; it does not descend into fields, unions, or slices (0011's F7), and checks no `minLength` (the
  schema's only one is `Notice.title`, 2 generated fields). No bound in `1.25.0` lies outside its Go type's range once S1 is applied.
* **S7. Dead `omitempty`.** 8 struct-valued fields (the capability structs on `InitializeRequest`, `InitializeResponse`, `AgentCapabilities`,
  `ClientCapabilities`) carry `omitempty`, which never omits a struct: a zero capability struct is sent as `{"c":{}}`, where `omitzero` would omit it.
  Each has a schema `default` of `{}`.
* **S8. Receiver names.** 21 generated types use two receiver names (`u` for the union codec, `v` for `Validate`). Go Code Review Comments: "Be
  consistent, too: if you call the receiver "c" in one method, don't call it "cl" in another."
* **S9. Decoding cost.** 290 of the 291 generated structs have a required key or a lenient field; 43 generated `UnmarshalJSON` methods exist today.
  `types_gen.go` has 12,190 lines.

## Decision Drivers

* The SDK accepts what the schema accepts and rejects what it rejects, with the leniency ACP's Rust implementation applies and no more.
* Every schema keyword has exactly one declared treatment; a keyword or format the generator cannot honour stops generation with a named error.
* A peer's data is not changed in transit: arbitrary JSON keeps every digit, unknown values are preserved (0011 phase 2).
* The generated code is idiomatic Go: sized integer types, typed maps, consistent receivers, no tag that does nothing.
* The exported API changes only where this record says so, in a new minor version on `v0` with every change listed.
* No new dependency ([0007-MADR-self-contained-acp-sdk.md](0007-MADR-self-contained-acp-sdk.md)); `encoding/json` v1 stays.

## Considered Options

* A keyword registry in the model, with one treatment per keyword, rendered by the emitters
* Point fixes in the emitters, one per finding
* Validation at run time against the embedded schema with a JSON Schema library
* Leave the generated code as it is

## Decision Outcome

Chosen option: "A keyword registry in the model, with one treatment per keyword, rendered by the emitters", because it gives every keyword one
reviewable place, carries 0011's model-first design through to the output, and makes the next schema's new keyword a named failure instead of a silent
gap; the other options either repeat 0011's original problem or add a dependency 0007 rules out.

### The framework

The loader refuses any keyword `load.Definition` does not declare (it decodes strictly). The model adds a registry, `model/keywords.go`, that gives
every declared keyword one treatment, and a test that requires the registry and `load.Definition` to list the same keywords:

| Keyword | Treatment | Go result |
| --- | --- | --- |
| `$ref`, `allOf`, `anyOf`, `oneOf`, `discriminator`, `not`, `unevaluatedProperties`, `properties`, `items`, `title` | structure | types, unions, variants (phases 1–2) |
| `type`, `format` (integer and number), `additionalProperties` with a schema, `enum`, `const` | type | the Go type; constants |
| `required` | decode | presence checked on decode |
| `default` | decode and encode | filled when absent; the fallback of a lenient field |
| `x-deserialize-default-on-error`, `x-deserialize-skip-invalid-items` | decode | lenient field and item decoding |
| `minimum`, `maximum` | type or validate | enforced by the Go type when it equals the type's range, else checked by `Validate` |
| `minLength` | validate | `Validate` counts characters (Unicode code points) |
| `format` (string), `description` | annotation | doc comments; no check |
| `x-method`, `x-side`, `x-docs-ignore` | protocol | methods, bindings, docs (phases 1, 3) |

A `format` the registry does not know on an `integer` or `number` is refused, naming the definition, property, and format; an unknown `format` on a
`string` is an annotation, as 2020-12 §7.2.3 requires.

### The treatments

1. **Integer formats** (0011's decision, kept): `int8`, `int16`, `int32`, `int64` → `int8` … `int64`; `uint8`, `uint16`, `uint32`, `uint64` →
   `uint8` … `uint64`; `int` → `int64`, `uint` → `uint64` (schemars' `isize` and `usize`); an integer without a format → `int64` (none at `1.25.0`). `float` → `float32`,
   `double` → `float64`. This applies to properties, named primitive types, and union variants alike.
2. **Typed `additionalProperties`** (0011's decision, kept): `map[string]T`. `additionalProperties: true` stays `map[string]any`.
3. **Arbitrary JSON** (decided 2026-10-10): a property or variant whose schema constrains no type becomes `json.RawMessage`, so it is carried byte for
   byte. (Amended 2026-10-10, see the amendment below: the rule is any property or variant the model gives no Go type.) The hand-written options `WithStartRawInput`, `WithStartRawOutput`, `WithUpdateRawInput`, and `WithUpdateRawOutput` take `json.RawMessage`,
   and `WithStartLocations` merges `path` into a raw input object through `map[string]json.RawMessage`. `_meta` stays `map[string]any`
   (`additionalProperties: true`), and so do the two `params` objects of `mcp/message`.
4. **Required keys** (decided 2026-10-10): a generated `UnmarshalJSON` fails when a required key is absent, naming the type and the property; dispatch
   turns it into `-32602`. In an inline variant of a discriminated union the discriminator is exempt, because the union already placed the payload
   (0011 phase 2's absent-key rule). JSON `null` for a struct that is not nullable reads as an object with no keys, so it fails when the struct has
   a required key.
5. **`x-deserialize-default-on-error`**: a present value that fails to decode takes the property's schema `default` when it declares one, else the Go
   zero value (`nil`, `0`, `false`, `""`, an empty slice), the value the field has when its key is absent. For an `Option` or a `Vec` this is
   what Rust's `Default` gives; where a Rust type's default differs (an enum's `#[default]` variant), the schema does not state it, and the Go
   field reads as absent. The key itself still has to be present when it is required, as in Rust.
6. **`x-deserialize-skip-invalid-items`**: an array item that fails to decode is dropped, silently, as in Rust. A value that is not an array is an
   error, unless the property also has default-on-error. An item that decodes to a union's `Unknown` or catch-all is kept.
7. **Validation** (decided 2026-10-10): `Validate` checks what the schema states and nothing else: a union has exactly one variant or `Unknown` set,
   and the set one validates; `minLength`; a bound outside the Go type's range; and every field whose type, slice element, map value, or pointer
   target validates. The `""` and `nil` checks go; presence is a decoding matter (4). Every type that has `Validate` today keeps it. The protocol
   table's validate rule for `ToolCallUpdate` (from `0011-PLAN-protocol-table.md`) is removed with them.
8. **`omitempty` on struct values** (decided 2026-10-10): dropped from the tag, so the tag says what the encoder does; the wire does not change.
9. **Receivers**: a union's methods use `u`, every other type's `v`.
10. **String `format`** (`uri` at `1.25.0`): annotation only.

### Consequences

* Good, because each keyword has one treatment, listed in one table and enforced by a test, and the next unknown keyword or numeric format stops
  generation with a named error.
* Good, because the SDK tolerates exactly what ACP's Rust implementation tolerates (S4) and rejects a missing required key the way it does (S5).
* Good, because integers carry their declared range and width on every platform, and arbitrary JSON keeps every digit (S1, S3).
* Good, because `Validate` stops rejecting schema-valid messages and starts finding an invalid value inside a union or a slice (S5, S6).
* Neutral, because a valid message encodes and decodes as today: no golden changes.
* Bad, because the exported API changes: 58 integer declarations, 6 map fields, 18 `any` fields, 4 option signatures and `StartEditToolCall`
  (amended 2026-10-10); code that reads `*params.Line`
  as an `int`, or sets `RawInput` to a map, stops compiling. The release notes list each.
* Bad, because messages the SDK accepts today now fail: one that omits a required key, or sends `null` for an object with required keys, gets
  `-32602`. A present empty string, which `Validate` rejects today, passes.
* Bad, because nearly every struct gains a generated `UnmarshalJSON` (S9) that reads the object twice, once for its keys and once for its fields;
  `types_gen.go` grows by several thousand lines.
* Bad, because `x-deserialize-*` are marked experimental upstream, so their meaning may change with a schema update.

### Confirmation

* A test requires the registry and `load.Definition` to list the same keywords, and every keyword the committed schema uses to be registered.
* For each treatment, a test that failed before its step passes after, with a planted breakage shown failing.
* The probes of S1, S3, S4, S5, S6 become tests: `-1` into a `uint64` field fails; `9007199254740993` in `rawInput` round-trips; `"agentCapabilities":
  "garbage"` decodes with zero capabilities; a `Plan` missing `entries` fails and one with an invalid entry drops it; a `PromptRequest` with an empty
  `ContentBlock` fails `Validate`; `{"sessionId":""}` passes it.
* No golden in `testdata/json_golden/` changes, and `TestGeneratedFilesUpToDate` passes after every step.
* The exported API difference from `08821e9` is exactly the list in the PLAN's release notes.

## Pros and Cons of the Options

### A keyword registry in the model, with one treatment per keyword, rendered by the emitters

* Good, because the treatment of every keyword is decided once, in the model, and the emitters stay renderers (0011's driver).
* Good, because the registry turns "did we handle this keyword?" into a test.
* Bad, because it is the largest change to `types_gen.go` since the port, and it breaks the API in several places at once.

### Point fixes in the emitters, one per finding

* Good, because each fix is small.
* Bad, because it rebuilds the scattered decisions 0011 removed, and gives no answer for the next keyword.

### Validation at run time against the embedded schema with a JSON Schema library

* Good, because a validator checks every keyword without generated code.
* Bad, because it adds a dependency, which [0007-MADR-self-contained-acp-sdk.md](0007-MADR-self-contained-acp-sdk.md) rules out, validates JSON
  rather than Go values, and does not implement `x-deserialize-*`.

### Leave the generated code as it is

* Good, because nothing breaks.
* Bad, because S1 to S8 stay: wrong integer widths, untyped maps, lost digits, rejected tolerant payloads, and a `Validate` that is both too strict and
  too shallow.

## Amendment 2026-10-10, the rule for raw JSON

Found in step 5 of the plan: treatment 3's rule, a node that constrains no type ("no `type`, `$ref`, `enum`, `const`, or composition"), covers 12 of
S3's 18 fields. The other six, the `params` and `result` of `AgentRequest`, `AgentResponse`, `AgentNotification`, `ClientRequest`,
`ClientResponse`, and `ClientNotification`, are inline `anyOf` compositions of every request or response type, which the model does not name, so it
gives them `any`; the decision named all 18.

Decision: the owner chose the rule "whatever has no Go type": a property or variant the model would give `any`, because its schema constrains no
type, is only `null`, or is an inline composition the model does not name, becomes `json.RawMessage`. All 18 fields of S3 change. A map value the
model gives no Go type (`additionalProperties: true`) stays `any`, so `_meta` is unchanged. Not chosen: keeping the narrow rule and 12 fields, or
naming the six envelope unions, which are `x-docs-ignore` routing types the SDK does not use.

## Amendment 2026-10-10, StartEditToolCall's content

Found in step 5: besides the four options of treatment 3, the hand-written `StartReadToolCall` and `StartEditToolCall` build a raw input from a map.
`StartReadToolCall`'s input is its `path` string, which always encodes. `StartEditToolCall(id, title, path string, content any, opts...)` would have
to encode `content` itself, with no error to return; today a value that cannot encode fails when the update is sent.

Decision: the owner chose to change `content` to `json.RawMessage`, so the caller encodes it, as for the four options. The exported API changes in a
fifth helper signature, listed in the release notes. Not chosen: keeping `content any` and leaving it out of the raw input when it cannot encode, or
storing an invalid value so the send fails as today.

## Amendment 2026-10-11, a null raw value is an absent one

The owner reported a regression that no compiler caught: empty raw tool input now shows as the literal `null`. Traced on `87f3db7`:

* **Printing.** In Go 1.27.2's default build `encoding/json` declares `type RawMessage = jsontext.Value` (`encoding/json/v2_stream.go:190`), and
  `func (v Value) String() string` returns `"null"` when `v == nil` (`encoding/json/jsontext/value.go:74`–`76`). `fmt` calls it, so a raw input
  that is absent prints as `null`. With `GOEXPERIMENT=nojsonv2`, `RawMessage` is `[]byte` (`encoding/json/stream.go:273`) and prints as `[]`.
  Before treatment 3 the field was `any`, and an absent value printed as `<nil>`. Go issue 81050, "encoding/json: fmt.Print representation of
  json.RawMessage changed in Go 1.27" (open, NeedsDecision), has a reply from the `encoding/json/v2` author: printing `null` for nil "seems like
  right behavior", and `fmt` output "is generally not covered by the Go compatibility agreement"
  (<https://github.com/golang/go/issues/81050>). The Go 1.27 release notes say the `nojsonv2` opt-out "is expected to be removed in a future
  release".
* **Decoding.** `json.RawMessage` keeps a JSON `null` as the four bytes `null`, where `any` held nil; Go has kept that behaviour since golang/go
  issue 40495. A probe in scratch copies at `633281b^` (the commit before raw JSON) and at `87f3db7`, decoding a `tool_call` and a
  `tool_call_update`, gave:

  | `rawInput` on the wire | `633281b^` | `87f3db7` |
  | --- | --- | --- |
  | absent | nil, prints `<nil>`, omitted when sent | nil, prints `null`, omitted when sent |
  | `null` | nil, prints `<nil>`, omitted when sent | the bytes `null`, prints `null`, sent as `"rawInput":null` |
  | `{}` | `map[]`, sent as `{}` | `{}`, sent as `{}` |

  An input decoded from `null` also defeats `WithStartLocations`: `null` decodes into a nil map, so `withRawPath` returns it unchanged and no
  `path` is added.
* **What ACP says.** The v1 tool-call documentation: "Omission and `null` are equivalent for `rawInput` and `rawOutput`. On an initial tool call,
  both mean no value is provided. On a tool call update, both leave the previously reported value unchanged." The Rust reference types them
  `Option<serde_json::Value>` under `#[skip_serializing_none]`, so a `null` is read as `None` and never sent
  (`agent-client-protocol-schema/src/v1/tool_call.rs` at `7725341d46d7`). The TypeScript SDK declares `rawInput?: unknown`, and the Python SDK
  `Optional[Any] = None` with `None` left out when it sends v1. ACP v2 gives these fields three states, omitted, `null`, and a value
  (`MaybeUndefined` in Rust), which v1 does not have.
* **Other Go code.** `github.com/coder/acp-go-sdk`, which this SDK was separated from, has declared `RawInput any` with `omitempty` in every
  version. The MCP Go SDK uses `any` on its client side and `json.RawMessage` with a manual `len(raw) == 0 || string(raw) == "null"` check on its
  server side. Kubernetes' `apiextensions` `JSON` and `runtime.RawExtension` map `null` to empty in `UnmarshalJSON`. `x/tools`' jsonrpc2 uses
  `*json.RawMessage`. `mark3labs/mcp-go` keeps a decoded value and a raw copy.
* **Scope.** All 18 fields of S3 are raw: 15 optional (`rawInput`, `rawOutput`, `data`, `params`, `stopReason`) and the 3 required `result` fields
  of `AgentResult`, `ClientResult`, and `UnstableMessageMcpResult`. `633281b`, which made them `json.RawMessage`, is in `v0.14.0`.
* **Why no compiler caught it.** `fmt` takes any value, and `string(v)`, `json.Unmarshal(v, &x)`, and `%v` all compile for `any` and for
  `json.RawMessage` alike; only what they produce changed.

Decision: the owner first chose an SDK type for raw values, `RawJSON`, which would print an empty value as nothing. Asked for a better solution,
in line with idiomatic Go and with ACP, and for options from ACP's documentation, the other SDKs, and other Go projects, the owner chose instead to
keep `json.RawMessage` and treat a `null` raw value as an absent one:

* The fields stay `json.RawMessage`, as do the five helpers' parameters; the exported API does not change.
* A JSON `null` in an optional raw field decodes to an empty value, as ACP v1 defines `null` and omission to be equivalent. An empty value is
  omitted when sent, so a `null` is no longer sent on. `withRawPath` treats a `null` input as empty, so `WithStartLocations` adds `path` to it.
* A required raw field, the three `result` fields, keeps `null`: it is the value the field carries.
* Every other value is kept byte for byte, digits included. A union's `Unknown`, a catch-all's `Extra`, and the `params` of an extension method are
  unchanged.
* The documentation says that an absent raw value is one with `len(v) == 0`, that `string(v)` gives `""` for it, and that `fmt` prints an empty
  `json.RawMessage` as `null` under Go 1.27's default build, as golang/go issue 81050 describes.

Not chosen: going back to `any`, decoding numbers as `json.Number`, which matches the original SDK but breaks code that reads the bytes and turns
`float64` assertions into run-time failures; and the SDK type `RawJSON`, which prints nothing for an empty value under both builds but breaks the
API again, puts a type of the SDK's own where Go code expects the standard one, and overrides how Go prints raw JSON. Neither gives ACP v2's
three states, which will need a type of their own.

Consequences:

* Good, because a `null` raw value is read and sent as ACP v1 defines it, and as before treatment 3, and `WithStartLocations` works on it again.
* Good, because the exported API and the standard type stay; nothing changes for code written against `v0.14.0`.
* Good, because a regression test pins what a raw value decodes to, encodes as, and gives from `string(v)`, under both builds.
* Bad, because an empty raw value printed with `%v`, `%s`, or `fmt.Sprint` still shows `null` under the default build: that is Go 1.27's
  behaviour for every `json.RawMessage`, documented here rather than overridden. Until the opt-out goes, the same value prints differently under
  `GOEXPERIMENT=nojsonv2`.
* Neutral, because an explicit `null` and an absent key are no longer told apart, as before treatment 3 and as ACP v1 defines them.

Implementation: step 6a of [0011-PLAN-engine-tests.md](0011-PLAN-engine-tests.md), added to that plan's scope by the owner on 2026-10-11.

## Amendment 2026-10-11, an untyped definition is raw JSON

Found by the conformance test of step 7 of [0011-PLAN-engine-tests.md](0011-PLAN-engine-tests.md): treatment 3 covers a property or variant the
model gives no Go type, not a definition. `ExtRequest`, `ExtResponse`, and `ExtNotification` are definitions that constrain no type, generated as
`type X any`, so a number decodes as `float64` and `9007199254740993` is sent as `9007199254740992`. They are the only such definitions; no
hand-written code uses them.

Decision: asked, the owner chose to extend treatment 3 to definitions. A definition the model gives no Go type is an alias of `json.RawMessage`,
`type ExtRequest = json.RawMessage`. An alias keeps `json.RawMessage`'s methods; a defined type over it would not, and would encode as a base64
string. The exported API changes in the type of these three names; the release notes list it. Not chosen: keeping `any` and recording the loss.

## Amendment 2026-10-11, key names, and invalid UTF-8 in raw values

Found by `FuzzRoundTrip` in step 8 of [0011-PLAN-engine-tests.md](0011-PLAN-engine-tests.md), and decided by the owner
([0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md), "Found in step 8 and decided").

* The decoders this record defines read a key as `encoding/json` reads a struct field's name: under Unicode case folding, the last matching key
  winning. That covers the required keys, the discriminator, the keys that pick a variant or member, the keys whose absence applies a default,
  and the keys a catch-all keeps. Before, they read exact names only, while the structs they decode did not.
* Treatment 3 carries a raw value byte for byte, invalid UTF-8 included: a string field replaces invalid UTF-8 with U+FFFD, and a raw value does
  not, as Go's `json.RawMessage` does not.

## Amendment 2026-10-11, the variant a payload without its discriminator fits

Found by `TestConformanceAbsentDiscriminator`, written in step 9 of [0011-PLAN-engine-tests.md](0011-PLAN-engine-tests.md) for survivors of
the generated-code mutation run. The README said a payload without its discriminator "decodes into the one variant whose required keys it
carries". The generated code picks that variant by its own required keys, without those the union requires of every variant
(`SessionConfigOption`'s `id` and `name`, `CreateElicitationRequest`'s `message`). A payload that lacks such a key fits a variant and is
refused naming the key, `SessionConfigOption: variant Boolean: SessionConfigOptionBoolean: missing required property "id"`, where the README
read as keeping it in `Unknown` or refusing it as missing its discriminator.

Decision: asked, the owner kept the behaviour and the README is clarified. A payload that lacks a key every variant requires is invalid, and
is refused naming the key, as a struct is; `Unknown` is for a payload a later schema could define, which would carry those keys. Not chosen:
counting the union's required keys in the fit, which would keep such payloads in `Unknown`.

## Amendment 2026-10-12, from 0014

Decided by the owner in [0014-MADR-union-constructors-take-every-required-key.md](0014-MADR-union-constructors-take-every-required-key.md),
and carried out by [0014-PLAN-union-constructors-take-every-required-key.md](0014-PLAN-union-constructors-take-every-required-key.md). Two
of its decisions change what this record asserts.

* **Outcome 7 gains one check.** A union's `Validate` refuses a catch-all whose discriminator is a value the catch-all's `not` excludes,
  naming the union, the key and the value (0014-MADR, C1). The schema states the exclusion, so the check follows outcome 7's rule of
  checking what the schema states and nothing else. A probe on `bc0cb78` found that `ElicitationPropertySchema{Other: {Type: "string"}}`
  passed `Validate`, was sent, and was decoded by the peer as the `String` variant. The decoder never builds such a value, since it routes
  each listed value to its own variant; the check fires only on a value built in Go.
* **A nil array variant is sent as `[]`.** A union's `MarshalJSON` writes `[]` for a set array variant holding `nil`, as required array
  fields already are (0014-MADR, item 7). A probe on `bc0cb78` found such a variant sent as `null`, which the schema does not allow, and
  decoded back with no variant set and no error.

## More Information

Sources:

* ACP: <https://agentclientprotocol.com/protocol/v1/extensibility>, <https://agentclientprotocol.com/protocol/v2/extensibility>,
  <https://agentclientprotocol.com/protocol/v2/migration>; the repository `agentclientprotocol/agent-client-protocol` at
  `382532e26eef2d779838448add825ac45a62914f`: `CONTRIBUTING.md`, `AGENTS.md`, `CHANGELOG.md`, `schema-generator/src/main.rs`,
  `agent-client-protocol-schema/src/v1/plan.rs`.
* serde_with 3.24.0: <https://docs.rs/serde_with/3.24.0/serde_with/struct.DefaultOnError.html>,
  <https://docs.rs/serde_with/3.24.0/serde_with/struct.VecSkipError.html>. schemars 1.2.2, `schemars/src/json_schema_impls/primitives.rs`.
* JSON Schema 2020-12: <https://json-schema.org/draft/2020-12/json-schema-validation>, <https://json-schema.org/draft/2020-12/json-schema-core>.
  OpenAPI: <https://spec.openapis.org/oas/v3.1.0.html#data-types>, <https://spec.openapis.org/registry/format/>.
* Go: <https://go.dev/doc/go1.24#encodingjsonpkgencodingjson> (`omitzero`), <https://go.dev/doc/go1.27> (`encoding/json` backed by v2, behaviour
  preserved), <https://pkg.go.dev/encoding/json>, <https://go.dev/wiki/CodeReviewComments#receiver-names>.

Out of scope, and why: `_meta` and the two `mcp/message` `params` maps keep `map[string]any`, so numbers inside them are still `float64` (the owner's
decision covered the 18 `any` fields); `RequestId` decoding any object as `Null` and `connection.go`'s `cancelRequestParams` (recorded in 0011's
plans); ACP's v2 schemas and their three-state fields; rendering in memory and type-checking the output (0011 phase 5).

Implementation: [0012-PLAN-schema-fidelity-framework.md](0012-PLAN-schema-fidelity-framework.md).
