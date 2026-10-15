---
status: accepted
date: 2026-10-15
decision-makers: Repository owner
---

# Keep the order of an elicitation form's properties before `v1.15.0`: a generated `PropertyOrder` beside the map

## Context and Problem Statement

GitHub issue #6, "ElicitationSchema loses the order of its properties: the agent's field order cannot be read or sent", opened
2026-10-15 and read with `gh issue view` that day, reports that `ElicitationSchema.Properties` is a
`map[string]ElicitationPropertySchema`: decoding an `elicitation/create` form discards the order in which the agent wrote its
fields, and encoding writes them sorted by name. A client cannot show the fields in the order the agent intended, and an agent that
builds a form with the SDK cannot choose the order its fields are sent in. The reporter, a downstream consumer that maps elicitation
forms onto its own question sheet, orders the fields as the `required` list and then the remaining names sorted, and records that
the agent's order is lost. The issue proposes a field that keeps the wire order beside the map.

The owner asked on 2026-10-15, before tagging `v1.15.0`
([0019-PLAN-generator-hardening.md](0019-PLAN-generator-hardening.md), step 8, whose CI on `3a84443` was then held by a macOS
runner that could not download its `actions/setup-go` action), for the issue to be investigated, and then decided that it is
resolved ahead of the tag. Which mechanism keeps the order is the decision this record asks for.

### What was established, 2026-10-15

* **It reproduces on the tree.** In a scratch copy of `3a84443`, under the default build and under `GOEXPERIMENT=nojsonv2`, with
  identical output: the schema sent as `{"type":"object","properties":{"name":…,"email":…,"age":…},"required":["email"]}`
  decodes into a map and is re-encoded as `{"properties":{"age":…,"email":…,"name":…},"required":["email"],"type":"object"}`,
  alone and inside a `CreateElicitationRequest`. `encoding/json` sorts the keys of a map it encodes, and a map keeps no order.
* **The schema says nothing about the order.** `ElicitationSchema.properties`, identical in `schema/schema.json` and
  `schema/schema.unstable.json`, is `{"type":"object","default":{},"additionalProperties":{"$ref":"#/$defs/ElicitationPropertySchema"}}`,
  described "Property definitions (must be primitive types)."; the definition calls itself "a JSON Schema object with
  primitive-typed properties, as required by the elicitation specification". ACP's elicitation page
  (`agentclientprotocol/agent-client-protocol` at `e5bcca615c2560a7d707365c773ae768ae595e88`, `docs/protocol/v1/elicitation.mdx`)
  does not mention order. RFC 8259 gives an object's member order no meaning, so keeping it is not a conformance requirement.
* **The Rust reference sorts too.** `agent-client-protocol-schema/src/v1/elicitation.rs:1142`, at the `schema-v1.25.0` tag's commit
  (`4cf3dd8`) and at `e5bcca6`, declares `pub properties: BTreeMap<String, ElicitationPropertySchema>`, which iterates in key order;
  the crate's `serde_json` `preserve_order` feature (`Cargo.toml:60-62`, "keeps caller-supplied object key order in arbitrary JSON
  values and maps") applies to `serde_json`'s own map, not to a `BTreeMap`. Upstream `main`, last changed in that file by `7628e1c` on
  2026-10-12, is the same. An agent written in TypeScript or Python, whose objects and dictionaries keep insertion order, sends its
  fields in the order it wrote them; one built on the Rust reference sends them sorted.
* **Only this map carries an order that means something.** The SDK's typed maps are `env` (two types), the elicitation answer
  `content` (two types), `headers`, and `properties`; the first three are keyed by name with no layout.
* **The generator owns the type's methods.** `ElicitationSchema` has a generated `MarshalJSON` (it fills a nil `Properties` with
  `{}`), `UnmarshalJSON` (defaults and lenient fields), and `Validate` (`types_gen.go`), so a hand-written method would collide; the
  generator must emit what keeps the order. Its `UnmarshalJSON` already holds the object's raw members by key (`objectKeys`), the
  `properties` member among them.

## Decision Drivers

* A form is laid out in the order its author wrote it: a client should be able to read that order, and an agent built on the SDK
  should be able to send one.
* The change is additive within `v1`: no exported name changes type or goes, and code that does not use the new field encodes
  exactly the bytes it encodes today.
* Both JSON builds give the same result, as every wire behaviour of the SDK does
  ([0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md), D7).
* The generator stays the single source of the generated files, and a protocol fact the schema states only in prose, or not at all,
  is named in the protocol table, as `PatchFields` and `NullRetained` are
  ([0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md](0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md),
  [0019-PLAN-generator-hardening.md](0019-PLAN-generator-hardening.md), G3).
* `v1.15.0` is the release that carries it: its notes name it, and the tag waits for it.

## Considered Options

* O1. A generated sibling field, `PropertyOrder []string`, named in the protocol table
* O2. A method over an unexported order
* O3. An ordered type for `Properties`
* O4. Keep the map, and document that the order is lost

## Decision Outcome

Chosen option: "O1. A generated sibling field, `PropertyOrder []string`, named in the protocol table", because it is the only
option that lets a client read the order and an agent set it, without changing an exported type, and it reuses the table and the
decoder's raw members the generator already has.

* **The table.** The protocol table gains `KeyOrders []OrderRule`, `OrderRule{Definition, Property, Field, Source string}`: a typed
  map property whose key order the protocol gives meaning, the Go name of the field that holds that order, and the reason. `ACP` has
  one rule: `ElicitationSchema` `properties`, field `PropertyOrder`, its source GitHub issue #6 and the definition's own description
  of a form. A rule whose definition or property the schema lacks is skipped; one on a property that is not a typed map, one whose
  field would collide with a field of the struct, and two rules for one property, are refused.
* **The field.** `PropertyOrder []string`, tagged `json:"-"`, documented as the order of the keys of `Properties`: filled on
  decode, used on encode, and settable by an agent that builds a form.
* **Decoding.** When the `properties` member is an object, `PropertyOrder` holds its keys in the order they appear, each once at its
  first appearance; when it is absent or `null`, `PropertyOrder` is nil. Nothing else of the decoding changes.
* **Encoding.** When `PropertyOrder` is empty, the struct encodes exactly as today: `properties` sorted by key. Otherwise
  `properties` is written with the keys `PropertyOrder` names that `Properties` holds, in that order and each once, then the keys it
  does not name, sorted. A name with no property is skipped. The other members keep their place.
* **Validation.** `Validate` is unchanged; a name in `PropertyOrder` with no property is not an error, so a stale order never
  refuses a form.
* **Release.** `v1.15.0` carries it; its notes add the field to "Additions" and name issue #6, which the owner closes when the
  release is published.

### Amended while executing step 2, 2026-10-15

Two more refusals, which the outcome above does not list, decided by the owner on 2026-10-15 when step 2 found them: a rule whose
property is not a field of an object (a union's own key, for one), and a rule on a struct with flattened members or kept extra keys,
whose `MarshalJSON` already ends in a path of its own (the members' merge, or `Extra`) that the ordered encoding would have to
compose with. No committed schema has either shape; the one rule is on `ElicitationSchema`, a plain object.

### Consequences

* Good, because a client can show a form's fields in the order the agent wrote them, and an agent can send them in the order it
  chooses.
* Good, because the exported API only grows, by one field, and a program that never sets it encodes the same bytes as before.
* Good, because the rule and its refusals live in the generator's table, as the other protocol facts the schema does not state.
* Neutral, because an agent built on the Rust reference still sends its fields sorted: the order the SDK keeps is the one on the
  wire.
* Bad, because `PropertyOrder` and `Properties` can disagree: a caller that adds a property and not its name gets that property
  after the named ones, sorted, and one that removes a property leaves a name that is skipped.
* Bad, because a decoded `ElicitationSchema` now carries a slice that compares in `reflect.DeepEqual` and test assertions; a test
  that builds the expected value by hand must set it, or compare the wire bytes.
* Bad, because the tag of `v1.15.0` waits for this record's PLAN.

### Confirmation

* Issue #6's reproduction, re-run on the implemented tree under both builds, gives the agent's order back when re-encoded, and the
  struct's other bytes are unchanged.
* A root test decodes and re-encodes forms with orders that differ from sorted, a duplicate key, an absent and a `null`
  `properties`, and an order that names a missing property and leaves one out, under both builds; and a value whose `PropertyOrder`
  is empty encodes byte for byte as before.
* An engine case pins the table rule's field, decoding and encoding through the generated code, and refuse cases pin each refusal;
  each test is seen to fail on a planted break.
* The API listing against `v1.14.0` adds exactly `field ElicitationSchema.PropertyOrder []string` to phase 3's two lines.

## Pros and Cons of the Options

### O1. A generated sibling field, `PropertyOrder []string`, named in the protocol table

* Good, because an agent sets the order with a struct literal or an assignment, and a client reads it as a slice.
* Good, because it is additive and leaves the bytes of every existing caller's output as they are.
* Good, because the decoder already holds the `properties` member's raw bytes, so reading its key order costs one token pass over
  bytes it has.
* Bad, because the two fields can disagree, which the encoding resolves by a stated rule rather than by construction.

### O2. A method over an unexported order

`OrderedProperties() []string` over an order the decoder keeps in an unexported field, as the issue suggests in passing.

* Good, because the exported struct does not grow a field.
* Bad, because an agent cannot set the order with a struct literal, so the agent side of the issue stays unsolved without a second
  method, a setter, which is the field with more ceremony.
* Bad, because an unexported field in a generated struct is a new kind of field for the generator, unlike `Extra`.

### O3. An ordered type for `Properties`

A slice of key and schema pairs, or an ordered-map type.

* Good, because order and content cannot disagree.
* Bad, because it changes the type of an exported field, which within `v1` needs a module path ending in `/v2`
  ([0020-MADR-retract-accidental-v1-tag.md](0020-MADR-retract-accidental-v1-tag.md)).
* Bad, because every reader of `Properties` by key, the SDK's `Validate` among them, changes.

### O4. Keep the map, and document that the order is lost

* Good, because nothing changes, and the SDK matches the Rust reference.
* Bad, because a client still cannot recover what a TypeScript or Python agent wrote, and an agent still cannot choose its order;
  the downstream consumer keeps its workaround.

## More Information

* **Evidence.** Issue #6, read with `gh issue view 6` on 2026-10-15. The reproduction, a probe test in a scratch copy of `3a84443`
  run with `go test -run TestProbeIssue6` under both builds. The Rust reference's `elicitation.rs` and `Cargo.toml`, read with
  `gh api` at `4cf3dd8`, at `e5bcca6`, and on `main`; ACP's `elicitation.mdx` at `e5bcca6`, searched for "order" with no match.
* **Related records.** [0019-PLAN-generator-hardening.md](0019-PLAN-generator-hardening.md), whose close and the tag of `v1.15.0`
  wait on this record's PLAN; [0019-REPORT-release-notes-v1-15-0.md](../reports/0019-REPORT-release-notes-v1-15-0.md), which gains
  the addition; [0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md](0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md), the
  precedent of issues resolved before a tag.
* **Implementation.** [0022-PLAN-keep-elicitation-property-order-before-v1-15-0.md](0022-PLAN-keep-elicitation-property-order-before-v1-15-0.md).
