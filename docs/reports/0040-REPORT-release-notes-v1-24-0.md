# Release notes for v1.24.0, against v1.23.0

Observed 2026-10-27 on `8baf2e5`, for
[0040-PLAN-check-elicitation-forms-answers-and-urls.md](../decisions/0040-PLAN-check-elicitation-forms-answers-and-urls.md),
phase 8. This report drafts the text of the GitHub release for `v1.24.0`, as `RELEASING.md` describes it. It decides nothing. It is
committed before the tag, so the tagged tree holds it; the commit that holds it changes only records and `tools/`, so the API it
tags is `8baf2e5`'s.

Each statement names its source:

- a record;
- a commit;
- or `make api`, run on `8baf2e5` with only `tools/consumer.py` and `tools/README.md` uncommitted, which hold no Go package.

Summary:

- Twenty-five additions to the root package's exported API; `agentproc`'s and `acptest`'s are unchanged. `make api` against
  `v1.23.0`: "0 incompatible, 25 compatible". The release is the minor `v1.24.0`, as `RELEASING.md` says for compatible changes.
- What a consumer may meet: nothing the SDK sends or accepts on the wire changes unless a side sets the new
  `WithElicitationCheck()`. The `go` lines and the requirements are unchanged, and package `acp` still imports no `net`.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.24.0

The ACP schema is still `1.25.0` (`git diff v1.23.0 -- schema/` is empty). This release adds what the specification asks of both
sides of an elicitation and leaves to them: a client SHOULD check a form's answer before it sends it, and an agent SHOULD check it
again ([0040-MADR-check-elicitation-forms-answers-and-urls.md](../decisions/0040-MADR-check-elicitation-forms-answers-and-urls.md)).
The adopters and the SDKs surveyed for it each wrote that check differently, or not at all. `make api` reports twenty-five
compatible additions and no incompatible change. Nothing sent or accepted on the wire changes unless a side opts in.

#### New for clients

- **`ElicitationSchema.Fields()`** lists a form's fields in the order the agent wrote them, the same order the SDK sends, each
  with whether it is required. **`ElicitationPropertySchema.Kind()`** names a field's type; a type this package does not know is
  a field too, which a client must not render as a known control. **`ElicitationSchema.Defaults()`** gives each declared default,
  typed, to pre-fill the form. (`e35a03e`.)
- **`ElicitationPropertySchema.ParseValue(text...)`** turns what a text interface collected into the value its field takes,
  without rounding or trimming: `3.7` and `1e20` are not integers, `9007199254740993` keeps every digit. (`9a6bf02`.)
- **`ParseElicitationURL(raw)`** refuses a URL-mode elicitation's URL that no client should offer to open: not absolute http or
  https, no host, userinfo, whitespace or a control character. It returns the host to show, whether the URL is https, and
  whether the host is Punycode. It opens nothing. (`7fd8322`.)

```go
schema := req.Form.RequestedSchema
for _, f := range schema.Fields() {
    // show f.Name, f.Required, f.Schema.Kind(), pre-filled from schema.Defaults()
}
if err := schema.Check(content); err != nil {
    // show the problems to the user, who corrects the answer
}
```

#### New for both sides

- **`ElicitationSchema.Check(content)`** checks an answer against its form by every rule the schema states, and returns every
  problem at once, in field order, each a `*ValidationError` whose message never quotes the value. A required field must be
  present, though an empty string or list is present; a key the form does not declare is refused; a number takes an `Integer`
  too, and an integer a whole `Number` such as `5.0`; lengths count characters; bounds compare exactly, `9007199254740993`
  above `9007199254740992`; `email` is an RFC 5321 mailbox, `uri` an absolute URI, `date` and `date-time` RFC 3339, lowercase
  `t` and `z` and leap seconds included; a `pattern` is matched by Go's RE2, and one RE2 cannot compile is reported as
  `RuleUnsupported`, never as a match. Nothing is trimmed. (`9a6bf02`.)
- **`ElicitationSchema.CheckSchema()`** reports a form no answer can satisfy, or that is ambiguous: a `required` name with no
  property, bounds the wrong way round, an empty or repeated list of choices, an unknown `format`, a `pattern` RE2 cannot compile,
  a `default` its own field refuses. (`2f974dd`.)
- **`ElicitationContentValue.Float64()`** and **`Int64()`** read a numeric answer whichever variant the client's encoder chose.
  (`9a6bf02`.)
- **`WithElicitationCheck()`** makes the connection of the side that sets it run `Check` on every form `accept`. On a client, an
  answer that fails is not sent and the agent is answered `-32603`; on an agent, `CreateElicitation` returns an error wrapping
  the problems. A `decline`, a `cancel` and a URL-mode `accept` pass as before. A client still calls `Check` first, where the
  user can correct the answer. (`e6237f0`.)
- **Thirteen `ValidationRule` constants**, `RuleRequired` to `RuleDefault`, name the rules the checks report, beside the
  existing `RuleMinLength`, `RuleMinimum` and `RuleMaximum`. (`2f974dd`.)

#### For maintainers

- **The generated dispatch** passes the request to `checkResult` as well as the handler's result, so a connection can hold a
  per-method check of a result against the request it answers: one unexported argument in each request case of `agent_gen.go`
  and `client_gen.go`, and nothing else in a generated file. (`e6237f0`.)
- **`TestRootDoesNotImportNet`** keeps package `acp` off package `net`. The `email` rule is written by hand, because `net/mail`
  would have made every program that imports `acp` link `net`. (`2f974dd`.)
- **Tools.** `tools/consumer.py` builds a module at the Go floor that also checks an elicitation answer through `Fields`,
  `ParseValue` and `Check`, and connects its test's pair with `WithElicitationCheck`.
- **Tests.** New at the root: `elicitation_fields_test.go`, `elicitation_schema_test.go`, `elicitation_check_test.go` with the
  57 answers of `testdata/elicitation/check/`, `elicitation_wire_test.go`, `elicitation_option_test.go` and
  `elicitation_url_test.go`. `TestRuntimeFilesCoverHandWritten` names `elicitation.go`.

## Sources not in the release text

- **Commits since `v1.23.0`** (`git log v1.23.0..8baf2e5`), eight:
  - `e35a03e`, `2f974dd`, `9a6bf02`, `e6237f0` and `7fd8322` change the package, as above; `e6237f0` also the generator's
    emitter and the two dispatch files;
  - `8baf2e5` changes `errors.go`'s `ValidationError` doc, `docs/architecture.md` and the records;
  - `e908875` and `52c9ee8` change records only (`git show --name-only`).
- **Go files changed.** `git diff --name-only v1.23.0 8baf2e5 -- '*.go' ':!*_test.go'` lists seven: `elicitation.go`,
  `errors.go`, `connection.go`, `agent_gen.go`, `client_gen.go`, and the emitter's `dispatch.go` and `dispatch_helpers.go`.
  `go.mod`, `go.sum`, `cmd/generate/go.mod`, the schema, the examples, `scripts/`, the workflows and `RELEASING.md` are unchanged
  (`git diff`).
- **`make api` on `8baf2e5`:** exit 0, "against v1.23.0: 0 incompatible, 25 compatible": `ElicitationContentValue.Float64` and
  `Int64`, `ElicitationField`, `ElicitationPropertySchema.Kind` and `ParseValue`, `ElicitationSchema.Check`, `CheckSchema`,
  `Defaults` and `Fields`, `ElicitationURL`, `ParseElicitationURL`, `WithElicitationCheck`, and `RuleBounds`, `RuleDefault`,
  `RuleDuplicate`, `RuleEnum`, `RuleFormat`, `RuleMaxItems`, `RuleMaxLength`, `RuleMinItems`, `RulePattern`, `RuleRequired`,
  `RuleType`, `RuleUndeclared` and `RuleUnsupported`, each "added"; and "reported packages unstable: 0 incompatible, not gated".
- **The generated diff:** 19 lines of `agent_gen.go` and 10 of `client_gen.go`, each a `checkResult` call gaining `&p`, by the
  0040 PLAN's phase 5 and 7 comparison against `v1.23.0`.
- **CI on `8baf2e5`:** run `40513229624`, "completed success", six jobs on Ubuntu, Windows and macOS at Go `1.27.3` and
  `1.26.10`; and the Docs workflow's run `40513230078`, "completed success".
- **The evidence:** the 0040 PLAN's phases 2 to 7, with their 42 planted breaks, the gates on macOS and on the Linux and Windows
  hosts, and its deviation entry, `net/mail` replaced by a hand-written `email` rule; the survey of grok-build, opencode,
  kilocode, codex, magic-cli-remote and the MCP Go SDK in the MADR's Context.
