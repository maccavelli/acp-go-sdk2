---
status: complete
date: 2026-11-11
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
| 3, the pin and the rules (D1, D4, D5, D6, D8) | `schema/version`, `version`, `schema/meta.json`, `schema/schema.json`, `schema/meta.unstable.json`, `schema/schema.unstable.json`; `cmd/generate/internal/model/acp.go`, `gates_test.go`, `sources_test.go`; `cmd/generate/main_test.go`; the generated `constants_gen.go`, `types_gen.go`, `agent_gen.go`, `client_gen.go`, `helpers_gen.go`, `schema_gen_test.go`, `unstable/unstable_gen.go` (new); `testdata/encodings.golden`; `system_prompt_test.go` (new); `default_on_null_test.go` (new). *Deviation 2 (2026-11-10) added `cmd/generate/internal/emit/unknownkind_test.go`. Deviation 3 (2026-11-10) added `negotiation_test.go`. Deviation 4 (2026-11-10) added `cmd/generate/internal/emit/file.go` and `file_test.go`.* |
| 4, the docs (D8) | `README.md`; `docs/architecture.md`; `doc.go` only if it lists the capabilities |
| 5, the close (D9) | `tools/consumer.py`, `tools/test_tools.py`, `tools/README.md`; `tools/consumer_probe.py` (new) and `docs/architecture.md`, both added by deviation 7; `docs/reports/0054-REPORT-release-notes-v1-36-0.md` (new); this PLAN; the MADR; `docs/README.md` |

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
* Phase 1 is staged for the owner's commit. The owner committed it as `0d8f211`.

### Phase 2, the gate kinds and the validation list (2026-11-10)

Base `0d8f211`, clean.

**What was done.**

* **`model/protocol.go`:**
  * `GateKind` gains `GatePresent`, `GateValue`, `GateUndeclaredValue` and `GateOnlyVariants`, each documented with
    the fields its rule takes and citing D5.
  * `checkCapabilityRules` gains their missing-field tests: present and undeclared value take a property and a
    capability and no variant; value and only variants take all three.
  * `Protocol.NonEmptyArrays []PatchRule`, checked in `Build` by `checkPropertyRules("non-empty", …)`.
* **`model/gates.go`:**
  * four `GateCheckKind`s; `GateCheck.Values`, `Others`, `CatchAll` and `CatchDisc`; and `VariantValue`;
  * cases in `gateCheck`, with `enumValues` and `onlyVariants`;
  * the prefix rules in the capability switch: undeclared value's prefix must resolve, and every segment of
    only variants' prefix but the last must.
* **`emit/gates.go`:** the present and value checks in `gateStmt`, using the existing `refuse` and `missing`.
  `undeclaredValueGate` and `onlyVariantsGate`, with `capabilityRefusal`.
* **`model/model.go`:** `nonEmptyProps` and `resolveNonEmptyArrays`, which requires the source in the property's
  description, as `resolveEmptyClears` does. **`model/validate.go`:** `CheckNonEmpty`, which `ownChecks` adds for a
  ruled array and refuses on any other shape. **`emit/types.go`:** `checkStmt` emits
  `if len(v.F) == 0 { return &ValidationError{…, Rule: RuleMinItems, …} }`.
* **`RuleMinItems`** was added to the rule stubs of `cmd/generate/main_test.go` and `emit/types_test.go`.
* **Engine cases:**
  * five accept cases, each with a behaviour test: `accept-capability-present`, `-value`, `-undeclared-value`,
    `-only-variants` and `accept-non-empty-array`;
  * seven refusal cases, exactly the PLAN's list.

  Each accept case states the rules in a `protocol.json`, the per-case table override of
  `cmd/generate/engine_test.go:228`-`:262`, over one shared schema shaped like the system prompt: optional
  `instructions`, an open `Mode` (`add`, `replace`, other), and a discriminated `Block` union (`text`, `image`,
  `Unknown`). The behaviour tests check a typed send's `*CapabilityError` (capability, field, variant) and a raw send
  answered `-32602` with the handler not called. They also check an advertised send that reaches the handler, and
  `RuleMinItems` from `Validate`, a typed send and a raw send.
* **Model tests:**
  * `TestCapabilityRulesRefuse` gains nine rows: a missing and an extra field for each kind, and an only-variants
    prefix of one segment;
  * `TestNonEmptyArraysRules` (`protocol_test.go`) covers the check of `items` and `need`, an array that is not ruled,
    and the four refusals: not an array, not stated, no source, two rules.

**Refinements, within the PLAN's text.**

* The value kinds accept the three string-enum kinds that list values: `KindEnum`, `KindStringConstUnion` and
  `KindOpenEnum`. `SystemPromptMode` is a `KindOpenEnum`; the PLAN's step 2 said "a `KindEnum`".
* `GateCheck.Values` holds the values alone. The generated code compares the field with string literals, which a named
  string type accepts, so no constant names are needed.
* The scratch generators of the cases are `p2_cases.py` and `p2_cases_write.py`. A probe case, `accept-zz-probe`, was
  used to read the generated API and then deleted.

**Generated output.** `git status --short` names no generated file. `model/acp.go` has no new rule yet, so nothing at
`1.25.0` uses the kinds.

**Plants** (`python3 tools/plants.py`, spec `plants_p2.json`; first run "6/9 as required", exit 1):

| Plant | Test | Failure |
| :--- | :--- | :--- |
| P2-1, the present check `!= nil` made `== nil` | `TestEngineBehaviour` | `the package's tests did not pass`, in `TestPresentGated` |
| P2-2, the value check `==` made `!=` | `TestEngineBehaviour` | the same, in `TestValueGated` |
| P2-3, the undeclared value comparing only the last listed value | `TestEngineBehaviour` | the same, in `TestUndeclaredValueGated` |
| P2-4, the only-variants condition negated | `TestEngineBehaviour` | the same, in `TestOnlyVariantsGated` |
| P2-5, the non-empty check `== 0` made `!= 0` | `TestEngineBehaviour` | the same, in `TestNonEmptyArray` |
| P2-6, present and undeclared value's field checks made `false` | `TestCapabilityRulesRefuse` | `present_without_a_property`: a nil pointer panic in `gateCheck` (`gates.go:365`), which the field check prevents |
| P2-7, value and only variants' field checks made `false` | `TestCapabilityRulesRefuse` | `value_without_a_variant` and three more rows: `Build = <nil>, want an error containing "it lacks a field its kind needs"` |
| P2-8, the non-empty source check removed | `TestNonEmptyArraysRules` | `protocol_test.go:592: not stated: Build = <nil>` |
| P2-9, a one-segment only-variants prefix accepted | `TestCapabilityRulesRefuse` | `only_variants_with_a_prefix_of_one_segment: Build = <nil>` |

P2-6, P2-7 and P2-9 failed on their rows in the first run. The spec's `want` text named the rows with spaces, where the
test output has the subtest names with underscores. With those names (`plants_p2_rerun.json`) the three ran again:
"3/3 as required", exit 0.

**Checks.**

* `make pre-add-check FILES="<the phase's 10 Go files and 12 engine cases>"` first exited 2 on `gocognit`: "cognitive
  complexity 52 of func `(*Model).gateCheck` is high (> 50)", for the host and for `GOOS=windows`. The capability
  switch at the end of `gateCheck` moved, unchanged in behaviour, into a method of its own, `ruleCapability`. The model
  tests passed again. P2-9's edit, which quoted the moved code, ran again as P2-9b against it (`plants_p2_9b.json`):
  "1/1 as required", the same failure. The check then exited 0: "10 file(s) gofmt-clean; both modules linted, vetted
  and tested (-race -shuffle=on, uncached), tidy, govulncheck."
* Phase 2 is staged for the owner's commit. It was committed as `06aa72c`, by the agent on the owner's ask in that
  turn ("Stage, commit to main, and proceed").

### Deviation 2, the `unknownKind` test's setup (2026-11-10)

* **Found:** in phase 3, after the rules were added and the tree regenerated. `TestUnknownKindWithoutRawKeyGate`
  (`cmd/generate/internal/emit/unknownkind_test.go`) failed: "SystemPrompt: a check still reads the raw key "type" by
  unknownKind" (`:37`). The file is not in phase 3's row.
* **Why:** the test guards that `unknownKind` is generated when no gate uses it, since `helpers.go` calls it
  ([0049-MADR-remediate-the-2026-11-01-debugging-pass.md](0049-MADR-remediate-the-2026-11-01-debugging-pass.md), G30).
  Its setup removes the table's `GateUndeclaredVariant` rules to get such a model (`:26`-`:28`), and checks that
  premise before its assertion (`:46`). R4 is a `GateOnlyVariants` rule, which reads an unknown block's type with
  `unknownKind` too, so the premise no longer held. The failure is not pre-existing: R4 causes it.
* **Put to the owner** with three options. The other two were reworking R4's emission not to use `unknownKind`, and
  stopping phase 3 for review.
* **Decided:** widen the test's setup to remove both kinds whose checks use `unknownKind` (the owner, 2026-11-10). The
  assertion is unchanged, and it is seen to fail on a plant.
* **Changed in this PLAN:** phase 3's row in Scope gains the file.

### Deviation 3, the record that advertises everything (2026-11-10)

* **Found:** in phase 3, step 7, before the new root tests were written. `allAdvertised()` (`negotiation_test.go:40`-`:87`)
  is documented as "a record in which each side advertises every capability the protocol table and its gates name".
  With R1 and R2 in the table it lacked `SessionCapabilities.SystemPrompt`. `checkValueGate`
  (`gates_test.go:562`-`:611`), which checks a rule both ways, builds its record with the capability from
  `allAdvertised()` alone. The file is not in phase 3's row. Nothing failed: the existing tests passed.
* **Put to the owner** with three options. The other two were building the record and a copy of the check inside
  `system_prompt_test.go`, and stopping phase 3 for review.
* **Decided:** update `allAdvertised()` to advertise the system prompt with `override` (the owner, 2026-11-10). The
  helper's doc stays true, and the new rules run through the harness every other rule uses.
* **Changed in this PLAN:** phase 3's row in Scope gains the file.

### Deviation 4, the generated import block (2026-11-10)

* **Found:** at the end of phase 3, by its pre-add check, which exited 2:
  `unstable/unstable_gen.go:10:1: File is not properly formatted (goimports)`, for the host and for `GOOS=windows`.
* **Why:** `importDecl` (`cmd/generate/internal/emit/file.go:212`-`:225`) wrote every import as one group sorted by
  path, so `acp "github.com/maccavelli/acp-go-sdk2"` sat between `"fmt"` and `"path/filepath"`. `gofmt` accepts
  that. `goimports`, which `.golangci.yml` runs as a formatter, requires the standard library's imports first, then a
  blank line, then the others. The defect was latent: until this pin `unstable/` had no generated file, and the root's
  generated files import only the standard library. The file is not in phase 3's row.
* **Put to the owner** with three options. The other two were running `goimports` over the generated files in
  `make version`, which adds a tool and breaks `TestGeneratedFilesUpToDate`, and stopping phase 3 for review.
  Excluding the file from the formatter was not offered: it loosens a check.
* **Decided:** fix the emitter to write imports as `goimports` formats them (the owner, 2026-11-10).
* **Changed in this PLAN:** phase 3's row in Scope gains `emit/file.go` and its test.

### Phase 3, the pin and the rules (2026-11-10)

Base `06aa72c`, clean.

**Steps 1 and 2, the pin.**

* `printf '1.31.0\n' > schema/version`, then `gmake version MDSH=true`. The owner stopped the first run while the
  generator was running (`gmake: *** [Makefile:170: version] Terminated: 15`). The state it left was read before
  anything else was done:
  * the four schema files downloaded, with the right hashes;
  * `version` still `1.25.0`;
  * no generated file written;
  * no process left running.

  The second run exited 0. It downloaded nothing again, since the files were newer than `schema/version`.
* The generator read the files as downloaded, with no edit: phase 1's two keys were enough.
* `shasum -a 256` printed MADR D1's four values:

  | File | SHA-256 |
  | :--- | :--- |
  | `schema/meta.json` | `061edb6efa8fb2aa2792459a86ec7268de5fe665bba48b2ffe7939df01481f88` |
  | `schema/meta.unstable.json` | `99f1b10aca5c1cf516bc1eb330e980e2fafd4029249bbe4c8557119ae1800ffc` |
  | `schema/schema.json` | `e747a7bec3c02708b70885bf3cccac5c72fa421390dd7e85758fd61e33b1ae21` |
  | `schema/schema.unstable.json` | `0aa22eba8becd67c67e6da0336b1761dc8684d2d12ba26baedeff1fd3ce257ea` |

* `cmp -s schema/version version` exited 0. `git diff --exit-code schema/unstable-v1.json README.md` exited 0.
* `unstable/unstable_gen.go` holds no proxy name (deviation 1).

**Steps 3 and 4, the rules.** `model/acp.go` gained the six capability rules and the `NonEmptyArrays` entry as the PLAN
wrote them, with `capSystemPrompt`, `defSystemPrompt`, `keySystemPrompt`, `keyMode`, `keyContent` and the five source
sentences. After `go run .` and `gofmt`, the generated code is:

```go
// in NewSessionRequest.gates, and the same in LoadSessionRequest's and ResumeSessionRequest's
if v.SystemPrompt != nil && n.Response.AgentCapabilities.SessionCapabilities.SystemPrompt == nil {
	return &CapabilityError{Method: method, Capability: "sessionCapabilities.systemPrompt", Field: "systemPrompt"}
}
// …
if v.SystemPrompt != nil {
	if err := v.SystemPrompt.gates(n, method); err != nil {
		return err
	}
}

// SystemPrompt.gates
if v.Mode == "override" && (… .SystemPrompt == nil || … .SystemPrompt.Mode == nil || … .SystemPrompt.Mode.Override == nil) {
	return &CapabilityError{Method: method, Capability: "sessionCapabilities.systemPrompt.mode.override", Field: "mode", Variant: "override"}
}
if v.Mode != "append" && v.Mode != "override" {
	return &CapabilityError{Method: method, Capability: "sessionCapabilities.systemPrompt.mode." + string(v.Mode), Field: "mode", Variant: string(v.Mode)}
}
for i := range v.Content {
	if v.Content[i].Text == nil {
		switch {
		case v.Content[i].Image != nil: // and Audio, ResourceLink, Resource, each with its own literal
			return &CapabilityError{Method: method, Capability: "sessionCapabilities.systemPrompt.content.image", Field: fmt.Sprintf("content[%d]", i), Variant: "image"}
		case v.Content[i].Unknown != nil:
			kind := unknownKind(v.Content[i].Unknown, "type")
			return &CapabilityError{Method: method, Capability: "sessionCapabilities.systemPrompt.content." + kind, Field: fmt.Sprintf("content[%d]", i), Variant: kind}
		}
	}
}

// SystemPrompt.validate, first
if len(v.Content) == 0 {
	return &ValidationError{Type: "SystemPrompt", Field: "content", Rule: RuleMinItems, msg: "SystemPrompt: content: must not be empty"}
}
```

(`types_gen.go:25903`, `:26134` and `:18785`; the composite literals are shown on one line each.)

**Step 5, the generator's tests.** With the rules in, `go test ./...` in `cmd/generate` failed four tests, three of
them the PLAN's:

* `TestProtocolTableAppliesToCommittedSchema`: "39 checks resolve from 36 capability rules, want 33 from 30". That is
  the predicted count. `checkCommittedGates` (`model/gates_test.go`) now requires 39 from 36, and its comment says why.
* `TestCapabilitySourcesQuoteTheSchema`: the three NES rules. `committedDescriptions` now reads the manifest too, by the
  loader's file-name constants, and the test requires each `NonEmptyArrays` source in a description.
* `TestGeneratedFilesUpToDate`: the run renders `unstable/unstable_gen.go`. Adding that name to `generatedFiles`, as the
  PLAN said, broke `TestRunGeneratesRecasedUnion` ("open …/unstable/unstable_gen.go: no such file or directory"):
  three other tests read `generatedFiles` as the root package's files. So `generatedFiles` is unchanged, and
  `TestGeneratedFilesUpToDate` compares with a new `repositoryGenerated()`, the root files and then package
  `unstable`'s. Both are in `cmd/generate/main_test.go`.
* `TestUnknownKindWithoutRawKeyGate`: deviation 2.

After those changes the module's seven packages pass.

**Step 6, the golden.** At the new pin the root module failed only `TestEncodingsUnchanged`. After
`go test -run '^TestEncodingsUnchanged$' -update .`, `git diff --numstat` gives `27 6`. The scratch script
`golden_check.py` printed: "6 lines replaced, each gaining only systemPrompt; 21 lines added:
{'SessionSystemPromptCapabilities': 3, 'SessionSystemPromptModeCapabilities': 3,
'SessionSystemPromptOverrideCapabilities': 3, 'SystemPrompt': 3, 'SystemPromptMode': 9}", exit 0. That is MADR F10's
measurement.

**Step 7, `system_prompt_test.go`.** Seven tests, on the helpers of `gates_test.go`:

* `TestSystemPromptGated` runs R1 on `session/new`, `session/load` and `session/resume` through `checkValueGate`, and
  reads the prompt the handler received.
* `TestSystemPromptOverrideGated` runs R2 through `checkValueGate`. It also refuses `override` when the agent's `mode`
  object advertises none, and passes `append`.
* `TestSystemPromptUndeclaredModes`: `_vendor`, `later`, `""`, and a raw `"mode": null`.
* `TestSystemPromptTextOnly`: `image`, `audio`, `resource_link`, `resource` and an unknown `video`, each as
  `content[1]`.
* `TestSystemPromptEmptyContent`: `[]` and nil, by `Validate`, by a typed send that writes nothing, and received.
* `TestSystemPromptInboundValidationOptIn`: a hook that accepts a `_vendor` mode or empty content, and sees the SDK's
  refusal.
* `TestSystemPromptAllowsValue`: seven records and requests.

**Step 8, `default_on_null_test.go`.** `TestDefaultOnNullLists` finds the 14 properties in `schema/schema.json` and
passes for each, named `<Definition>.<property>`.

**Step 9, the plants.** All as required.

Regenerated in a copy of the working tree by the scratch script `regen_plant.py` (spec `plants_p3_regen.json`; exit 0,
"all as required"). Each ran every test matching `SystemPrompt`:

| Plant | Tests that failed | First failure |
| :--- | :--- | :--- |
| P3-1, R1's three rules removed | `TestSystemPromptGated`, `TestSystemPromptAllowsValue` | `system_prompt_test.go:303: a prompt, nothing advertised: AllowsValue = <nil>` |
| P3-2, R2 removed | `TestSystemPromptOverrideGated`, `TestSystemPromptAllowsValue` | `:303: override, append advertised: AllowsValue = <nil>` |
| P3-3, R3 removed | `TestSystemPromptUndeclaredModes`, `TestSystemPromptInboundValidationOptIn`, `TestSystemPromptAllowsValue` | `:303: a vendor mode, everything advertised: AllowsValue = <nil>` |
| P3-4, R4 removed | `TestSystemPromptTextOnly`, `TestSystemPromptAllowsValue` | `:303: an image, everything advertised: AllowsValue = <nil>` |
| P3-5, the `NonEmptyArrays` entry removed | `TestSystemPromptEmptyContent`, `TestSystemPromptInboundValidationOptIn` | `:268: the hook saw [<nil>], want the SDK's refusal among them` |

By `python3 tools/plants.py` (spec `plants_p3.json`; "5/5 as required", exit 0):

| Plant | Test | Failure |
| :--- | :--- | :--- |
| P3-6, `committedDescriptions` without the manifest | `TestCapabilitySourcesQuoteTheSchema` | `sources_test.go:94: the nes.jump rule on SuggestNesResponse jump quotes …, which no description of the committed schema contains` |
| P3-7, `unknownKind` marked only for a gate (`t.HasUnknown && false`) | `TestUnknownKindWithoutRawKeyGate` | `unknownkind_test.go:49: types_gen.go lacks unknownKind, which helpers.go calls` |
| P3-8, `McpServerStdio`'s decoder refusing a nil `args` | `TestDefaultOnNullLists` | `default_on_null_test.go:59: … did not decode: McpServerStdio: args is null`, in `McpServerStdio.args` |
| P3-9, `srcEmptyContent` reworded | `TestCapabilitySourcesQuoteTheSchema` | `sources_test.go:100: the non-empty rule on SystemPrompt.content quotes "Clients MUST NOT send an empty list."` |
| P3-10, `allAdvertised()` without the system prompt | `TestSystemPromptGated` | `system_prompt_test.go:111: session/new with an advertised prompt: … capability sessionCapabilities.systemPrompt not advertised` |

P3-7 shows deviation 2's widened setup still reaches the test's assertion. Before the setup was widened, the same test
failed at its premise (`:37`), as deviation 2 records.

The golden script was fed the real diff with one line added: a removed `ToolCall` line gave "a removed line of
ToolCall, which is not a session request", exit 1; an added `PromptResponse` line gave "an added line of
PromptResponse, which is not one of the five new types", exit 1.

**Step 10, `make api BASE=v1.35.0`.** Exit 0: "against v1.35.0: 0 incompatible, 57 compatible" and "reported packages
unstable, remote: 0 incompatible, not gated".

* In `acp`, eleven additions, the PLAN's list: `LoadSessionRequest.SystemPrompt`, `NewSessionRequest.SystemPrompt`,
  `ResumeSessionRequest.SystemPrompt`, `SessionCapabilities.SystemPrompt`, `SessionSystemPromptCapabilities`,
  `SessionSystemPromptModeCapabilities`, `SessionSystemPromptOverrideCapabilities`, `SystemPrompt`, `SystemPromptMode`,
  `SystemPromptModeAppend`, `SystemPromptModeOverride`.
* In `unstable`, 46 additions and no proxy name: the method constants, types and constructors of `auth/status`,
  `auth/status_update`, `session/archive`, `session/unarchive`, `session/options`, `session/set_title` and
  `session/status`, and `ToolCallContentSession` with `DecodeToolCallContentSession`. The full list is the scratch
  file `p3_api.out`, and equals MADR F9's.

**Step 11.** `go test -race -count=20 -run 'SystemPrompt|DefaultOnNull' .` exited 0.

**Deviation 4's fix.** In `cmd/generate/internal/emit/file.go`:

* `importDecl` writes the standard library's imports first, then the others, each sorted by path. `standardImport`
  tells them apart as `goimports` does: a first path element without a dot.
* `layout.genDecl` leaves a blank line before the first import of another module (`startsOtherImports`).
* `layout.spec` positions a named import's name as well as its path. Without that the first attempt changed nothing:
  the aliased `acp` import starts at its name, which had no line, so the printer saw no gap and gofmt sorted the block
  back into one group.

`unstable/unstable_gen.go` now begins:

```go
import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	acp "github.com/maccavelli/acp-go-sdk2"
)
```

The root's generated files import only the standard library and did not change by this fix. `TestRenderLayout`
(`emit/file_test.go`) gains two cases: imports of two other modules after the standard library's, and a named import
alone in its group, which is the shape of `unstable_gen.go`.

Plants, by `tools/plants.py` (spec `plants_p3_d4.json`):

| Plant | Case of `TestRenderLayout` | Failure |
| :--- | :--- | :--- |
| P3-11, the blank line's condition made `&& false` | imports of another module follow the standard library's after a blank line | `file_test.go:221: render mismatch` |
| P3-12, the named import's name given no position | a named import of another module alone after the standard library's | `file_test.go:221: render mismatch` |

The first run of these two was "0/2 as required", and both misses were the plants' and the test's, not the fix's:

* P3-11's edit removed the whole block and left a variable unused, a compile error. The edit now makes the condition
  false.
* P3-12 passed, because the first test case had the named import second in its group, after an import that already
  had a line. The second case was added for it.

The second run was "2/2 as required", exit 0.

**Checks.**

* `make pre-add-check FILES="<the phase's files>"` first exited 2 on deviation 4's `goimports` finding, and nothing
  else. After the fix it exited 0, with the two emitter files among those named: "15 file(s) gofmt-clean; both modules
  linted, vetted and tested (-race -shuffle=on, uncached), tidy, govulncheck."
* After the last regeneration, `make api BASE=v1.35.0` exited 0 again with the same counts, the four schema hashes were
  unchanged, `cmp -s schema/version version` exited 0, and `git diff --exit-code schema/unstable-v1.json README.md`
  exited 0.
* Phase 3 is staged for the owner's commit. The owner committed it as `894726f`.

### Deviation 5, a Go advisory stops the pre-add check (2026-11-10)

* **Found:** at the end of phase 4, by its pre-add check, which exited 2 at its last step alone:
  `govulncheck (.): failed`, "Your code is affected by 1 vulnerability from the Go standard library". The steps
  before it, which print only on failure, passed: gofmt, `golangci-lint` for both platforms, `go vet`, the tests of both
  modules and `go mod tidy -diff`.
* **Why:** GO-2026-6859 (CVE-2026-97054), "HTTP/1 server crash due to MaxBytesReader header race in net/http", was
  published on 2026-11-11 at 02:51 UTC, after phase 3's check passed. It is found in `net/http@go1.27.3` and fixed in
  go1.26.11 and go1.27.4. This repository pins `go 1.26.10` and `toolchain go1.27.3` in both `go.mod` files, and CI's
  matrix runs both. `remote/httpserver.go:86` calls `http.MaxBytesReader`. The failure is not phase 4's: its three
  files are documentation, and the check fails the same on `894726f`.
* **Put to the owner** with three options. The other two were staging phase 4 with the failure recorded, an exception
  to `AGENTS.md`'s "a file that fails is not committed", and stopping.
* **Decided:** the Go pin gets a record of its own first (the owner, 2026-11-10), as
  [0028-MADR-pin-go-1-27-3-and-floor-1-26-10.md](0028-MADR-pin-go-1-27-3-and-floor-1-26-10.md) was for the last one.
  Phase 4's files stay written and unstaged until that record is executed. Phase 4's check then runs again.
* **Changed in this PLAN:** phase 4's record below says where it stopped. Phase 5's release notes (step 5) also name
  the Go floor and toolchain of
  [0055-MADR-pin-go-1-27-4-and-floor-1-26-11.md](0055-MADR-pin-go-1-27-4-and-floor-1-26-11.md), which `v1.36.0` is the
  first release to carry.

### Phase 4, the docs (2026-11-10)

Base `894726f`, clean.

**What was done.**

* **`README.md`:**
  * A new `#### System prompts` at the end of "Initialization and capabilities". It covers what the agent advertises
    and what the client sends, each with a code block; a table of the five refusals with the error, the capability
    and the field of each; that an agent advertising `append` alone never sees the rest in its handler; `AllowsValue`;
    the validation hook that accepts a mode of the agent's own, with a code block; the specification's re-send on load
    and resume, which the SDK does not check; and `session/fork`, whose frozen request carries no prompt.
  * The capability list's "Values" item names the two system prompt capabilities and links the subsection.
  * "Wire encoding" says what `x-deserialize-default-on-null` does.
* **`docs/architecture.md`:**
  * the protocol table: 36 capability rules where it said 30, the four new kinds, and `NonEmptyArrays`;
  * the loader paragraph: `proxyMethods`, read and placed in no side;
  * the decoding paragraph: `x-deserialize-default-on-null`, recorded and not emitted, with the two tests that hold the
    decoders to it;
  * the unstable boundary: what `unstable_gen.go` holds at `1.31.0`, without the proxy methods, and how the emitter
    writes its imports (deviation 4), where it said "At `1.25.0`, `unstable/` holds only `doc.go`";
  * the test list: `system_prompt_test.go`, `default_on_null_test.go` and the six new accept cases.
* **`docs/README.md`:** an "I want to…" row for the 1.31.0 pin and the system prompt, beside 0004's, which now reads
  "was 1.25.0".
* **`doc.go`:** nothing. It names the error types and sentinels, not the capabilities, as the PLAN allowed for.

**Not changed.** `RELEASING.md` still uses `1.25.0` in its examples ("for example `1.25.0`"). It is not in this phase's
row, and the examples stand for any version.

**The README's code is real code.** Its three blocks are outside the markers `docs_test.go` reads, so no test compiles
them. A scratch module with a `replace` to this tree held the three blocks as written. `go vet` passed, and the
program printed, for the table's rows:

```text
unadvertised           capability=sessionCapabilities.systemPrompt field=systemPrompt variant=""
override, append only  capability=sessionCapabilities.systemPrompt.mode.override field=mode variant="override"
_review mode           capability=sessionCapabilities.systemPrompt.mode._review field=mode variant="_review"
empty mode             capability=sessionCapabilities.systemPrompt.mode. field=mode variant=""
image block            capability=sessionCapabilities.systemPrompt.content.image field=content[0] variant="image"
append, append only    <nil>
override, both         <nil>
empty content: true SystemPrompt: content: must not be empty
```

No new check was added in this phase, so no plant ran.

**Checks.**

* `make check-records`, `make markdownlint` and `make disclosure-check` exited 0.
* `make pre-add-check FILES="README.md docs/architecture.md docs/README.md"` exited 2 at `govulncheck` alone
  (deviation 5). Phase 4 is written and not staged. It waits for the Go pin's record.
* **Run again on 2026-11-11,** on base `4d82028`, which holds 0055's pins and deviation 6's fix: the same command
  exited 0, "0 file(s) gofmt-clean; both modules linted, vetted and tested (-race -shuffle=on, uncached), tidy,
  govulncheck." `make check-records`, `make markdownlint` and `make disclosure-check` exited 0 again. Since the first
  run, `docs/README.md`'s row for this PLAN names deviations 5 and 6. Nothing else in the phase's three files changed.

### Deviation 6, a phase 3 test fails on Windows (2026-11-10)

* **Found:** by step 4 of
  [0055-PLAN-pin-go-1-27-4-and-floor-1-26-11.md](0055-PLAN-pin-go-1-27-4-and-floor-1-26-11.md), the first run of this
  tree's tests on the Windows host since phase 3. The root module passed. In `cmd/generate`, one test failed:
  `TestGeneratedFilesUpToDate`, "`main_test.go:79`: `unstable\unstable_gen.go` is a generated file run would remove".
  The Linux host passed both modules.
* **Why:** the test's loop over `emit.Dirs` built each path with `filepath.Join(d, e.Name())` (`main_test.go:70`), a
  backslash path on Windows, and looked it up among the rendered files' paths, which `emit.File.Path` writes with a
  slash on every platform. The file was not found there, so the test read it, saw the generated header, and called it
  stale. The line is from `5124800`. It could not fail before phase 3 (`894726f`): until then no generated file was in
  a subdirectory. The generator is right: `output.staleFiles` builds the path with `emit.File.Path`
  (`output/write.go:127`), and `output`'s own tests passed on the Windows host. The Go pin has no part in it.
* **Left alone:** CI's Windows jobs run `go test -shuffle=on ./...` in `cmd/generate` (`ci.yml:117`), so both would
  fail on the first push of `894726f`.
* **Put to the owner** with three options. The other two were the fix inside 0055's commit, and a new record pair.
* **Decided:** a deviation of this PLAN, in a commit of its own, before 0055's (the owner, 2026-11-10).
* **Changed:** `cmd/generate/main_test.go:70` builds the path as `output.staleFiles` does,
  `emit.File{Dir: d, Name: e.Name()}.Path()`. The read below it joins that path to the repository root with
  `filepath.Join`, which turns the slash into the platform's separator. The file is in phase 3's row; no file is added
  to the PLAN's scope.
* **The check was seen to fail, and still fails when it should.**
  * Before the fix, on the Windows host, the one test run alone with `-v`: exit 1, the message above.
  * After the fix, on a scratch copy of the tree, the test passed here and on the Windows host (go1.27.4).
  * On that copy with `unstable/old_gen.go` planted, the generated header and `package unstable`, the test failed on
    both, exit 1: "`main_test.go:82`: `unstable/old_gen.go` is a generated file run would remove". So the test still
    finds a stale file in package unstable, and names it with a slash on Windows.
* **Checks.**
  * `make pre-add-check FILES="cmd/generate/main_test.go"` exited 0: "1 file(s) gofmt-clean; both modules linted,
    vetted and tested (-race -shuffle=on, uncached), tidy, govulncheck." It ran with 0055's pins in the tree, without
    which `govulncheck` fails (deviation 5).
  * `tools/gates.py windows` exited 0: both modules passed at `go version go1.27.4 windows/amd64`, and the host's
    directory was removed.
  * The Linux host was not run again after the fix. It had passed both modules before it, and the changed line yields
    the same path where the separator is a slash.
* The fix and this entry are staged alone, for a commit before 0055's. Deviation 5 and phase 4's record above stay
  unstaged with phase 4. Committed as `8cf69bf`, at the owner's word.

### Deviation 7, the consumer probe becomes a tool (2026-11-11)

* **Found:** in phase 5, step 1. `tools/consumer.py VERSION` fetches a release from the proxy, so it cannot run before
  the tag. To prove the consumer's new test before it, the agent wrote a scratch script: the consumer's program and
  test in a module whose requirement is a `replace` onto the working tree.
  [0052-PLAN-agent-tool-call-snapshot-stream-and-message-size-default.md](0052-PLAN-agent-tool-call-snapshot-stream-and-message-size-default.md),
  phase 6, did the same with a scratch script of its own, which was not kept.
* **Decided:** the owner, 2026-11-11: "The consumer probe should be kept for reuse. Once it is written ensure it is
  saved in the repo." No options were put: it is the owner's instruction.
* **Changed in this PLAN:** phase 5's row gains `tools/consumer_probe.py`, new. `tools/test_tools.py` and
  `tools/README.md`, already in the row, gain its tests and its row. The tool is written as the other tools are: the
  shared helpers, `--help`, `main_guard`, and the Go caches of `consumer.py` in the tool's cache directory. It is not
  added to `tools/gates.py`'s gate set: it is run by the step that changes `consumer.py`, as `consumer.py` is.
  `docs/architecture.md`, whose tooling paragraph lists the tools, gains a sentence for it; that file joins the row too.
* The MADR is not amended: no decision or asserted fact changes.
* **What was built.** `tools/consumer_probe.py [--tree DIR] [--run PATTERN]` writes `consumer.py`'s own program and test
  into a module at the `go` line, in the tool's cache directory, with `require` and `replace` onto the repository or
  `DIR`. With the floor's toolchain forced it runs `go mod tidy`, `go vet`, the build, the program, `go test` and the
  Windows build. It is as required when every step exits 0, `go version` names the floor, the program prints
  `consumer.WANT_RUN`, and `go.mod` keeps the `go` line and the replace with no toolchain line. It exits 2 for a `DIR`
  with no `go.mod`. The module is removed after; nothing is written in the tree.
* **Its tests,** in `tools/test_tools.py`: `test_consumer_probe_go_mod`, `test_consumer_probe_problems` and
  `test_consumer_probe_uses_the_consumer`. Phase 5's record below has the runs and the planted breaks.

### Phase 5, the close (2026-11-11)

Base `c36fdfe`, which is phase 4, committed at the owner's word after 0055's `4d82028`.

**Step 1, the consumer.**

* `tools/consumer.py`: the docstring names the system prompt check and the floor `v1.36.0`. Its test template gains
  `TestSystemPrompt`, on an `acptest` pair whose agent is a `ScriptAgent` that advertises
  `SessionCapabilities.SystemPrompt` with no mode and records each `session/new`'s prompt:
  * an append prompt is sent, and the agent is given it, its mode and its one text block;
  * an override prompt returns a `*CapabilityError` for `sessionCapabilities.systemPrompt.mode.override`, and the
    agent is given nothing;
  * `Negotiated`'s `AllowsValue` returns nil for the first and the same refusal for the second.
* `tools/test_tools.py`: `test_consumer_checks_the_system_prompt` requires the template's parts and the docstring's
  floor. The docstring check moved there from `test_consumer_streams_a_tool_call`. `tools/README.md`: the row.
* **Before the tag,** `python3 tools/consumer_probe.py` (deviation 7): exit 0, "as required".
  `go version go1.26.11 darwin/arm64`; `go mod tidy`, `go vet`, the build and the Windows build exited 0; the program
  printed `consumer.WANT_RUN`; `TestSystemPrompt`, `TestPresentNotice`, `TestToolCallStream` and
  `TestTranscriptReplay` passed. `--tree` on a directory with no `go.mod` exited 2.
* `python3 tools/consumer.py v1.36.0` needs the tag on the proxy: it runs after the release, step 7.

**Step 1's planted breaks,** by a scratch script, each in a copy outside the tree; 8 of 8 failed as required:

| Plant | Run | Failure |
| :--- | :--- | :--- |
| A, a copy of the tree with rule R2 removed from `model/acp.go`, regenerated | the probe, `--tree` the copy, `--run '^TestSystemPrompt$'` | exit 1, "`consumer_test.go:54`: an override prompt to an agent that advertises append alone gave `<nil>`, want a CapabilityError for sessionCapabilities.systemPrompt.mode.override" |
| B, a copy of `tools/` whose template's agent advertises nothing | that copy's probe, on the tree | exit 1, "`consumer_test.go:42`: an agent that advertises the system prompt refused an append one: session/new: capability sessionCapabilities.systemPrompt not advertised: systemPrompt" |
| C, a copy of `tools/` whose template does not check what the agent was given | `test_consumer_checks_the_system_prompt` | exit 1, "AssertionError: 'len(agent.got) != 0' not found in" the template |
| D, a copy of `tools/` whose consumer docstring says `v1.35.0` or later | the same test | exit 1, "AssertionError: 'VERSION is v1.36.0 or later' not found in" the docstring |
| E, a copy of `tools/` whose probe does not compare the program's output | `test_consumer_probe_problems` | exit 1, "AssertionError: Lists differ: [] != ['run: the program did not print consumer.WANT_RUN']" |
| F, a copy of `tools/` whose probe accepts a toolchain line | `test_consumer_probe_go_mod` | exit 1, "AssertionError: True is not false" |
| G, a copy of `tools/` whose probe does not read `consumer.WANT_RUN` | `test_consumer_probe_uses_the_consumer` | exit 1, "AssertionError: 'consumer.WANT_RUN)' not found in" the tool |
| H, a copy of `tools/` whose `consumer.WANT_RUN` names protocol version 2 | that copy's probe, on the tree | exit 1, "consumer_probe: run: the program did not print consumer.WANT_RUN" |

Plants A to D ran first against the scratch script the probe was, with the same four failures, before deviation 7.

`make check-tools`, after the last change to `tools/`: exit 0, 93 tests of `tools/`, 41 of `scripts/`,
"check-ci-filters: 0 problem(s)".

**Step 2, the gates.** `python3 tools/gates.py --all`: exit 0, "all gates passed", 18 of 18: `regen`, `precheck`,
`newfiles`, `nojsonv2`, `winvet`, `records`, `mdlint`, `idscan`, `tools`, `lint`, `fix`, `tidy`, `api`, `race3`,
`floor`, `platforms`, `windows` and `linux`.

* `floor`, which is `tools/floor.py`: `go version go1.26.11 darwin/arm64`; `go vet` and `go test -race -count=1` passed
  in both modules. The three `agentproc` tests of 0055's step 3 passed.
* `linux`: both modules, `go version go1.27.4 linux/amd64`. `windows`: both modules, `go version go1.27.4
  windows/amd64`. Each printed that its directory on the host was removed. The copy leaves out macOS `._` files
  (`_common.files_of_tree`).
* `python3 tools/interop.py`, run after the gate set: exit 0, "all pairs passed", eight pairs (`ts-http`, `ts-ws`,
  `py-http`, `py-ws`, `go-ts-http`, `go-ts-ws`, `go-py-http`, `go-py-ws`), the TypeScript SDK at `08a0d30` and the
  Python SDK at `9853d8b`. `node`, `npm`, `npx` and `uv` were present.
* **When it ran.** The gate set ran with step 1's `consumer.py`, `test_tools.py` and `tools/README.md` in the tree, and
  before `tools/consumer_probe.py`, its tests, the report and the records were. No Go file changed after it. What the
  later files touch was run again after them: `make check-tools` (above), and the pre-add check and the record gates
  (below).

**Step 3, gobble-cli, read only.** By a scratch script: `git archive d6c10a5` of gobble-cli unpacked into the scratch
directory; `go mod edit -replace` of this module onto this tree; then, at `go version go1.27.4 darwin/arm64`:
`go build ./...` exit 0, `go vet ./...` exit 0, `go test -count=1 ./...` exit 0, 38 packages ok and none failing. The
copy's `go.mod` gained the replace line alone, and its `go.sum` did not change. gobble-cli's `go` line is `1.27.3` and
it requires `v1.35.0`. Nothing was written in gobble-cli's tree: its `git status` listed the same 11 entries before and
after, its own uncommitted work.

**Step 4, `make api`,** in step 2's gate set: exit 0, "against v1.35.0: 0 incompatible, 57 compatible", the 11 `acp`
names and 46 `unstable` names of phase 3's step 10, and "reported packages unstable, remote: 0 incompatible, not
gated". The release commit is this phase's, which changes no Go file, so its API is the tree's the gate read.

**Step 5, the report.**
[0054-REPORT-release-notes-v1-36-0.md](../reports/0054-REPORT-release-notes-v1-36-0.md), in the form of 0052's. Beyond
the PLAN's list it names the Go floor and toolchain of 0055 (deviation 5), the Windows test fix (deviation 6) and the
probe (deviation 7).

**Step 6, the records.**

* **The MADR's Consequences, against what was built:** none differs, so the MADR is not amended.
  * No exported name changed or went away: `make api`, 0 incompatible.
  * 46 names are in package `unstable`: `make api`.
  * The generator gained four gate kinds and a validation list: phases 2 and 3.
  * An SDK client cannot send an unadvertised prompt, an unadvertised mode, a non-text block or an empty array, and an
    agent opts in to a `_` mode or an empty prompt with `WithInboundValidation`: `system_prompt_test.go`.
  * "134 definitions' doc comments change wording and links" was the MADR's own measurement and was not counted again.
* **The MADR's Confirmation:** `schema/version` reads `1.31.0`, `cmp -s schema/version version` exited 0, and the four
  schema files' SHA-256 are D1's, read again in this phase. The other items are phase 3's tests and this phase's steps
  2 to 4.
* **Two things the MADR did not foresee,** neither a change to a decision: `v1.36.0` also carries 0055's Go pin, and the
  repository gained `tools/consumer_probe.py`.
* `docs/README.md`: the report's row, and this PLAN's.

**Checks.**

* `make pre-add-check` with the phase's eight files (the three of `tools/` and the new tool, `docs/architecture.md`,
  `docs/README.md`, the report and this PLAN): exit 0, "0 file(s) gofmt-clean; both modules linted, vetted and tested
  (-race -shuffle=on, uncached), tidy, govulncheck."
* `make check-records`, `make markdownlint` (55 files, 0 issues) and `make disclosure-check` exited 0.
  `make disclosure-check` reads tracked files, so the two new files were also scanned by
  `python3 tools/idscan.py --paths` with the phase's eight files: "8 file(s), 21 pattern(s), 0 match(es)".
* `python3 tools/release_body.py v1.36.0 <the phase's commit>` builds the release's body from the report as committed,
  and checks its links there. It is run after the commit, and its result is in the hand-off, not here.

**Not done:** step 7, the release, waits for the owner's ask in that turn: the push, CI on the pushed commit, the tag
`v1.36.0`, the release's body and publication, and `tools/consumer.py v1.36.0`. This PLAN stays `in-progress` until
then, as 0052's did until `v1.35.0` was published.

### Phase 5, step 7, the release (2026-11-11)

* **The commit and CI on `main`.** Phase 5 was committed as `770ada6`, at the owner's word, and the owner pushed
  `main`. `python3 tools/watch_ci.py 770ada6 --branch main`: CI's run `43039146840`, "completed success", its six
  `validate` jobs (ubuntu-24.04, macos-15 and windows-2025, each at 1.26.11 and 1.27.4); the Docs run `43039146723`,
  "completed success". The first watch exited 1 when its settle time ran out with one job still running; the second,
  with `--settle 900`, exited 0.
  [0055-PLAN-pin-go-1-27-4-and-floor-1-26-11.md](0055-PLAN-pin-go-1-27-4-and-floor-1-26-11.md), step 5, has both.
* **The tag.** The owner committed 0055's completion as `3be7af6`, two record files on top of `770ada6`, and pushed
  `v1.36.0`, annotated, naming `3be7af6` (`git ls-remote --tags origin`). Its CI,
  `python3 tools/watch_ci.py 3be7af6 --branch v1.36.0 --settle 900`: exit 0, run `43042525561`, "completed success", the
  six `validate` jobs.
* **Not in step 7's order.** The step lists `git fetch` and `make push-guard` before the tag. The owner tagged and
  pushed the tag first, and `main` was not pushed with it: `origin/main` was at `770ada6`, so `3be7af6` was on GitHub
  through the tag alone. Put to the owner with three options, the others being that the owner pushes and that `main`
  stays, the owner chose that the agent push. `git fetch`, then `make push-guard`: "1 outgoing commit(s),
  770ada6..3be7af6, main to origin", "the guard passed (exit 0)". `git push origin main`: `770ada6..3be7af6`. The Docs
  run `43043527542` of that push: "completed success". CI does not run for it: the commit is docs-safe.
* **`make api`** on `3be7af6`: exit 0. With the tag fetched it compares the tree with `v1.36.0` itself, "0 incompatible,
  0 compatible". The comparison with `v1.35.0` is step 4's, on the same Go files.
* **The release.** `python3 tools/release_body.py v1.36.0 v1.36.0`: exit 0, "97 lines, 2 links", its identifier scan "0
  match(es)". Put to the owner with three options, the others being a draft and that the owner creates it, the owner
  chose that the agent publish it once the tag's CI had passed.
  `gh release create v1.36.0 --verify-tag --title v1.36.0 --notes-file …`, the title as `v1.35.0`'s: published, not a
  draft or prerelease.
* **`python3 tools/publication.py v1.36.0 3be7af6 <body>`:** exit 0, "10 of 10 checks hold": the release exists, its
  body is the extracted text, it is marked latest, the tag is annotated and names `3be7af6`, which is on origin's
  `main`, CI passed on `main`'s head and on the tag (run `43042525561`), the proxy serves the `.info` at that commit,
  `go list -m @latest` gives `v1.36.0`, and `go mod download` of the tag passes with the checksum database on.
* **`python3 tools/consumer.py v1.36.0`:** exit 0, "as required". A module at the `go 1.26.11` floor fetched `v1.36.0`
  from the proxy; its program printed `WANT_RUN`; `go test -count=1 .` passed, on the test file that holds
  `TestSystemPrompt` (the run is not verbose, so it prints the package's `ok` alone); it built for windows/amd64; its `go.mod` kept the `go` line and gained no toolchain line. It ran after the tag was
  pushed and before the release was created.

Every Verification item holds and the release is published: this PLAN is `complete`.

### After the release: reports 0051 and 0053 corrected (2026-11-11)

Not a phase of this PLAN. It is recorded here because F9 of the MADR is the fact the correction carries, and this
PLAN's release is what overtook the two reports.

* **Found** on 2026-11-11, listing open work after the release.
  [0053-REPORT-acp-1-25-0-completeness-parity-and-adherence.md](../reports/0053-REPORT-acp-1-25-0-completeness-parity-and-adherence.md)
  (`:289`, `:310`) and
  [0051-REPORT-acp-1-25-0-featureset-coverage-and-three-extensions.md](../reports/0051-REPORT-acp-1-25-0-featureset-coverage-and-three-extensions.md)
  (`:226`, `:362`) give 0025's 115 incompatible changes as the cost of a later schema at the commits they read. The
  frozen manifest of 0039 had ended that cost in `v1.23.0`. The MADR's F9 says as much: "The NES names stay in `acp`,
  built from the manifest as 0039 designed, so 0025's 115 incompatible changes do not occur."
* **The owner's word:** "proceed with the 0053 and 0051 report correction".
* **Done:** an addendum at the end of each report, dated, as
  [0032-REPORT-windows-floor-runtime-crashes.md](../reports/0032-REPORT-windows-floor-runtime-crashes.md) carries its
  later findings. Neither body is changed. Their two `docs/README.md` rows say so.
* **Checked for the addenda:** `make api BASE=v1.35.0` on `8c0170d`: exit 0, "against v1.35.0: 0 incompatible, 57
  compatible"; `SystemPrompt` is a definition of `schema/schema.json`, the stable file.
* **Not changed:** 0039's and 0025's own texts, which state their measurements with their baselines; this PLAN's
  status.
