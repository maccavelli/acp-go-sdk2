---
status: in-progress
date: 2026-11-10
associated-madr: "0054-MADR-adopt-acp-schema-1-31-0-and-client-system-prompts.md"
---

# Implement 0054: ACP schema 1.31.0, and the client system prompt gated and checked

Associated MADR: [0054-MADR-adopt-acp-schema-1-31-0-and-client-system-prompts.md](0054-MADR-adopt-acp-schema-1-31-0-and-client-system-prompts.md)

## Goal

Ship the MADR's O1 as `v1.36.0`. At the end of it:

* `schema/version` is `1.31.0`, and the four schema files are the release assets byte for byte (D1).
* The generator models `x-deserialize-default-on-null` (D2) and `proxyMethods` (D3). It reads `1.31.0` with no edit to the
  files.
* `NewSessionRequest`, `LoadSessionRequest` and `ResumeSessionRequest` carry `SystemPrompt *SystemPrompt`, and
  `SessionCapabilities` carries `SystemPrompt *SessionSystemPromptCapabilities` (D4).
* Four capability rules, R1 to R4, and one validation rule are enforced on send and on receipt, and through
  `Negotiation.AllowsValue` (D5, D6). They refuse:
  * an unadvertised prompt;
  * `override` without `mode.override`;
  * a mode the schema does not list;
  * a content block other than `text`;
  * empty content.
* `make api` against `v1.35.0` reports no incompatible change in a gated package (D9).
* The docs, the consumer check and the release notes describe it, and gobble-cli builds against it unchanged.

## Scope

**Preconditions.**

* The MADR is `accepted`, with its questions 1 to 6 answered as recommended (2026-11-10). The owner approves this PLAN.
* The untracked `0053-REPORT-acp-1-25-0-completeness-parity-and-adherence.md` and its two `docs/README.md` rows are
  committed first. They are not this PLAN's, and the pair's commit must not carry them unreviewed.
* The MADR, this PLAN and their `docs/README.md` rows are committed alone, as the bootstrap exception allows. That commit
  is the base.
* Before phase 1, `git status --short` shows the base clean, and `make test` passes on it.

**Tools.** The checks used during planning:

* `/usr/bin/make` is GNU Make 3.81, which cannot read `schema/version` (`Makefile:37`-`:46`). `gmake` is GNU Make 4.4.1
  (`/opt/homebrew/bin/gmake`). Every target that reads `ACP_VERSION` runs as `gmake`.
* `mdsh` is not installed. `version` depends on `README.md`, whose recipe runs `$(MDSH) --input README.md`
  (`Makefile:214`-`:216`). README holds no mdsh directive: no `BEGIN mdsh`, no `` `$ `` command and no `<!-- $` block.
  Phase 3 therefore runs `gmake version MDSH=true`, the override `RELEASING.md:17`-`:18` documents. README must then be
  unchanged (`git diff --exit-code README.md`). If it is not, stop.

**Files changed.** By phase:

| Phase | Files |
| :--- | :--- |
| 1, the two keys (D2, D3) | `cmd/generate/internal/load/load.go`, `merge.go`, `manifest.go`, `load_test.go`; `cmd/generate/internal/model/keywords.go`, `schema.go`, `fields.go`, `schema_test.go`; `cmd/generate/testdata/engine/` new cases. *Deviation 1 (2026-11-10) dropped `load/unstable.go`, `unstable_test.go`, `model/methods.go`, `model.go`, `emit/constants.go` and `emit/unstable.go`: the proxy methods emit nothing.* |
| 2, the gate kinds and the validation list (D5, D6 machinery) | `cmd/generate/internal/model/protocol.go`, `gates.go`, `model.go`, `validate.go`, `gates_test.go`, `protocol_test.go`; `cmd/generate/internal/emit/gates.go`, `types.go`, `types_test.go`; `cmd/generate/main_test.go` (the rule stub, if its type check needs `RuleMinItems`); `cmd/generate/testdata/engine/` new cases |
| 3, the pin and the rules (D1, D4, D5, D6, D8) | `schema/version`, `version`, `schema/meta.json`, `schema/schema.json`, `schema/meta.unstable.json`, `schema/schema.unstable.json`; `cmd/generate/internal/model/acp.go`, `gates_test.go`, `sources_test.go`; `cmd/generate/main_test.go`; the generated `constants_gen.go`, `types_gen.go`, `agent_gen.go`, `client_gen.go`, `helpers_gen.go`, `schema_gen_test.go`, `unstable/unstable_gen.go` (new); `testdata/encodings.golden`; `system_prompt_test.go` (new); `default_on_null_test.go` (new) |
| 4, the docs (D8) | `README.md`; `docs/architecture.md`; `doc.go` only if it lists the capabilities |
| 5, the close (D9) | `tools/consumer.py`, `tools/test_tools.py`, `tools/README.md`; `docs/reports/0054-REPORT-release-notes-v1-36-0.md` (new); this PLAN; the MADR; `docs/README.md` |

Each phase also updates this PLAN's execution record and its row in `docs/README.md`. A file a phase needs that is not in
its row is a deviation: stop, record it, and ask.

**Not changed:**

* `go.mod`, `go.sum` and `cmd/generate/go.mod`, so no dependency changes;
* `.github/`;
* `schema/unstable-v1.json`, the frozen manifest (`RELEASING.md:79`-`:84`);
* every existing exported signature;
* what the SDK sends or accepts for a message without `systemPrompt`.

**Out of scope:**

* gobble-cli's and magic-cli-remote's adoption (the MADR's gobble notes are informational);
* the empty `prompt` array on `session/prompt` (MADR D6);
* `session/fork`'s `systemPrompt` (MADR D4);
* a runtime for the new unstable methods (`session/status`, `auth/status`, the proxy methods and the others), whose
  types alone reach `unstable`;
* helpers (MADR D7);
* [0048-MADR-create-releases-in-ci.md](0048-MADR-create-releases-in-ci.md).

## Implementation Steps

**Planted breaks.** Every new test or check is first seen to fail on a planted break in a scratch copy, never in the tree:

* **Root module:** a plant that edits a hand-written file runs through `python3 tools/plants.py SPEC.json`, one spec per
  phase. The spec is kept in the scratch directory and not committed.
* **Generator, or anything that needs a regeneration:** run a scratch script in a `git archive` copy. It applies the
  edit, runs `cd cmd/generate && env -u GOPATH -u GOMODCACHE go run .`, then runs the named tests, as
  [0050-PLAN-auth-method-ids-and-the-prompt-result-for-issue-14.md](0050-PLAN-auth-method-ids-and-the-prompt-result-for-issue-14.md)
  did. `TestEngineCases` runs whole in such a plant, not filtered.
* **The record:** for each plant, the execution record gives the edit, the command, its exit status and the failing
  test's line.

**End of each phase.**

1. Run `make pre-add-check FILES="<the phase's files>"`. Redirect its output to a scratch file and capture its exit status
   before any filter.
2. Run `make check-records`, `make markdownlint` and `make disclosure-check`. Run `make check-tools` too when `tools/` or
   `scripts/` changed.
3. Write the execution record: what was done, each command with its exit status, each plant, and every deviation, dated.
4. Stage what passed. On `main` the agent commits only with the owner's explicit permission in that turn (`AGENTS.md`,
   "Commits"), using `git commit --no-edit`; otherwise the owner commits. The next phase starts from that commit.

### Phase 1: the two keys, D2 and D3

The schema stays at `1.25.0` in this phase, so the generated files must not change by a byte. `TestGeneratedFilesUpToDate`
passing and `git status` naming no `*_gen.go` file are this phase's proof.

1. **`x-deserialize-default-on-null` in the loader.** `load.Definition` (`load.go:77`-`:78`) gains
   `XDeserializeDefaultOnNull bool \`json:"x-deserialize-default-on-null"\``. Its comment says a decoder reads `null` as
   the empty value, and cites the MADR, D2. The strict decode (`load.go:99`-`:101`, `:115`) then accepts the keyword.
2. **Its treatment.** `model/keywords.go` gains `"x-deserialize-default-on-null": TreatDecode` beside the other two
   `x-deserialize` entries (`:48`-`:50`). `TestKeywordRegistryMatchesLoader` (`keywords_test.go:18`) keeps the table and the
   loader in step.
3. **Its check.** The keyword list at `model/schema.go:322` gains the keyword. Beside the
   `x-deserialize-skip-invalid-items` check (`:476`-`:477`), the generator refuses the keyword on a node whose primary type is
   not an array or an object, with "x-deserialize-default-on-null on %s, not an array or an object".
4. **Its record in the model.** `model.Field` gains `DefaultOnNull bool` beside `DefaultOnError` and `SkipInvalidItems`
   (`fields.go:113`-`:115`), set from the property (`:621`-`:622`). The emitter does not read it in this phase. MADR F11
   showed the decoders already read `null` as empty at all 14 properties, and the engine case in step 8 pins that.
5. **`proxyMethods` in the loader.** `load.Meta` (`load.go:19`-`:25`) gains
   `ProxyMethods map[string]string \`json:"proxyMethods,omitempty"\``.
   * In `MergeStableAndUnstable` (`merge.go:90`-`:96`) and `MergeManifest` (`manifest.go:139`-`:144`), a stable
     `meta.json` that holds proxy methods is refused: "proxy methods in the stable schema: the SDK dispatches only agent,
     client and protocol methods". Upstream's stable `meta.json` holds none.
   * ~~The live unstable meta's proxy methods are new unstable methods by 0039's rule. `NewUnstable` copies them into the
     meta of package `unstable`.~~ *Deviation 1 (2026-11-10): the proxy methods are merged into no side, so none of
     their definitions is reached and `unstable` holds no proxy name. `Meta.ProxyMethods`' doc comment says so and cites
     the MADR's amendment.*
6. ~~**The proxy side in the model of `unstable`.** A `"proxy"` side in `model/methods.go`'s side table, `ProxyMethod<Name>`
   constants, and the five linked definitions listed for G18.~~ *Dropped by deviation 1: the generator cannot type them
   (alias definitions, a dual-role wire, a binding-less side; the MADR's amendment).*
7. **Stop condition (MADR D3).** *Met on 2026-11-10 before any edit; deviation 1 records it, and the MADR is amended.*
8. **Engine cases** (`cmd/generate/testdata/engine/`):
   * `accept-default-on-null.txtar`: an object with a required array, an optional array and an optional map, each with
     the keyword. Its `behaviour_test.go` decodes `null` into each with no error, requires `Validate` to pass, and
     requires the re-encoding to hold `[]` for the required array and to omit the optional ones.
   * `refuse-default-on-null-scalar.txtar`: the keyword on a string. `want.err` is step 3's message.
   * `accept-unstable-proxy-methods-omitted.txtar`, *renamed by deviation 1*: a manifest and a live unstable meta with
     `proxyMethods` and the 1.31.0 proxy shapes: an `allOf` alias definition, and a wire with a request, a response and a
     notification. Generation succeeds. `want.api` (acp) is the same as without them, and `want.api.unstable` holds no
     proxy name.
   * `refuse-stable-proxy-methods.txtar`: a stable `meta.json` with `proxyMethods`. `want.err` is step 5's message.
9. **Unit tests.** `load_test.go` decodes both keys. *Deviation 1: `unstable_test.go` is not changed; the engine case
   carries the omission.*
10. **Plants:**
    * the `keywords.go` entry removed fails `TestKeywordRegistryMatchesLoader`;
    * step 3's check removed fails `refuse-default-on-null-scalar`;
    * *(deviation 1)* `MergeStableAndUnstable` made to merge the live proxy methods into the agent side fails
      `accept-unstable-proxy-methods-omitted`, on the alias or the dual-role refusal;
    * the stable refusal removed fails `refuse-stable-proxy-methods`;
    * the emitter's nil-to-`[]` encoding of a required array, at the site the phase finds and records, made to keep `null`,
      fails `accept-default-on-null`.

### Phase 2: the gate kinds and the validation list, D5 and D6

The schema stays at `1.25.0`, and `model/acp.go` gains no rule, so again no generated file changes.

1. **Four kinds** in `model/protocol.go`'s `GateKind` (`:135`-`:161`), each with a doc comment stating what it checks, the
   fields its rule takes, and the MADR, D5:

   | Kind | Checks | Rule takes |
   | :--- | :--- | :--- |
   | `GatePresent` | Property, an optional (pointer) property, unset unless Capability is advertised | Property, Capability; no Variant |
   | `GateValue` | Property, an enum, not holding Variant unless Capability is advertised | Property, Variant (a value the enum lists), Capability |
   | `GateUndeclaredValue` | Property, an open enum, holding only a value the enum lists | Property, Capability (the prefix the refusal names); no Variant |
   | `GateOnlyVariants` | Property, a union or an array of one, holding only the variant Variant | Property, Variant (the allowed variant), Capability (the prefix the refusal names) |

   `checkCapabilityRules` (`:511`-`:545`) gains a case for each, with the missing-field test the table states.
2. **Their resolution** in `model/gates.go`:
   * Four `GateCheckKind`s (`:23`-`:38`).
   * Cases in `gateCheck` (`:305`-`:337`):
     * present: the field's type must be `RefPointer`;
     * value: the field's type must be a `KindEnum` that lists Variant (`model.go:165`, `EnumValue` at `:266`);
     * undeclared value: the field's type must be a `KindEnum`; the check records its values and constant names in a new
       `GateCheck.Values`;
     * only variants: `ruleUnion` (`:358`-`:371`) and `variantOf` (`:395`-`:405`) find the union and the allowed variant.
       The union must be discriminated, so a refusal can name the block's type, read with `unknownKind` for `Unknown`.
   * Every rule's subject keeps the existing refusal of a property in copies the rule cannot reach (`:281`-`:302`).
   * Capabilities, in the switch at `:338`-`:351`:
     * present and value resolve their capability with `capabilitySteps` (`protocol.go:917`);
     * undeclared value requires its prefix to resolve with `capabilityPath` (`:932`), as `GateUndeclaredVariant` does;
     * only variants requires every segment but the last to resolve. Its last segment names the property a future
       sub-capability will sit under: `sessionCapabilities.systemPrompt.content`, where `content` is no capability yet
       (the RFD, L305).
3. **Their code** in `emit/gates.go`'s `gateStmt` (`:147`-`:191`), with `markGateHelpers` (`:59`-`:73`) marking
   `unknownKind` for only-variants. The generated shapes, for a receiver `v`:
   * present:
     `if v.F != nil && <capability missing> { return &CapabilityError{Method: method, Capability: "<cap>", Field: "<key>"} }`
   * value: `if v.F == <Const> && <capability missing> { return &CapabilityError{…, Field: "<key>", Variant: "<value>"} }`
   * undeclared value:
     `if v.F != <Const1> && v.F != <Const2> { return &CapabilityError{…, Capability: "<prefix>." + string(v.F), Field: "<key>", Variant: string(v.F)} }`.
     `Field` is always set, so a received one is answered `-32602`, not `-32601` (`connection.go:3337`).
   * only variants, for each item `i` whose allowed variant is nil: the item's discriminator value is taken from its set
     variant, or with `unknownKind` from `Unknown`. It returns
     `&CapabilityError{…, Capability: "<prefix>." + kind, Field: fmt.Sprintf("<key>[%d]", i), Variant: kind}`.
4. **The validation list.**
   * `Protocol` gains `NonEmptyArrays []PathRule`, checked in `Build` beside the absolute-path rules (`protocol.go:417`-`:426`):
     definition, property and source are required, and two rules may not name one property.
   * `model.go` resolves each rule as it resolves `AbsolutePaths` (`:1420`-`:1434`). A rule whose property is not an
     array is refused.
   * `emit/types.go`, beside the absolute-path checks (`:1449`-`:1458`), emits
     `if len(v.F) == 0 { return <ValidationError Type, Field: "<key>", Rule: RuleMinItems, "<Type>: <key>: must not be empty"> }`.
   * The check runs wherever `validate(how)` runs, on send and on receipt (`connection.go:3306`-`:3308`), and is not tied
     to the `closed` flag that holds back closed enumerations on receipt.
   * `RuleMinItems` already exists in package `acp`. The stub rule constants of `cmd/generate/main_test.go:220` and
     `emit/types_test.go:276` gain it if the generator's type check needs them.
5. **Engine cases**, each with a `behaviour_test.go` covering a typed send (a `*CapabilityError` with the capability,
   field and variant, and nothing written), a receipt (answered `-32602`, handler not called) and an advertised send
   (written):
   * `accept-capability-present.txtar`;
   * `accept-capability-value.txtar`;
   * `accept-capability-undeclared-value.txtar`, with `""`, a `_`-prefixed value and an unlisted one;
   * `accept-capability-only-variants.txtar`, with a listed variant, another declared one and an unknown type;
   * `accept-non-empty-array.txtar`: `Validate` returns `RuleMinItems` for `[]`, a send is refused, a receipt is answered
     `-32602`, and an item is accepted.

   Refusal cases:
   * `refuse-capability-present-not-pointer.txtar`
   * `refuse-capability-value-not-enum.txtar`
   * `refuse-capability-value-unlisted.txtar`
   * `refuse-capability-undeclared-value-not-enum.txtar`
   * `refuse-capability-only-variants-not-union.txtar`
   * `refuse-capability-only-variants-unknown-variant.txtar`
   * `refuse-non-empty-not-array.txtar`
6. **Model tests.** `model/gates_test.go`'s rule-misuse table (`:162`-`:173`) gains a row per kind for a missing and an
   extra field. `protocol_test.go` covers the new list's checks.
7. **Plants**, each in a `git archive` copy:
   * each emitted condition inverted (`!= nil` to `== nil`, `==` to `!=`, one `&&` dropped from the undeclared value's
     list, the only-variants nil check negated) fails its engine case;
   * the non-empty emission removed fails `accept-non-empty-array`;
   * each `checkCapabilityRules` condition removed fails its model test row.

### Phase 3: the pin and the rules, D1, D4, D5, D6 and D8

1. **The pin.** Run `printf '1.31.0\n' > schema/version`, then `gmake version MDSH=true`, with its output to a scratch file
   and its exit status captured. It downloads the four assets and regenerates. Any refusal by the generator is a
   deviation: stop.
2. **The files as downloaded.** `shasum -a 256 schema/meta.json schema/meta.unstable.json schema/schema.json
   schema/schema.unstable.json` must print MADR D1's four values. `cmp -s schema/version version` must pass.
   `git diff --exit-code schema/unstable-v1.json README.md` must pass.
3. **The rules** in `model/acp.go`'s `CapabilityRules` (`:202`-`:238`), after the existing ones, with constants for the
   capability and the sources beside `:323`-`:356`:

   ```go
   // capSystemPrompt is the agent's capability for a client system prompt
   // (0054-MADR-adopt-acp-schema-1-31-0-and-client-system-prompts.md, D5).
   capSystemPrompt = "sessionCapabilities.systemPrompt"

   {Side: sideAgent, Definition: defNewSession, Property: keySystemPrompt, Capability: capSystemPrompt, Source: srcSystemPromptGate, Kind: GatePresent},
   {Side: sideAgent, Definition: defLoadSession, Property: keySystemPrompt, Capability: capSystemPrompt, Source: srcSystemPromptGate, Kind: GatePresent},
   {Side: sideAgent, Definition: defResumeSession, Property: keySystemPrompt, Capability: capSystemPrompt, Source: srcSystemPromptGate, Kind: GatePresent},
   {Side: sideAgent, Definition: defSystemPrompt, Property: "mode", Variant: "override", Capability: capSystemPrompt + ".mode.override", Source: srcOverrideGate, Kind: GateValue},
   {Side: sideAgent, Definition: defSystemPrompt, Property: "mode", Capability: capSystemPrompt + ".mode", Source: srcUnsupportedMode, Kind: GateUndeclaredValue},
   {Side: sideAgent, Definition: defSystemPrompt, Property: "content", Variant: "text", Capability: capSystemPrompt + ".content", Source: srcTextOnly, Kind: GateOnlyVariants},
   ```

   The sources, each a sentence of the `1.31.0` schema, whitespace collapsed as `TestCapabilitySourcesQuoteTheSchema`
   compares (`sources_test.go:23`-`:26`):
   * `srcSystemPromptGate`: "Clients MUST only send this when the Agent advertises `sessionCapabilities.systemPrompt`."
     (`SystemPrompt`'s description);
   * `srcOverrideGate`: "MUST only use it when the Agent advertises `sessionCapabilities.systemPrompt.mode.override`."
     (`SystemPromptMode`'s `override`);
   * `srcUnsupportedMode`: "An Agent MUST return an error for a mode it does not support rather than treating it as
     `append` or `override`." (`SystemPromptMode`);
   * `srcTextOnly`: "Clients MUST NOT send other content block types until a future capability allows them"
     (`SystemPrompt.content`).

   `NonEmptyArrays` gains `{Definition: defSystemPrompt, Property: "content", Source: srcEmptyContent}`, where
   `srcEmptyContent` is "Clients MUST NOT send an empty array." The constants `defSystemPrompt = "SystemPrompt"` and
   `keySystemPrompt = "systemPrompt"` join `:242`-`:272`.
4. **Regenerate:** `cd cmd/generate && env -u GOPATH -u GOMODCACHE go run .`, then `gofmt -w . ./cmd/generate`. Read the
   generated `gates` methods of the three requests and of `SystemPrompt`, and `SystemPrompt`'s `validate`. Quote them in the
   record.
5. **The generator's tests that move (D8):**
   * `cmd/generate/main_test.go:26`-`:33`: the generated files gain `unstable/unstable_gen.go`.
   * `model/sources_test.go`: `committedDescriptions` (`:29`-`:61`) also walks the `$defs` of `schema/unstable-v1.json`,
     whose fragments hold the frozen rules' sentences. `TestCapabilitySourcesQuoteTheSchema` (`:68`) also requires each
     `NonEmptyArrays` source in a description.
   * `model/gates_test.go:230`-`:231`: the count is predicted to become 36 rules resolving 39 checks, from 30 and 33. Six
     rules are added, and each is predicted to resolve on one type: `ruleTypes` (`gates.go:208`-`:219`) adds an unstable
     copy only under another name, and no copy of `SystemPrompt` or `UnstableSystemPrompt` exists in `acp` (MADR F9).
     The actual count is recorded and explained before it is written into the test.
6. **The golden.** Run `go test -run TestEncodingsUnchanged -update .`. A scratch script then reads the diff of
   `testdata/encodings.golden` and requires:
   * no removed line except the six instances of `NewSessionRequest`, `LoadSessionRequest` and `ResumeSessionRequest`;
   * each of those six replaced by the same line with a `systemPrompt` key added;
   * every added line belonging to one of the five new types (MADR F10: 21 lines).
7. **`system_prompt_test.go`**, at the root, on `acptest` pairs and the raw wire as `gates_test.go` uses them:
   * `TestSystemPromptGated`: on `session/new`, `session/load` and `session/resume`, without the capability:
     * a typed send returns a `*CapabilityError` with `Capability` `sessionCapabilities.systemPrompt` and `Field`
       `systemPrompt`, and writes nothing;
     * a received one is answered `-32602`, and the handler is not called.

     With the capability, the prompt is written and the handler receives it intact.
   * `TestSystemPromptOverrideGated`: `override` is refused both ways without `mode.override` (capability
     `…systemPrompt.mode.override`, variant `override`). `append` passes with `systemPrompt: {}`.
   * `TestSystemPromptUndeclaredModes`: `"_vendor"`, `"later"` and `""` are refused both ways with every capability
     advertised. A raw `"mode": null` is answered `-32602`.
   * `TestSystemPromptTextOnly`: `image`, `audio`, `resource_link`, `resource` and an unknown type are each refused both
     ways, the field `content[i]` and the variant naming the type. `text` passes.
   * `TestSystemPromptEmptyContent`: `Validate` returns a `*ValidationError` with `RuleMinItems`. A send is refused, and a
     receipt is answered `-32602`.
   * `TestSystemPromptInboundValidationOptIn`: an agent whose `WithInboundValidation` accepts the `*CapabilityError` for
     `"_vendor"` receives that prompt in its handler. The hook sees the gate's error.
   * `TestSystemPromptAllowsValue`: `Negotiation.AllowsValue` returns each refusal above, and nil when advertised.
8. **`default_on_null_test.go`**: `TestDefaultOnNullLists` reads `schema/schema.json` and finds every property with
   `x-deserialize-default-on-null`; it requires 14. For each, it builds a valid instance of the definition with
   `instances_test.go`'s builder, sets the property to `null`, and decodes it into the registered type
   (`schema_gen_test.go`). It requires no error, `Validate` nil, and a re-encoding that holds `[]` (or `{}`) for a
   required property and omits an optional one.
9. **Plants:**
   * `committedDescriptions` without the manifest fails `TestCapabilitySourcesQuoteTheSchema` on the three NES rules;
   * each R rule removed from `acp.go` fails its test in `system_prompt_test.go`, after regeneration in the copy;
   * the `NonEmptyArrays` entry removed fails `TestSystemPromptEmptyContent`;
   * one generated decoder (`McpServerStdio`'s, in the copy) made to refuse a `null` `args` fails `TestDefaultOnNullLists`;
   * the golden script fed a diff with a removed line of another type fails.
10. **`make api`** against `v1.35.0`, with its output to the record:
    * no incompatible change in a gated package;
    * in `acp`, exactly the eleven additions of MADR F9: `SystemPrompt`, `SystemPromptMode`, `SystemPromptModeAppend`,
      `SystemPromptModeOverride`, `SessionSystemPromptCapabilities`, `SessionSystemPromptModeCapabilities`,
      `SessionSystemPromptOverrideCapabilities`, and the four `SystemPrompt` fields;
    * in `unstable`, F9's 46 additions and no proxy name (deviation 1), each listed in the record.

    A gate adds no exported name.
11. **Flakiness:** `go test -race -count=20 -run 'SystemPrompt|DefaultOnNull' .`, recorded.

### Phase 4: the docs, D8

* **`README.md`**, "Initialization and capabilities" (`:527`): a new `#### System prompts` covering:
  * the agent's capability (`SessionCapabilities.SystemPrompt`, and `Mode.Override` for `override`);
  * the client's prompt on new, load and resume, and that the SDK refuses to send one the agent did not advertise;
  * the receipt side: `-32602` before the handler, for an unadvertised prompt or mode, a non-text block or empty content;
  * `WithInboundValidation` to accept a `_` mode or empty content;
  * the book's re-send on load and resume (L400);
  * `AllowsValue` for a dry run.

  Its code, if any, is a fenced block outside the `<!-- example -->` markers that `docs_test.go` reads.
* **`docs/architecture.md`:**
  * `:978` ("At `1.25.0`, `unstable/` holds only `doc.go`") becomes what `unstable/unstable_gen.go` holds at `1.31.0`;
  * the merge, manifest and unstable section (`:915`-`:978`) names `proxyMethods` and the proxy side;
  * the keyword paragraph names `x-deserialize-default-on-null`;
  * the capability-gate paragraph names the four new kinds and `NonEmptyArrays`, found by searching for
    `GateUndeclaredVariant`.
* **`doc.go`:** a sentence on the system prompt if it lists the capabilities. Otherwise nothing, and the record says so.

### Phase 5: the close, D9

1. **`tools/consumer.py`:** the consumer module's floor becomes `v1.36.0`. On an `acptest` pair whose agent advertises
   `SystemPrompt: {}`, it:
   * sends an `append` prompt, which the agent receives;
   * gets a `*CapabilityError` from an `override` one;
   * checks `AllowsValue`.

   `tools/test_tools.py` and `tools/README.md`'s table follow. Run `make check-tools`.
2. **The gates:**
   * `python3 tools/gates.py` and `python3 tools/floor.py`;
   * `python3 tools/remote.py tree --host linux` and `--host windows`, the copy made without macOS `._` files;
   * `python3 tools/interop.py` when `node`, `npm`, `npx` and `uv` are present, else recorded as not run, with the reason.
     `remote` did not change, so this is a regression check.

   Each output goes in the record.
3. **gobble-cli, read only:**
   * `git -C <gobble-cli> archive d6c10a5`, unpacked into the scratch directory;
   * `go mod edit -replace github.com/maccavelli/acp-go-sdk2=<this tree>` in that copy;
   * `go build ./...`, `go vet ./...` and `go test ./...` there.

   They must pass with no change to gobble's code (MADR F20). Nothing is written in gobble-cli's tree.
4. **`make api`** again on the release commit; the same report as phase 3, step 10.
5. **`docs/reports/0054-REPORT-release-notes-v1-36-0.md`**, against `v1.35.0`, in the form of
   `0052-REPORT-release-notes-v1-35-0.md`:
   * the pin;
   * the system prompt's types and their gates, and what an SDK client and an SDK agent now refuse, as a change on the
     wire limited to messages that carry `systemPrompt`;
   * the keyword;
   * the new `unstable` names, and the proxy methods that are not among them (deviation 1);
   * `make api`'s report.
6. **The records:**
   * this PLAN is `complete` only when every Verification item holds;
   * the MADR's Consequences are checked against what was built, with an amendment for any difference;
   * the rows in `docs/README.md`.
7. **The release, only on the owner's ask in that turn:**
   * `git fetch`;
   * `make push-guard`;
   * the tag `v1.36.0`;
   * `python3 tools/release_body.py v1.36.0 <rev>`;
   * `python3 tools/publication.py`;
   * `python3 tools/consumer.py v1.36.0`.

## Verification

* Each phase: `make pre-add-check FILES="…"` passes. It runs:
  * `gofmt`;
  * `golangci-lint` with `.golangci.yml`, for the host and for `GOOS=windows`;
  * `go vet`;
  * `go test -race -shuffle=on`, uncached, in both modules;
  * `go mod tidy -diff`;
  * `govulncheck`.
* Phases 1 and 2 change no generated file (`TestGeneratedFilesUpToDate`, and `git status`).
* `schema/version` reads `1.31.0`, the four schema files hash to MADR D1's values, `schema/unstable-v1.json` and
  `README.md` are unchanged by the bump, and `cmp -s schema/version version` passes.
* Every test and engine case named in phases 1 to 3 exists and was seen to fail on its plant, recorded with the failure
  line.
* `go test -race -count=20 -run 'SystemPrompt|DefaultOnNull' .` passes.
* `make api` against `v1.35.0` reports no incompatible change in a gated package, and exactly phase 3, step 10's
  additions.
* The golden's diff holds only phase 3, step 6's lines.
* `make check-records`, `make markdownlint`, `make disclosure-check` and, where changed, `make check-tools` pass.
* The Linux and Windows hosts pass the tree's tests. Interop passes, or is recorded as not run.
* gobble-cli at `d6c10a5` builds, vets and passes its tests against the tree with no change.
* `AllowsValue` returns R1's error for a prompt without the capability, R2's for `override` without `mode.override`, and
  nil when both are advertised.
* A received `session/new` with an unadvertised prompt is answered `-32602`, and the agent's `NewSession` is not called
  (MADR, Confirmation).

## Rollout and Rollback

* **Rollout:** one minor, `v1.36.0`, after phase 5, on the owner's ask. Consumers adopt under their own records:
  * gobble-cli advertises `SessionCapabilities.SystemPrompt` and maps the prompt into its sections (MADR, "For
    gobble-cli");
  * magic-cli-remote, a client of `gobble acp`, sends it.
* **Rollback before the tag:** revert the phases' commits, newest first. Phase 3 depends on phases 1 and 2, which change
  no output and can stay.
* **Rollback after the tag:** a patch that removes a rule from `model/acp.go`, or the `NonEmptyArrays` entry, and
  regenerates. That removes the gate and keeps the types. The types, the pin and the `acp` names stay, under the `v1`
  promise. The pin cannot go back to `1.25.0` in a `v1` minor: the eleven new `acp` names would go away.

## Execution record

### Approval (2026-11-10)

* The 0053 report and its rows were committed alone (`9491138`, on the owner's ask). The owner committed the MADR, this
  PLAN and their rows (`130ea34`), and said "committed proceed". That is the approval, and `130ea34` is the base.
* Preconditions: `git status --short` was clean on `130ea34`. `make test` exited 0, every package of both modules `ok`.

### Deviation 1, the proxy methods (2026-11-10)

* **Found:** in phase 1, before any edit, by reading the generator for step 6. The `1.31.0` proxy methods hold three
  shapes the generator refuses:
  * `InitializeProxyRequest` and `InitializeProxyResponse` are `allOf` aliases on a definition
    (`model/schema.go:357`-`:358`);
  * `proxy/successor` links a request and a notification to one wire (`model/methods.go:213`-`:214`);
  * `"x-side": "proxy"` takes no binding (`methods.go:268`-`:283`).

  Step 7's stop condition was met.
* **Put to the owner** with three options. The other two were building the generator support, and the constants only.
* **Decided:** model the key and emit nothing (the owner, 2026-11-10). The MADR's D3 and question 4 carry an amendment.
* **Changed in this PLAN:**
  * phase 1, steps 5 to 10, struck through or annotated;
  * phase 1's row in Scope;
  * one engine case renamed `accept-unstable-proxy-methods-omitted`.

  Phase 3, step 10's `unstable` additions are now F9's 46, with no proxy names. The release notes (phase 5) name the
  omission.

### Phase 1, the two keys (2026-11-10)

**What was done.**

* **`load/load.go`:** `Definition.XDeserializeDefaultOnNull`, with the doc of the three `x-deserialize` fields citing
  D2. `Meta.ProxyMethods`, whose doc says the methods are read, merged into no side and refused in a stable meta,
  citing the amendment. `ReadMeta` refuses a stable `meta.json` with proxy methods.
* **`load/merge.go`:** `refuseStableProxyMethods`, called first in `MergeStableAndUnstable`. **`load/manifest.go`:** the
  same, in `MergeManifest`.
* **`model/keywords.go`:** `"x-deserialize-default-on-null": TreatDecode`. **`model/schema.go`:** the keyword in
  `refSiblings`, and the shape check in `walkRefs` ("x-deserialize-default-on-null on %s, not an array or an object"),
  named in `checkRefs`' doc. **`model/fields.go`:** `Facts.DefaultOnNull`, set by `factsOf`. No emitter reads it.
* **Engine cases:**
  * `accept-default-on-null`: required `args`, optional `more` and `env`. Its behaviour test decodes `null` into each
    and requires `{"args":[]}` on encode.
  * `refuse-default-on-null-scalar`;
  * `accept-unstable-proxy-methods-omitted`: `accept-unstable-empty` plus `proxyMethods`, an `allOf` alias pair, and a
    request, response and notification on `proxy/successor`. Its `want.api` is byte-identical to
    `accept-unstable-empty`'s, and it has no `want.api.unstable`;
  * `refuse-stable-proxy-methods`.
* **`load/load_test.go`:** `TestParseSchemaKeepsDefaultOnNull`. `TestMetaProxyMethods` requires the refusal from
  `ReadMeta`, `MergeStableAndUnstable` and `MergeManifest`. `TestDeepCopyDefinitionCoversEveryField`'s fixture sets the
  new field, which that test requires of every `Definition` field (it failed first: "the fixture leaves
  XDeserializeDefaultOnNull zero").

**Found on the way, inside the phase's files.**

* `refuse-stable-proxy-methods` was first accepted. A schema with no unstable files is read by `Committed` without
  either merge (`load.go:413`-`:414`), so the refusal also went into `ReadMeta`.
* `accept-default-on-null`'s behaviour test first failed to compile in `TestEngineBehaviour`: `l.Validate undefined`.
  `Validate` is generated only for a type with checks. The call was removed; phase 3's `TestDefaultOnNullLists`
  validates the schema's own 14 properties.
* The PLAN's Scope named `model/schema_test.go`. It was not needed, because the engine case carries the check.

**Generated output.** `git status --short` names no generated file. At `1.25.0` nothing carries the new keys, so the
output is unchanged.

**Plants** (`python3 tools/plants.py`, scratch spec `plants_p1.json`, each in its own copy of the working tree; exit 0,
"5/5 as required"):

| Plant | Test | Failure |
| :--- | :--- | :--- |
| P1-1, the `keywords.go` entry removed | `TestKeywordRegistryMatchesLoader` | `keywords_test.go:30: registry:` |
| P1-2, `walkRefs`' shape check made `false &&` | `TestEngineCases/refuse-default-on-null-scalar` | `engine_test.go:50: run accepted the schema` |
| P1-3, the live proxy methods merged into the agent side | `TestEngineCases/accept-unstable-proxy-methods-omitted` | `engine_test.go:50: unstable: model: InitializeProxyRequest: allOf on a definition is not supported` |
| P1-4, `ReadMeta`'s refusal given an empty meta | `TestEngineCases/refuse-stable-proxy-methods` | `engine_test.go:50: run accepted the schema` |
| P1-5, `emitObjectMarshal`'s fill of nil required collections skipped (`fills[:0]`) | `TestEngineBehaviour` | `behaviour_test.go:80: the package's tests did not pass`, in `accept-default-on-null` |

P1-3's failure is deviation 1's evidence, run: the generator refuses the proxy alias when the proxy methods reach a
side.

**Checks.**

* `make pre-add-check FILES="<the phase's 7 Go files and 4 engine cases>"` first exited 2: gofmt wanted the keyword
  table's column realigned after the new entry and its comment (`model/keywords.go:54`). After `gofmt -w` on that file,
  it exited 0: "7 file(s) gofmt-clean; both modules linted, vetted and tested (-race -shuffle=on, uncached), tidy,
  govulncheck."
* `make check-records`, `make markdownlint` and `make disclosure-check` exited 0: 0 issues, "813 tracked file(s),
  0 finding(s)".
* Phase 1 is staged for the owner's commit.
