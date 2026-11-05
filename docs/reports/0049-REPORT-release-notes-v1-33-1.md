# Release notes for v1.33.1, against v1.33.0

Observed 2026-11-05 on `e02e63f`, for [0049-PLAN-runtime-fixes.md](../decisions/0049-PLAN-runtime-fixes.md), phase 6,
step 3. This report drafts the text of the GitHub release for `v1.33.1`, as `RELEASING.md` describes it. It decides nothing.
It is committed before the tag, so the tagged tree holds it. The commit that adds it changes records and nothing else, so the
API it tags is `e02e63f`'s.

Each statement names its source:

- a record;
- a commit;
- or `make api`, run on `e02e63f` with nothing uncommitted but records: this report and the runtime PLAN's step 2 record.

Summary:

- No exported API change. `make api` against `v1.33.0`: "0 incompatible, 0 compatible", so the release is a patch, as
  `RELEASING.md` says and the runtime PLAN's phase 5 planned.
- One fix to the connection runtime, R11 of
  [0049-MADR-remediate-the-2026-11-01-debugging-pass.md](../decisions/0049-MADR-remediate-the-2026-11-01-debugging-pass.md),
  rated medium.
- What a consumer may meet: at the notification queue's limit, a notification is no longer closed on or dropped because
  the notification loop had not yet taken the one before it. None is refused earlier than before. The queue's channel holds
  one slot more.
- The tooling fixes of [0049-PLAN-tooling-fixes.md](../decisions/0049-PLAN-tooling-fixes.md), T1 to T14 and F-C, which had
  no release of their own, are in this one. One of them removes an unexported generated method no call reached.
- The `go` lines, the requirements and the schema are unchanged.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.33.1

The ACP schema is still `1.25.0` (`git diff v1.33.0 -- schema/` is empty). The exported API is unchanged: `make api` reports no
change against `v1.33.0`. This release fixes one defect of the connection runtime, R11 of
[0049-MADR-remediate-the-2026-11-01-debugging-pass.md](../decisions/0049-MADR-remediate-the-2026-11-01-debugging-pass.md), and
carries the tooling fixes of the same pass. The fix has tests that fail without it.

#### Fixed: the notification queue's limit no longer depends on scheduling (R11)

`WithMaxQueuedNotifications(n)` lets `n` notifications wait while a notification handler runs. A notification was admitted
when the send to the queue's channel succeeded, and the channel held `n`, or `2n` under `OverflowBlock`. But the notification
the loop runs next stays in the channel until the loop takes it. So when a burst reached the limit before the loop had been
scheduled to take its head, the next notification found the channel full one notification early:

- under `OverflowCloseConnection`, the connection closed with `acp.ErrNotificationQueueOverflow`;
- under `OverflowDropNewest`, the notification was dropped;
- under `OverflowBlock`, on its read-on path, which a connection takes while its own request awaits the peer's answer, such as
  `Initialize`, the connection closed, and that request ended with "connection ended before response".

The same input from the peer was handled, dropped or closed on, by an ordering the program does not control. It was rare at
the default of 1024, and likely at a small queue. `TestOverflowBlockHeldInitialize` failed this way twice in this
repository's own runs. (`03acda5`.)

A notification is now admitted by the count of notifications received and not completed: up to `n + 1`, or `2n + 1` on
`OverflowBlock`'s read-on path, the `+ 1` being the notification whose handler runs or is next to run, whether or not the loop
has taken it. The channel holds that many, so a send the count admits cannot fail. What changes:

- **At the limit,** the policy meets the same input the same way, whatever the order the goroutines run in.
- **No notification is refused earlier** than before; one that a promptly scheduled loop admitted is still admitted.
- **The channel** holds `n + 1` slots, or `2n + 1` under `OverflowBlock`, one more than before: still 64 MiB at the maximum
  `n`.
- **Unchanged:** `OverflowBlock`'s pause and the `WithMaxQueuedBytes` budget, which already counted.

[0044-MADR-notification-queue-back-pressure.md](../decisions/0044-MADR-notification-queue-back-pressure.md)'s D4 is amended
for the capacity.

#### For maintainers

- **Tests.** `TestOverflowBoundaryCounted` and `TestOverflowBlockHeldInitializeLoopBehind`, in
  `connection_overflow_boundary_test.go`, hold the notification loop before it takes anything, through an unexported seam,
  `withBeforeDequeue`. `TestQueueChannelCapacity` checks the channel's capacity under each policy. Each was seen to fail with
  the fix reverted in a copy of the tree. With the loop made to yield before each receive, the unchanged
  `TestOverflowBlockHeldInitialize` failed 303 of 2000 runs before the fix and 0 of 2000 after. The runtime PLAN's phases 5
  and 6 record them. (`03acda5`.)
- **The benchmarks.** Allocations per operation are unchanged.
- **The tooling,** by [0049-PLAN-tooling-fixes.md](../decisions/0049-PLAN-tooling-fixes.md), T1 to T14 and F-C, each with a
  test that fails without it, or a corrected comment:
  - `tools/publication.py` and `tools/ci_jobs.py` read the CI workflow's runs alone, not the Docs workflow's. (`72a4667`.)
  - `make check-records` and `make markdownlint` read the files git tracks or would, and check the links of `RELEASING.md`,
    `tools/README.md` and `AGENTS.md`. (`992f6a5`.)
  - `tools/publication.py` and `tools/consumer.py` refuse a Go environment that goes around the public proxy or the checksum
    database; `tools/idscan.py --range` scans each commit's files; `tools/find.py` names a path it cannot read;
    `tools/release_body.py` says why its identifier scan failed.
    (`01dfcec`.)
  - The Makefile stops on an empty schema version under Make 3.81, and `make help` lists every target; dead `.golangci.yml`
    exclusions are removed. (`719198f`.)
  - The generator emits `gates` only for the types the dispatch can call it on, a method's request, response or
    notification type and what their fields, members and variants hold, so `types_gen.go` no longer has the unexported,
    never-called `ConfigOptionUpdate.gates`. `agentproc` gains an unexported test seam, `attachTree`, for
    `Start`'s clean-up when it cannot connect an agent. (`32ca4c7`, `ea32a99`.)

## Sources not in the release text

- **Commits since `v1.33.0`** (`git log v1.33.0..e02e63f`), twelve:
  - `1fe3dd0` records the subpackage PLAN complete in `v1.33.0`;
  - `4e3d9cc` adds the tooling PLAN;
  - `72a4667`, tooling phase 1 (T2, T14, T7): `tools/ci_jobs.py`, `tools/publication.py`, the two workflows' comments,
    `tools/test_tools.py`;
  - `992f6a5`, tooling phase 2 (T1, T3): `scripts/check_records.py` and its test, `.markdownlint-cli2.jsonc`, `AGENTS.md`;
  - `01dfcec`, tooling phase 3 (T4, T5, T6, T11): `tools/_common.py`, `consumer.py`, `find.py`, `idscan.py`, `publication.py`,
    `release_body.py`, `scripts/check_disclosure.py`, their tests;
  - `719198f`, tooling phase 4 (T9, T10, T12, T13): the `Makefile`, `.golangci.yml`, `ci.yml`'s comment, `RELEASING.md`,
    `tools/platforms.py`, `tools/test_tools.py`;
  - `32ca4c7`, tooling phase 5 (F-C): `cmd/generate/internal/model/gates.go` and its test, an engine case, `types_gen.go`,
    `agentproc/agentproc.go`, two new or extended tests in `agentproc/`, `elicitation_schema_test.go`;
  - `ea32a99`, tooling phase 6's deviations: the engine case's `allOf` route, `tools/test_tools.py`;
  - `c69499d` marks the tooling PLAN complete;
  - `a4d8ef2` records R11's cause and Q20's answer, amends the 0044 MADR's D4, and adds the runtime PLAN's phases 5 and 6;
  - `03acda5`, runtime phase 5 (R11): `connection.go`, `connection_overflow_boundary_test.go` (new),
    `connection_queue_size_test.go`, `docs/architecture.md`;
  - `e02e63f` records phase 6's gates.
- **Go files changed.** `git diff --name-status v1.33.0 e02e63f -- '*.go'` lists ten:
  - the non-test files: `connection.go`, `types_gen.go`, `agentproc/agentproc.go`, and
    `cmd/generate/internal/model/gates.go`;
  - two new test files: `connection_overflow_boundary_test.go` and `agentproc/abandon_test.go`;
  - four test files changed.
- **Unchanged** (`git diff --stat v1.33.0 e02e63f`, empty for these):
  - `go.mod`, `go.sum` and `cmd/generate/go.mod`;
  - the schema, `README.md`;
  - `acptest/`, `clienthost/`, `remote/`, `internal/`, `unstable/`;
  - every `*_gen.go` file but `types_gen.go`, which loses `ConfigOptionUpdate.gates`, 9 lines.
- **`make api` on `e02e63f`:** exit 0, "against v1.33.0: 0 incompatible, 0 compatible"; "reported packages unstable, remote: 0
  incompatible, not gated". Step 6.1's `api` gate had the same.
- **The gate set** on `03acda5`'s tree, the runtime PLAN's phase 6 step 1: 18 of 18, with the Linux and Windows hosts.
- **CI on `e02e63f`:** run `42074319831`, "completed success", six jobs (`validate` on Ubuntu, Windows and macOS at Go
  `1.27.3` and `1.26.10`), the push's paths taking in `connection.go`; the Docs workflow's run `42074319832`, "completed
  success".
- **The consumer.** `tools/consumer.py` changed in `01dfcec` only to refuse a Go environment that goes around the proxy or the
  checksum database; its program is unchanged. `consumer.py v1.33.1` runs after the tag.
