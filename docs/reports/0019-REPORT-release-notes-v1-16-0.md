# Release notes for v1.16.0, against v1.15.2

Observed 2026-10-19 on `82d071d`, for [0019-PLAN-idioms-performance.md](../decisions/0019-PLAN-idioms-performance.md), step 10.
This report drafts the text of the GitHub release for `v1.16.0`, as `RELEASING.md` describes it. It decides nothing.

Each statement names its source:

- a record;
- a commit;
- a benchmark run described below;
- or the exported API listing of the 0013 report's listing program, run on `git archive v1.15.2` (2429 lines) and on the tree at
  `82d071d` (2429 lines).

Bring it up to date if more changes land before the tag. No exported API changes. The owner decided on a minor release, `v1.16.0`
([0019-MADR-remediate-debugging-pass-findings.md](../decisions/0019-MADR-remediate-debugging-pass-findings.md), question 21), and
confirms it at the tag.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.16.0

The ACP schema is still `1.25.0` (`git diff v1.15.2 -- schema/` is empty), and the exported API is unchanged. This release is phase 5
of [0019-MADR-remediate-debugging-pass-findings.md](../decisions/0019-MADR-remediate-debugging-pass-findings.md): the generated code
encodes and decodes with fewer allocations, the connection does less work per message, and both the SDK and its generator follow
current Go idioms. The bytes on the wire do not change: a new test compares the encoding of every schema instance, 1248 of them, with
a committed file.

#### Performance

Medians of six runs on one host, `v1.15.2` against this release. The benchmarks are new in this release, so `v1.15.2` was measured
with them added and nothing else changed:

| Benchmark | ns/op, `v1.15.2` | ns/op, `v1.16.0` | allocs/op, `v1.15.2` | allocs/op, `v1.16.0` |
| :--- | ---: | ---: | ---: | ---: |
| `SessionUpdate` encode | 3,188 | 943 | 37 | 9 |
| `SessionUpdate` decode | 5,959 | 4,086 | 45 | 27 |
| `SessionNotification` decode | 7,516 | 5,723 | 58 | 39 |
| `session/update` sent and received over pipes | 18,526 | 13,167 | 115 | 65 |
| a prompt round trip over pipes | 30,974 | 25,496 | 149 | 97 |
| the key of JSON-RPC id `12345` | 472 | 15 | 10 | 1 |
| outbound validation of a value | 51 | 53 | 2 | 2 |

- **Union encoding** marshals once. A union whose variant carries its discriminator sets it on a copy of the variant, where the
  generated code marshalled the variant, decoded it into a map, set the discriminator, and marshalled again. (`5d32071`.)
- **Union decoding** parses the object once at each level. The chosen variant decodes from the key map its union already built, except
  six catch-all variants that keep undeclared keys as received. (`6d60d84`.)
- **Schema defaults** are Go values written by the generator. A missing field with a default no longer decodes the default's JSON at
  run time. (`d494964`.)
- **A plain integer JSON-RPC id** is its own key, without a JSON decoder; any other id still takes the decoder. A fuzz target checks
  the two agree on every input. (`95d66e3`.)
- **The generated sends and results** pass pointers, so a request's parameters and a handler's result are not copied to be validated.
  (`95d66e3`.)
- **The connection's waits** are woken through `context.AfterFunc`, where three helper goroutines waited for a context to end.
  (`95d66e3`.)

#### Behaviour changes

- **`Prompt` logs a cancel it could not send.** When `Prompt`'s context ends after its request was sent, it sends `session/cancel` on a
  best-effort basis, as before. If that send fails, the error is now logged at Debug on the connection's logger, "failed to send the
  cancel of a prompt turn", where it was discarded. Nothing else changes. (`172e9cc`.)

#### For contributors

- **Lint covers the generated files.** `.golangci.yml` adds `copyloopvar`, `intrange`, `modernize`, `perfsprint`, `thelper` and
  `usestdlibvars`, and lints `_gen.go` files except for `goconst`, `gocognit` and `gocyclo`. Each finding is fixed in the generator or
  the hand-written code. `make lint` reports 0 issues in both modules, for the host and for `GOOS=windows`. (`711a1d5`, `82d071d`.)
- **`go fix -diff ./...` reports nothing** in either module. (`711a1d5`.)
- **New checks:** `TestEncodingsUnchanged` and `testdata/encodings.golden` (`-update` rewrites the file), `TestSchemaDefaultsMatchTheirJSON`,
  and `FuzzCanonicalIDKey` in `make fuzz`. (`62f5989`, `d494964`, `95d66e3`.)
- **`make bench`** runs the seven benchmarks of `bench_test.go` six times. CI does not run them. (`62f5989`.)
- **The mutation tool stops on an interrupt.** `go run ./internal/mutate` ends its running `go` commands on `SIGINT` or `SIGTERM`,
  starts no other mutant, removes its temporary files, and writes no report. (`711a1d5`.)

## Sources not in the release text

- **Commits since `v1.15.2` that change Go files outside `cmd/generate`**
  (`git log v1.15.2..HEAD -- '*.go' ':!cmd/generate'`): `62f5989`, `172e9cc`, `d494964`, `5d32071`, `6d60d84`, `95d66e3`, `711a1d5`
  and `82d071d`.
  - The SDK's own non-test files that change are `types_gen.go`, `agent_gen.go`, `client_gen.go`, `connection.go`, `errors.go` and
    `extensions.go`: 3754 lines added and 2314 removed, nearly all in `types_gen.go`.
  - The other commits since `v1.15.2` change records.
- **The API listing:** 0 lines removed, 0 added, `v1.15.2` against `82d071d`.
- **The benchmark run:** `git archive` of `62f5989`, which adds `bench_test.go` to the `v1.15.2` code and changes no other non-test Go
  file (`git diff v1.15.2 62f5989 -- '*.go' ':!*_test.go'` is empty), and of `82d071d`. Each ran
  `go test -run '^$' -bench . -benchmem -count=1 .` six times, alternating with the other, on one `darwin/arm64` host with `go1.27.2`.
  The ns/op figures vary with the host. The allocs/op figures are what the PLAN's acceptance compares, and each was the same in all
  six runs of `82d071d`.
  - The table's rows are, in order, `BenchmarkSessionUpdateMarshal`, `BenchmarkSessionUpdateUnmarshal`,
    `BenchmarkSessionNotificationUnmarshal`, `BenchmarkSessionUpdateNotify`, `BenchmarkPromptRoundTrip`, `BenchmarkCanonicalIDKey`
    and `BenchmarkValidateOutboundValue`.
  - `BenchmarkValidateOutboundValue` keeps its 2 allocations: the value is still copied to be addressed, and the PLAN claimed no saving
    there.
- **Behaviour kept, with its evidence in the PLAN's execution record:**
  - the wire bytes: `TestEncodingsUnchanged`, from step 2 on, under both JSON backends;
  - the decoded values: a dump of 7,812 mutated payloads, decoded before and after step 6, was identical;
  - `(*RequestError).Error`'s text: `TestRequestErrorText` passes on the code before step 7 and after it;
  - the defaults: `TestSchemaDefaultsMatchTheirJSON` requires each rendered default to equal its JSON decoded.
- **Not visible to callers:** a handler's result now reaches the connection as a pointer (`95d66e3`). No exported hook receives it, and
  it is only encoded. Every superseded prompt turn's cause is one package-level error (`172e9cc`), with the same text and still
  wrapping `acp.ErrPromptCancelled`.
- **Execution records:** each step, its planted breaks, its gates and a Windows run, with the deviations of 2026-10-18 and 2026-10-19,
  are in [0019-PLAN-idioms-performance.md](../decisions/0019-PLAN-idioms-performance.md).
- **Not in this release:** schema `1.26.0`, which the owner decided not to pin yet (the MADR's question 16).
