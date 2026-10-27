---
status: accepted
date: 2026-10-26
decision-makers: Repository owner
---

# Check elicitation forms, answers and URLs with standard-library helpers on the generated types

## Context and Problem Statement

Elicitation is stable at the pinned schema `1.25.0`: `schema/meta.json` lists `elicitation/create` and `elicitation/complete`
among the client methods, and the 22 elicitation definitions of `schema.unstable.json` are identical to those of `schema.json`
(compared key by key in the session's scratch directory). [0039-MADR-five-adopter-additions-on-schema-1-25-0.md](0039-MADR-five-adopter-additions-on-schema-1-25-0.md)
left it as the next wave ("C3. `ElicitationSchema.Fields` and `Check`"), as
[0035-REPORT-consumer-acp-code-for-the-sdk.md](../reports/0035-REPORT-consumer-acp-code-for-the-sdk.md) C3 first proposed. The
owner asked, on 2026-10-26, for the most idiomatic, robust and compatible way to implement it, after evaluating this codebase, the
specification, and the sibling repositories grok-build, opencode, kilocode and codex. The owner then chose the scope: elicitation
only; the file-system and terminal hosts 0039 grouped with it stay for a later record.

Findings, from a read-only investigation at `e908875` on 2026-10-26.

### What the specification requires

From the v1 elicitation page (<https://agentclientprotocol.com/protocol/v1/elicitation.md>), quoted:

| Topic | Rule |
| :--- | :--- |
| Client checks the answer | "Clients SHOULD validate submitted values before responding" (Form mode) |
| Agent checks it again | "and Agents SHOULD validate them again." (Form mode) |
| Review | "For form mode, Clients MUST let users review and modify responses before sending them." |
| Conformance | "For an accepted form elicitation, content SHOULD conform to the requested schema." (Responses) |
| Defaults | "Clients that support schema defaults SHOULD pre-populate form fields with their declared default values." |
| Schema shape | "Form schemas are flat objects whose properties use the supported primitive and enum schemas." |
| Capabilities | "Agents MUST NOT request a mode the Client has not advertised." An omitted or `null` capability means no support; each mode is supported only when present; unlike MCP, `{}` advertises no mode ("…where an empty elicitation capability retains the original form-only meaning", said of MCP) |
| Unadvertised mode | "Requests using a mode the Client has not advertised produce JSON-RPC -32602 (Invalid params)." |
| Secrets | "Form mode MUST NOT be used to request secrets or credentials that grant access or authorize transactions"; "Agents MUST use URL mode for sensitive interactions"; without URL support "the Agent MUST NOT fall back to form mode" |
| URL consent | "Clients MUST NOT prefetch the URL or open it without explicit user consent"; the Client must show the full URL and the target host first, and SHOULD highlight the domain and warn about suspicious or Punycode hosts |
| URL accept | "An accept response means the user consented to open the URL." |
| URL credentials | Agents MUST NOT put credentials, personal data or pre-authenticated access in the URL, and SHOULD use HTTPS outside development |
| URL ids | "The Agent MUST keep each elicitationId unique among outstanding URL elicitations…", "and the Client MUST treat it as opaque"; on `elicitation/complete`, "Clients MUST ignore unknown or already-completed IDs." |
| Failure | Agents MUST NOT assume an elicitation succeeds, and MUST handle `decline`, `cancel` and failures |

The page does not say which side checks the `requestedSchema` itself, and it is silent on timeouts and on `$/cancel_request` of a
pending elicitation. The v1 prompt-turn page's cancellation rule names only `session/request_permission`
(<https://agentclientprotocol.com/protocol/v1/prompt-turn.md>): it says nothing of a pending `elicitation/create`.

The schema (`schema/schema.json`) fixes what a form may hold:

* `ElicitationSchema`: `type` `"object"`, `properties` of `ElicitationPropertySchema`, optional `required`, `title`, `description`.
* `ElicitationPropertySchema`, a union on `type`: `string` (`minLength`, `maxLength` as `uint32`; `pattern`; `format` one of
  `email`, `uri`, `date`, `date-time`; `enum`; titled `oneOf` of `EnumOption`; `default`), `number` (`minimum`, `maximum`
  inclusive, `float64`; `default`), `integer` (the same as `int64`), `boolean` (`default`), `array` (`items` of `enum` or titled
  `anyOf`; `minItems`, `maxItems` as `uint64`; `default` string list), and a catch-all that clients "MUST NOT render … as a known
  input control".
* `ElicitationContentValue`, the answer's value, `anyOf` string, `int64` integer, `double` number, boolean, string array.

### What this SDK does now

| Layer | State at `e908875` | Evidence |
| :--- | :--- | :--- |
| Types, constructors, outbound methods, dispatch | generated | `types_gen.go:3537` (`CreateElicitationRequest`), `:3934` (`CreateElicitationResponse`), `:5543` (`ElicitationSchema`), `:5265` (`ElicitationPropertySchema`), `:4555` (`ElicitationContentValue`); `agent_gen.go:854`, `:879`; `client_gen.go:163` |
| Capability gates | generated, both ways | `(*CreateElicitationRequest).gates` refuses `form` without `elicitation.form`, `url` without `elicitation.url`, and any other mode (`types_gen.go:24007`); `elicitation/complete` needs `elicitation.url` (`cmd/generate/internal/model/acp.go:85`–`:87`, `:175`–`:179`); [0019-PLAN-api-decisions.md](0019-PLAN-api-decisions.md) B20 |
| Field order | kept | `PropertyOrder` (`types_gen.go:5575`); the encoder writes `PropertyOrder`'s names first, then the rest sorted (`orderedObject`, `:23827`) ([0022-MADR-keep-elicitation-property-order-before-v1-15-0.md](0022-MADR-keep-elicitation-property-order-before-v1-15-0.md)) |
| `Validate` | structure only | one variant set per union, one scope member, a catch-all not naming a known type (`:3661`, `:3752`, `:4645`, `:5445`, `:5657`). A `required` name with no property, an unknown `format` and `minLength > maxLength` all pass |
| The answer | unchecked | the client dispatch sends the handler's response after `Validate` only (`client_gen.go:163`–`:178`); `CreateElicitation` returns it after `Validate` only (`agent_gen.go:879`) |
| Refusal | `ClientRefusals.CreateElicitation` answers `-32601`; `CompleteElicitation` returns nil | `refusals.go:23`, `:28` |
| Withdrawal | an agent whose `CreateElicitation` context ends sends `$/cancel_request` | `connection.go:2060`–`:2077` |

A probe at the floor (`go1.26.10`, in the session's scratch directory) recorded what a check would rest on:

* An answer of `5` decodes as `Integer`; `5.0`, `1e3` and `1e20` decode as `Number`; `9007199254740993` keeps every digit as
  `Integer`; `{"a":1}` and `[1]` are refused ("no variant matches"); a `null` value leaves no variant set, which `Validate` then
  refuses. So a `number` property is often answered with the `Integer` variant.
* `time.Parse(time.RFC3339, …)` refuses a lowercase `t` or `z` and a leap second `60`, both of which RFC 3339 allows; it accepts
  fractional seconds. `time.Parse(time.DateOnly, …)` refuses `2026-1-1` and `2023-02-29`.
* `net/mail.ParseAddress` accepts `Ann <a@b.example>` and ` a@b.example`, returning an address that differs from the input;
  requiring the two to be equal refuses display names and surrounding space. It accepts `a@b` and `a@-bad.com`.
* `url.Parse` accepts `/relative` and `example.com` (no scheme) and `http://x/a b` (a space in the path); it refuses a space in
  the host.
* `regexp` (RE2) refuses lookahead `(?=`, backreferences `\1` and `\cA`; `MatchString` is unanchored, as ECMA-262's `test` is.
* An unknown `format` such as `"phone"` decodes into `StringFormat` without error.

### What the sibling repositories do

Read-only surveys of each repository, with the claims this record relies on read again directly:

| Repository | Elicitation | Content checked against the schema | Notes |
| :--- | :--- | :--- | :--- |
| magic-cli-remote, this SDK's consumer | ACP client, form mode, its own 0191 MADR | yes, `checkForm` (`internal/provider/acpagent/elicitation.go:302`–`:430`): required, rune length, `format`, `pattern`, choices, bounds, item counts | trims input (`:307`); a `pattern` RE2 cannot compile is not enforced (`:232`–`:238`); integer bounds compared as `float64` (`:373`); re-implements the field order (`:151`–`:191`) |
| grok-build (Rust) | MCP client; forwards to its own ACP client through an extension method, `x.ai/mcp/elicit`, not `elicitation/create` | yes, hand-written (`crates/codegen/xai-grok-tools/src/mcp_elicitation/validate.rs`) | lossless `i64` integers (`:166`–`:171`); no trimming; rune length; unknown formats as annotations; no `pattern` support; defaults pre-fill only; the agent does not re-check |
| opencode (TypeScript) | ACP agent that sends form elicitations; MCP client | its own validator (`packages/core/src/form.ts:235`–`:370`) | refuses credential-looking fields with a regular expression before sending (`packages/cli/src/acp/elicitation.ts:22`); makes a required string `minLength ≥ 1` (`:185`); counts length in UTF-16 units; a lax date-time; collapses `decline` into `cancel` |
| kilocode (TypeScript, an opencode fork) | none | — | its MCP `elicitation: {}` capability is commented out (`packages/opencode/src/mcp/index.ts:60`–`:61`) |
| codex (Rust) | MCP client only, no ACP | no: only that required fields are answered | trims text; no `format`, length, bounds or `pattern` checks; numbers and multi-select not rendered |
| MCP Go SDK `v1.8.0` (module cache) | MCP client and server | yes, automatically on both sides, through `github.com/google/jsonschema-go` | the client refuses a malformed schema `-32602` before its handler runs (`mcp/client.go:868`–`:919`, `validateElicitSchema` `:923`); both sides validate the answer and then apply defaults to it (`:892`–`:905`; `mcp/server.go:1771`–`:1789`). `jsonschema-go` ignores `format` (`jsonschema/doc.go:84`–`:85`) and treats any integral `float64` as an integer (`jsonschema/util.go:261`) |
| ACP TypeScript SDK | elicitation stabilized in `1.4.0` (2026-08-20) | no helper, by its changelog and by the `0.19.2` code on disk | the `0.19.2` copy is in an npx cache, older than the versions opencode (`1.6.0`) and kilocode (`0.21.0`) pin, neither of which is installed |

Two conclusions. No SDK of any language that these repositories use ships a stand-alone answer check, so each adopter writes one,
and each differs: trimming or not, UTF-16 or code points, `float64` or lossless integers, `pattern` enforced, ignored or not
supported. And the one SDK that enforces the check, the MCP Go SDK, does it inside the connection with a validator that skips
`format` and loses integer precision.

### The question

How should this SDK help a client check a form and its answer, an agent check the answer again, and either side handle a URL
elicitation safely, within the `v1` promise and this repository's rules: no dependency without a record, the standard library at
the `go 1.26.10` floor, and hand-written code beside the generated types?

## Decision Drivers

* **The specification's rules, as written.** A SHOULD is a helper the caller runs; a MUST the SDK can decide alone is enforced, as
  the capability gates already are. Nothing is inferred beyond what the page and the schema say.
* **One correct implementation** of the rules every adopter otherwise writes differently: lossless integers, code-point lengths,
  the four formats, `pattern` under RE2 with its misses reported, and the choice lists.
* **Compatibility.** Additions only, under `make api`; no change to what the SDK sends or accepts on the wire unless a side asks
  for one.
* **No dependency.** `AGENTS.md`: "No dependency may be added without a MADR in this repository that names it." The standard
  library has what the rules need: `regexp`, ~~`net/mail`~~, `net/url`, `time`, `unicode/utf8`, `strconv`, `math/big`.
  *Amended 2026-10-26, by the PLAN's phase 3 deviation: `net/mail` imports `net`, which package `acp` did not import, so the
  email rule is written by hand, with `net/netip` for address literals, and package `acp` stays off `net`.*
* **The user stays in the loop.** "Clients MUST let users review and modify responses": a failed check goes back to the user, so
  the SDK must hand it to the client, not decide the answer.
* **Idiomatic Go.** Methods on the generated types, from a hand-written file, as `helpers.go` adds `SessionUpdate.Kind` and
  `SessionInfo.Updated`; errors that `errors.As` and `errors.Join` work with, reusing `*ValidationError` and its rules.

## Considered Options

* **O1.** Hand-written, standard-library helpers on the generated types, run by the caller on either side: a form's fields in
  order, its defaults, a check of the schema, a check of the answer, parsing of text input, numeric accessors, and a URL check.
* **O2.** O1, and the connection runs the answer check itself: the client dispatch refuses an `accept` whose content fails, and
  the agent's `CreateElicitation` returns an error for one, as the MCP Go SDK does.
* **O3.** A JSON Schema validator dependency, `github.com/google/jsonschema-go`, applied to the re-encoded `requestedSchema`.
* **O4.** Generated checks: protocol-table rules that emit content checks into `types_gen.go`.
* **O5.** Nothing: each adopter keeps its own check.
* **O6.** O1, and a connection option, `WithElicitationCheck()`, under which the connection runs the answer check itself on the
  side that sets it: O2, opt-in. *Added 2026-10-26 as question 2's third answer, which the owner chose.*

## Decision Outcome

Chosen option: "O6", decided by the owner on 2026-10-26 (question 2). It keeps O1's helpers, which implement every rule the
specification and schema state, once, losslessly and in the standard library, and leave the user's review loop with the
client. It adds the enforcement O2 would impose, for the sides that ask for it, so that nothing changes for a `v1` adopter that
does not set the option. O2 imposed on everyone changes what a `v1` client sends and an agent receives, on a SHOULD. O3 needs a
dependency that skips `format` and treats `1e20` as an integer. O4 cannot see the request from the response: the check needs
both, which the caller holds and, under O6, the connection is given. O5 leaves the divergence the survey found.

*The draft recommended O1. The owner's answers are under "Decided with the owner"; item 6 is the option they chose.*

### The additions

All are in a new hand-written root file, `elicitation.go`. It names ACP types, so `TestRuntimeFilesCoverHandWritten` lists it
beside `compaction.go`, `helpers.go` and `refusals.go`. Each exported name has a doc comment that opens with its name and cites
this record. No exported signature changes. Items 1 to 5 change no generated file, and every function they add is pure and safe
for concurrent use. Item 6 changes one unexported call in the generated dispatch, as it says.

#### 1. A form's fields, their kind and their defaults

* `type ElicitationField struct { Name string; Required bool; Schema ElicitationPropertySchema }`
* `func (s ElicitationSchema) Fields() []ElicitationField`: the properties in the order the encoder writes them, the names of
  `PropertyOrder` that have a property first, each once, then the rest sorted, so a form shows its fields in the order it was
  sent. `Required` is true for a name `Required` lists. A catch-all property is included; its `Kind` says what it is, and a
  client MUST NOT render it as a known control.
* `func (p ElicitationPropertySchema) Kind() string`: `string`, `number`, `integer`, `boolean`, `array`, the catch-all's `type`,
  or `""` for the zero value, as `Kind()` on `ContentBlock` and the other unions does
  ([0038-MADR-consumer-helpers-kind-and-agent-refusals.md](0038-MADR-consumer-helpers-kind-and-agent-refusals.md)).
* `func (s ElicitationSchema) Defaults() map[string]ElicitationContentValue`: each declared `default`, typed, for the
  "pre-populate" SHOULD. It is never merged into an answer. ACP asks for pre-filled fields that the user reviews and may change.
  MCP differs: its SEP-1034 has the client fill defaults into the answer, which the MCP Go SDK does on both sides
  (`ApplyDefaults`, `mcp/client.go:901`), and which Codex does not, so the five SEP-1034 checks are in the `failing` list of its
  conformance baseline (`scripts/mcp_conformance/regression-baseline-v1.json:84`–`:114`;
  `codex_conformance_adapter.py:142`–`:144`, "must materialize the JSON Schema defaults"). A bridge from MCP can merge
  `Defaults()` itself.

#### 2. A check of the schema

`func (s ElicitationSchema) CheckSchema() error` reports what makes a form impossible or ambiguous to fill, beyond `Validate`:

* a `required` name with no property;
* `minLength > maxLength`, `minimum > maximum`, `minItems > maxItems`;
* an empty `enum`, `oneOf`, `items.enum` or `items.anyOf`; a value listed twice; a string property with both `enum` and `oneOf`;
* a `format` other than the four;
* a `pattern` RE2 cannot compile, as its own rule, `RuleUnsupported`;
* a `default` its own property's rules refuse.

A client may answer a schema that fails it with `cancel`, or decline it by its own policy. An agent may run it before sending. The
SDK runs it nowhere itself, `WithElicitationCheck()` included, since the specification does not say who checks the schema.

#### 3. A check of the answer

`func (s ElicitationSchema) Check(content map[string]ElicitationContentValue) error` checks an accepted answer against its form, on
either side. It returns nil, or every problem joined with `errors.Join`, in field order, each a `*ValidationError` whose `Type` is
`"CreateElicitationAccept"` and whose `Field` is `content["<name>"]`, as `Validate` names content entries. Its message names the
field and the rule, never the value, which stays in `Value` for the caller: an answer may hold personal data. The rules:

* **Presence.** Each `required` field is present. Present is JSON Schema's meaning: an empty string or an empty list is present
  (question 5).
* **Undeclared keys.** A key with no property is refused, `RuleUndeclared` (question 1).
* **Type.** `string` takes `String`; `boolean` `Boolean`; `array` `StringArray`; `number` takes `Number` or `Integer`; `integer`
  takes `Integer`, or a `Number` that is integral and within `int64` (question 3).
* **String.** `minLength` and `maxLength` in code points (`utf8.RuneCountInString`); membership in `enum` and in `oneOf`'s
  `const`s, whichever are set; `pattern` by RE2, unanchored as ECMA-262's `test`, with a pattern RE2 cannot compile reported as
  `RuleUnsupported`, never a pass; and `format`:
  * `email`: ~~`net/mail.ParseAddress` succeeds with no display name and returns the input unchanged~~ *amended 2026-10-26,
    by the PLAN's phase 3 deviation:* an RFC 5321 §4.1.2 Mailbox, the grammar JSON Schema's `email` names, checked by hand: a
    Dot-string or Quoted-string local part, `@`, and a domain of letter-digit-hyphen labels or an address literal; ASCII only,
    since non-ASCII addresses are the separate `idn-email` format; no length limits;
  * `uri`: an absolute URI, `url.Parse` with a scheme, and no whitespace or control character;
  * `date`: RFC 3339 `full-date`, `time.DateOnly`;
  * `date-time`: RFC 3339 `date-time`, with `t` and `z` accepted in lowercase and a leap second `60` accepted, as RFC 3339 §5.6
    allows and `time.RFC3339` does not;
  * a `format` outside the four is not checked here; `CheckSchema` reports it.
* **Numbers.** `minimum` and `maximum` inclusive, compared exactly: `int64` against `int64`, and through `math/big` when an
  `Integer` meets a `float64` bound or the reverse. A non-finite `Number` is refused.
* **Array.** `minItems` and `maxItems`; each item in `items.enum` or in `items.anyOf`'s `const`s. A repeated item is allowed:
  the schema has no `uniqueItems`.
* **Catch-all.** A value for a catch-all property is not checked beyond presence.

Strings are never trimmed or changed: what the user wrote is what is checked and sent, which magic-cli-remote and Codex trim and
grok-build does not.

The rules use `*ValidationError` and the `ValidationRule` constants that exist (`RuleMinLength`, `RuleMinimum`, `RuleMaximum`),
and add, under the same type: `RuleRequired`, `RuleUndeclared`, `RuleType`, `RuleMaxLength`, `RulePattern`, `RuleFormat`,
`RuleEnum`, `RuleMinItems`, `RuleMaxItems`, `RuleUnsupported`, and for `CheckSchema` `RuleBounds`, `RuleDuplicate` and
`RuleDefault`. `ValidationError`'s doc comment, "the error a generated Validate check returns", gains these checks.

A client runs `Check` before it sends `accept`, and on failure returns the problems to the user. An agent runs it on the answer it
receives: `form.RequestedSchema.Check(resp.Accept.Content)`. The SDK runs it itself only on a side that sets
`WithElicitationCheck()` (item 6).

#### 4. Text input and numeric values

* `func (p ElicitationPropertySchema) ParseValue(text ...string) (ElicitationContentValue, error)` turns what a text interface
  collected into the typed value its property takes: one string, verbatim; a number by `strconv.ParseFloat`, finite; an integer by
  `strconv.ParseInt` base 10, losslessly, so `3.7` and `1e20` are refused, not truncated; a boolean by `strconv.ParseBool`; an
  array from every text given. A catch-all property is refused. It checks no constraint; `Check` does. magic-cli-remote and
  grok-build each wrote this conversion.
* `func (v ElicitationContentValue) Float64() (float64, bool)` for a `Number` or an `Integer`, and `Int64() (int64, bool)` for an
  `Integer` or an integral `Number` within range, so an agent reads a numeric answer whichever variant the client's encoder chose.

#### 5. URL mode

* `type ElicitationURL struct { URL *url.URL; Host string; Secure bool; Punycode bool }`
* `func ParseElicitationURL(raw string) (ElicitationURL, error)`: refuses a URL that does not parse, is not absolute, has a scheme
  other than `http` and `https`, has no host, carries userinfo, or holds whitespace or a control character; otherwise it returns
  the host to show (`Host`), whether the scheme is `https` (`Secure`), and whether any host label is Punycode, `xn--` (`Punycode`).
  A client runs it before it asks the user's consent, shows the full URL and `Host`, and warns when `Punycode` is set or `Secure`
  is not; an agent runs it before it sends. It opens nothing and fetches nothing.
* An agent's id: the documentation gives `acp.ElicitationId(rand.Text())` (`crypto/rand`, Go 1.24 and later), unique by
  construction, rather than a new function.
* A client keeps its own pending ids and ignores an `elicitation/complete` for an id it does not hold, as the specification
  requires; the documentation says so. The SDK holds no elicitation state.

#### 6. The opt-in connection check

*Added 2026-10-26, by the owner's answer to question 2.*

`func WithElicitationCheck() ConnectionOption`, in `elicitation.go`, makes the connection of the side that sets it run item 3's
`Check` on every `accept` of a form-mode `elicitation/create`, against the form it answers. Without the option nothing changes.

* **On a client**, after its `CreateElicitation` handler returns: an `accept` whose content fails is not sent. The agent is
  answered `-32603`, with the problems in `data`'s `"error"`, and the connection logs "handler result fails validation; not sent",
  as for a handler result that fails `Validate` (`connection.go:2335`–`:2343`). The handler cannot return a refused answer to the
  user, so a client that wants the user to correct an answer still calls `Check` itself first; the option is its backstop.
* **On an agent**, when the client's answer arrives: `CreateElicitation` returns an error wrapping the `*ValidationError`s,
  `elicitation/create response: …`, and the zero response, as for a response that fails `Validate`
  (`connection.go:1855`, `:1885`). A `decline` or `cancel` is returned as before.
* **What is checked.** Only a form's `accept`. A URL-mode `accept`, a `decline`, a `cancel` and a catch-all action pass as
  today. The schema itself is not checked: `CheckSchema` stays the caller's.
* **How.** The connection holds, per method, an unexported check of a result against the params it answers, `nil` by default,
  which the option sets for `elicitation/create`. The agent side runs it where `sendRequestSent` decodes a response, which already
  holds the params. The client side runs it in `checkResult`, which the generated dispatch calls without the params today
  (`cmd/generate/internal/emit/dispatch_helpers.go:124`–`:129`, `dispatch.go:668`). The emitter therefore passes `&p` to it, for
  every request method: one argument in each dispatch case of `agent_gen.go` and `client_gen.go`, and no exported change. The
  check runs after `Validate` and the gates, so a result they refuse never reaches it.
* `connection.go` stays a runtime file that names no ACP type: the check it holds is a `func(params, result any) error`, and
  only `elicitation.go` knows its types.

#### Documentation

`README.md` gains an "Elicitation" section: the two capabilities and the gates; the client's loop, `Fields`, `Defaults`,
`ParseValue`, `Check`, review, and `accept`, `decline` or `cancel`; the agent's `Check` of the answer and its fallback when a mode
is not advertised; form mode's ban on secrets; the URL consent rules and `ParseElicitationURL`; withdrawal by ending the context,
which sends `$/cancel_request`. `docs/architecture.md`'s list of hand-written files names `elicitation.go`.

### Questions for the owner

1. **An answer key the form does not declare.**
   * Recommended: refused, `RuleUndeclared`. The schema is a closed, flat form; a key no field asked for is a client defect or
     data the user was never shown. opencode's validator refuses one too (`packages/core/src/form.ts:238`).
   * Alternative: allowed, as JSON Schema allows unlisted properties when `additionalProperties` is absent. The AJV validation of
     the MCP TypeScript SDK accepts them.
2. **Whether the connection runs `Check` itself (O1 or O2).**
   * Recommended: no; the helpers are the caller's (O1).
   * Alternative: O2. The client dispatch refuses an `accept` whose content fails, `-32603` as for a result that fails `Validate`,
     and the agent's `CreateElicitation` returns the error. That is the MCP Go SDK's choice. It changes `v1` behaviour on a
     SHOULD, and a refused answer cannot go back to the user.
   * Alternative, added when the owner was asked: O2 behind a connection option, run only on a side that sets it (O6).
3. **An `integer` field answered with an integral `Number`, such as `5.0`.**
   * Recommended: accepted, since JSON Schema's integer is any number with a zero fractional part, and Python's encoder writes
     `5.0`; `Int64()` reads it.
   * Alternative: refused unless it is the `Integer` variant.
4. **URL mode in this record.**
   * Recommended: item 5 as written, a check and documentation, with no enforcement.
   * Alternative: item 5, and a refusal on send of a URL with userinfo, the one credential the SDK can recognise. That is a new
     protocol-table rule and generator work, and a change on the wire.
   * Alternative: form mode only; URL mode in a later record.
5. **What "required" means for an empty value.**
   * Recommended: present, as in JSON Schema; an agent that needs text asks for `minLength: 1`, as opencode does.
   * Alternative: an empty string or list counts as missing, as opencode's validator treats it.

The PLAN's own question, P1, is how the work is released.

#### Decided with the owner, 2026-10-26

The owner was asked each question with three options, the recommendation first, and chose:

| Question | Answer | Not chosen |
| :--- | :--- | :--- |
| 1. An undeclared answer key | As recommended: refused, `RuleUndeclared` | allowed, as JSON Schema allows; ignored by `Check` and listed by a separate `UndeclaredKeys` |
| 2. Where the check runs | Not the recommendation: an opt-in connection option, O6, item 6 | the caller only (O1); always in the connection (O2) |
| 3. `5.0` for an `integer` | As recommended: accepted when integral and within `int64`, and `Int64()` reads it | refused unless the `Integer` variant; accepted, with a `Normalize` helper that rewrites it |
| 4. URL mode | As recommended: item 5, a check and documentation, no enforcement | a refusal on send of a URL with userinfo as well; form mode only |
| 5. An empty required value | As recommended: present, as in JSON Schema | missing; present, with a helper that tightens a schema to `minLength: 1` and `minItems: 1` |
| P1. Releases | As recommended: one minor at the end | a minor after the form phases and another after URL mode; a minor per phase |

### Consequences

* Good, because a client and an agent each check an answer with one call, by every rule the schema states, losslessly, and the
  same way on both sides.
* Good, because the misses other implementations hide are reported: a `pattern` RE2 cannot compile, an unknown `format`, an
  impossible `required`.
* Good, because `Fields`, `Defaults` and `ParseValue` replace code magic-cli-remote and grok-build each wrote, in the order the
  form was sent.
* Good, because no dependency is added, and nothing on the wire changes for a side that does not set `WithElicitationCheck()`.
* Good, because a side that wants the SHOULDs enforced sets one option, and its connection refuses a failing answer on either side.
* Neutral, because an adopter that does not call the helpers or set the option behaves as today.
* Neutral, because the generated dispatch changes by one unexported argument per request method, which `TestGeneratedFilesUpToDate`
  holds to the emitter.
* Bad, because a client that relies on the option alone sends `-32603` for a failing answer instead of returning it to the user,
  which the specification's review MUST asks the client to do; the documentation says to call `Check` first.
* Neutral, because ~~`email` is checked as `net/mail` parses it, which accepts `a@b`; JSON Schema's RFC 5321 mailbox is not
  checked further~~ *(amended 2026-10-26)* `email` is checked by RFC 5321's grammar, which accepts a one-label domain such as
  `a@b` and checks no length. The rule is documented.
* Good, because package `acp` imports no `net`, so a program that imports it links no resolver for it
  (*added 2026-10-26, by the PLAN's phase 3 deviation*).
* Neutral, because RE2 and ECMA-262 differ beyond what fails to compile, for example in Unicode classes; the rule is documented as
  RE2's.
* Bad, because the root package grows by twelve names (`ElicitationField`, `Fields`, `Kind`, `Defaults`, `CheckSchema`, `Check`,
  `ParseValue`, `Float64`, `Int64`, `ElicitationURL`, `ParseElicitationURL`, `WithElicitationCheck`) and thirteen
  `ValidationRule` constants, each a `v1` promise.
* Bad, because a later schema that adds a property named `fields`, `defaults` or `check` to `ElicitationSchema`, or `kind` to a
  property schema, would collide with a method; the generator's type check refuses that output, and the method would move.
* Bad, because the SDK enforces the SHOULDs only where the option is set, so a client that neither calls `Check` nor sets it
  still sends an unchecked answer.

### Confirmation

A later PLAN shows each of the following, every new check first seen to fail on a planted break in a scratch copy:

* `Fields` gives `PropertyOrder`'s order with the rest sorted, the same order `MarshalJSON` writes, with `Required` set and a
  catch-all included; `Kind` names each variant and a catch-all's type.
* `Defaults` gives each declared default, typed, and nothing else.
* `CheckSchema` reports each listed defect once, and a schema without one returns nil.
* `Check` accepts a valid answer and reports, in field order and all together, each rule: presence, an undeclared key, each type,
  code-point lengths, each format with the probe's cases, a pattern match and a pattern RE2 cannot compile, the choice lists,
  exact integer and mixed bounds, item counts; its messages hold no answer value.
* `ParseValue` converts each kind losslessly and refuses what it cannot convert; `Float64` and `Int64` read both numeric variants.
* `ParseElicitationURL` refuses each listed URL and reports `Host`, `Secure` and `Punycode`.
* A round trip over `acptest.Connect`: an agent sends a form, a client fills it with `Fields`, `ParseValue` and `Check`, and the
  agent's `Check` of the answer passes; an answer that fails returns to the client's caller before anything is sent.
* Under `WithElicitationCheck()`, over `acptest.Connect`: a client whose handler returns a failing `accept` sends nothing for it,
  and the agent's `CreateElicitation` returns a `-32603` `*RequestError`; an agent that receives a failing `accept` from a
  client without the option gets an error wrapping `*ValidationError` and the zero response; a `decline`, a `cancel`, a URL-mode
  `accept`, and either side without the option, behave as before.
* `make api` lists only compatible changes; the generated files differ only by the params argument of `checkResult`;
  `TestRuntimeFilesCoverHandWritten` names `elicitation.go`.

## Pros and Cons of the Options

### O1. Standard-library helpers, run by the caller

* Good, because every rule is implemented once, with the precision the survey found others lose.
* Good, because the client keeps the user's review loop, as the specification requires.
* Good, because it is additive, with no wire change and no dependency.
* Bad, because the SHOULDs stay the caller's to run.

### O2. O1, enforced in the connection

* Good, because no adopter can forget the check.
* Bad, because it changes what a `v1` client sends and an agent receives, on a SHOULD.
* Bad, because a refused answer cannot be returned to the user who wrote it; the client handler has already returned.
* Bad, because a strictness choice of the SDK, such as its `email` rule, would refuse answers the adopter accepts, with no way out
  but a hook.

### O3. A JSON Schema validator dependency

* Good, because it implements JSON Schema in general.
* Bad, because `jsonschema-go` ignores `format` and treats `1e20` as an integer (`jsonschema/doc.go:84`–`:85`,
  `jsonschema/util.go:261`), so it misses two of the rules the specification's types state.
* Bad, because it is a dependency, which needs this record to name it, and a conversion from the typed schema back to raw JSON.
* Bad, because its errors are not `*ValidationError`, and its `pattern` is RE2 as well.

### O4. Generated checks

* Good, because the generator is where this SDK states its schema rules.
* Bad, because the check relates a request to its response, and the generated code handles each alone.
* Bad, because the rules are about one definition's keywords, which hand-written code reads as directly, with less machinery.

### O5. Nothing

* Good, because it costs nothing.
* Bad, because each adopter keeps a different check: the survey found trimming, UTF-16 lengths, `float64` integers and ignored
  patterns, among five implementations.

### O6. O1, with an opt-in connection check

* Good, because it has O1's helpers, and enforcement for the sides that ask for it.
* Good, because nothing changes for a side that does not set the option.
* Neutral, because the generated dispatch passes the request to `checkResult`, one unexported argument per request method.
* Bad, because a client that relies on the option alone answers `-32603` where the user should have been asked again.
* Bad, because it adds a twelfth name and a mechanism in `connection.go`, a per-method result check, that only this option uses.

## More Information

### Sources

* v1 elicitation: <https://agentclientprotocol.com/protocol/v1/elicitation.md>
* v1 prompt turn, cancellation: <https://agentclientprotocol.com/protocol/v1/prompt-turn.md>
* ACP TypeScript SDK changelog: <https://github.com/agentclientprotocol/typescript-sdk/blob/main/CHANGELOG.md> ("Stabilize
  elicitation APIs", `1.4.0`, 2026-08-20)
* RFC 3339 §5.6, `date-time`, lowercase `t` and `z`, and leap seconds
* JSON Schema 2020-12 validation: `integer`, `minLength` in characters, `format` as an annotation by default

### Tree citations

* `schema/version` = `1.25.0`; `go 1.26.10`; `toolchain go1.27.3`; HEAD `e908875`
* `ValidationRule` and `ValidationError`: `errors.go:66`, `:99`; `RuleMinLength`: `:74`
* `Kind()` precedent: `helpers.go:285` (`SessionUpdate.Kind`); `ContentBlock.Kind` from 0038
* `TestRuntimeFilesCoverHandWritten`: `cmd/generate/engine_test.go:206`, its list at `:218`

### Not this decision

* The file-system and terminal hosts (0035 C4, C5) and `agentproc.LineTap` (C7), by the owner's scope.
* `Negotiation.AllowsValue` (0035 C6).
* A scripted elicitation answer in `acptest.MemoryClient`, which still refuses elicitation through `ClientRefusals`.
* Elicitation in `example/client`, which advertises no elicitation capability.
* A credential detector for form fields, such as opencode's regular expression: a heuristic, whose misses the specification's
  MUST would then appear to cover.
* Pending elicitations cancelled on `session/cancel`: the specification names only permission requests.
