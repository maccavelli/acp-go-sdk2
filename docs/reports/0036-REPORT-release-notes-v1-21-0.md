# Release notes for v1.21.0, against v1.20.0

Observed 2026-10-25 on `7f3b210`, the commit the annotated tag `v1.21.0` points at, for
[0036-PLAN-session-update-constructors-kind-and-client-refusals.md](../decisions/0036-PLAN-session-update-constructors-kind-and-client-refusals.md),
step 6. This report drafts the text of the GitHub release for `v1.21.0`, as `RELEASING.md` describes it. It decides nothing.

The tag was pushed before this report was written, so the tagged tree does not hold it. The report describes `7f3b210`, and is
committed after it; the PLAN records the order as a deviation of step 6.

Each statement names its source:

- a record;
- a commit;
- or `make api`, run on `7f3b210` with nothing uncommitted.

Summary:

- Eleven additions to the root package's exported API; `agentproc`'s is unchanged. `make api` against `v1.20.0`: "0 incompatible,
  11 compatible". The release is the minor `v1.21.0`, as `RELEASING.md` says for compatible changes.
- What a consumer may meet: nothing the SDK sends or accepts on the wire changes. The `go` lines and the requirements are unchanged.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.21.0

The ACP schema is still `1.25.0` (`git diff v1.20.0 v1.21.0 -- schema/` is empty). This release adds what issue
[#11](https://github.com/maccavelli/acp-go-sdk2/issues/11) asks for: a constructor for every stable `SessionUpdate` variant,
`SessionUpdate.Kind()`, and `ClientRefusals`, an embed for a client that offers no file-system, terminal or elicitation methods
([0036-MADR-session-update-constructors-kind-and-client-refusals.md](../decisions/0036-MADR-session-update-constructors-kind-and-client-refusals.md)).
`make api` reports eleven compatible additions and no incompatible change. Nothing sent or accepted on the wire changes.

#### New: the stable `SessionUpdate` constructors

Each takes the variant's required keys, in the schema's order, and sets the variant's `sessionUpdate` field. Optional fields are set
on the variant it returns. Each one's encoding equals the composite literal's. (`4189d9a`.)

| Constructor | Variant |
| :--- | :--- |
| `UpdateAvailableCommands(commands []AvailableCommand)` | `available_commands_update` |
| `UpdateCurrentMode(id SessionModeId)` | `current_mode_update` |
| `UpdateConfigOptions(opts []SessionConfigOption)` | `config_option_update` |
| `UpdateSessionTitle(title Patch[string])` | `session_info_update` |
| `UpdateUsage(used, size uint64)` | `usage_update` |
| `UpdateNotice(severity NoticeSeverity, title string)` | `notice` |
| `UpdateCompaction(id CompactionId, status CompactionStatus)` | `compaction_update` |
| `UpdateCompactionSummary(id CompactionId, content ContentBlock)` | `compaction_summary_chunk` |
| `UpdateCompactionSummaryText(id CompactionId, text string)` | `compaction_summary_chunk`, with a text block |

- `UpdateSessionTitle` takes a `Patch[string]`: `PatchValue` sets the title, `PatchNull` clears it, and a zero `Patch` leaves it.
- Sending `UpdateNotice`'s update fails with a `*CapabilityError`, and writes nothing, unless the client advertised `session.notices`;
  the three compaction constructors' updates, unless it advertised `session.compaction`. This is the gate the SDK already applied to
  those variants.
- `UpdateNotice` with an empty title fails `Validate`.
- The constructors that existed before, `UpdateUserMessage`, `UpdateAgentMessage`, `UpdateAgentThought`, their `Text` forms,
  `UpdatePlan`, `StartToolCall` and `UpdateToolCall`, are unchanged: they leave the variant's `sessionUpdate` field empty, which
  `MarshalJSON` fills.

#### New: `SessionUpdate.Kind()`

`Kind` returns the `sessionUpdate` value of the variant a `SessionUpdate` sends: the first set variant, in the order `MarshalJSON`
checks them, unstable variants included. It never reads the variant's own field, so a constructed value and a decoded one agree. For
an update this SDK does not know, held in `Unknown`, it returns the payload's `sessionUpdate` as decoding reads it, keys folded and the
last match winning, when that is a non-empty string, and `"unknown"` otherwise. The zero value returns `""`. (`e0f4854`.)

```go
var u acp.SessionUpdate
if err := json.Unmarshal(payload, &u); err != nil {
    return err
}
log.Printf("declined %s", u.Kind()) // "future_update", for an unknown variant
```

#### New: `ClientRefusals`

A struct to embed in a client that implements only `SessionUpdate` and `RequestPermission`. It answers method-not-found (`-32601`),
with data naming the method, for `ReadTextFile`, `WriteTextFile`, `CreateTerminal`, `KillTerminal`, `TerminalOutput`,
`ReleaseTerminal`, `WaitForTerminalExit` and `CreateElicitation`; `CompleteElicitation` returns nil. It has exactly these nine
methods, so `SessionUpdate` and `RequestPermission` stay required at compile time. (`4152eb7`.)

- The SDK refuses a method whose capability the client did not advertise before the client's method runs, so these answer only for a
  capability the client advertised.
- A method the embedding type declares takes precedence over the promoted one.
- A client that implements some of these methods for real should implement all of them itself rather than embed `ClientRefusals`: a
  real method renamed or removed would otherwise be refused without a compile error.

```go
type client struct{ acp.ClientRefusals }

func (client) SessionUpdate(context.Context, acp.SessionNotification) error { return nil }

func (client) RequestPermission(context.Context, acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
    return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeCancelled()}, nil
}
```

#### Documentation

- The README's helper list names every `SessionUpdate` constructor and `SessionUpdate.Kind()`, and its client section names
  `ClientRefusals`; `docs/architecture.md` lists `refusals.go`. New examples: `ExampleUpdateSessionTitle`,
  `ExampleSessionUpdate_Kind` and `ExampleClientRefusals`. (`4189d9a`, `e0f4854`, `4152eb7`.)

#### For maintainers

- **Verification tools in the repository.** `tools/` holds the checks a PLAN's steps run: the gate set, the test hosts, planted
  breaks, the release and publication checks, CI watching and CI job history, and `make check-tools`, which CI runs
  ([0031-MADR-verification-tools-in-repository.md](../decisions/0031-MADR-verification-tools-in-repository.md)).
- **CI.** The race detector runs on the Windows jobs as on Linux
  ([0034-MADR-race-detector-on-windows-ci.md](../decisions/0034-MADR-race-detector-on-windows-ci.md)), and the Windows floor job runs
  with `GOTRACEBACK=crash` ([0033-MADR-traceback-on-windows-floor-ci.md](../decisions/0033-MADR-traceback-on-windows-floor-ci.md)).
  (`b42620c`, `57ce3f2`, `616864e`.)
- **Tests.** `agentproc`'s tap drain test blocks its tap, so it exercises the grace whatever the machine's speed (`347949e`). New at
  the root: `session_update_helpers_test.go`, `kind_test.go`, `refusals_test.go` and eight golden files. The generator's
  `TestRuntimeFilesCoverHandWritten` excludes `refusals.go` beside `helpers.go`. (`4189d9a`, `e0f4854`, `4152eb7`.)

## Sources not in the release text

- **Commits since `v1.20.0`** (`git log v1.20.0..v1.21.0`), thirty-two:
  - `4189d9a`, `e0f4854` and `4152eb7` change the package, as above;
  - `347949e` changes `agentproc/tap_test.go`;
  - `57ce3f2` and `616864e` change `.github/workflows/ci.yml`;
  - `d6f3e78`, `e7bf6c4`, `22fbf93`, `ee6c90f`, `85fadd3`, `b42620c`, `c39195a`, `bf0b215`, `2f58588` and `d2c5cd6` add and change
    `tools/`; `d6f3e78` changes the `Makefile`, and `b42620c` `.github/workflows/ci.yml` and `AGENTS.md`;
  - the other sixteen change records only (`git show --name-only`).
- **Go files changed.** `git diff --name-only v1.20.0 v1.21.0 -- '*.go' ':!*_test.go'` lists two: `helpers.go` and `refusals.go`. No
  `*_gen.go` changes. `go.mod`, `go.sum`, `cmd/generate/go.mod`, the schema, `RELEASING.md` and the examples are unchanged
  (`git diff`).
- **`make api` on `7f3b210`:** `scripts/check_api.py v1.20.0`, exit 0, "against v1.20.0: 0 incompatible, 11 compatible":
  `ClientRefusals`, `SessionUpdate.Kind`, `UpdateAvailableCommands`, `UpdateCompaction`, `UpdateCompactionSummary`,
  `UpdateCompactionSummaryText`, `UpdateConfigOptions`, `UpdateCurrentMode`, `UpdateNotice`, `UpdateSessionTitle` and `UpdateUsage`,
  each "added". `apidiff` lists a new type once, without its methods.
- **CI on `7f3b210`:** run `40192665703`, "completed success", its six jobs on Ubuntu, Windows and macOS at Go `1.27.3` and `1.26.10`
  (`tools/watch_ci.py`).
- **The evidence:** the 0036 PLAN's steps 1 to 5, with their planted breaks, the gates on macOS and on the Linux and Windows hosts,
  and their deviations: the six old constructors' round trip, the named constants of `Kind`, and the generator test's exclusion.
- **Issue #11** is open (`gh issue view 11`); the owner closes it with the release. Its consumers' issues, gobble-cli [#5](https://github.com/maccavelli/gobble-cli/issues/5) and
  magic-cli-remote #28, are to pin this tag.

*Note, 2026-10-31: gobble-cli was written `<cli-b>` in this record until the owner lifted that redaction ([0044-PLAN-notification-queue-back-pressure.md](../decisions/0044-PLAN-notification-queue-back-pressure.md), phase 5).*
