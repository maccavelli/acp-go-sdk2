---
status: in-progress
date: 2026-10-10
associated-madr: "0011-MADR-harden-code-generator-engine.md"
---

# Implement phase 5 of the generator hardening: safe output

Associated MADR: [0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md)

## Goal

The generator renders all five files in memory, type-checks them with the hand-written package of the output directory, and only then writes them.
It stages every changed file beside its destination and renames it into place, so the tree is never half-written by a failure before the first rename.
It leaves unchanged files alone and writes mode `0644`. No step of the engine panics. The generated files and the SDK's exported API do not change.
This is phase 5 of the MADR, as refined by its amendment "phase 5 facts and decisions", whose decisions the owner made on 2026-10-10.

## Scope

In scope:

* `cmd/generate/internal/emit`: `file.go`, `types.go`, `constants.go`, `dispatch.go`, `helpers.go`, and their tests (`file_test.go`,
  `dispatch_test.go`)
* a new package `cmd/generate/internal/output`, with `check.go`, `write.go`, and their tests
* `cmd/generate/main.go` and `main_test.go`
* `docs/architecture.md` and `RELEASING.md` (step 6)

Out of scope:

* phase 6 of the MADR, engine tests beyond the ones this plan adds;
* what [0012-PLAN-schema-fidelity-framework.md](0012-PLAN-schema-fidelity-framework.md) left undone: the dead trial decode in five unions, `_meta`
  numbers as `float64`, `RequestId`'s `Null`, `cancelRequestParams`, ACP's v2 schemas;
* syncing the output directory, retrying a rename on Windows, a lock against two runs at once, a flag that turns the check off, and
  `golang.org/x/tools/go/packages` (the amendment's decisions);
* a `.gitignore` entry for leftover temporary files, and any `Makefile` change;
* the goldens, `encoding/json/v2`, a tag, a push, an agent commit on `main`.

## Grounding

Read on `d534f27`; the probes ran in scratch copies and scratch modules, and nothing in the tree changed. The amendment holds the measurements and
the quotations; this section names where the code stands.

* **G1. The write path.** `run` (`main.go:25`) loads the schema and builds the model, then `os.MkdirAll(*outDir, 0o750)` (`main.go:54`), then
  `emit.WriteConstants`, `WriteTypes`, `WriteDispatch`, `WriteHelpers` (`main.go:58`–`68`). `WriteDispatch` (`dispatch.go:11`) calls `writeAgentGen`
  and `writeClientGen`. Each ends in `goFile.write` (`file.go:72`): `render`, then `os.WriteFile(filepath.Join(outDir, name), src, generatedFileMode)`.
  `generatedFileMode` is `0o600` (`constants.go:18`).
* **G2. The panics.** `fieldSet.fieldList` (`types.go:29`–`34`), reached through `structType` and `interfaceType`, from `types.go:98`, `:178`,
  `:824` and `file_test.go:65`. `printedSize` (`file.go:188`–`194`), called by `layout.decl` (`file.go:242`). `layout.apply` (`file.go:223`–`235`),
  called by `goFile.layout` (`file.go:145`), which `render` and `TestLayoutStaysBelowPrinterSentinel` (`file_test.go:165`) call.
* **G3. Tests that write.** `TestGeneratedFilesUpToDate` (`main_test.go:32`) calls the four `Write*` into `t.TempDir()` and compares bytes;
  `TestDispatchFollowsTheModel` (`dispatch_test.go:54`) calls `WriteDispatch`, and the protocol-constant test (`dispatch_test.go:118`)
  `WriteConstants`. `TestRunRefusesBeforeWriting` (`main_test.go:96`) runs five hostile schemas through `run` and requires only `keep.txt` to remain.
  `TestRunGeneratesRecasedUnion` (`main_test.go:163`) runs `run` into an empty directory, then type-checks the output with a stub of the hand-written
  names (`handWritten`, `main_test.go:150`).
* **G4. The two failures to fix.** The amendment's scenarios, kept as `p5_failmodes.py` in the session scratchpad and rerun in steps 3 and 5: a
  declaration that cannot be printed leaves `constants_gen.go` rewritten and exits 1; a call to `toReqErrMissing` rewrites all five files and exits 0.
* **G5. Lint.** gosec reports `os.WriteFile` with `0o644` (G306) and `os.ReadFile` of a joined path (G304); `load.go` carries
  `//nolint:gosec // schemaDir is the generator's explicit input.` for the latter. `(*os.File).Chmod(0o644)`, `os.CreateTemp`, and `os.Rename` pass.
* **G6. The output directory.** The repository root holds seven hand-written non-test files, none with a build constraint or a platform suffix, and
  `go.mod` with `go 1.27.2`. The root imports only the standard library.
* **G7. Coverage on `d534f27`.** `internal/model` 93.9%, `internal/emit` 81.2%.

## Design

### The flow

`run` becomes: load the schema; build the model; `files, err := emit.Render(m)`; `output.Check(outDir, files)`; `os.MkdirAll(outDir, 0o750)`;
`output.Write(outDir, files)`. Any error returns before the next step, so nothing is created or written unless every step before it passed. `run`'s
doc comment says so.

### Rendering

```go
// File is one generated file: its name in the output directory and its formatted source.
type File struct {
	Name string
	Src  []byte
}

// Render builds and formats every generated file, in the order the generator writes them.
func Render(m *model.Model) ([]File, error)
```

`Render` returns `constants_gen.go`, `types_gen.go`, `agent_gen.go`, `client_gen.go`, `helpers_gen.go`, in that order, or the first error, as
`<name>: <cause>`. The bodies of `WriteConstants`, `WriteTypes`, `writeAgentGen`, `writeClientGen`, and `WriteHelpers` become unexported functions that
return the `*goFile`; the four exported `Write*` functions, `goFile.write`, and `generatedFileMode` go.

### Errors instead of panics

* `goFile` gains `err error`, the first failure recorded while the file is built, and `render` returns it before printing.
* `fieldSet.fieldList`, `structType`, and `interfaceType` take the `*goFile`. A doc comment with no field after it records
  `doc comment "<first line>" has no field after it` on the file and is dropped.
* `printedSize` returns `(int, error)`: `print <type>: <cause>`. `layout` records the first error; `layout.apply` returns `line table rejected` when
  `SetLines` fails. `goFile.layout` returns `(*ast.File, error)`.

### The check

```go
// Check type-checks files together with the hand-written Go files of dir, and returns an error listing what does not check.
func Check(dir string, files []emit.File) error
```

* **Hand-written files.** `build.Default.ImportDir(dir, 0)`, less the generated names. A missing directory, or `*build.NoGoError`, means none. Any
  other error is returned as `output directory: <cause>` (a `*build.MultiplePackageError` among them). A cgo file is refused as
  `cgo file <name> is not supported`.
* **Parsing.** Every file is parsed with `parser.SkipObjectResolution` under its path `filepath.Join(dir, name)`; a generated file is parsed from
  memory. A parse error is returned as it is.
* **Version.** When `dir/go.mod` exists, the first line of the form `go <version>` sets `GoVersion` to `go<version>`; otherwise it stays empty. A
  `go.mod` that cannot be read is an error.
* **Types.** `types.Config{GoVersion: ..., Importer: importer.ForCompiler(fset, "source", nil), Error: collect}`; `Check` is called with the
  package name of the generated files. The errors are sorted by file, line, and column, a secondary error (text starting with a tab) kept after its
  primary, and reported as `generated code does not type-check with <dir>:` followed by one line per error, at most 10, then `and <n> more`.

### The write

```go
// Write replaces the files of dir that differ from files, staging all of them before replacing any.
func Write(dir string, files []emit.File) error
```

1. **Compare.** Read `dir/<name>` (with `//nolint:gosec` naming G304 and the reason, as `load.go` does). Equal bytes: leave the file alone. Not found:
   stage it. Any other read error: return it, nothing staged.
2. **Stage.** For each file to replace: `os.CreateTemp(dir, "."+name+".*.tmp")`, `Write`, `Sync`, `Chmod(0o644)`, `Close`. On any error, close and
   remove every temporary file created so far and return `stage <name>: <cause>`; no file is replaced.
3. **Replace.** Rename each temporary file to `dir/<name>`, in `Render`'s order. On an error, remove the temporary files not yet renamed and return
   `replace <name>: <cause>; replaced: <names>; left as they were: <names>`. A renamed temporary file is never removed.

The mode is a constant `fileMode = 0o644` in `output`.

## Implementation Steps

1. Work on `main`. Each step writes its tests first and records their failure on the current tree, then changes the generator. It regenerates with
   `cd cmd/generate && env -u GOPATH -u GOMODCACHE go run .` and compares every generated file with the previous commit's by script. It shows each new
   check failing on a planted breakage in a scratch copy, never in the tree. It runs `make pre-add-check`, `make check-records`, and
   `make markdownlint`, records the output in this plan's execution record, and stops for the owner's commit. In every step the generated files must
   be byte-identical to `d534f27`'s, and `git status` after regenerating must show no generated file.
2. **Errors instead of panics.** Tests in `internal/emit`, `TestRenderErrors`: a file holding `typeDecl("1bad", id("int"))` fails to render with an
   error containing `print` (corrected 2026-10-10: `format:` and `expected 'IDENT'`, see step 2's record); a struct `fieldSet` with a trailing doc comment fails with `has no field after it` and the comment's text;
   `printedSize(&ast.Field{})` returns an error and does not panic; a layout with a negative reservation fails with `line table rejected`. Change
   `file.go` and `types.go` to the design; update `file_test.go` for the new signatures. Plants, in a scratch copy, one at a time: each of the three
   `panic` calls restored makes its case fail.
3. **Render in memory.** Tests: `TestGeneratedFilesUpToDate` calls `emit.Render` and compares in memory, and requires the five names in order;
   `TestDispatchFollowsTheModel` and the protocol-constant test read `Render`'s output. Add `File` and `Render`; remove the `Write*` functions,
   `goFile.write`, and `generatedFileMode`. `run` writes `Render`'s files in order with `os.WriteFile` at `0o600`, as today, until step 4. Plants:
   `Render` returning `helpers_gen.go` before `client_gen.go` fails the order check; dropping a file fails the comparison. Rerun `p5_failmodes.py`:
   expected, the render failure exits 1 with no file rewritten; the `toReqErrMissing` scenario is unchanged until step 5.
4. **Stage, then replace.** Tests in `internal/output`:
   * `TestWriteReplacesChangedFiles`: into a directory holding a stale `types_gen.go` at `0o600` and a sentinel, all five are written, each `0644`
     (Unix only; on Windows the test checks the bytes), the sentinel unchanged.
   * `TestWriteLeavesUnchangedFiles`: with modification times set an hour back by `os.Chtimes`, a second `Write` of the same files changes none,
     and a `Write` with one file changed moves only that file's time.
   * `TestWriteStagingFailureReplacesNothing`: in a directory made read-only (skipped on Windows and as root), `Write` fails with `stage` and every
     existing file keeps its bytes.
   * `TestWriteRenameFailureReports`: with a directory named `helpers_gen.go` in the way, `Write` fails naming it, lists the four replaced files,
     and the four hold the new bytes. (Changed 2026-10-10, see the deviation in step 4's record: the rename is injected, and the directory in the
     way is a test of the compare step.)
   * every test requires that no `.*.tmp` file remains.
   Add `write.go`; `run` calls `output.Write`. Plants: writing each file as it is staged fails the staging test; `0o600` fails the mode test; no
   comparison fails the modification-time test; no cleanup after a failed rename fails the temporary-file check. The `Sync` call has no test that can
   observe it; review shows it. After the step, running the generator in the tree leaves `git status` clean and the five files' modification times as
   they were.
5. **Type-check before writing.** Tests in `internal/output`:
   * `TestCheckAcceptsRepository`: `emit.Render` of the committed schema, checked against the repository root, returns nil.
   * `TestCheckRefuses`, one case each: an empty directory (`undefined: AgentSideConnection`); a hand-written file that redeclares a generated type
     (`redeclared`); a `go.mod` at `go 1.21` with a hand-written `for range 3` (`requires go1.22`); a stray `package other` file (`output directory`);
     a cgo file (`cgo file`).
   * `TestCheckSelectsLikeTheGoCommand`: the stub of the hand-written names plus a `//go:build ignore` file, a `_test.go` file, a `.x.tmp` file, and
     an `_plan9.go` file, each with an undefined name, returns nil.
   * `TestCheckReportsAtMostTen`: output with 12 errors reports 10 lines and `and 2 more`.
   In `cmd/generate`: `TestRunRefusesOutputThatDoesNotTypeCheck` runs the recased-union schema into a directory holding only `keep.txt` and requires
   the error to contain `undefined: AgentSideConnection` and the directory to hold only `keep.txt`; `TestRunGeneratesRecasedUnion` writes
   `handWritten` into its directory first. Add `check.go`; `run` calls `output.Check` before `os.MkdirAll`. Plants: `run` without the check fails
   `TestRunRefusesOutputThatDoesNotTypeCheck`; `UseAllFiles` set fails the selection test; `GoVersion` left empty fails the version case; no cap fails
   the cap test. Rerun `p5_failmodes.py`: expected, both scenarios exit 1 with no file rewritten, the second naming `agent_gen.go` and
   `toReqErrMissing`. Time the generator binary before and after, three runs each.
6. **Docs.** `docs/architecture.md`: the generator paragraph names `internal/output` and the order render, check, write; it states the mode, the
   temporary-file names, and that unchanged files are left alone; "Checks that guard this" names `TestCheckAcceptsRepository`. `RELEASING.md`: when
   `make version` refuses output that does not type-check, change the hand-written code to the new names first. The name check of phase 4
   (`w10_names.py`) runs over both files.
7. **Review and close.** Run the verification below, record each result, set this plan to `complete`, and record in the MADR that its phase 5
   amendment's decisions were carried out, or how they changed.

## Verification

* After every step, the five generated files are byte-identical to `d534f27`'s, and `TestGeneratedFilesUpToDate` passes.
* No `panic` call remains in a non-test file of `cmd/generate` (a script lists them).
* Each new test failed first, and each planted breakage of steps 2 to 5 failed a test, as the execution record shows.
* `p5_failmodes.py` after step 5: both scenarios exit 1 and leave all five files as they were.
* `go test -race -count=1 ./...` passes at the root and in `cmd/generate`; `go build ./example/...` passes.
* `go test -cover`: `internal/output` at or above 85%; `internal/emit` and `internal/model` reported, not below `d534f27`'s.
* The SDK's exported API is unchanged (the API listing program of the earlier phases, `d534f27` against the last step).
* The generator's run time is recorded before and after.
* `make pre-add-check`, `make check-records`, and `make markdownlint` pass; the identifier scan finds nothing.

## Rollout and Rollback

Commits land on `main`, made by the owner, one per step. No tag; a push waits for the owner's ask. The SDK's API and wire behaviour do not change, so
the release notes need no entry. The developer-visible changes are in `RELEASING.md`: `make version` can refuse output, and files are written `0644`.

Rollback is `git revert` of the step commits, newest first. Each step leaves the generator working and the generated files unchanged.

## Execution record

The owner decided the MADR's phase 5 amendment on 2026-10-10, committed it with this plan as `f9537bd`, and approved the plan ("proceed").

### Step 2, errors instead of panics, 2026-10-10

Ran on `main` from `f9537bd`; changed `cmd/generate/internal/emit/file.go`, `types.go`, `file_test.go`, and this record. The agent did not commit.

`TestRenderErrors` (in `file_test.go`) has four cases: a declaration named `1bad`, a struct `fieldSet` with a trailing doc comment, `printedSize` of an
`*ast.Field`, and a layout whose reservation makes the line table decrease. `TestRenderLayout` and `TestLayoutStaysBelowPrinterSentinel` take the new
signatures.

Failing first. On `f9537bd` the test file does not compile (`too many arguments in call to st.structType`, `f.layout returns 1 value`,
`printedSize returns 1 value`, `l.apply(...) (no value) used as value`). Adapted to the old signatures in a scratch copy (`s2_oldsig.py`), three
cases panic: `emit: doc comment with no field after it`, `emit: print *ast.Field: go/printer: unsupported node type *ast.Field`, and
`emit: invalid line table`.

**Correction, 2026-10-10.** The plan expected the `1bad` declaration's error to contain `print`. On `f9537bd`, render already returns an error for
it, `format: 5:6: expected 'IDENT', found 1`: `go/printer` prints the declaration and `format.Source` fails to parse it back. The case asserts `format:`
and `expected 'IDENT'`, and it guards existing behaviour rather than failing first. No decision changes.

Change, as the design says:

* `goFile` has `err` and `fail`; `render` returns the recorded error before laying out, and `goFile.layout` returns `(*ast.File, error)`.
* `fieldSet.fieldList`, `structType`, and `interfaceType` take the `*goFile`. A dangling doc comment records `doc comment "<first line>" has no field
  after it` and is dropped. The three call sites in `types.go` pass `f`.
* `printedSize` returns `(int, error)`, `print <type>: <cause>`; `layout.decl` records the first as `func <name>: ...`. `layout.apply` returns
  `line table rejected`.

No `panic` call remains in a non-test file of `cmd/generate` (`grep -n 'panic('`, no match).

Plants, one at a time in a scratch copy (`s2_plants.py`), each putting one panic back:

```text
fieldList panics: exit=1
    --- FAIL: TestRenderErrors (0.00s)
    panic: emit: doc comment with no field after it [recovered, repanicked]
printedSize panics: exit=1
    --- FAIL: TestRenderErrors (0.00s)
    panic: emit: print *ast.Field: go/printer: unsupported node type *ast.Field [recovered, repanicked]
apply panics: exit=1
    --- FAIL: TestRenderErrors (0.00s)
    panic: emit: invalid line table [recovered, repanicked]
```

The third plant first left the `errors` import unused and failed to compile; it was changed to `panic(errors.New(...))`, and the result is the one
above.

Regenerated with `cd cmd/generate && env -u GOPATH -u GOMODCACHE go run .`: exit 0, `git diff --quiet d534f27 -- '*_gen.go'` exits 0, and
`git status` shows no generated file. `go test -cover`: `internal/emit` 81.5%, `internal/model` 93.9%. `make pre-add-check`:
`go-precheck: 72 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).` `make check-records` and `make markdownlint` exit 0.

Commit point: the agent stopped here. The owner commits step 2 on `main`.

### Step 3, render in memory, 2026-10-10

The owner committed step 2 as `838a4c4`. Step 3 ran on `main` from that clean tree and changed `cmd/generate/main.go`, `main_test.go`, and, in
`internal/emit`, `file.go`, `constants.go`, `types.go`, `dispatch.go`, `helpers.go`, `dispatch_test.go`, `file_test.go`, `types_test.go`; and this
record. The agent did not commit.

Tests first. `TestGeneratedFilesUpToDate` calls `emit.Render`, requires the names `constants_gen.go`, `types_gen.go`, `agent_gen.go`, `client_gen.go`,
`helpers_gen.go` in that order, and compares each file's bytes in memory. `TestDispatchFollowsTheModel` and `TestProtocolMethodConstants` read
`Render`'s output through a helper, `rendered`. On `838a4c4` neither package's tests compile: `undefined: emit.Render` and `undefined: Render`.

Change, as the design says: `File` and `Render` in `file.go`; `WriteConstants`, `WriteTypes`, `WriteHelpers`, `writeAgentGen`, and `writeClientGen`
became `constantsFile`, `typesFile`, `helpersFile`, `agentFile`, and `clientFile`, each returning the `*goFile`; `WriteDispatch`, `goFile.write`, and
`generatedFileMode` are gone. `run` renders, then creates the directory, then writes each file with `os.WriteFile` at `0o600`, until step 4.

The first `make pre-add-check` failed on two findings in this step's own code: revive's `confusing-naming` (`Render` against the test helper
`render`) and unconvert (`string(b)` where `b` had become a string). The test helper is now `mustRender`, which is why `types_test.go` changed (its
five calls), and the conversion is gone. The exported name `Render` is the design's and stays.

Plants, in scratch copies of the repository (`s3_plants.py`), against `TestGeneratedFilesUpToDate`:

```text
helpers before client: exit=1
    --- FAIL: TestGeneratedFilesUpToDate (0.11s)
    main_test.go:52: Render returned [constants_gen.go types_gen.go agent_gen.go helpers_gen.go client_gen.go], want [constants_gen.go types_gen.go agent_gen.go client_gen.go helpers_gen.go]
helpers dropped: exit=1
    --- FAIL: TestGeneratedFilesUpToDate (0.12s)
    main_test.go:52: Render returned [constants_gen.go types_gen.go agent_gen.go client_gen.go], want [constants_gen.go types_gen.go agent_gen.go client_gen.go helpers_gen.go]
```

`p5_failmodes.py`, rerun: the render failure now exits 1 with nothing rewritten; the second scenario is unchanged, as expected until step 5.

```text
render failure in types_gen.go: generator exit=1 generate: types_gen.go: print: format.Node internal error (401:6: expected 'IDENT', found 1 (and 10 more errors))
    rewritten: []; untouched: ['constants_gen.go', 'types_gen.go', 'agent_gen.go', 'client_gen.go', 'helpers_gen.go']
output that does not type-check: generator exit=0
    rewritten: ['constants_gen.go', 'types_gen.go', 'agent_gen.go', 'client_gen.go', 'helpers_gen.go']; untouched: []
    go build after generation: exit=1 ./agent_gen.go:174:16: too many errors
```

Regenerated: exit 0, `git diff --quiet d534f27 -- '*_gen.go'` exits 0, and `git status` shows no generated file. `go test -cover`:
`internal/emit` 84.0%, `internal/model` 93.9%. `make pre-add-check`: `go-precheck: 72 file(s) clean (gofmt, golangci-lint, go vet, go test,
govulncheck).` `make check-records` and `make markdownlint` exit 0.

Commit point: the agent stopped here. The owner commits step 3 on `main`.

### Step 4, stage, then replace, 2026-10-10

**Deviation, 2026-10-10, before the step's tests were written.** The plan's `TestWriteRenameFailureReports` puts a directory named `helpers_gen.go`
in the way. The design's first stage, Compare, reads `dir/helpers_gen.go` before anything is staged, and reading a directory fails ("is a directory"),
so `Write` would return at Compare and the rename-failure path would never run. The plan was wrong, not the code. Asked, the owner chose to inject the
rename: `Write` calls an unexported `writeWith(dir, files, rename)` with `os.Rename` (named `write` when the owner was asked;
revive's `confusing-naming` rejects `write` beside `Write`), and the test passes a rename that fails on `helpers_gen.go`. The
directory in the way stays as a second test, of a failure at Compare: an error, nothing replaced, no temporary file. The options declined were
changing Compare to stage over a directory, which changes `Write`'s behaviour beyond the MADR, and dropping the test. No decision of the MADR
changes; files in scope are unchanged.

The owner committed step 3 as `9989a5c`. Step 4 ran on `main` from that clean tree and added `cmd/generate/internal/output/write.go` and
`write_test.go`, changed `cmd/generate/main.go` and this record. The agent did not commit.

Tests first, in `write_test.go`, over five small files and a check after each that no `.*.tmp` file remains:

* `TestWriteReplacesChangedFiles`: a stale `types_gen.go` at `0o600` and a sentinel; all five written, each `0644` (not on Windows), the sentinel
  unchanged.
* `TestWriteLeavesUnchangedFiles`: modification times set an hour back; a second `Write` of the same files moves none, and one with `types_gen.go`
  changed moves only that one.
* `TestWriteStagingFailureReplacesNothing`, two cases: a third file named `sub/x_gen.go`, which `os.CreateTemp` refuses after two files were staged;
  and, as the plan says, a read-only directory (skipped on Windows and as root). The first case is an addition: the read-only directory fails on the
  first file, so it cannot tell staging from writing in place, as the first plant below shows.
* `TestWriteRenameFailureReports`, as the deviation above: the injected rename fails on `helpers_gen.go`; the error contains `replace
  helpers_gen.go`, the cause, `replaced: constants_gen.go, types_gen.go, agent_gen.go, client_gen.go`, and `left as they were: helpers_gen.go`; the
  four hold the new bytes and `helpers_gen.go` the old.
* `TestWriteCompareFailureReplacesNothing`: a directory named `helpers_gen.go`; the error names it, and the other four keep their bytes.

On `9989a5c` the package does not compile: `undefined: Write` (seven places) and `undefined: write`.

Change, as the design says: `Write` and `writeWith` (compare, stage, replace), `stage` (`CreateTemp` named `.<name>.*.tmp`, `Write`, `Sync`,
`Chmod(fileMode)`, `Close`, and removal on failure), `remove`, and `nameList`; `fileMode = 0o644`. A failure to remove a temporary file is joined to the
returned error rather than discarded (errcheck's `check-blank` rules out `_ =`). The `os.ReadFile` of the compare step carries `//nolint:gosec // G304:
dir is the generator's explicit output directory.`. `run` returns `output.Write(*outDir, files)`, and its doc comment says that `Write` stages every
changed file before it replaces any. The first `make pre-add-check` failed on revive's `confusing-naming` (`write` beside `Write`); the function is
`writeWith`, and the deviation entry says so.

Plants, one at a time in scratch copies of `cmd/generate` (`s4_plants.py`), against the package's tests:

```text
replace each file as it is staged: exit=1
    --- FAIL: TestWriteStagingFailureReplacesNothing/a_file_that_cannot_be_staged_after_others_were (0.07s)
    write_test.go:152: constants_gen.go holds "package acp\n\n// constants_gen.go v2\n", want "package acp\n\n// constants_gen.go v1\n"
mode 0o600: exit=1
    --- FAIL: TestWriteReplacesChangedFiles (0.13s)
    write_test.go:85: constants_gen.go has mode -rw-------, want 0644
no comparison: exit=1
    --- FAIL: TestWriteLeavesUnchangedFiles (0.12s)
    write_test.go:121: unchanged constants_gen.go was rewritten: modified 2026-10-10 22:15:31.075128622 -0500 CDT, want 2026-10-10 21:15:31 -0500 CDT
no cleanup after a failed rename: exit=1
    --- FAIL: TestWriteRenameFailureReports (0.09s)
    write_test.go:202: temporary file .helpers_gen.go.1510374310.tmp left in the output directory
no cleanup after a failed stage: exit=1
    --- FAIL: TestWriteStagingFailureReplacesNothing/a_file_that_cannot_be_staged_after_others_were (0.10s)
    write_test.go:153: temporary file .constants_gen.go.1874360174.tmp left in the output directory
```

The five were run again after the rename to `writeWith`; each exits 1. The `Sync` call has no test that can observe it; the code shows it.

In the tree: `stat` of the five generated files (modification time and mode) before and after `cd cmd/generate && env -u GOPATH -u GOMODCACHE go
run .` is identical, each `-rw-r--r--`; no `.tmp` file is left; `git diff --quiet d534f27 -- '*_gen.go'` exits 0, and `git status` shows no
generated file. `go test -cover`: `internal/output` 92.9%, `internal/emit` 84.0%, `internal/model` 93.9%. `make pre-add-check`: `go-precheck: 72
file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).` `make check-records` and `make markdownlint` exit 0.

Commit point: the agent stopped here. The owner commits step 4 on `main`.

### Step 5, type-check before writing, 2026-10-10

The owner committed step 4 as `66e907e`. Step 5 ran on `main` from that clean tree and added `cmd/generate/internal/output/check.go` and
`check_test.go`, changed `cmd/generate/main.go`, `main_test.go`, `internal/output/write.go` (the package comment), and this record. The agent did not
commit.

Tests first. In `internal/output`, over a synthetic generated file that refers to a hand-written `RequestError`:

* `TestCheckRefuses`: an empty directory (`does not type-check`, `types_gen.go:5:`, `undefined: RequestError`); a missing directory; a hand-written
  file that redeclares a generated type (`redeclared`); a `go.mod` at `go 1.21` with a hand-written `for range 3` (`requires go1.22`); a stray
  `package other` file (`output directory`, `other`); a file that imports `"C"` (`cgo file c.go`).
* `TestCheckSelectsLikeTheGoCommand`: the stub plus a `//go:build ignore` file, a `_test.go` file, `.x.tmp`, `.types_gen.go.1`, and an `_plan9.go`
  file, each with an undefined name; `Check` returns nil.
* `TestCheckReportsAtMostTen`: 12 undefined names; 10 reported, the first `types_gen.go:3:9: undefined: undefinedA`, ending `and 2 more`.
* `TestCheckAcceptsHandWrittenPackage`, added: the stub and the generated file check. It is the positive control of the cases above.

In `cmd/generate`: `TestRunRefusesOutputThatDoesNotTypeCheck` (the recased-union schema, now the constant `recasedUnion`, into a directory holding
only `keep.txt`); `TestRunGeneratesRecasedUnion` writes `handWritten` into its directory as `hand_written.go` first; `TestCheckAcceptsRepository`.

**Placement, 2026-10-10.** The plan lists `TestCheckAcceptsRepository` among the tests of `internal/output`. It is in `cmd/generate/main_test.go`,
because rendering the committed schema needs `loadSchema`, which merges the unstable schema and lives in package `main`; the test does what the plan
says, `emit.Render` of the committed schema checked against the repository root. The test of the empty directory in `internal/output` expects
`undefined: RequestError` of its synthetic file; `undefined: AgentSideConnection`, the plan's text, is what `TestRunRefusesOutputThatDoesNotTypeCheck`
expects of the real output.

On `66e907e` neither package's tests compile (`undefined: Check`, `undefined: output.Check`). In a scratch copy without `check_test.go` and
`TestCheckAcceptsRepository` (`s5_first.py`), the run-level test fails on the unchanged generator: `main_test.go:210: run error = <nil>, want one naming
undefined: AgentSideConnection`; `TestRunGeneratesRecasedUnion` passes.

Change, as the design says: `Check`, `handWritten`, `goVersion`, and `report` in `check.go`; `run` calls `output.Check` before `os.MkdirAll`, and its
doc comment says so. Two details the design did not spell out:

* The selection copies `build.Default` with `CgoEnabled` set, so a file that imports `"C"` is listed as a cgo file, and refused, whatever
  `CGO_ENABLED` says; with cgo off, `go/build` would leave it out silently.
* `conf.Check`'s returned error is discarded with `//nolint:errcheck`, because `Error` has already collected it with every other.

Plants, one at a time in scratch copies of `cmd/generate` (`s5_plants.py`; a temporary path is shortened to `<tmp>`):

```text
run without the check: exit=1
    --- FAIL: TestRunRefusesOutputThatDoesNotTypeCheck (0.06s)
    main_test.go:211: run error = <nil>, want one naming undefined: AgentSideConnection
UseAllFiles set: exit=1
    --- FAIL: TestCheckSelectsLikeTheGoCommand (0.00s)
    check_test.go:94: output directory: found packages acp (hand.go) and main (ignored.go) in <tmp>
GoVersion left empty: exit=1
    --- FAIL: TestCheckRefuses/a_feature_newer_than_the_go_line_of_go.mod (0.00s)
    check_test.go:70: Check accepted the output
no cap: exit=1
    --- FAIL: TestCheckReportsAtMostTen (0.00s)
    check_test.go:110: error reports 12 errors, want 10:
```

`p5_failmodes.py`, rerun: both scenarios exit 1 and rewrite nothing. The second, printed in full with the scratch copy's path shortened to `<copy>`:

```text
output that does not type-check: generator exit=1 generate: generated code does not type-check with <copy>:
    <copy>/agent_gen.go:22:16: undefined: toReqErrMissing
    ... eight more lines in agent_gen.go, lines 40 to 156 ...
    <copy>/agent_gen.go:174:16: undefined: toReqErrMissing
    and 30 more
    rewritten: []; untouched: ['constants_gen.go', 'types_gen.go', 'agent_gen.go', 'client_gen.go', 'helpers_gen.go']
```

The render failure gives the same result as in step 3.

Run time (`s5_time.py`: the binaries of `66e907e` and of this step, three runs each, on a scratch copy of the repository): before 1.55, 0.12, 0.13 s;
after 2.25, 0.60, 0.60 s. The first run of each fills the build cache; warm, the check adds about 0.48 s, inside the MADR's 0.5 to 0.9 s.

In the tree: `stat` of the five generated files is identical before and after regenerating; no `.tmp` file is left; `git diff --quiet d534f27 --
'*_gen.go'` exits 0. No `panic` call is left in a non-test file of `cmd/generate`. `go test -cover`: `internal/output` 92.3%, `internal/emit` 84.0%,
`internal/model` 93.9%. `make pre-add-check`: `go-precheck: 74 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).` `make
check-records` and `make markdownlint` exit 0.

Commit point: the agent stopped here. The owner commits step 5 on `main`.

### Step 6, docs, 2026-10-10

The owner committed step 5 as `fb82147`. Step 6 ran on `main` from that clean tree and changed `docs/architecture.md`, `RELEASING.md`, and this
record. The agent did not commit.

`docs/architecture.md`:

* the generator paragraph names `internal/output`, and a new paragraph states the order (render, type-check with the hand-written files the go command
  would build, at the language version of `go.mod`, then write), the refusal with the first 10 errors, that unchanged files are left alone, the
  temporary-file names `.<name>.*.tmp`, the sync, the mode `0644`, and the renames after all are staged;
* "Checks that guard this" names `TestCheckAcceptsRepository` and the tests of `internal/output`;
* the tree lists `internal/output/`, and `internal/emit/` as rendered in memory.

`RELEASING.md`: after the `make version` paragraph, a paragraph says the generator type-checks before it writes; that when a new schema renames or
removes a type the hand-written code uses, `make version` stops with the type checker's errors and changes no generated file; to change the
hand-written code first and rerun; and that files are written `0644` and an unchanged file is not rewritten. `README.md` names nothing that changed and
is unchanged.

The name check of phase 4 (`w10_names.py`) over both files, before and after the change. Before: 116 names, 6 failures, all in `RELEASING.md` lines this
step did not touch, names outside the tree the script has no kind for (`PATH`, `agentclientprotocol/agent-client-protocol`, `curl`, `git`, `gmake`,
`gofmt`). After: 122 names; the new ones are `TestCheckAcceptsRepository` (identifier), `go/types` (standard package), `internal/output` (path), and
three it cannot check, which were checked by hand against the source: `.<name>.*.tmp`, reported as a missing path, is the pattern
`"."+f.Name+".*.tmp"` of `stage` (`write.go:74`); `0644`, twice, is `fileMode = 0o644` (`write.go:19`); "the first 10 errors" is `maxReported = 10`
(`check.go:23`). No other failure is new.

No Go file changed, so `make pre-add-check` was not run. `make check-records` and `make markdownlint` exit 0.

Commit point: the agent stopped here. The owner commits step 6 on `main`.
