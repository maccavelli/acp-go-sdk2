---
status: in-progress
date: 2026-10-22
associated-madr: "0033-MADR-traceback-on-windows-floor-ci.md"
---

# Implement 0033: print every goroutine's stack when the Windows floor CI job crashes

Associated MADR: [0033-MADR-traceback-on-windows-floor-ci.md](0033-MADR-traceback-on-windows-floor-ci.md)

## Goal

The Windows floor CI job, `validate (windows-2025, 1.26.10)`, runs with `GOTRACEBACK=crash`, and the other five jobs as before, so that
its next crash prints every goroutine's stack.

## Scope

* **`.github/workflows/ci.yml`:** the `include` entry and the `env` line of the MADR's specification, with a comment.
* **[0032-REPORT-windows-floor-runtime-crashes.md](../reports/0032-REPORT-windows-floor-runtime-crashes.md):** a dated addendum: the
  owner's answers to its open options, this record for the first; and its item on
  [golang/go#80311](https://github.com/golang/go/issues/80311), answered: the leak guard reads a fresh count (below).
* **The records:** this pair and their rows in `docs/README.md`.
* **Not in scope:** any test, `GOEXPERIMENT`, the leak guard, another job.

## The leak guard and golang/go#80311

The report left open whether `Profile.Count` being stale for `goroutineleak` affects the root package's leak guard. A probe, a program
that leaks goroutines and reads the profile, run at go1.27.3 and at go1.26.10 with `GOEXPERIMENT=goroutineleakprofile`, gave the same
both times:

| When | `Count` | The profile `WriteTo` wrote |
| :--- | :--- | :--- |
| nothing leaked, no `WriteTo` yet | 0 | |
| 3 leaked, before `WriteTo` | 0, stale | |
| after `WriteTo` | 3 | total 3 |
| 2 more leaked, before `WriteTo` | 3, stale | |
| after `WriteTo` | 5 | total 5 |

So `Count` reports the last `WriteTo`'s count, as the issue says, and right after a `WriteTo` it is that profile's. The guard calls
`WriteTo` (`leak_test.go:49`) and then `Count` (`:54`) on every read, so it reads the count of the profile it has just written. The
addendum records this; nothing changes in the guard.

## Implementation Steps

1. **Records:** this pair and their rows; the report's addendum. This PLAN `in-progress` once approved.
2. **The workflow:** `ci.yml` as specified; `actionlint` v1.7.12; `make check-records`, `make markdownlint`, the identifier scan.
3. **Verification and close:** `make push-guard`; after the owner's push, `tools/watch_ci.py` on it; the environment check below on its
   logs; this PLAN and its row `complete`.

## Verification

**The environment check,** a script outside the tree: for a CI run, it reads each `validate` job's log (`gh run view --job --log`) and
the `env:` block GitHub prints for the job's steps, and requires `GOTRACEBACK: crash` in the Windows floor job's and no `GOTRACEBACK`
value in the other five. It is seen to fail on the logs of run `39825211145`, from before the change, where no job sets it.

| Break | The check that must fail |
| :--- | :--- |
| the run before the change, `39825211145` | the environment check: the Windows floor job without `GOTRACEBACK: crash` |

Acceptance:

* `actionlint` and the docs gates pass;
* CI's six jobs pass on the pushed commit, and the environment check holds on its logs;
* no job's tests or steps differ but for that variable.

## Rollout and Rollback

One commit for the records, one for the workflow; the workflow's reverted by reverting it. It changes what a crash of one CI job prints,
and nothing a consumer of the module sees.

## Execution record

### Step 1, records, 2026-10-22

The owner approved this PLAN. What changed: this PLAN `in-progress`, its row and the MADR's in `docs/README.md`;
[0032-REPORT-windows-floor-runtime-crashes.md](../reports/0032-REPORT-windows-floor-runtime-crashes.md)'s addendum, the owner's answers to
its open options and the leak guard's answer.

**Gates,** each exit 0: `make check-records`, `make markdownlint` and `tools/idscan.py`. No file but records changed.

### Step 2, the workflow, 2026-10-22

What changed: `.github/workflows/ci.yml`, an `include` entry `{os: windows-2025, go: '1.26.10', traceback: crash}` with a comment citing
the report and the MADR, and in the job's `env`, `GOTRACEBACK: ${{ matrix.traceback }}` with a comment that it is empty, the default, on
every job but that one.

**Checks,** each exit 0: `actionlint` v1.7.12 over the workflows, nothing printed; `make check-records`, `make markdownlint` and
`tools/idscan.py`.

**The environment check,** a script outside the tree, was first seen to fail on the logs of run `39825211145`, from before the change:
"MISS validate (windows-2025, 1.26.10): GOTRACEBACK in 0 env block(s), values none; want crash", the other five "ok", "does NOT hold",
exit 1. It runs on the pushed commit's run in step 3.
