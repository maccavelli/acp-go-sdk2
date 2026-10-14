---
status: accepted
date: 2026-10-14
decision-makers: Repository owner
---

# Release phases 1 and 2 of 0019 as `v1.14.0`, retracting the accidental `v1.13.0`

## Context and Problem Statement

A tag `v1.13.0` was pushed to `origin` at `943f073`, an unfinished commit of
[0019-PLAN-api-decisions.md](0019-PLAN-api-decisions.md), step 6, whose test updates landed in the next commit. Its CI run,
`38505697749`, failed. No GitHub release was made from it. Until then the SDK's versions were `v0.x`, and the next release was to
be `v0.18.0` ([0019-MADR-remediate-debugging-pass-findings.md](0019-MADR-remediate-debugging-pass-findings.md), decision 12).

The owner deleted the remote tag on 2026-10-14. The Go module proxy had already cached it, and a cached version stays:

* `https://proxy.golang.org/github.com/maccavelli/acp-go-sdk2/@v/v1.13.0.info` answered `200`, `{"Version":"v1.13.0",
  "Time":"2026-10-14T03:19:16Z", … "Hash":"943f07320f85920e7af4714865955bbf3d48a56b"}`, after the tag was gone from `origin`.
* `…/@latest` answered `v1.13.0`.

The module path has no major-version suffix, and `v1` needs none, so the go command takes `v1.13.0` as this module's highest
version: `go get github.com/maccavelli/acp-go-sdk2@latest`, and `go get -u` in a module that requires `v0.17.1`, resolve to a commit
whose tests fail, and a `v0.18.0` would not change that. Which version does the release take, and what becomes of `v1.13.0`?

## Decision Drivers

* `@latest` and `go get -u` must reach the release, not a commit whose tests fail.
* A version the proxy has cached cannot be removed or replaced; only the module's own `go.mod` can mark it.
* Whoever pins `v1.13.0`, or has it in a `go.sum`, should be told it is broken.
* The change must be small, reviewable, and provable before and after publication.

## Considered Options

* V1. Release as `v1.14.0`, its `go.mod` retracting `v1.13.0`.
* V2. Release as `v1.14.0` with `go.mod` unchanged.
* V3. Release as `v0.18.0` and publish a `v1.13.1` whose `go.mod` retracts `v1.13.0` and itself.
* V4. Release as `v0.18.0` and change nothing else.

## Decision Outcome

Chosen option: "V1. Release as `v1.14.0`, its `go.mod` retracting `v1.13.0`", because the owner chose on 2026-10-14 to move the
module to `v1` with this release, and the retraction, one line, makes the go command exclude `v1.13.0` and name it to whoever
uses it.

* `go.mod` gains, after the `go` line:

  ```text
  retract v1.13.0 // Published accidentally from an unfinished commit.
  ```

* Phases 1 and 2 of 0019 are released as `v1.14.0`, tagged on the first commit that carries the directive, once CI has passed on it;
  that replaces decision 12's `v0.18.0`. The release notes, drafted for `v0.18.0` in
  [0019-REPORT-release-notes-v0-18-0.md](../reports/0019-REPORT-release-notes-v0-18-0.md), say it is the first `v1` release and that
  `v1.13.0` is retracted.
* The directive stays in `go.mod` from then on.

### Consequences

* Good, because `@latest` and `go get -u` resolve to `v1.14.0`, and `v1.13.0` is no longer listed by `go list -m -versions`.
* Good, because a module that pins `v1.13.0` is told it is retracted, with the directive's comment, and offered `v1.14.0`.
* Good, because one tag and one line do it, against two tags and a retracted `v1.13.1` for V3.
* Neutral, because `v1.13.0` stays in the proxy, listed by `go list -m -retracted -versions`, and fetchable by its exact version.
* Bad, because the SDK is `v1` from this release on. Go's module rules then require a later incompatible change of the exported API
  to be released under a new module path ending in `/v2`; until now a `v0` minor release could break it, as phases 1 and 2 do.
  Phases 3 to 5 of 0019, planned to ride the releases after each lands (decision 12), are bound by that.

### Confirmation

* Before publication, a probe with a scratch file proxy, `GOPROXY=file://` and a scratch module cache, holding `v0.17.1`,
  `v1.13.0` and a `v1.14.0` whose `.mod` is the committed `go.mod`: `@latest` is `v1.14.0`, `-versions` leaves out `v1.13.0`, and
  `v1.13.0` pinned reports its retraction.
* After publication, from a scratch consumer with the default proxy and a scratch module cache: `go list -m
  github.com/maccavelli/acp-go-sdk2@latest` gives `v1.14.0`, and `go list -m -versions` does not list `v1.13.0`.

## Pros and Cons of the Options

### V1. `v1.14.0`, retracting `v1.13.0`

* Good, because the probe below shows `@latest` at `v1.14.0` and `v1.13.0` reported as retracted when pinned.
* Bad, because it needs one commit before the tag.

### V2. `v1.14.0`, `go.mod` unchanged

* Good, because it can be tagged on `873322d` at once, whose CI passed.
* Bad, because `v1.13.0` stays listed as a valid version, and pinning it reports nothing (the probe's `retracted=[]`).

### V3. `v0.18.0`, with a self-retracting `v1.13.1`

* Good, because the module stays `v0`, as decision 12 had it; it is the Go module reference's procedure for a version published by
  accident.
* Bad, because it publishes a second `v1` tag, retracted from birth, and both tags must go out together.
* Not chosen: the owner chose to release as `v1`.

### V4. `v0.18.0`, nothing else

* Good, because it costs nothing.
* Bad, because `@latest` and `go get -u` keep resolving to `v1.13.0`.

## More Information

* **Probe, 2026-10-14.** Scratch proxies for `example.com/m`, run with `go list` from a scratch consumer, `GOSUMDB=off`,
  `GOTOOLCHAIN=local`:

  | Proxy holds | `@latest` | `-versions` | `v1.13.0` pinned, `-u -retracted` |
  | :--- | :--- | :--- | :--- |
  | `v0.17.1`, `v0.18.0`, `v1.13.0` | `v1.13.0` | `v0.17.1 v0.18.0 v1.13.0` | |
  | the same and `v1.13.1` retracting `v1.13.0` | `v1.13.1` | `v0.17.1 v0.18.0 v1.13.1` | |
  | the same and `v1.13.1` retracting both | `v0.18.0` | `v0.17.1 v0.18.0` | |
  | `v0.17.1`, `v1.13.0`, `v1.14.0` | `v1.14.0` | `v0.17.1 v1.13.0 v1.14.0` | `retracted=[]`, update `v1.14.0` |
  | the same, `v1.14.0` retracting `v1.13.0` | `v1.14.0` | `v0.17.1 v1.14.0` | `retracted=[Published accidentally from an unfinished commit.]`, update `v1.14.0` |

* **Source.** The Go module reference, <https://go.dev/ref/mod>, "retract directive": "To retract a version, a module author should
  add a `retract` directive to `go.mod`, then publish a new version containing that directive. The new version must be higher than
  other release or pre-release versions"; "Retracted versions are excluded when resolving version queries like `@>=v1.2.3` or
  `@latest`."
* **Related records.** [0019-MADR-remediate-debugging-pass-findings.md](0019-MADR-remediate-debugging-pass-findings.md), decision 12,
  amended by this record; [0019-REPORT-release-notes-v0-18-0.md](../reports/0019-REPORT-release-notes-v0-18-0.md), the release's
  notes.
* **Implementation.** [0020-PLAN-retract-accidental-v1-tag.md](0020-PLAN-retract-accidental-v1-tag.md).
