# Release notes for v0.16.0, against v0.15.0

Observed 2026-10-12 on `3ca6357`, for
[0014-PLAN-union-constructors-take-every-required-key.md](../decisions/0014-PLAN-union-constructors-take-every-required-key.md), step 7,
as amended by [0015-PLAN-validate-absolute-protocol-paths.md](../decisions/0015-PLAN-validate-absolute-protocol-paths.md), step 2. This
report drafts the text of the GitHub release for `v0.16.0`, as `RELEASING.md` describes it, and the GitHub security advisory for
`v0.15.0`. It decides nothing. Each statement names its source: a record, a commit, or the exported API listing of the 0013 report's
listing program, run on `git archive v0.15.0` (2308 lines) and on `3ca6357` (2390 lines). Bring it up to date if more changes land before
the tag. The version is the next minor, `v0.16.0`; the owner confirms it at the tag.

The section "Release text" is what goes into the GitHub release; "Security advisory for v0.15.0" is what the owner publishes with it.

## Release text

### v0.16.0

The ACP schema is still `1.25.0` (`schema/version`; `git diff v0.15.0 -- schema/` is empty). This release enforces ACP's rule that file
paths in the protocol are absolute, in both directions, which closes a regression of `v0.15.0` with security weight (issue #2). It gives
every union a constructor for each variant, and changes the signatures of two constructors.

#### Security

`v0.15.0` stopped refusing an empty `cwd` on `session/new`, `session/load`, `session/resume` and `session/fork`, and an empty `path` on
`fs/read_text_file` and `fs/write_text_file`, which `v0.14.0` refused with `-32602`; no version refused a relative one. An agent that
confines its file tools to the session's working directory, and treats an empty one as no confinement, could be handed a session without
one. This release refuses an empty or relative value in every property ACP requires to be absolute, before the handler runs. Upgrade
from `v0.15.0`. A security advisory for `v0.15.0` is published with this release. (Issue #2;
[0015-MADR-validate-absolute-protocol-paths.md](../decisions/0015-MADR-validate-absolute-protocol-paths.md); `992cf48`, `b83b84d`,
`a438f99`.)

#### Breaking changes

- **`NewSessionConfigOptionSelect` and `NewSessionConfigOptionBoolean` take the option's `id` and `name`.** They sent both empty before.
  Replace `NewSessionConfigOptionSelect(currentValue, options)` with `NewSessionConfigOptionSelect(id, name, currentValue, options)`, and
  `NewSessionConfigOptionBoolean(currentValue)` with `NewSessionConfigOptionBoolean(id, name, currentValue)`. (`75bf968`;
  0014-MADR, P1; API listing.)
- **A path that is not absolute is no longer sent.** The typed methods, `SendRequest`, `SendRequestNoResult` and `SendNotification`
  return a `*ValidationError` and write nothing when one of the 17 path properties below is empty or relative; there is no option to turn
  this off. A path absolute in POSIX or Windows form passes, as the sender cannot know the receiver's operating system. A handler's result
  that fails is not sent; the peer gets `-32603`. Extension methods with raw JSON params are not checked. (`b83b84d`; 0015-MADR, E1 and
  A4.)
- **A received path that is not absolute is refused.** Every request, notification and response is checked with the host's
  `filepath.IsAbs`: a request is answered `-32602` naming the type and property, such as `NewSessionRequest: cwd: "" is not an absolute
  path`; a notification is dropped and logged; a response is returned to the caller as an error. On Linux and macOS, `C:\p` is refused.
  Responses were not validated before. (`992cf48`, `a438f99`; 0015-MADR, E1, A4 and H1.)

#### Changes to what the SDK accepts and sends

- **Absolute paths, 17 properties.** `cwd` and `additionalDirectories` of `session/new`, `session/load`, `session/resume` and the
  unstable `session/fork`; the `cwd` filter of `session/list`, and the `cwd` and `additionalDirectories` of each session it lists; the
  `command` of a stdio MCP server; the `path` of `fs/read_text_file` and `fs/write_text_file`; the `cwd` of `terminal/create`; and the
  `path` of a tool-call location and of a diff. A client that forwards an MCP server configured by a bare name, such as `npx`, resolves it
  first, for example with `exec.LookPath`. (0015-MADR, F1; `992cf48`.)
- **Hooks for what is received.** `WithInboundRewrite` sees each decoded message before it is validated and may change it, for example
  to resolve a bare command or to translate a path from another host; `WithInboundValidation` sees the SDK's result and returns the error
  to act on, so it can add a rule, or accept a violation the application chooses to answer for. (`a438f99`; 0015-MADR, H1.)
- **The other empty-string checks `v0.15.0` removed stay removed**, as the standard states none of them: the `Method` of `AgentRequest`,
  `AgentNotification`, `ClientRequest`, `ClientNotification`, `UnstableMessageMcpRequest` and `UnstableMessageMcpNotification`; the
  `Message` of the three `CreateElicitationRequest` variants; `CreateTerminalRequest.Command`; `WriteTextFileRequest.Content`;
  `ReadTextFileResponse.Content`; `TerminalOutputResponse.Output`; the `Uri` of the five `UnstableDid…DocumentNotification` types, with
  `Text` and `LanguageId` on open; `UnstableSuggestNesRequest.Uri`; and `UnstableSetProviderRequest.BaseUrl`. The six that remain of the
  28 are the paths above, now refused when empty or relative: the `Cwd` of the four session requests and the `Path` of the two `fs`
  requests. `WithInboundValidation` can add any of the others back. (0013 report, "Erratum"; 0015-MADR, "Not changed".)
- **`Validate` refuses a catch-all holding another variant's value**, such as `ElicitationPropertySchema: variant Other: type "string"
  names another variant`; decoding sends a known value to its own variant, so this catches a value built by hand. (`23d03b5`;
  0014-MADR, C1.)
- **A set array variant holding nil is sent as `[]`**, not `null`, such as `NewSessionConfigSelectOptionsUngrouped(nil)`. (`23d03b5`;
  0014-MADR, item 7.)
- **`Validate` errors are typed.** Every check returns a `*ValidationError` with `Type`, `Field`, `Value` and `Rule`, reachable with
  `errors.As` through a parent's wrapping; the message text is unchanged. (`967dd26`; 0015-MADR, X1.)

#### Additions

- 56 union constructors, `New<Union><Variant>`, one for every variant of 16 unions that had none (API listing): `IdleStateUpdate` 8,
  `ElicitationPropertySchema` 6, `StateUpdate` 5, `ElicitationContentValue` 5, `McpServer` 4, `UnstableMcpServer` 4,
  `CreateElicitationResponse` 4, `RequestId` 3, `MultiSelectItems` 3, `CreateElicitationRequest` 3, `UnstableMessageMcpResponse` 2,
  `SetSessionConfigOptionRequest` 2, `SessionConfigSelectOptions` 2, `EmbeddedResourceResource` 2, `AuthMethod` 2,
  `AvailableCommandInput` 1. Every union now has them, 67 in all, except the hand-written `ContentBlock`, `SessionUpdate` and
  `ToolCallContent`, and `AgentResponse` and `ClientResponse`, which the schema keeps out of its documentation. Parameters are the
  variant's required keys, named after them; a catch-all takes its discriminator, and a non-object variant takes `value`. (`75bf968`,
  `4b21cbe`, `23d03b5`; 0014-MADR.)
- `ValidationError` and `ValidationRule` with its seven constants: `RuleAbsolutePath`, `RuleMinLength`, `RuleMinimum`, `RuleMaximum`,
  `RuleOneVariant`, `RuleOneMember`, `RuleExcludedValue`. (API listing; `967dd26`.)
- `WithInboundRewrite` and `WithInboundValidation`. (API listing; `a438f99`.)
- `Roots()` on `NewSessionRequest`, `LoadSessionRequest`, `ResumeSessionRequest`, `UnstableForkSessionRequest` and `SessionInfo`: the
  session's root set, `cwd` then `additionalDirectories`. The README shows confining file access with `os.OpenRoot` per root.
  (API listing; `08268b4`.)
- `Validate` on `Diff`, `McpServerStdio`, `SessionInfo`, `ToolCallContentDiff` and `ToolCallLocation`, whose only check is a path.
  (API listing; `992cf48`.)
- The README's "Wire encoding" section states the rules above, with a "Paths" part; "Helper constructors" describes the generated ones.

#### For the generator

These change nothing in the generated SDK beyond what is listed above:

- It refuses a union key that a variant's Go type cannot hold. (0014-MADR, R1; `75bf968`.)
- It names every constructor parameter after its key: a Go keyword or predeclared name gains `Value`, and a key that is not a Go name is
  joined in camel case; two parameters with one name are refused. (0014-MADR, N1, K2 and L1; `4b21cbe`.)
- It refuses a catch-all whose `not` excludes no discriminator value. (0014-PLAN, step 5 deviation; `23d03b5`.)
- The protocol table lists the properties ACP requires to be absolute, each with its source, and the model refuses one whose property is
  not a string, an optional string or an array of strings. (0015-MADR, W1; `992cf48`.)

## Security advisory for v0.15.0

A draft for the owner to publish on GitHub when `v0.16.0` is tagged (0015-MADR, C1 and "Decided with the acceptance"). Severity and CVSS
are the owner's; none is suggested here.

- **Package:** Go, `github.com/maccavelli/acp-go-sdk2`.
- **Affected versions:** `v0.15.0`.
- **Patched versions:** `v0.16.0`.
- **Weakness:** CWE-20, Improper Input Validation.
- **Title:** `Validate` accepts an empty session `cwd` and file `path` in `v0.15.0`.
- **Description:**

  > In `v0.15.0`, the generated `Validate` no longer refuses an empty `cwd` on `session/new`, `session/load`, `session/resume` and
  > `session/fork`, or an empty `path` on `fs/read_text_file` and `fs/write_text_file`. `v0.14.0` refused them with `-32602`. An agent
  > that confines its file tools to the session's working directory, and treats an empty one as no confinement, can be given a session
  > with none; a client can be asked to read or write an empty path. No version before `v0.16.0` refused a relative path in these
  > properties, although the protocol requires them to be absolute.
  >
  > `v0.16.0` refuses an empty or relative value in every property the protocol requires to be absolute, in what the SDK receives
  > (requests, notifications and responses) and in what it sends. Applications that cannot upgrade must check these values themselves
  > before using them. The 28 checks `v0.15.0` removed are listed in `docs/reports/0013-REPORT-release-notes-v0-15-0.md`, "Erratum".
  > Reported in issue #2.

## Sources not in the release text

- The commits since `v0.15.0` that change the SDK's Go code: `75bf968`, `23d03b5`, `967dd26`, `992cf48`, `b83b84d`, `fe6b1cb` (a test
  only), `a438f99`, `08268b4` (`git log v0.15.0..HEAD -- '*.go' ':!cmd/generate'`); and the generator's: `75bf968`, `4b21cbe`, `23d03b5`,
  `967dd26`, `992cf48`, `b83b84d`, `a438f99` (`git log v0.15.0..HEAD -- cmd/generate`).
- The API listing difference, classified: changed, 2 lines, the two `NewSessionConfigOption…` signatures; removed, none; added, 56
  `func New…` constructors, the 7 `Rule…` constants, `ValidationError` with its 4 exported fields and `Error`, `ValidationRule`,
  `WithInboundRewrite`, `WithInboundValidation`, 5 `Roots` and 5 `Validate` methods. `grep -c '^func New' helpers_gen.go` is 67, against
  11 in `v0.15.0`.
- `v0.15.0`'s own release gains the erratum text of the 0013 report, appended by the owner.
- Issue #2 has the reply drafted in the 0015 PLAN's step 8, posted 2026-10-12; it is closed when `v0.16.0` is tagged.
- Not in this release, though in the records: the decisions of
  [0017-MADR-remediate-acp-v1-conformance-violations.md](../decisions/0017-MADR-remediate-acp-v1-conformance-violations.md), whose PLAN
  starts after this tag and ships as `v0.17.0`.
