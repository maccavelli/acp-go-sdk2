# Release notes for v1.34.2, against v1.34.1

Observed 2026-11-07 on `e31707c`, for [0049-PLAN-generator-fixes.md](../decisions/0049-PLAN-generator-fixes.md), phase 10,
step 5. This report drafts the text of the GitHub release for `v1.34.2`, as `RELEASING.md` describes it. It decides nothing.
The release is a patch by the owner's Q29 (b) in
[0049-MADR-remediate-the-2026-11-01-debugging-pass.md](../decisions/0049-MADR-remediate-the-2026-11-01-debugging-pass.md).

Each statement names its source:

- a record;
- a commit;
- or a command, run on `e31707c`.

Summary:

- No change a consumer meets: no exported API, no generated file, no schema, and no `go` line or requirement changes.
- The generator's findings of that MADR, G1 to G13 and G15 to G37, are fixed in `cmd/generate`, which consumers do not import.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.34.2

Nothing changes for a program that uses the SDK. The exported API is `v1.34.1`'s (`make api` reports no change), and so are
the generated files, the schema, ACP `1.25.0`, the `go` lines and the requirements. This release carries the fixes to the
code generator, `cmd/generate`, for the generator findings, G1 to G13 and G15 to G37, of
[0049-MADR-remediate-the-2026-11-01-debugging-pass.md](../decisions/0049-MADR-remediate-the-2026-11-01-debugging-pass.md),
by [0049-PLAN-generator-fixes.md](../decisions/0049-PLAN-generator-fixes.md). Each fix keeps the committed schemas' output
byte for byte; each refusal names the schema path or the protocol-table rule; and each was seen to fail, with the fix
removed, in a copy of the tree.

#### For maintainers: what the generator now does

- **It writes what it checked.** It deletes a stale `*_gen.go` or `*_gen_test.go` only when its first line is the generated
  header, so a hand-written `helpers_gen_test.go` survives, and it names each file it removes. Its type check reads the tree it
  will leave, `unstable/`'s hand-written files included (G16, G1).
- **It checks more of what builds.** The root's external tests and the module packages they import, against the new root
  (G27); `linux/amd64` and `windows/amd64` too, through the go command, when their files differ from the host's (G9); at the
  language version of the `go.mod` that holds the output (G28).
- **It refuses what it would crash on or mistype:** a default that recurses (G2); a stable enum value only the live
  unstable schema adds, where it panicked, while a reworded value is no change (G15); a property of two non-null `anyOf`
  members (G17); a manifest change the stable schema has since redefined (G3).
- **It refuses what it dropped or loosened silently:** `not` and `unevaluatedProperties` off a catch-all, `const` on an
  object property, a bound under `items`, a keyword beside `$ref`, a `$ref`-only definition, an unknown `type` name, and a
  repeated JSON key (G5); a default it would not apply (G7); a `meta.json` without a version (G8); a `null` definition
  (G32); skip-invalid items that admit `null` in ways G14 missed (G37); name collisions that only the type check found
  (G26); a capability rule that cannot gate every copy of its property (G20).
- **It types more faithfully:** integer bounds exactly (G4); an optional field with a non-zero default as a pointer when
  zero is valid (G6); a one-value `enum` discriminator (G21); an `anyOf` of string consts alone as closed (G22); a required
  nullable collection keeping `null` (G23); an unknown closed value taking its default (G24); a named-array union variant
  validated and gated by item (G19); a copied deprecated property's notice (G25); `unknownKind` whenever a union keeps an
  `Unknown` (G30).
- **Its command line:** `-h` exits 0 (G33); a stray argument is refused (G11); the manifest's nested keys are matched
  exactly (G31).
- **Its tests:** the protocol table's tests read the model the SDK ships (G18); three wording scans require a rule wherever
  the schema asks for a gate or an absolute path (G29); the table's lookup takes the side (G36); two rule kinds gain tests
  (G35); the fuzzer starts from each case's meta, 123 of 230 seeds building where 1 did (G12); an engine case may carry
  `protocol.json` for a rule ACP's table lacks; `Model.Validates` is a test helper (G13); `docs/architecture.md` lists the
  table's `DocNotes` (G34).

## Sources not in the release text

- **Commits since `v1.34.1`** (`git log v1.34.1..e31707c`), eleven, and the commit that adds this report:
  - `73b0537` marks G14 released;
  - `189a307` adds the PLAN and the fifth re-assessment, with G37 and Q22 to Q29;
  - the nine phases: `5e0761b`, `1a8d09c`, `5124800`, `ac41197`, `624edc1`, `08ad4c1`, `78f72e9`, `2d8b62f` and `e31707c`;
  - the commit that adds this report: these notes, phase 10's record, and the MADR's F-G findings marked done.
- **What changed** (`git diff --name-status v1.34.1 e31707c -- . ':(exclude)cmd/generate' ':(exclude)docs'`): nothing. The
  changes are in `cmd/generate`, 83 files, 39 of them engine cases (`git diff --shortstat v1.34.1 e31707c -- cmd/generate`),
  and in `docs`.
- **Unchanged** (`git diff --stat v1.34.1 e31707c`, empty for these): `go.mod`, `go.sum`, `cmd/generate/go.mod`,
  `cmd/generate/go.sum`, the schema, `.github/`, `tools/`, `internal/`, `unstable/`, `remote/`, `agentproc/`, `clienthost/`,
  `acptest/`, `RELEASING.md`, and every `*_gen.go` and `*_gen_test.go`.
- **`make api` on `e31707c`:** "check_api: against v1.34.1: 0 incompatible, 0 compatible"; "reported packages unstable,
  remote: 0 incompatible, not gated".
