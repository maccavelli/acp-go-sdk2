# Release notes for v1.31.1, against v1.31.0

Observed 2026-11-01 on `32d0b56`, for
[0046-PLAN-sending-and-receiving-middleware.md](../decisions/0046-PLAN-sending-and-receiving-middleware.md), phase 8, step 6. This
report drafts the text of the GitHub release for `v1.31.1`, as `RELEASING.md` describes it. It decides nothing. It is committed
before the tag, so the tagged tree holds it. The commit that adds it changes records and nothing else, so the API it tags is
`32d0b56`'s.

Each statement names its source:

- a record;
- a commit;
- or `make api`, run on `32d0b56` with nothing uncommitted.

Summary:

- No exported API change. `make api` against `v1.31.0`: "0 incompatible, 0 compatible", so the release is a patch, as
  `RELEASING.md` says, and as the owner chose (the 0046 MADR's "Amendment, 2026-11-01", Q3).
- No change of behaviour: of the root package's non-test `.go` files, only `observer.go` changed, in nine comment lines
  (`git diff v1.31.0 32d0b56 -- observer.go`).
- What a consumer may meet: `WithObserver`'s godoc and the README now say when the observer runs, which `v1.31.0` did as it does
  now but did not document.
- The `go` lines, the requirements, the schema and every generated file are unchanged.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.31.1

The ACP schema is still `1.25.0` (`git diff v1.31.0 -- schema/` is empty). The exported API and the package's behaviour are
unchanged: `make api` reports no change against `v1.31.0`. This release documents when the observer `v1.31.0` added runs, and pins
it with a test ([0046-MADR-sending-and-receiving-middleware.md](../decisions/0046-MADR-sending-and-receiving-middleware.md),
"Amendment, 2026-11-01: when the observer runs").

#### Documented: when the observer runs

`WithObserver`'s function runs after the connection has acted on the message:

- after the answer to a fault or a refusal is written;
- after the request a `$/cancel_request` names is cancelled;
- after the connection's own `$/cancel_request` is written;
- after the drop handler, for a notification dropped under `OverflowDropNewest`;
- after a response is found to match no call.

The one exception is the notification whose overflow closes the connection: it is reported before the connection closes.

So a peer can read a refusal before the observer has counted it. Code that counts refusals, and checks the count when its refused
call returns, waits for the observer rather than for the answer. `v1.31.0` already behaved this way; it is now stated in the godoc
and in the README's "What the chains do not see", and `TestObserverRunsAfterActing` fails if a site's order changes. (`cbdd788`.)

#### For maintainers

- **Tests.** `TestObserverRunsAfterActing`, seven subtests in synctest bubbles: a fault, a refusal over the concurrency limit and
  one past the byte budget, a cancel received, a cancel sent, a drop, and the overflow that closes the connection. Each was seen
  to fail with its site's order reversed. (`cbdd788`.)
- **The pre-add check.** `make pre-add-check` now lints, vets and tests both modules whole, with `go test -race -shuffle=on`,
  uncached, as CI's race step does, whatever files are named. Before, it tested only the packages of the named files, and from
  the test cache, which does not see the root files `cmd/generate`'s tests read, nor what `TestExamplePair` builds in a
  subprocess. A change to `README.md`, `schema/`, `testdata/` or `go.mod` alone is now checked; when every named file is
  docs-safe, by CI's list, no Go check runs. `make test` runs uncached too, and the gate set's `newfiles` gate runs gofmt only.
  ([0047-MADR-pre-add-check-full-coverage.md](../decisions/0047-MADR-pre-add-check-full-coverage.md); `7ded01e`, `3f0684e`.)
- **`scripts/check_ci_filters.py --docs-safe PATH...`** answers whether every path is one CI ignores. (`7ded01e`.)

## Sources not in the release text

- **Commits since `v1.31.0`** (`git log v1.31.0..32d0b56`), eight:
  - `cc2a72d` marks the 0046 PLAN complete;
  - `f00a567` proposes the 0047 pair and the 0046 amendment;
  - `7ded01e` changes `scripts/check_ci_filters.py` and `scripts/test_check_ci_filters.py`;
  - `3f0684e` changes `scripts/go-precheck.sh`, `Makefile`, `tools/gates.py`, `AGENTS.md` and `docs/architecture.md`;
  - `bccb108` and `a863ba1` record 0047's phase 3 and its close;
  - `cbdd788` changes `observer.go`, `connection_observer_test.go` and `README.md`;
  - `32d0b56` records `cbdd788`'s CI in the 0046 PLAN;
  - each changes `docs/README.md`, a MADR, a PLAN or a report as well (`git show --name-only`).
- **Go files changed.** `git diff --name-status v1.31.0 32d0b56 -- '*.go'` lists two: `observer.go`, nine comment lines added,
  and `connection_observer_test.go`.
- **Unchanged** (`git diff --stat v1.31.0 32d0b56`, empty): `go.mod`, `go.sum`, `cmd/generate/`, the schema, the workflows,
  `RELEASING.md`, `acptest/`, `agentproc/`, `clienthost/`, `remote/`, `internal/`, every `*_gen.go` file, `connection.go`,
  `middleware.go` and `errors.go`.
- **`make api` on `32d0b56`:** exit 0, "against v1.31.0: 0 incompatible, 0 compatible"; "reported packages unstable, remote: 0
  incompatible, not gated".
- **CI on `cbdd788`**, pushed with `a863ba1`: run `41338264422`, "completed success", six jobs on Ubuntu, Windows and macOS at Go
  `1.27.3` and `1.26.10`; the Docs workflow's run `41338264421`, "completed success". CI on `bccb108`, 0047's last code commit:
  run `41334498180`, "completed success".
- **The consumer.** `tools/consumer.py` is unchanged: since `v1.31.0` its program waits for the observer's report of the refusal,
  as the godoc now says to. `consumer.py v1.31.1` runs after the tag.
- **The evidence:**
  - the 0046 PLAN's phase 8: the test, its five planted breaks, and the gates, 18 of 18, on macOS and on the Linux and Windows
    hosts;
  - the 0047 PLAN: its research, its phase 3 confirmation in scratch clones, and its gates.
