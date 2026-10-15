---
status: in-progress
date: 2026-10-15
associated-madr: "0022-MADR-keep-elicitation-property-order-before-v1-15-0.md"
---

# Implement a generated `PropertyOrder` that keeps an elicitation form's property order, before `v1.15.0`

Associated MADR: [0022-MADR-keep-elicitation-property-order-before-v1-15-0.md](0022-MADR-keep-elicitation-property-order-before-v1-15-0.md)

## Goal

Before `v1.15.0` is tagged ([0019-PLAN-generator-hardening.md](0019-PLAN-generator-hardening.md), step 8), resolve GitHub issue #6:
`ElicitationSchema` gains `PropertyOrder []string`, the order of the keys of `Properties`, filled on decode and used on encode,
generated from a protocol-table rule, the same under both JSON builds; a value that does not set it encodes as before. The issue is
closed when the release that carries it is published.

## Scope

In scope:

* Generator, `cmd/generate`: `internal/model` (`OrderRule` and `KeyOrders` in `Protocol`, their checks, the rule in `ACP`, their
  resolution onto the type), `internal/emit` (the field, the decoder's key order, the encoder's ordered `properties`, and their
  helpers, emitted once when used), the tests of each, new engine cases.
* The regenerated `types_gen.go`; a root test of the field under both builds.
* `README.md`, "Wire encoding"; `docs/architecture.md`, the protocol table; the release notes,
  [0019-REPORT-release-notes-v1-15-0.md](../reports/0019-REPORT-release-notes-v1-15-0.md); these records, a dated entry in the 0019
  phase 3 PLAN, and their `docs/README.md` rows.

Out of scope: the order of any other map (`env`, `headers`, the elicitation answer `content`); a `Validate` check of `PropertyOrder`
against `Properties` (0022-MADR: a stale name is skipped, not refused); a helper that adds a property and its name together, which
can be added compatibly later; the macOS CI runner's download failure, which is GitHub's and is brought to the owner if it persists.

## Implementation Steps

Each step ends with its gates and an execution record; the owner commits it. Agents do not commit on `main`. Every new test is seen
to fail on a planted break in a scratch copy, never in the tree, before it is relied on.

### Step 1, records

This PLAN and its `docs/README.md` row, the MADR's row; a dated entry in
[0019-PLAN-generator-hardening.md](0019-PLAN-generator-hardening.md) recording that its close and the tag of `v1.15.0` wait on this
PLAN, so they follow the commit this PLAN verifies, not `3a84443`. When the owner approves this PLAN, the MADR's status becomes
`accepted`, dated, in the same records commit. Before step 2, a script asserts each code fact step 2 relies on, snippet at line.
Gates: `make check-records`, `make markdownlint`, the identifier scan.

### Step 2, the rule, the field and their code

Model, `cmd/generate/internal/model`:

* `protocol.go`: `Protocol.KeyOrders []OrderRule`, `OrderRule{Definition, Property, Field, Source string}`, documented as a typed
  map property whose key order the protocol gives meaning; `checkRules` refuses a rule without a definition, a property, a field or a
  source, a field that is not an exported identifier, and two rules for one property.
* `acp.go`: one rule, `ElicitationSchema` `properties`, field `PropertyOrder`, its source the reason the MADR gives (an elicitation
  schema is a form; GitHub issue #6).
* `model.go`, `resolveKeyOrders`, after `resolveNullRetained`: a rule whose definition or property the schema lacks is skipped; one on
  a property whose field is not a `RefMap`, or whose field name the struct already has, is refused; otherwise the type gains a
  `KeyOrder{Field, Map, Key}` (the order field's name, the map field's name, the wire key) on `Type.KeyOrders`.

Emitter, `cmd/generate/internal/emit`:

* The struct gains, after its fields, `PropertyOrder []string` tagged `json:"-"`, documented: the order of the keys of `Properties`,
  filled when decoded, used when encoded (names it holds first, in its order, then the others sorted; a name with no entry skipped);
  nil keeps the sorted order.
* `UnmarshalJSON`: the type decodes through `objectKeys` (it does today, for its defaults); after the decode, `a.PropertyOrder =
  keyOrder(m["properties"])`. `keyOrder(raw json.RawMessage) []string`, emitted once when used: nil for an empty raw value, `null`,
  or a value that is not an object; otherwise the object's keys in order, each once, by a `json.Decoder` token pass.
* `MarshalJSON`: when `len(v.PropertyOrder) == 0`, the method is unchanged, so the bytes are; otherwise it encodes the alias as
  today, decodes it into `map[string]json.RawMessage`, replaces `properties` with `orderedObject(a.Properties, v.PropertyOrder)`, and
  encodes the map, whose keys `encoding/json` sorts as the struct's fields are. `orderedObject[V any](m map[string]V, order []string)
  (json.RawMessage, error)`, emitted once when used, writes `{`, the named keys present in `m` in order and each once, then the
  others sorted, each `json.Marshal`ed key and value, and `}`.
* The struct type and the generated methods otherwise stay as they are.

Regenerate: `types_gen.go` changes in `ElicitationSchema` (the field, `MarshalJSON`, `UnmarshalJSON`) and gains `keyOrder` and
`orderedObject`; nothing else changes, which a scratch diff checks.

Tests:

* Root, `TestElicitationSchemaKeepsPropertyOrder` (`elicitation_order_test.go`, new, or beside the elicitation tests if one file
  holds them): decode and re-encode `name, email, age`, a reversed order, a duplicate key (first place kept, last value as today), an
  absent and a `null` `properties` (nil order, `{}` sent); an order set by hand that names a missing property and leaves one out;
  a `CreateElicitationRequest` round trip; and, for every case without an order, the bytes of the struct encoded before this step,
  kept as literal expectations. Run under both builds by the gates.
* Model: `TestKeyOrderRules` (the rule applies; a missing definition or property is skipped; each refusal of `checkRules` and of
  `resolveKeyOrders`); `TestProtocolTableAppliesToCommittedSchema` gains that the ACP rule gives `ElicitationSchema` its
  `PropertyOrder`.
* Engine cases: `accept-key-order` (a definition with a typed map and a rule, its `want.api`, and a behaviour test of decode,
  encode, a set order and an unset one); `refuse-key-order-not-map`; `refuse-key-order-field-taken`.
* Planted breaks: `keyOrder` returns nil (the order is not read); `orderedObject` sorts every key (the order is not written); the
  unset case goes through `orderedObject` with an empty order and a different layout (checked by the byte expectation); duplicate
  names written twice; a missing name not skipped; each refusal removed.

Documentation: `README.md`, "Wire encoding", a passage on `PropertyOrder`; `docs/architecture.md`, the protocol table's `KeyOrders`
and the two helpers.

### Step 3, release notes

[0019-REPORT-release-notes-v1-15-0.md](../reports/0019-REPORT-release-notes-v1-15-0.md): its preamble names this amendment and the
listing at the implemented commit; the release text's summary names issue #6 and recounts the API (three entries added, none
removed); "Additions" gains `ElicitationSchema.PropertyOrder`, what it holds both ways and how an agent sets it; "For the generator"
gains `KeyOrders`; "Sources not in the release text" gains the commits and the recounted listing difference, and this PLAN among the
execution records.

### Step 4, verification and close

The gates over the tree; issue #6's reproduction re-run under both builds (the agent's order comes back); the API listing against
`v1.14.0`; Windows. After the owner pushes and CI passes on the pushed commits, this PLAN and its row become `complete`; then the 0019
phase 3 PLAN closes on the same verified commit, the owner tags `v1.15.0` and publishes its release from the report, and closes issue
#6 naming the commit.

## Verification

Per code step, through the Python runner outside the tree: regenerate, and the diff of the generated files is exactly the step's;
`make pre-add-check`; `scripts/go-precheck.sh` on each new `.go` file; `GOEXPERIMENT=nojsonv2 go test ./...`; `GOOS=windows go vet
./...` in both modules; the generator suite under `-race`, engine behaviour cases included; each new test seen to fail on its planted
break in a scratch copy; `make check-records`, `make markdownlint`, the identifier scan; Windows `go test -count=1 -shuffle=on ./...`
in both modules; the API listing against `v1.14.0`.

Expected API listing difference against `v1.14.0`: phase 3's two lines and `field ElicitationSchema.PropertyOrder []string`; nothing
else.

Acceptance: every step's checks pass and its record is written; issue #6's reproduction gives the agent's order back under both
builds; a value without an order encodes byte for byte as before; the API listing matches; CI passes on the pushed commits.

## Rollout and Rollback

The change ships in `v1.15.0`, whose notes name it. Before the tag, each step is one commit, reverted by reverting it, and the 0019
phase 3 PLAN then closes on the commit before this PLAN's. After the tag, `PropertyOrder` stays within `v1`; it can be deprecated, not
removed.

## Execution record

### Step 1, records, 2026-10-15

The owner approved this PLAN on 2026-10-15 ("proceed"), and the MADR is `accepted`, choosing O1. Written: this PLAN,
`in-progress`, and the MADR's and its `docs/README.md` rows; a dated entry in
[0019-PLAN-generator-hardening.md](0019-PLAN-generator-hardening.md), "After step 8, 2026-10-15: the tag waits on 0022".

The code facts step 2 relies on, asserted by a Python script outside the tree, snippet at line, on `3a84443` (this step changes no
code): 14 of 14 hold, among them `NullRetained` in the table (`model/protocol.go:40`, `model/acp.go:101`) and its resolution
(`model/model.go:325`, `:1357`), the emitter's struct, codec and once-emitted helpers (`emit/types.go:201`, `:254`, `:296`, `:358`;
`emit/decode.go:31`), and the generated `ElicitationSchema`'s typed map, `MarshalJSON` and `objectKeys` (`types_gen.go:4950`,
`:4965`, `:4976`). The check was seen to fail on a planted break in a scratch copy: with `types_gen.go:4976` and
`emit/decode.go:31` changed, it named both and exited 1.

Gates, through the Python runner: `make check-records` and `make markdownlint` exit 0; the identifier scan finds nothing. No Go file
changed in this step.

### Deviation, 2026-10-15, during step 2

Found: step 2's code refuses two cases "Design"'s step 2 does not list: a rule whose property is not a field of an object
(`resolveKeyOrders` finds no field of the property on a `KindObject`; a union's own key is one), and a rule on a struct with flattened
members or kept extra keys, whose `MarshalJSON` ends in the members' merge or the `Extra` path, which the ordered encoding would have
to compose with. Decision (owner, 2026-10-15): keep both refusals, and record them here and in the MADR ("Amended while executing step
2, 2026-10-15"). No committed schema has either shape. `TestKeyOrderRules` pins both, and each was seen to fail on a planted break.

The owner committed step 1's record as `98d3e7c`.

### Step 2, the rule, the field and their code, 2026-10-15

What was built, on `98d3e7c`:

* Model. `Protocol.KeyOrders []OrderRule`, `OrderRule{Definition, Property, Field, Source}`, checked by `checkRules` (a definition, a
  property, a field and a source; an exported field name; no two rules for one property); `ACP` gains one rule, `ElicitationSchema`
  `properties`, field `PropertyOrder`, its source `srcFormOrder`. `resolveKeyOrders`, after `resolveNullRetained`, skips a rule whose
  definition or property the schema lacks, refuses one whose property is not a field of an object, is not a typed map, sits on a
  struct with flattened members or kept extra keys, or whose field name the struct has, and otherwise gives the type
  `KeyOrder{Field, Map, Key}` on `Type.KeyOrders`.
* Emitter. `emitStruct` and its codec take the type's key orders: the struct gains `PropertyOrder []string`, tagged `json:"-"` and
  documented; `UnmarshalJSON`, whose `objectKeys` map now always binds and folds the map's key, sets the order from
  `keyOrder(m["properties"])`; `MarshalJSON` (`orderedMarshal`) returns the alias's encoding as before when every order is empty,
  and otherwise encodes the alias, decodes it into `map[string]json.RawMessage`, writes each present map through `orderedObject`, and
  encodes the map. `keyOrder` (a `json.Decoder` token pass) and `orderedObject[V any]` are emitted once, when used.
* Regenerated: `types_gen.go` changes in `ElicitationSchema` alone (the field, `MarshalJSON`, `UnmarshalJSON`), and gains `keyOrder`
  and `orderedObject`; 82 lines added, 1 removed. No other generated file changes.
* Tests. Root, `TestElicitationSchemaKeepsPropertyOrder` (`elicitation_order_test.go`, new): the agent's order, a reversed one, a
  duplicate key (first place, last value), an absent and a `null` `properties`, an order set by hand that names a missing key, leaves
  one out and repeats one, no order, and a `CreateElicitationRequest` round trip, under both builds. Its expectations for no order
  are the bytes the code before this step gives: a probe on `git archive HEAD` under both builds printed exactly them. Model:
  `TestKeyOrderRules`; `checkCommittedKeyOrders`, called by `TestProtocolTableAppliesToCommittedSchema`, requires the committed
  `ElicitationSchema`, and no other type, to have its order. Engine cases: `accept-key-order` (its `want.api` written by `-update` and
  read; a behaviour test of decode, encode, a set order and none), `refuse-key-order-not-map`, `refuse-key-order-field-taken`.
* Documentation: `README.md`, "Wire encoding", a passage on `PropertyOrder`; `docs/architecture.md`, the protocol table's
  `KeyOrders` and the two helpers.

Issue #6's reproduction, on the tree: `name, email, age` is read as that order and sent back in it, alone and in a request.

Planted breaks, each in a fresh scratch copy (16 of 16 seen to fail):

| Break | What failed |
| :--- | :--- |
| the order not read (`a.PropertyOrder = nil`), at the root and through the engine | the root test: "PropertyOrder = [], want …"; `accept-key-order`: "PropertyOrder = []" |
| the order not written (no named key placed first), both | the root test's order cases; `accept-key-order`: sent sorted |
| a repeated name written twice, both | the root test: the request's encoding fails; `accept-key-order`: every key twice |
| a missing name not skipped, both | the root test; `accept-key-order`: `"gone":""` sent |
| the unset case through `orderedObject`, which drops unnamed keys | the root test's "no order" case: `properties` sent empty |
| each of the seven refusals removed | `TestKeyOrderRules`, and for the typed-map and field-taken refusals their engine cases |

The first version of the unset plant added whitespace to `orderedObject`'s output and passed: the outer `json.Marshal` compacts a
`json.RawMessage`, so that layout cannot reach the wire. It was replaced by the plant above, which the byte expectations catch.

Gates, through the Python runner, on the tree: regenerating changes no generated file; `make pre-add-check` exit 0, the new test file
among its files; `GOEXPERIMENT=nojsonv2 go test ./...` exit 0; `GOOS=windows go vet` exit 0 in both modules; `make check-records` and
`make markdownlint` exit 0; the identifier scan finds nothing; Windows `go test -count=1 -shuffle=on ./...` exit 0 at the root and in
`cmd/generate`. The API listing: 2426 lines at `v1.14.0`, 2429 in the tree; 0 removed and 3 added, exactly as expected:

```text
+ field ElicitationSchema.PropertyOrder []string
+ field UnstableMcpError.Extra map[string]json.RawMessage
+ method (UnstableMcpError) MarshalJSON () ([]byte, error)
```

The owner committed step 2's record as `561d12b`.

### Step 3, release notes, 2026-10-15

[0019-REPORT-release-notes-v1-15-0.md](../reports/0019-REPORT-release-notes-v1-15-0.md), amended on `561d12b`: its preamble names this
amendment and the listing at `561d12b` (2429 lines); the release text's summary names issue #6, says two types change, and counts
three API entries added and none removed; "Additions" gains `ElicitationSchema.PropertyOrder`, what it holds when decoded, how an
agent sets it, and that a form without an order is sent as before, and the README entry names all three passages; "For the
generator" gains `KeyOrders` and the two helpers; "Sources not in the release text" gains `561d12b` in both commit lists, the
generated diff against `v1.14.0` recounted (117 lines added, 3 removed in `types_gen.go`), the listing difference at `561d12b` (0
removed, 3 added), and this PLAN among the execution records.

Checked: `git log v1.14.0..HEAD -- '*.go' ':!cmd/generate'` lists `561d12b` and `131fa07`; `-- cmd/generate` lists `561d12b`,
`3704df9`, `852054f`, `8d2286a`, `1ee1eb7` and `131fa07`; `git diff --stat v1.14.0 -- types_gen.go` is 117 lines added and 3
removed; the API counts are step 2's gate's, on the same code.

Gates, through the Python runner: `make check-records` and `make markdownlint` exit 0; the identifier scan finds nothing. No Go file
changed in this step.
