# Release notes for v1.34.0, against v1.33.1

Observed 2026-11-05 on `3013065`, for
[0050-PLAN-auth-method-ids-and-the-prompt-result-for-issue-14.md](../decisions/0050-PLAN-auth-method-ids-and-the-prompt-result-for-issue-14.md),
phase 4, step 3. This report drafts the text of the GitHub release for `v1.34.0`, as `RELEASING.md` describes it. It decides
nothing. It is committed before the tag, so the tagged tree holds it. The commit that adds it changes records and nothing
else, so the API it tags is `3013065`'s.

Each statement names its source:

- a record;
- a commit;
- or `make api`, run on `3013065` with nothing uncommitted but records.

Summary:

- 2 compatible additions and nothing incompatible. `make api` against `v1.33.1`: "0 incompatible, 2 compatible",
  `AuthMethod.ID` and `AuthenticateMethodIDs`, both in the root package, so the release is a minor, as `RELEASING.md` says
  and the MADR's D9 decided.
- It answers GitHub issue #14, by
  [0050-MADR-auth-method-ids-and-the-prompt-result-for-issue-14.md](../decisions/0050-MADR-auth-method-ids-and-the-prompt-result-for-issue-14.md),
  in D9's order:
  - D4: the refusal of an unadvertised id names the ids accepted;
  - D3: two helpers give those ids and one entry's id;
  - D1: a behaviour change, `"unknown"` no longer names an entry without a string id;
  - D5: `PromptResponse`'s godoc says what a zero `StopReason` does;
  - D6: the documentation.
- The schema, the `go` lines and the requirements are unchanged.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.34.0

The ACP schema is still `1.25.0` (`git diff v1.33.1 -- schema/` is empty). `make api` reports two compatible additions
against `v1.33.1`, both in the root package, and nothing incompatible. This release answers issue #14: the `authenticate`
ids a client may send, and the `PromptResponse` an agent may return
([0050-MADR-auth-method-ids-and-the-prompt-result-for-issue-14.md](../decisions/0050-MADR-auth-method-ids-and-the-prompt-result-for-issue-14.md)).

One change refuses a call that `v1.33.1` sent. Read "Changed: an unknown entry counts only by a string id" before upgrading
a client that authenticates against methods this SDK does not know.

#### Changed: the refusal names the ids accepted (D4)

`Authenticate` with an id the agent's `initialize` answer did not advertise returns an error wrapping
`acp.ErrUnadvertisedAuthMethod`, and sends nothing; received, it is answered `-32602`. Its text now ends with the ids the
answer accepts:

```text
authenticate: methodId "nope": acp: methodId names no auth method the agent advertised; authenticate accepts "openai", "grok"
```

or `…; authenticate accepts none` when there are none. The prefix and the sentinel are unchanged, so `errors.Is` holds. The
same text reaches the peer in the `-32602`'s `data.error`. It lists only the agent's own ids, which the client received in
`initialize`. (`ba14c67`.)

#### Added: `AuthenticateMethodIDs` and `AuthMethod.ID` (D3)

- **`acp.AuthenticateMethodIDs(methods []AuthMethod) []AuthMethodId`** returns the ids `Authenticate` sends, from an
  `initialize` answer's `AuthMethods`, as `Initialize` returns it or as `Negotiated()` records it. They are given in the
  answer's order, each once: every `agent` method's id, and the string `"id"` of an entry of a type this SDK does not know.
  It leaves out any id a `terminal` method names, even one an `agent` method also names, and returns nil for none.
  - The connection's gate reads the same generated list, so an id it returns is sent, and every other id is refused before
    sending.
  - A client that wants `agent` methods alone filters by `Kind()` first.
  - `ExampleAuthenticateMethodIDs` keeps a configured id when it is accepted, and falls back to the first accepted id
    otherwise. (`ea5de92`.)
- **`AuthMethod.ID() AuthMethodId`** returns one entry's id: the `Terminal`'s or the `Agent`'s `Id`, or an unknown entry's
  string `"id"`. It returns `""` for an unknown entry without one, and for the zero value. It sits beside `Kind()`, for
  display and matching. (`ea5de92`.)

#### Changed: an unknown entry counts only by a string id (D1)

An auth method of a type this SDK does not know is kept in `AuthMethod.Unknown`. Through `v1.33.1`, the gate read its
`"id"` through the helper that reads discriminators, which gives `"unknown"` when the key is absent, empty or not a string.
So such an entry advertised the id `"unknown"`, and `Authenticate` sent `methodId: "unknown"` against it, which the schema
forbids.

- **Now** such an entry counts by its `"id"` only when that is a JSON string, read with the decoder's key folding, so
  `"ID"` counts too.
- **An entry whose `"id"` is `""`** advertises `""`, as an `agent` method with an empty `id` does, and no longer
  `"unknown"`.
- **An entry with no `"id"`,** or one that is `null`, a number or an object, advertises nothing. `methodId: "unknown"`
  against it is refused with `ErrUnadvertisedAuthMethod`, before sending; received, it is answered `-32602`.
- **An entry whose `"id"` is the string `"unknown"`** is still accepted. (`ba14c67`.)

#### Documented: a `PromptResponse` needs a `StopReason` (D5)

`PromptResponse`'s godoc now says that `StopReason` is required and has no default. Its zero value, `""`, is not a
`StopReason`, so an agent whose `Prompt` returns it answers `-32603` instead of sending it. A successful turn sets one of
the constants, such as `StopReasonEndTurn`. The behaviour is unchanged from `v1.33.1`; the godoc is new. (`d339cf1`.)

#### Documented: the auth sentinels and the helpers (D6)

- The package doc's list of sentinels, and `Negotiation.AllowsValue`'s godoc, name `ErrUnadvertisedAuthMethod`.
- The README's `authenticate` passage gives the refusal's text, the unknown entry's rule and the two helpers, and its client
  section points to `AuthenticateMethodIDs`.
- `docs/architecture.md` names the helpers and the generated code behind them. (`ea5de92`.)

#### For maintainers

- **The generator.** The protocol table links each advertised check to the terminal check on the same property. The emitter
  writes, beside the gate, an unexported function from the answer's list to the ids accepted, which the gate and
  `AuthenticateMethodIDs` share, and the helpers `unknownString` and `quotedIDs`. `unknownKind` reads through
  `unknownString` and keeps its fallback. A new engine case, `accept-advertised-auth-ids`, checks the emitted list and gate on
  a synthetic schema. (`ba14c67`.)
- **The protocol table** gains a doc note on `PromptResponse`, and the test that counts the notes counts ten. (`d339cf1`.)
- **The test agents.** The three `acp_test.go` stubs whose `Prompt` returned `PromptResponse{}` return `StopReasonEndTurn`.
  The root's only zero result returned without an error is the one `TestClosedEnumsRefusedOnSend` plants. (`d339cf1`.)
- **Tests.** Each decision has a test that fails with its change undone in a copy of the tree. The PLAN's phases 1 to 3
  record each planted break and its failure, and phase 4 lists them by decision.
- **Benchmarks.** Allocations per operation are unchanged in the five prompt and notification benchmarks of `bench_test.go`,
  before phase 1 and after phase 3.

## Sources not in the release text

- **Commits since `v1.33.1`** (`git log v1.33.1..3013065`), six:
  - `27f0cc5` marks the runtime PLAN and R11 complete;
  - `4037547` adds the 0050 MADR and PLAN;
  - `ba14c67`, phase 1 (D1, D2, D4): `cmd/generate/internal/model/gates.go`, `emit/decode.go`, `emit/gates.go`, the engine
    case `accept-advertised-auth-ids.txtar` (new), `types_gen.go`, `gates_surface_test.go`, `model/gates_test.go`;
  - `ea5de92`, phase 2 (D3, D6): `helpers.go`, `doc.go`, `docs_test.go`, `auth_method_ids_test.go` and
    `example_auth_test.go` (new), `README.md`, `docs/architecture.md`;
  - `d339cf1`, phase 3 (D5, D7, and the deviation): `cmd/generate/internal/model/acp.go`, `model/patch_test.go`,
    `types_gen.go`, `docs_test.go`, `acp_test.go`;
  - `3013065`, phase 4's gates and the table of decisions.
- **Go files changed.** `git diff --name-status v1.33.1 3013065 -- '*.go'` lists 14: `helpers.go`, `doc.go`, `types_gen.go`,
  five root test files, two of them new, `auth_method_ids_test.go` and `example_auth_test.go`, and six in `cmd/generate/internal/`.
- **Unchanged** (`git diff --stat v1.33.1 3013065`, empty for these): `go.mod`, `go.sum`, `cmd/generate/go.mod`,
  `cmd/generate/go.sum`, the schema, `.github/`, `tools/`, `internal/`, `unstable/`, `remote/`, `agentproc/`, `clienthost/`,
  `acptest/` and `RELEASING.md`.
- **`make api` on `3013065`,** with only records uncommitted: exit 0, "compatible: AuthMethod.ID: added", "compatible:
  AuthenticateMethodIDs: added", "against v1.33.1: 0 incompatible, 2 compatible"; "reported packages unstable, remote: 0
  incompatible, not gated". Step 4.1's `api` gate had the same.
- **The gate set,** the PLAN's step 4.1: 18 of 18 on `d339cf1`, with the Linux and Windows hosts.
- **CI,** step 4.2: on `3013065`, the Docs run and the six `validate` jobs, `success`.
- **The consumers,** the MADR's F10, read as data; this repository changes neither:
  - magic-cli-remote, on `v1.33.0`, builds its own list of advertised ids from `agent` entries alone, and authenticates a
    configured pin, or the spec's key method, without checking it. From `v1.32.0` on, an unadvertised pin is refused
    before sending. `AuthenticateMethodIDs` is the list the gate reads.
  - gobble-cli, on `v1.31.2`, has two tests that expect an unknown id back as `-32602`. They fail on its upgrade to
    `v1.32.0` or later, where the client refuses the id before sending; this release does not change that.
  - `tools/consumer.py v1.34.0` runs after the tag.
