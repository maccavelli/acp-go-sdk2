---
status: in-progress
date: 2026-10-26
associated-madr: "0040-MADR-check-elicitation-forms-answers-and-urls.md"
---

# Implement 0040: elicitation form fields, schema and answer checks, text input, URL checks, and an opt-in connection check

Associated MADR: [0040-MADR-check-elicitation-forms-answers-and-urls.md](0040-MADR-check-elicitation-forms-answers-and-urls.md)

## Goal

The MADR's six items ship in the root package, hand-written in a new `elicitation.go`, on the schema `1.25.0` pin, using only the
standard library:

1. `ElicitationField`, `ElicitationSchema.Fields`, `ElicitationPropertySchema.Kind` and `ElicitationSchema.Defaults`;
2. `ElicitationSchema.CheckSchema`;
3. `ElicitationSchema.Check`, with the new `ValidationRule` constants;
4. `ElicitationPropertySchema.ParseValue`, `ElicitationContentValue.Float64` and `Int64`;
5. `ElicitationURL` and `ParseElicitationURL`;
6. `WithElicitationCheck`, the opt-in connection check.

`make api` lists only compatible changes in `acp`, `agentproc` and `acptest`. The generated files change only by the params
argument item 6 adds to `checkResult`. Nothing the SDK sends or accepts on the wire changes for a side without the option.

**Answered 2026-10-26.** The owner answered the MADR's questions 1 to 5 and this PLAN's P1, each from three options, as the MADR's
"Decided with the owner" records:

* questions 1, 3, 4 and 5 and P1 as recommended, so phases 4 and 6 and the release are as first written;
* question 2 not as recommended: the opt-in connection option (the MADR's O6, item 6). Phase 5 is added for it, and the later
  phases are renumbered.

**P1. Releases.** One minor, `v1.24.0`, at the end, after phase 7.

## Prerequisites

* [0039-PLAN-five-adopter-additions-on-schema-1-25-0.md](0039-PLAN-five-adopter-additions-on-schema-1-25-0.md) is `complete`
  (`v1.23.0`). Phases 4 and 5 use its `acptest.Connect`.
* The MADR is `accepted`, with its questions answered.

## Scope

* **Root, hand-written:**
  * `elicitation.go`, new (phases 2 to 6);
  * `errors.go` (phase 3: the rule constants, and `ValidationError`'s doc comment, which names the elicitation checks beside the
    generated ones);
  * `connection.go` (phase 5: the per-method result check in `connectionConfig` and `Connection`, `checkResult`'s params
    argument, and the check where `sendRequestSent` and `sendRequestCommit` decode a response).
* **Generator, phase 5:** `cmd/generate/internal/emit/dispatch_helpers.go` (`checkedResult`) and `dispatch.go` (the `initialize`
  case), which pass `&p` to `checkResult`; `cmd/generate/internal/emit/dispatch_test.go`'s expected lines; the regenerated
  `agent_gen.go` and `client_gen.go`.
* **Generator test, phase 2:** `cmd/generate/engine_test.go`, whose `TestRuntimeFilesCoverHandWritten` names `elicitation.go`
  beside `compaction.go`, `helpers.go` and `refusals.go`.
* **Tests, `package acp_test`:**
  * `elicitation_fields_test.go` (phase 2);
  * `elicitation_schema_test.go` (phase 3);
  * `elicitation_check_test.go` and `elicitation_wire_test.go` (phase 4);
  * `elicitation_option_test.go` (phase 5);
  * `elicitation_url_test.go` (phase 6).

  The existing in-package `elicitation_order_test.go` and `elicitation_validate_test.go` are unchanged.
* **Fixtures:** `testdata/elicitation/check/*.json`, one form, its answers and their expected problems a file (phase 4).
* **Docs:**
  * `README.md`, a new "Elicitation" section (phases 4 to 6);
  * `docs/architecture.md`: its list of hand-written files (phase 2), its account of `ValidationError` (phase 3), and the
    result check in its account of `connection.go`'s checks (phase 5).
* **Tools:** `tools/consumer.py`, in the release's commit (phase 8).
* **Records:** this pair and their rows; the release-notes report.
* **Not in scope:**
  * any other generated change; `Agent` or `Client`; `acptest`; `example/client`;
  * the file-system and terminal hosts; `Negotiation.AllowsValue`; a credential heuristic;
  * the schema check under the option;
  * cancelling pending elicitations on `session/cancel`;
  * the rest of the MADR's "Not this decision".

## Implementation Steps

Each phase ends with its checks, an entry in the execution record, and its files staged for the owner, who commits on `main`
(`AGENTS.md`, "Commits"). Every new check is seen to fail on a planted break, each in its own copy of the tree, through
`tools/plants.py` or a scratch script, and the record keeps each break's failing line.

Every code phase's gates:

* `make pre-add-check` on the changed `.go` files;
* `(cd cmd/generate && go test -count=1 ./...)` in phases 2 and 5, which change `cmd/generate`;
* `make check-records`, `make markdownlint`, `make disclosure-check` and `make api`;
* `tools/gates.py --all`, with `ACP_LINUX_HOST` and `ACP_WINDOWS_HOST` set;
* `tools/idscan.py --paths` over the staged files.

The rules below are the MADR's item text. Where a step names a behaviour the MADR states, the MADR's wording governs.

### 1. Records

* This pair and their rows in `docs/README.md`, and an "I want to…" row.
* The owner answers the MADR's questions 1 to 5 and P1 (done, 2026-10-26). The MADR is `accepted`; this PLAN becomes
  `in-progress` on the owner's approval to execute.

Checks: `make check-records`, `make markdownlint`, `make disclosure-check`, `tools/idscan.py --paths` over the three files.

### 2. Fields, kinds and defaults (MADR item 1)

`elicitation.go`, new, with its package doc reference to the MADR:

* `ElicitationField` and `ElicitationSchema.Fields`. The order is computed as `orderedObject` (`types_gen.go:23827`) computes it:
  each name of `PropertyOrder` that has a property, once, in that order, then the other property names sorted. `Required` is true
  for a name `Required` lists; a `Required` name with no property yields no field. A catch-all property yields a field.
* `ElicitationPropertySchema.Kind`: the set variant's `type` constant, the catch-all's `Type`, or `""` with no variant set.
* `ElicitationSchema.Defaults`: one entry per property whose `Default` is set (a non-nil pointer, or a non-nil slice for an
  array), typed by its variant: `String`, `Number`, `Integer`, `Boolean`, `StringArray`; `nil` when there is none. The values are
  copies, so changing the map changes nothing in the schema.

`cmd/generate/engine_test.go`: `TestRuntimeFilesCoverHandWritten` names `elicitation.go`, and its doc comment says so. Before the
edit, the test is seen to fail on the new file.

Tests, `elicitation_fields_test.go`:

* `Fields` of a schema decoded from JSON whose properties are written `b, a, c`, with `required: ["c", "zz"]`: `b`, `a`, `c`, with
  only `c` required and nothing for `zz`;
* `Fields` of a schema built in Go, with no `PropertyOrder`: sorted;
* a `PropertyOrder` that repeats a name and names a missing property: each field once, the missing one skipped;
* for each of those, and for a table of 20 orders generated by a fixed seed, `Fields`' names equal the key order of
  `json.Marshal(schema)`'s `properties`, read back with a `json.Decoder`;
* a catch-all property is a field whose `Schema.Kind()` is its type;
* `Kind` of each variant, of a catch-all, and of the zero value;
* `Defaults` of one property of each kind with a default, one without, and a catch-all: exactly the five entries, typed; changing
  the returned slice leaves the schema's `Default` as it was.

Docs: `docs/architecture.md`'s hand-written list names `elicitation.go` (0040).

Planted breaks:

| Break | The check that must fail |
| :--- | :--- |
| `Fields` sorted, ignoring `PropertyOrder` | the decoded order case and the equality with `MarshalJSON` |
| a repeated `PropertyOrder` name kept twice | the repeat case |
| `Required` not set | the decoded order case |
| a catch-all property dropped | the catch-all case |
| `Kind` of a catch-all `""` | the `Kind` case |
| `Defaults` returns the schema's slice | the copy case |
| `elicitation.go` missing from `TestRuntimeFilesCoverHandWritten` | that test, through `cmd/generate` |

### 3. The schema check (MADR item 2)

* `errors.go`: the thirteen constants the MADR names, each with a one-line comment in the file's form, and `ValidationError`'s doc
  comment extended to the elicitation checks, with the rule that their messages name the field and the rule and never the value.
* `elicitation.go`: `ElicitationSchema.CheckSchema`. Each defect is a `*ValidationError` with `Type` `"ElicitationSchema"` and a
  `Field` in the generated checks' form (`required[1]`, `properties["age"].maximum`). They are joined with `errors.Join`, in field
  order, then in `required`'s order for undeclared names:
  * a `required` name with no property: `RuleUndeclared`;
  * `minLength > maxLength`, `minimum > maximum`, `minItems > maxItems`: `RuleBounds`;
  * an empty `enum`, `oneOf`, `items.enum` or `items.anyOf`: `RuleEnum`; a value or `const` listed twice: `RuleDuplicate`;
    both `enum` and `oneOf` on one string property: `RuleEnum`;
  * a `format` other than the four: `RuleFormat`;
  * a `pattern` `regexp.Compile` refuses: `RuleUnsupported`;
  * a `default` its property's own rules refuse, by the same code phase 4's `Check` runs on one value: `RuleDefault`, wrapping
    the value's problem.

  Phase 3 writes the per-value rules `RuleDefault` needs, as unexported functions, and phase 4's `Check` calls them.

Tests, `elicitation_schema_test.go`, table-driven, one case per defect, each with its `Field` and `Rule` read through
`errors.As` over the joined errors, and:

* a schema of every kind with every keyword set validly: nil;
* a schema with three defects: three problems, in field order;
* a `pattern` with lookahead: `RuleUnsupported`, and `errors.As` finds the `*ValidationError`.

Planted breaks:

| Break | The check that must fail |
| :--- | :--- |
| undeclared `required` names accepted | its case |
| `minimum > maximum` accepted | its case |
| a duplicate `const` accepted | its case |
| an uncompilable `pattern` accepted | its case |
| an unknown `format` accepted | its case |
| only the first defect returned | the three-defect case |

### 4. The answer check, text input and numeric values (MADR items 3 and 4)

* `elicitation.go`: `ElicitationSchema.Check`, by the MADR's rules.
  * Problems come in field order, then the undeclared keys sorted. Each is a `*ValidationError` with `Type`
    `"CreateElicitationAccept"` and `Field` `content["<name>"]`.
  * Within one field, a type mismatch is that field's only problem; otherwise each broken rule is reported.

  Specifics the MADR leaves to the code:
  * an `Integer` against a `float64` bound, or an integral `Number` against an `int64` bound, is compared with `big.Float` at
    precision 64 or more, exact for both;
  * `date-time`: the value is checked as RFC 3339 after `t` and `z` are read as `T` and `Z`, and a seconds field of `60` as `59`;
  * a `pattern` is compiled once per `Check`, and an uncompilable one is `RuleUnsupported` for every value it would check.
* `ElicitationPropertySchema.ParseValue`, `ElicitationContentValue.Float64` and `Int64`, by the MADR. `ParseValue`'s failures are
  `*ValidationError`s with `Rule` `RuleType`; it never trims.
* Fixtures, `testdata/elicitation/check/*.json`. Each holds a form's `requestedSchema` and a list of answers, each an answer's
  `content` as raw JSON and its expected problems as `{field, rule}`. The test decodes both with the SDK's types, so each answer
  passes through the wire decoding the probe recorded (`5.0` as `Number`, `1e20` as `Number`). The fixtures cover, at least:
  * presence, an empty string and an empty list as present, `content: null` and `{}` against a required field;
  * an undeclared key;
  * each type, a `Number` and an `Integer` for `number`, an integral `Number` for `integer`, and `1e20` for `integer`;
  * code-point lengths with `héllo` and `👍🏽`;
  * each format with the probe's cases: `a@b.example` and `Ann <a@b.example>`; `https://example.com/a`, `urn:isbn:1`,
    `/relative` and `http://x/a b`; `2024-02-29`, `2023-02-29` and `2026-1-1`; `2026-10-26t12:00:00z`, `2016-12-31T23:59:60Z` and
    `2026-10-26T12:00Z`;
  * a `pattern` match, a miss, and a pattern with lookahead;
  * `enum`, `oneOf`, `items.enum` and `items.anyOf` membership, and a repeated item;
  * the bounds exactly: `9007199254740993` against `maximum: 9007199254740992` as `integer`, and against a `number` maximum of
    `9007199254740992.0`;
  * `minItems` and `maxItems`;
  * a value for a catch-all property.

Tests:

* `elicitation_check_test.go`:
  * the fixtures, and that no problem's `Error()` holds the answer's value;
  * `ParseValue` per kind: `3.7`, `1e20` and `9007199254740993` for `integer`; `NaN` and `Inf` for `number`; `yes` for
    `boolean`; a catch-all;
  * `Float64` and `Int64` over both numeric variants, an integral and a fractional `Number`, and `1e20`.
* `elicitation_wire_test.go`, over `acptest.Connect`, the client advertising `elicitation.form`, and neither side setting the
  option:
  * a test client embeds `acptest.MemoryClient` and answers `CreateElicitation` by `Fields`, `ParseValue` from a script, and
    `Check`;
  * its first scripted answer fails `Check`, which is returned to the script as a user would see it; the second passes and is
    sent as `accept`;
  * the agent's `CreateElicitation` returns it, and the agent's `Check` of it is nil;
  * a `decline` from the script reaches the agent as `decline`.

Docs: `README.md`'s "Elicitation" section, for forms, as the MADR's "Documentation" lists.

Planted breaks:

| Break | The check that must fail |
| :--- | :--- |
| lengths in bytes (`len`) | the `héllo` fixture |
| strings trimmed before the check | a fixture with surrounding space against `pattern` `^\S+$` |
| an `Integer` refused for `number` | its fixture |
| integer bounds through `float64` | the `9007199254740993` fixture |
| an uncompilable `pattern` treated as a match | the lookahead fixture |
| lowercase `t` and `z` refused | its fixture |
| `Ann <a@b.example>` accepted as `email` | its fixture |
| an undeclared key accepted | its fixture |
| a value in a message | the no-value check |
| only the first problem returned | a fixture with two problems |
| `ParseValue` for `integer` through `ParseFloat` | the `3.7` case |
| `Int64` of a fractional `Number` true | its case |
| the test client sends without `Check` | the wire test's first answer |

### 5. The opt-in connection check (MADR item 6)

* `connection.go`, which names no ACP type:
  * `connectionConfig` and `Connection` gain `resultChecks map[string]func(params, result any) error`, nil by default, copied
    from the config in `buildConnection`;
  * `checkResult(method, params, result, gate)` runs the method's check, when one is set, after `validateOutbound` and the gate.
    A failure is answered and logged as those are (`connection.go:2335`–`:2343`);
  * `sendRequestSent` and `sendRequestCommit` run it after `checkInbound` on a decoded response (`:1855`, `:1885`). A failure is
    returned as theirs is: `"<method> response: %w"`, and the zero result.
* `cmd/generate/internal/emit`: `checkedResult` (`dispatch_helpers.go:124`–`:129`) and the `initialize` case
  (`dispatch.go:668`) pass `addr(id("p"))` as the second argument. `dispatch_test.go`'s expected lines follow. Regenerated:
  `agent_gen.go` and `client_gen.go`. A scratch script requires every changed line of the two files to be a `checkResult` call
  gaining `&p`, and no other line to change.
* `elicitation.go`: `WithElicitationCheck()`. It sets the check for `ClientMethodElicitationCreate`, which takes a
  `*CreateElicitationRequest` and a `*CreateElicitationResponse`. It runs `Check` only when the request is `Form` and the
  response is `Accept`, and returns nil otherwise.

Tests, `elicitation_option_test.go`, over `acptest.Connect`, the client advertising `elicitation.form`:

* the client sets the option, and its handler returns a failing `accept`: the agent's `CreateElicitation` gets a `*RequestError`
  with code `-32603`, whose `Reason()` names the field and the rule; an `acptest.Recorder` on the client's writer shows no
  `accept` frame; the client's log has "handler result fails validation; not sent";
* the agent sets the option, and a client without it returns a failing `accept`: `CreateElicitation` returns an error wrapping a
  `*ValidationError` with `Rule` `RuleRequired`, and the zero response;
* with the option on both sides: a passing `accept`, a `decline`, a `cancel`, and a URL-mode `accept` with no content each
  arrive as sent;
* with the option on neither side: the failing `accept` arrives unchecked, as phase 4's wire test shows.

The engine's behaviour cases and `TestGeneratedFilesUpToDate` run through `cmd/generate`'s tests.

Docs:

* `README.md`'s section gains the option: what it checks, on each side, and that a client calls `Check` first so the user can
  correct an answer.
* `docs/architecture.md`: the result check in its account of `connection.go`'s checks.

Planted breaks:

| Break | The check that must fail |
| :--- | :--- |
| the client side's check not run (`checkResult` ignores the method's check) | the client-option case |
| the agent side's check not run (`sendRequestSent` ignores it) | the agent-option case |
| the check run for a `decline` | the `decline` case |
| the check set without the option, by default | the neither-side case |
| `checkedResult` emitted without `&p` | `cmd/generate`'s `dispatch_test.go`, and the build of the root |
| a changed generated line other than `checkResult`'s | the scratch script over the regenerated files |

### 6. URL mode (MADR item 5)

* `elicitation.go`: `ElicitationURL` and `ParseElicitationURL`, by the MADR.
  * `Host` is the URL's host name, lowercased, without port or brackets.
  * `Punycode` is true when any dot-separated label of it starts with `xn--`, in any case.
  * Failures are `*ValidationError`s with `Type` `"CreateElicitationUrl"`, `Field` `url` and `Rule` `RuleFormat`, naming which
    rule, never the URL.

Tests, `elicitation_url_test.go`, table-driven:

* refused: `""`, `not a url`, `/relative`, `example.com`, `javascript:alert(1)`, `file:///etc/passwd`,
  `https://user:pw@example.com/`, `https://user@example.com/`, `https:///path`, `https://exa mple.com`,
  `https://example.com/a b`, and a URL holding `\x7f`;
* accepted: `https://example.com/a?b=1` (`Secure`); `http://localhost:8080/cb` (not `Secure`); `https://XN--80AK6AA92E.com/`
  (`Punycode`, `Host` lowercased); `https://[::1]:8443/` (`Host` `::1`).

Docs: the README section's URL subsection, with `acp.ElicitationId(rand.Text())` for an id and the client's rule for unknown ids
on `elicitation/complete`.

Planted breaks:

| Break | The check that must fail |
| :--- | :--- |
| userinfo accepted | the two userinfo cases |
| any scheme accepted | `javascript:` and `file:` |
| `xn--` matched case-sensitively | the uppercase Punycode case |
| whitespace in the path accepted | its case |

### 7. Close the implementation

* `make api` against the newest `v1` tag: only compatible changes, the MADR's twelve names and thirteen constants; record what it
  lists.
* `tools/gates.py --all`.
* `README.md`, `docs/architecture.md` and `errors.go`'s `ValidationError` doc read in full against the code.
* The MADR's "Confirmation" walked item by item in the execution record, each with the check that meets it.

### 8. The release, only on the owner's ask in that turn

Through `RELEASING.md`, "Tag and Publish", as 0039's phase 8 ran:

* the release-notes report, `0040-REPORT-release-notes-v1-24-0.md`, and its row, committed before the tag;
* in the same commit, `tools/consumer.py` extended to use `Fields`, `ParseValue`, `Check` and `WithElicitationCheck`, and
  `make check-tools`;
* `tools/release_body.py`;
* the annotated tag and its push, on the owner's ask;
* the GitHub release, then `tools/publication.py` and `tools/consumer.py v1.24.0`.

When the release is published, this PLAN and its row are `complete`.

## Verification

* Every item of the MADR's "Confirmation" holds, each by its phase's tests.
* `make api` lists only compatible changes in `acp`, `agentproc` and `acptest`.
* `git diff` against the PLAN's starting commit:
  * in `*_gen.go`, changes only the `checkResult` calls of `agent_gen.go` and `client_gen.go`;
  * touches no file outside the Scope.
* `TestRuntimeFilesCoverHandWritten`, `TestGeneratedFilesUpToDate` and `TestExportedDeclarationsDocumented` pass.
* `make pre-add-check`, `make lint`, and `tools/gates.py --all` at every code phase, on the Linux and Windows hosts and at the
  floor, `GOEXPERIMENT=nojsonv2` included.
* `make disclosure-check` and `tools/idscan.py`: nothing found.
* Every planted break in phases 2 to 6 fails its check, each in its own copy of the tree, and the execution record keeps each
  one's failing line.

## Rollout and Rollback

* **Rollout.** Phases 2 to 6 are separate commits. Under P1, one minor, `v1.24.0`, carries them. Each consumer adopts it under
  its own records: magic-cli-remote can replace `checkForm`, its field order and its string conversion.
* **What a consumer meets on the release.** New names only. Nothing it sends or receives changes, and no check runs, unless it
  calls a helper or sets `WithElicitationCheck()`.
* **Rollback before a tag:** revert the phase's commits.
* **Rollback after a tag:** a name cannot be removed under `v1`; a defect is fixed forward in a patch. A rule that proves too strict,
  such as the `email` rule, is relaxed in a patch, which only accepts more answers.

## Execution record

### Phase 1, records, 2026-10-26

* The pair and its rows were written, then amended for the owner's answers to questions 1 to 5 and P1, each asked with three
  options. The MADR is `accepted`, with item 6 and option O6 added for question 2; this PLAN gained phase 5. The owner committed
  them as `52c9ee8` and approved execution: "committed, proceed".
* Checks, each exit 0: `make check-records`, `make markdownlint`, `make disclosure-check`; `tools/idscan.py --paths` over the
  three files: 0 matches.

### Phase 2, fields, kinds and defaults, 2026-10-26

* **`elicitation.go`**, new: `ElicitationField`; `ElicitationSchema.Fields`, whose order is computed as `orderedObject`'s;
  `ElicitationPropertySchema.Kind`; `ElicitationSchema.Defaults`, which clones an array default and is nil when no property has
  one. Unexported constants name the five property types.
* **`cmd/generate/engine_test.go`:** `TestRuntimeFilesCoverHandWritten` and its doc comment name `elicitation.go`. Before the
  edit the test failed on the new file: "hand-written files of the root are [compaction.go connection.go doc.go elicitation.go
  …], want [compaction.go connection.go doc.go errors.go …]".
* **Tests**, `elicitation_fields_test.go`, `package acp_test`:
  * `TestElicitationFields`: the decoded form (`b`, `a`, `c`; only `c` required; nothing for `zz`), a form built in Go
    (sorted), and a repeated and missing `PropertyOrder` name, each also compared with the key order `json.Marshal` writes,
    read back by a `json.Decoder`;
  * `TestElicitationFieldsMatchWire`: 20 property sets and orders from a fixed PCG seed, repeats and missing names among them;
  * `TestElicitationPropertyKind`: each variant, a catch-all, the zero value, and a decoded catch-all property kept by
    `Fields`, required, of kind `x-color`;
  * `TestElicitationDefaults`: five typed defaults, `9007199254740993` kept exactly as `Integer`; a property without one and a
    catch-all with one left out; the array copied; nil for a form without defaults.
* **Docs:** `docs/architecture.md`'s hand-written list names `elicitation.go`.
* **Planted breaks:** each was planted in its own copy of the tree through a scratch script, and each failed as required:

  | Break | Failing line |
  | :--- | :--- |
  | the row's: `Fields` sorted, ignoring `PropertyOrder` | `elicitation_fields_test.go:90: decoded: Fields = ["a" "b" "c"] …`; `:93: … but MarshalJSON writes ["b" "a" "c"]` |
  | the row's: a repeated name kept twice | `:90: repeated and missing: Fields = ["c" "c" "a" "b"] …`; `:93` the same against MarshalJSON |
  | the row's: `Required` not set | `:90: decoded: Fields = ["b" "a" "c"], required []; want … required ["c"]` |
  | the row's: a catch-all property dropped | `:141: Fields of a form with an unknown property type = [{Name:name …}]` |
  | the row's: `Kind` of a catch-all `""` | `:135: Kind = "", want "x-color"` |
  | the row's: `Defaults` returns the schema's slice | `:170: changing a default changed the schema: ["changed"]` |
  | the row's: `elicitation.go` not named | `engine_test.go:220: hand-written files of the root are [compaction.go connection.go doc.go elicitation.go …]` |

* **Checks**, each exit 0:
  * `make pre-add-check`: "206 file(s) clean". It runs `gofmt` over tracked files only, so `scripts/go-precheck.sh` was run on
    the two new files as well: "2 file(s) clean";
  * `(cd cmd/generate && go test -count=1 -run '^TestRuntimeFilesCoverHandWritten$' .)`; the new tests under `-race`;
  * `tools/gates.py --all`, with the Linux and Windows hosts reached over SSH: all 18 `ok`. `newfiles` covered
    `elicitation.go` and `elicitation_fields_test.go`. `api` reported "against v1.23.0: 0 incompatible, 4 compatible":
    `ElicitationField`, `ElicitationPropertySchema.Kind`, `ElicitationSchema.Defaults` and `ElicitationSchema.Fields`, each
    added;
  * `make check-records`, `make markdownlint`, `make disclosure-check`; `tools/idscan.py --paths` over the changed and new
    files: 0 matches.

### Deviation, 2026-10-26: phase 3, `net/mail` brings package `net` into `acp`

* **Found** by phase 3's first `tools/gates.py --all`. 17 gates were `ok`. `platforms` failed eight of its 13 checks with C
  compile errors in Go's runtime while cross-building `agentproc` for linux, freebsd, openbsd, netbsd and illumos, such as "`note:
  'setreuid' declared here`" and "`error: use of undeclared identifier '_SIG_WORDS'`".
* **Cause.** The MADR's `email` rule used `net/mail`, which imports `net`. `go list -deps .` showed that package `acp` did not
  depend on `net` before the phase, and did after it, with `mime`, `net/textproto` and the DNS packages; `agentproc` imports
  `acp`. The host's Go environment file has set `CGO_ENABLED=1` since 2026-10-20, so `net`'s cgo resolver was compiled for each
  cross target with the macOS clang, which fails.
* **Measured.** A copy of `HEAD` vets for linux with the same environment: exit 0. The working tree with `CGO_ENABLED=0`: exit 0.
  `net/url` and `net/netip` do not import `net`.
* **Consequence beyond the gate:** every program that imports `acp` would have linked `net`, and on Unix with cgo on, its
  resolver. The MADR did not state it.
* **Pre-existing:** no; introduced by this phase, before any commit.
* **Decided** by the owner, out of three resolutions: "Hand-written addr-spec". Not chosen: keeping `net/mail` and setting
  `CGO_ENABLED=0` in `tools/platforms.py`; both.
* **The fix.**
  * `elicitation.go`'s `isMailbox` checks an RFC 5321 §4.1.2 Mailbox, the grammar JSON Schema's `email` names, by hand: a
    Dot-string or Quoted-string local part, `@`, and letter-digit-hyphen labels or an address literal, `net/netip` parsing an
    IPv4 or IPv6 literal. It is ASCII only, as `idn-email` is a separate format, and checks no length.
  * `TestElicitationEmailFormat` covers 28 addresses, the probe's among them. Unlike `net/mail`, the rule now refuses
    `a@-bad.com`, and accepts `"x y"@b.example`, which RFC 5321 allows.
  * `TestRootDoesNotImportNet` walks package `acp`'s imports with `go/build` and fails when `net` is reached, so the guard does
    not depend on a host's cgo setting.
* **The MADR** is amended where it named `net/mail`: the decision driver listing the standard-library packages, item 3's `email`
  rule, and the `email` consequence, with a new consequence that `acp` imports no `net`.
* **Scope added to phase 3:** `TestElicitationEmailFormat` and `TestRootDoesNotImportNet`, in `elicitation_schema_test.go`.

### Phase 3, the schema check, 2026-10-26

* **`errors.go`:** the thirteen constants, `RuleRequired` to `RuleDefault`, in a block of their own citing the MADR, each with a
  one-line comment. `ValidationRule`'s and `ValidationError`'s doc comments name the elicitation checks, and say their messages
  never quote a value. `errors.go` is a runtime file; the engine cases' API listings cover generated files only
  (`cmd/generate/internal/enginetest/api_test.go:41`), so none changed.
* **`elicitation.go`:**
  * `ElicitationSchema.CheckSchema`, with `schemaError` and `checkPropertySchema`. Problems come in `Fields` order, then the
    undeclared `required` names. Within a property: bounds, empty or repeated choices, both `enum` and `oneOf`, `format`,
    `pattern`, `default`.
  * A refused default is one `defaultError`, whose `Unwrap` gives the `RuleDefault` problem first, then each rule broken, so
    `errors.As` finds `RuleDefault`.
  * The per-value rules phase 4's `Check` will share: `checkValue`, `checkString`, `checkNumber` (an `Integer` against a
    `float64` bound through `math/big`), `checkInteger` (`integral` takes a whole `Number` within `int64`), `checkArray`,
    `knownFormat`, `matchesFormat` and `rfc3339Strict` (a lowercase `t` and `z`, and seconds `60`).
  * Lengths are compared as `int64`, which gosec accepts for a rune count where a `uint64` conversion is flagged (G115).
* **Tests**, `elicitation_schema_test.go`, `package acp_test`:
  * `TestElicitationCheckSchema`: 25 cases, one defect each, each read by its `Field` and `Rule` with `errors.As` over the
    joined errors. They cover an undeclared `required` name; the four bounds; the four empty lists of choices; the four
    duplicates; `enum` and `oneOf` together; an unknown `format`; a lookahead and a backreference `pattern`; and eight refused
    defaults, each also by the rule it breaks (`maxLength` with `héllo`, `enum`, `format`, `pattern`, `maximum`, an exact
    `minimum` at `9007199254740993`, an item not a choice, too many items);
  * `TestElicitationCheckSchemaValid`: a form of every kind with every keyword set validly, a catch-all among them: nil. Three
    defects, written `c`, `a`, `b`, come back in that order. `errors.As` of an uncompilable pattern finds `RuleUnsupported`;
  * `TestElicitationEmailFormat` and `TestRootDoesNotImportNet`, from the deviation.
* **Docs:** `docs/architecture.md`'s hand-written list gains `CheckSchema`, and its account of `ValidationError` the elicitation
  checks.
* **Planted breaks:** each was planted in its own copy of the tree through a scratch script, and each failed as required, on the
  final code:

  | Break | Failing line |
  | :--- | :--- |
  | the row's: undeclared `required` names accepted | `elicitation_schema_test.go:97: undeclared required: CheckSchema = [] (<nil>), want {required[1] undeclared}` |
  | the row's: `minimum > maximum` accepted | `:97: number bounds: CheckSchema = [] (<nil>), want {properties["n"].maximum bounds}` |
  | the row's: a duplicate `const` accepted | `:97: duplicate oneOf: CheckSchema = [] (<nil>), want {properties["s"].oneOf[1].const duplicate}` |
  | the row's: an uncompilable `pattern` accepted | `:97: lookahead pattern: CheckSchema = [] (<nil>), want {properties["s"].pattern unsupported}` |
  | the row's: an unknown `format` accepted | `:97: unknown format: CheckSchema = [] (<nil>), want {properties["s"].format format}` |
  | the row's: only the first defect returned | `:136: CheckSchema of three defects = [{properties["c"].enum enum}], want […3 problems…]` |
  | a default not checked | `:97: default too long: CheckSchema = [] (<nil>), want {properties["s"].default default}` |
  | a lowercase `t` and `z` refused | `:127: CheckSchema of a valid form = … properties["seen"].default: default: … is not a valid date-time, want nil` |
  | the deviation's: `net/mail` imported | `:211: package acp imports net, through [net/mail net]` |
  | the deviation's: a hyphen-edged domain label accepted | `:188: email "a@-bad.com": accepted true, want false`; the same for `a@bad-.com` |
  | the deviation's: a display name accepted | `:188: email "Ann <a@b.example>": accepted true, want false` |

* **Fixes the checks asked for:** `errorlint`, on the test's error walk, rewritten with `errors.AsType`; `gosec` G115 on a rune
  count; and, on the hand-written email code, `gocritic` `initClause`, `intrange`, `modernize` `stringsseq` and staticcheck
  QF1001.
* **Checks**, each exit 0:
  * `make pre-add-check`: "208 file(s) clean"; `scripts/go-precheck.sh elicitation_schema_test.go`, the new file;
  * `tools/platforms.py`: "13 of 13 passed";
  * `tools/gates.py --all`, with the Linux and Windows hosts reached over SSH: all 18 `ok` on the second run, the first being the
    deviation's. `api` reported "against v1.23.0: 0 incompatible, 18 compatible": phase 2's four names, `CheckSchema`, and the
    thirteen constants;
  * `make check-records`, `make markdownlint`, `make disclosure-check`; `tools/idscan.py --paths` over the changed and new
    files: 0 matches.
