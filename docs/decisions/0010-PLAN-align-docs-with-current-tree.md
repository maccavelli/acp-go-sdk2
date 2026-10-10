---
status: complete
date: 2026-10-10
associated-madr: "0010-MADR-align-docs-with-current-tree.md"
---

# Implement Bring the user-facing docs in line with the current tree

Associated MADR: [0010-MADR-align-docs-with-current-tree.md](0010-MADR-align-docs-with-current-tree.md)

## Goal

`README.md`, `RELEASING.md`, `docs/architecture.md`, and `docs/README.md` describe the tree at `f8244c9`, and every name they state exists in it.

## Scope

In scope:

* `README.md`, `RELEASING.md`, `docs/architecture.md`, `docs/README.md`
* This pair, and its rows in `docs/README.md`

Out of scope:

* Go source, generated files, tests, `Makefile`, CI, and `communique.toml`
* `AGENTS.md` and the per-agent pointer files, which match the tree
* Decision records and reports other than this pair; they are historical
* Release notes for 0009; they belong to the GitHub release at the next tag
* A tag, a push, or a commit by the agent on `main`

## Implementation Steps

1. Work on `main`. At the end, run `make check-records` and `make markdownlint`, record their output, and stop. The owner commits with
   `git commit --no-edit`.
2. `README.md`:
   * In "If you're building a Client", replace "Implement the `acp.Client` interface (and optionally `acp.ClientTerminal` for terminal features)" with
     "Implement the `acp.Client` interface. It includes the file-system and terminal methods, such as `ReadTextFile` and `CreateTerminal`."
   * Add `-yolo` to the flags listed for `go run ./example/gemini`.
   * After the helper-constructor list, add a section "### Wire encoding" stating: the SDK encodes and decodes with `encoding/json` (v1); encoding a
     message whose required union has no variant set fails with an error that names the union, such as "RequestPermissionOutcome: no variant set";
     a required array or map left nil is sent as `[]` or `{}`; decoding a union whose discriminator is present but not a string fails with an error
     naming the union and the key; and the decision behind these is [0009-MADR-fix-generated-output-on-json-v1.md](0009-MADR-fix-generated-output-on-json-v1.md).
3. `RELEASING.md`:
   * Use `1.25.0` as the example schema stamp in "Bump the Schema Version", Option A, and Option B, in place of `v0.4.3` and `0.4.3`.
   * Use `vX.Y.Z` as the module tag in "Tag and Publish", in place of `v0.4.3`.
   * In "Tag and Publish" step 2, ask the release notes to call out any change to what the SDK sends or accepts on the wire, as the decision records
     describe it.
4. `docs/architecture.md`: keep the present-tense, no-history rule. Rewrite "What it is" and extend "Tree" to cover:
   * the root package `acp`: hand-written `agent.go`, `client.go`, `connection.go`, `errors.go`, `extensions.go`, `helpers.go`, `doc.go`; generated
     `constants_gen.go`, `types_gen.go`, `agent_gen.go`, `client_gen.go`, `helpers_gen.go`, never edited by hand;
   * `cmd/generate`, its own module with no requirements, with `internal/load` (reads the schema), `internal/ir` (method groups), `internal/emit`
     (builds `go/ast` and prints through `go/format`), and `internal/util`; `make version` runs it;
   * encoding: `encoding/json` v1 only, and the generated encoding rules stated in step 2;
   * the guards: `TestGeneratedFilesUpToDate`, the goldens in `testdata/json_golden/` checked by `json_parity_test.go`, `json_v1_pin_test.go`,
     `generated_output_test.go`; `make pre-add-check` through `scripts/go-precheck.sh`, `make check-records`, `make markdownlint`, and
     `.github/workflows/ci.yml`.
5. `docs/README.md`: reword the three rows the MADR names to the present ("know why the generator uses go/ast instead of Jennifer", "know why the
   generated output changed after the go/ast port", "see how unset unions, nil required arrays, and union docs were fixed"); add a row "know what the SDK
   sends on the wire" pointing at `architecture.md`; add this pair's rows to the records table.
6. Check every type, function, file, path, flag, and Make target that the changed documents name against the tree with a script, and record the result.
   Run the identifier scan over the changed files.

## Verification

* The script in step 6 finds every named item in the tree.
* `make check-records` and `make markdownlint` exit 0. Both lint `README.md`, `RELEASING.md`, `docs/README.md`, and `docs/architecture.md`, and
  `check-records` resolves their relative links.
* `git diff --stat` lists only the four documents and this pair.
* The identifier scan of the changed files is clean.

## Rollout and Rollback

The owner commits on `main`. No tag. A push waits for the owner's explicit ask. Rollback is `git revert` of that commit.

## Execution record

### Steps 1 to 6, 2026-10-10

The owner accepted [0010-MADR-align-docs-with-current-tree.md](0010-MADR-align-docs-with-current-tree.md) on 2026-10-10. The steps ran on `main` from a
clean tree at `f8244c9`. The agent did not commit.

Before writing, the claims the new text makes were checked against the tree: `acp.Client` declares `CreateTerminal`, `KillTerminal`, `TerminalOutput`,
`ReleaseTerminal`, and `WaitForTerminalExit`, and no `ClientTerminal` type exists; `example/gemini` defines `-gemini`, `-model`, `-sandbox`, `-yolo`, and
`-debug`; `scripts/go-precheck.sh` lints for the host and `windows` and, with no files named, vets and tests both modules; `make version` downloads the
schema files with `curl` when `schema/version` changes and runs the generator.

* Step 2, `README.md`: the client bullet names `acp.Client` with its file-system and terminal methods; `-yolo` is listed; a "Wire encoding" section states
  the `encoding/json` v1 rule and the three generated encoding rules, and links the 0009 MADR.
* Step 3, `RELEASING.md`: the schema examples use `1.25.0`, the tag examples use `vX.Y.Z`, and the publish step asks for wire-level changes in the release
  notes. No `0.4.3` remains in the file.
* Step 4, `docs/architecture.md`: "What it is" names the hand-written and generated files, the generator module and its packages, the encoding rules, and
  the checks; "Tree" lists the product layout as well as the workspace files. A first draft said every generated `UnmarshalJSON` fills schema defaults;
  only those of structs with defaults do, and the sentence was corrected before the checks below.
* Step 5, `docs/README.md`: the three rows are reworded to the present, two rows are added, and this pair is indexed.
* Step 6: a script read every code span on an added line of the four documents and checked it against the tree. Result:
  `spans checked: 62; missing: 1; unclassified: 1`. Both were the script's limits, not errors in the text: `GOOS=windows` is set in
  `scripts/go-precheck.sh` through a variable (`GOOS="$goos"` with `windows` in the loop) and named in the `lint` target of the `Makefile`, and `go.mod`
  exists. The script was seen to fail: in a scratch copy with `acp.ClientTerminal`, `-nosuchflag`, `make nosuch`, and `docs/nosuch.md` added to
  `README.md`, it reported all four as missing.

Verification:

* `git diff --stat` lists `README.md`, `RELEASING.md`, `docs/README.md`, and `docs/architecture.md`, and this pair is untracked. No other file changed.
* `make check-records` exits 0, and resolves the new relative links, including the README's link to the 0009 MADR.
* `make markdownlint` exits 0: `Linting: 8 files`, `Summary: 0 issues in 0 files`. The 8 include the four documents.
* The identifier scan of the changed and new files found nothing.

Every acceptance criterion in Verification holds. The status is `complete`, and `docs/README.md` shows it. The owner commits on `main`.
