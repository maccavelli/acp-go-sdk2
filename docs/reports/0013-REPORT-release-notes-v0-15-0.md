# Release notes for v0.15.0, against v0.14.0

Observed 2026-10-11 on `e3c412a`, for
[0013-PLAN-close-phase-6-follow-ups.md](../decisions/0013-PLAN-close-phase-6-follow-ups.md). This report drafts the text of the GitHub release
for `v0.15.0`, as `RELEASING.md` describes it. It decides nothing. Each statement names its source: a record, a commit, or the exported API
listing of phase 6's listing program, run on `git archive v0.14.0` (2181 lines) and on `e3c412a` (2308 lines). Bring it up to date if more
changes land before the tag.

The section "Release text" is what goes into the GitHub release.

## Release text

### v0.15.0

The ACP schema is still `1.25.0` (`schema/version`, unchanged since `v0.14.0`). This release changes what the SDK accepts and sends in the
cases below, and changes the Go types of `ErrorCode` and of the three extension types.

#### Breaking changes

- **`ErrorCode` is an `int32`, with a constant per known code.** It was a union of nine pointer fields, each a named `int32` type. Every code
  used to decode as `ParseError`. Now any code decodes, and a known one equals its constant. Replace `ErrorCode{MethodNotFound: &v}` with
  `ErrorCodeMethodNotFound`. Replace a test of `code.MethodNotFound != nil` with `code == ErrorCodeMethodNotFound`. Replace
  `ErrorCodeOther(n)` with `ErrorCode(n)`. The constants are `ErrorCodeParseError`, `ErrorCodeInvalidRequest`, `ErrorCodeMethodNotFound`,
  `ErrorCodeInvalidParams`, `ErrorCodeInternalError`, `ErrorCodeRequestCancelled`, `ErrorCodeAuthenticationRequired`, and
  `ErrorCodeResourceNotFound`. The nine `ErrorCode…` variant types and `ErrorCode`'s `MarshalJSON`, `UnmarshalJSON`, and `Validate` are
  removed. (`0e32255`; 0011-MADR, "Found in step 7 and decided".)
- **`ExtRequest`, `ExtResponse`, and `ExtNotification` are aliases of `json.RawMessage`.** They were `any`, so a number in them decoded as
  `float64` and lost digits beyond 2^53. Code that put a Go value in them encodes it with `json.Marshal` first. (`0e32255`; 0012-MADR,
  "an untyped definition is raw JSON".)

#### Changes to what the SDK accepts and sends

- **Elicitation requests carry their scope.** A form or URL elicitation now carries the session or request it belongs to: `sessionId`,
  `toolCallId`, or `requestId`. `ElicitationFormMode` and `ElicitationUrlMode` keep their own properties: `requestedSchema`, and
  `elicitationId` and `url`. These were dropped on decode and never sent. New fields: `CreateElicitationForm.Session` and `.Request`,
  `CreateElicitationUrl.Session` and `.Request`, `ElicitationFormMode.RequestedSchema`, `ElicitationUrlMode.ElicitationId` and `.Url`.
  Exactly one of `Session` and `Request` is set; decoding picks the one whose required keys the payload carries, and `Validate` requires one.
  (`0e32255`; 0011-MADR, "Found in step 7 and decided".)
- **Keys in another case.** A key that differs from the schema's name only in case is read as that name, as `encoding/json` reads a struct
  field, the last matching key winning. That now holds for required keys, discriminators, and the keys that pick a variant or member.
  Before, `{"StopReason":"end_turn"}` decoded as `IdleStateUpdateNone` and was sent on as `end_turn`, and a wrong-case required key was
  refused. Keys are still sent under the schema's names. (`77d053b`; 0011-MADR, "Found in step 8 and decided"; 0012-MADR, amendment "key
  names".)
- **An optional raw `null` is absent.** `rawInput` and `rawOutput` that arrive as `null` decode to an empty `json.RawMessage` and are not
  sent on, as ACP v1 defines omission and `null` to be equivalent. Test them with `len(v) == 0`. A required raw value, such as a response's
  `result`, keeps `null`. (`6786efe`; 0012-MADR, amendment "a null raw value is an absent one".)
- **Lenient properties.** A property the schema marks `x-deserialize-default-on-error` takes its default, or its zero value, when its value is
  malformed, where decoding failed before. An array marked `x-deserialize-skip-invalid-items` drops the items that do not decode. A number
  outside its type's range in such a property falls back the same way. (`7eedeba`; 0012-PLAN, execution record.)
- **`Validate` checks what the schema states.** It checks at every depth: union variants, `minLength`, bounds the Go type does not enforce,
  and nested values, slice elements, and map values. It no longer rejects a present empty string; `v0.14.0` did, in 28 places. 44 types gain
  a `Validate`. (`0174f4e`; 0012-PLAN, execution record.)
- **Raw values keep invalid UTF-8.** Invalid UTF-8 a peer sends in a raw value is carried byte for byte, while a string field replaces it
  with U+FFFD. This was so before; it is now stated. (0012-MADR, amendment "key names, and invalid UTF-8 in raw values".)

#### Additions

- 87 types gain a generated `UnmarshalJSON`, and `CreateElicitationForm` and `CreateElicitationUrl` a `MarshalJSON`. (API listing.)
- The README's "Wire encoding" section states every rule above.

#### For the generator

These change nothing in the generated SDK of this release:

- The generator refuses lossy schema shapes.
- It refuses keys that one decoder reads differing only in case.
- It refuses integer `const` values outside their format.
- It refuses a catch-all variant holding flattened members.
- It reads a `$ref` variant's keys through its definition.
- It type-checks its output before it writes anything.
- It is tested by synthetic engine cases, fuzz targets, and a mutation tool.

(0011-MADR, phases 5 and 6; [docs/architecture.md](../architecture.md).)

## Sources not in the release text

- The five commits since `v0.14.0` that change the SDK's code: `7eedeba`, `0174f4e`, `6786efe`, `0e32255`, `77d053b`
  (`git log v0.14.0..HEAD -- '*_gen.go'` and the hand-written root files).
- The API listing difference, classified: changed, 4 types (`ErrorCode`, `ExtNotification`, `ExtRequest`, `ExtResponse`); removed, 21 lines,
  all of the old `ErrorCode` (9 fields, 3 methods, 9 types); added, 8 constants, 7 fields, 87 `UnmarshalJSON`, 44 `Validate`, and 2
  `MarshalJSON` methods.
- Not in this release, though in the records: sized integer fields, typed maps, raw JSON for `rawInput`, `rawOutput`, `params`, `result`, and
  `data`, and the refusal of a missing required key with `-32602`. These shipped in `v0.14.0`: its `types_gen.go` already has them and
  `objectKeys`.
- `Validate` on `Error`, `ErrorStopReason`, and `IdleStateUpdateError` was added after `v0.14.0` and removed again in `0e32255`, so the
  release does not change it.

## Erratum, 2026-10-12

The release text says `Validate` "no longer rejects a present empty string; `v0.14.0` did, in 28 places", and does not name them.
GitHub issue #2 found that the `Cwd` and `Path` checks among them carried security weight. A script over every `Validate` body of `git
show v0.14.0:types_gen.go` names the 28; none survives in `v0.15.0`:

- the `Method` of `AgentRequest`, `AgentNotification`, `ClientRequest`, `ClientNotification`, `UnstableMessageMcpRequest` and
  `UnstableMessageMcpNotification`;
- the `Message` of the `Form`, `Url` and `Other` variants of `CreateElicitationRequest`;
- `CreateTerminalRequest.Command`;
- **the `Cwd` of `NewSessionRequest`, `LoadSessionRequest`, `ResumeSessionRequest` and `UnstableForkSessionRequest`**;
- **`ReadTextFileRequest.Path`**, and `WriteTextFileRequest.Path` and `.Content`;
- `ReadTextFileResponse.Content` and `TerminalOutputResponse.Output`;
- the `Uri` of `UnstableDidOpenDocumentNotification`, `UnstableDidChangeDocumentNotification`,
  `UnstableDidCloseDocumentNotification`, `UnstableDidFocusDocumentNotification` and `UnstableDidSaveDocumentNotification`, with `Text`
  and `LanguageId` on open;
- `UnstableSuggestNesRequest.Uri` and `UnstableSetProviderRequest.BaseUrl`.

The bold ones carry security weight: on `v0.15.0` an agent receives an empty `cwd`, and a client an empty `fs/read_text_file` or
`fs/write_text_file` path, where `v0.14.0` answered `-32602`; an application that relied on that refusal must check the value itself
until it upgrades. No version refused a relative path. The release that carries
[0015-MADR-validate-absolute-protocol-paths.md](../decisions/0015-MADR-validate-absolute-protocol-paths.md) refuses an empty or relative
value in every property ACP requires to be absolute, in both directions.

Text to append to the published `v0.15.0` GitHub release, by the owner:

> **Erratum.** `Validate` in this release no longer refuses an empty `cwd` on `session/new`, `session/load`, `session/resume` and
> `session/fork`, or an empty `path` on `fs/read_text_file` and `fs/write_text_file`, which `v0.14.0` refused with `-32602`; no version
> refused a relative path. Agents and clients on `v0.15.0` must check these values themselves. The 28 removed checks are listed in
> `docs/reports/0013-REPORT-release-notes-v0-15-0.md`, "Erratum". See issue #2; the fix is in the next release.
