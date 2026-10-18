# Release notes for v1.15.1, against v1.15.0

Observed 2026-10-18 on `9a70c6a`, for [0019-PLAN-examples-docs-tests-tooling.md](../decisions/0019-PLAN-examples-docs-tests-tooling.md),
step 8. This report drafts the text of the GitHub release for `v1.15.1`, as `RELEASING.md` describes it. It decides nothing.

Each statement names its source:

- a record;
- a commit;
- or the exported API listing of the 0013 report's listing program, run on `git archive v1.15.0` (2429 lines) and on the tree at
  `9a70c6a` (2429 lines).

Bring it up to date if more changes land before the tag. No exported API or SDK behaviour changes, so the release is a patch
([0019-PLAN-examples-docs-tests-tooling.md](../decisions/0019-PLAN-examples-docs-tests-tooling.md), "Rollout and Rollback"), which the
owner confirms at the tag.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.15.1

The ACP schema is still `1.25.0` (`git diff v1.15.0 -- schema/` is empty). The exported API is unchanged, and so is what the SDK
does. This release is phase 4 of
[0019-MADR-remediate-debugging-pass-findings.md](../decisions/0019-MADR-remediate-debugging-pass-findings.md): documentation on every
exported identifier, example programs that behave as ACP asks, runnable `Example` functions, and tests and tooling.

#### Documentation

- **Every exported identifier now has a doc comment.** 654 had none; pkg.go.dev now shows text for each. (`9a70c6a`; I12.)
  - Each `Validate`, `MarshalJSON` and `UnmarshalJSON` names the schema definition its type comes from, or the variant of one.
  - Each enum constant names its wire value, then gives the value's schema description.
  - Each typed outbound method, such as `(*ClientSideConnection).Prompt`, says what it sends and to which side, then repeats the
    interface method's doc.
  - `Agent`, `Client`, `RequestIdNull` and `(*RequestError).Error` are documented.
- **The generated files change in comments only.** Their syntax trees, compared without comments or positions, equal those at
  `v1.15.0`. (`9a70c6a`.)
- **The `Example` functions run under `go test`.** `Example_agent` and `Example_client` run an agent and a client in one process and
  end in checked `// Output:`. All three are in `package acp_test`, use the package as a caller does, and check every error.
  `Example_gemini` starts `gemini --acp` and stops its child when done. (`4da5878`; E8.)
- `docs/architecture.md` states how a message without `method` is classified as a response, as the code does it. It also describes
  the generated docs. (`fdbdff6`, `9a70c6a`; E14, I12.)

#### Example programs

The programs under `example/` are not part of the module's API. They change behaviour:

- **Adapter and CLI flag.** `example/claude-code` runs Claude Code's ACP adapter, `@agentclientprotocol/claude-agent-acp`.
  `example/gemini` passes `--acp`. (`fdbdff6`; E5.)
- **Cancellation in the example agent.** `Prompt` runs under the context the SDK cancels for `session/cancel`. A turn whose
  permission request is answered cancelled ends `cancelled`, where it ended `end_turn`. (`fdbdff6`; E6.)
- **Terminal capability.** The example clients advertise no terminal capability, and their terminal methods answer method not found.
  (`fdbdff6`; E7.)
- **File writes.** Each example client writes only inside the session's working directory. It refuses a path that leaves it, and
  writes through an `os.Root`, so a symbolic link cannot lead out either. (`fdbdff6`; E7.)
- **The example client's run.** It starts the example agent, or the command given as its arguments. It prints the turn's stop reason,
  exits 1 when a call fails, and stops its agent by closing its input and waiting. (`fdbdff6`; E12, E14.)
- **Lint.** The examples are linted with the rest of the module. (`fdbdff6`; E12.)

#### For contributors

These change no file of the module a consumer builds:

- **Tests.** New tests run the documented branches the coverage profile showed unrun; root coverage goes from 88.8% to 89.3%.
  - Three tests that slept a fixed time now run in `testing/synctest` bubbles.
  - `TestConformanceConstructors` runs all 67 constructors, where it skipped 4.
  - (`d13d2d5`; E9, E10.)
- **Example tests.** `TestExamplePair` builds the example agent and client and runs a turn each way. Each example client's tests
  check its write confinement and its terminal answers. (`fdbdff6`; E12, E7.)
- **Vulnerability check.** `govulncheck` is pinned at v1.9.0. `scripts/go-precheck.sh` says when it skipped the check and why.
  (`394572f`; E15, E16.)
- **Release message.** `make release` no longer tells you to tag the module with the schema version. (`394572f`; E13.)
- **Removed file.** `mise.lock`, which no configuration read, is gone. (`394572f`; E17.)
- **Records checker.** `scripts/check_records.py` refuses a malformed record and checks the anchor of a link as well as its file.
  (`ca166fb`; E11.)

## Sources not in the release text

- **Commits since `v1.15.0` that change Go files outside `cmd/generate`** (`git log v1.15.0..HEAD -- '*.go' ':!cmd/generate'`):
  `fdbdff6`, `4da5878`, `d13d2d5` and `9a70c6a`.
  - Of the non-test files of the root package, only `errors.go` (a doc comment) and the three regenerated files change:
    `agent_gen.go`, `client_gen.go` and `types_gen.go`, 1091 lines added and 56 removed in all.
  - The generator changes in `9a70c6a` only. The other commits change records, tooling, examples and tests.
- **The API listing:** 0 lines removed, 0 added, `v1.15.0` against `9a70c6a`.
- **Execution records:** each change, its tests, the planted breaks each test was seen to fail on, and a Windows run for each step
  are in [0019-PLAN-examples-docs-tests-tooling.md](../decisions/0019-PLAN-examples-docs-tests-tooling.md), with its two deviations.
  - On 2026-10-16, four gosec G204 launches in the examples were marked, not removed.
  - On 2026-10-17, the codec and `Validate` docs were set to name where a type's schema sits.
- **Known, not changed:** the example clients print a tool call update's status as a pointer (step 4's record). The decision is left
  to the owner.
- **Not in this release:** phase 5 of 0019 (idioms and performance), which rides the release after it lands (0019-MADR, decision 12).
