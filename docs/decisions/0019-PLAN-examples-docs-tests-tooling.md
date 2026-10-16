---
status: in-progress
date: 2026-10-16
associated-madr: "0019-MADR-remediate-debugging-pass-findings.md"
---

# Implement phase 4 of 0019: examples, documentation, tests and tooling

Associated MADR: [0019-MADR-remediate-debugging-pass-findings.md](0019-MADR-remediate-debugging-pass-findings.md)

## Goal

Implement phase 4 of the MADR's outcome, as "Amended while planning phase 4, 2026-10-16" sets it: E5 to E15, the pins of E16, E17,
and I12 over every exported identifier, with E11 through the workspace template (decision 11). Each change has a test, an engine case
or a recorded check, each seen to fail on a planted break first. No exported API changes, and no behaviour of the SDK changes; the
committed generated files change in comments only. Phase 4 rides the next release (decision 12), a patch, `v1.15.1`, which the owner
confirms at the tag.

Phases 1 to 3 are [0019-PLAN-behaviour-fixes.md](0019-PLAN-behaviour-fixes.md),
[0019-PLAN-api-decisions.md](0019-PLAN-api-decisions.md) and [0019-PLAN-generator-hardening.md](0019-PLAN-generator-hardening.md),
complete and released as `v1.14.0` and `v1.15.0`.

## Standards this plan follows

| Id | Source | Text | Bears on |
| :--- | :--- | :--- | :--- |
| Q1 | `go.dev/doc/comment`, "Declarations" | "Every exported (capitalized) name should have a doc comment." | I12 |
| Q2 | `AGENTS.md`, "Records" | "Never reuse a number, never renumber an existing record, never leave a gap deliberately." `make check-records` "validates naming, pairing, placement, and every relative link". | E11 |
| Q3 | `npm view @zed-industries/claude-code-acp deprecated`, 2026-10-16 | "This package has been renamed to @agentclientprotocol/claude-agent-acp. Please migrate to continue receiving updates." | E5 |
| Q4 | `@google/gemini-cli` 0.64.0, its bundled option table | `experimental-acp`: "Starts the agent in ACP mode (deprecated, use --acp instead)"; `acp`: "Starts the agent in ACP mode". | E5 |
| Q5 | the MADR, decision 11 | "fix the workspace template's `check_records.py` first, then copy it, so the copy stays byte-identical to its template" | E11 |

## Facts this plan rests on

Each finding was re-checked on `0acc8e1`, read-only, in scratch copies of the tree; the tree was not written. Step 1 re-asserts each
code fact, snippet at line, before any code changes.

| Finding | The code as it is | Re-check on `0acc8e1` |
| :--- | :--- | :--- |
| E5 | `example/claude-code/main.go:203` runs `npx -y @zed-industries/claude-code-acp@latest`; `example/gemini/main.go:204` and `example_gemini_test.go:73` pass `--experimental-acp`. | Q3 and Q4. `@agentclientprotocol/claude-agent-acp` is at 0.91.0, its binary `claude-agent-acp`. The gemini CLI was not run. |
| E6 | `example/agent/main.go:193` takes `Prompt(_ context.Context, …)` and makes its own context at `:211`; its own cancel map at `:22`, `:183-191`, `:202-210`; a cancelled permission returns nil at `:359-361`, so the turn ends `end_turn` (`:226`). | Read. |
| E7 | `example/client/main.go:226` and `example/gemini/main.go:245` advertise `Terminal: true`, with stubs (`client` `:142-161`, `:176-179`; `gemini` `:157-176`, `:191-194`; `claude-code` carries the stubs, `:157-176`, `:191-194`, without advertising). `WriteTextFile` writes any absolute path (`client` `:103-116`, `claude-code` `:119-132`, `gemini` `:119-132`). | Read. |
| E8 | `example_agent_test.go`, `example_client_test.go`, `example_gemini_test.go`: `package acp`, no `// Output:` (`go test -list Example .` lists none), errors dropped, `testAbsPath` (`paths_test.go:24`) called, `Example_client` kills its child without waiting, `Example_gemini` never stops its child. | Read; the listing. |
| E9 | Root coverage 88.8%. `(*AgentSideConnection).NotifyExtension` 0% (`extensions.go:91`); `SendRequestNoResult` 59.1%, nothing after a successful write run (`connection.go:1885-1896`); `waitNotificationsUpTo` 55.9% (`:1795-1807`, `:1790`); `handleCancelRequest` 58.8% (`:1436-1455`); `handleInbound`'s nil handler and marshal failure (`:1471-1477`, `:1501-1502`); the id-canonicalisation fallbacks (`:1177-1179`, `:1295`, `:1406-1408`, `:1446-1448`); `answerUnlessCancelled` (`:251-252`, `:262-263`); `NewAuthRequired`, `Ptr`, `WithStartRawOutput` 0%; `(*RequestError).Error` and `Unwrap` nil receivers. | Coverage profile. An id such as `1e5000` passes `validID` (`connection.go:1057`, first byte only), fails canonicalisation, and is keyed by its raw bytes, logged: handled, not a defect. |
| E10 | Fixed sleeps before an assertion: `acp_test.go:766`, `:1066`, and `connection_cancel_test.go:688` (new since the pass). No test uses `testing/synctest`. `TestConformanceConstructors` skips 4 of 67 subtests every run (`conformance_test.go:1126`): a nested required union the zero-argument constructor leaves empty. | `-v` run: `SessionConfigOption_Select`, `ElicitationPropertySchema_Array`, `CreateElicitationRequest_Url`, `CreateElicitationRequest_Form` skip. |
| E11 | `scripts/check_records.py` is byte-identical to the template (`cmp` exit 0). `next_number` returns the lowest gap (`:77-82`); `check_structure` only warns (`:85-109`); `find_records` skips a name `RECORD_RE` (`:30`) does not match (`:69-74`); `check_links` drops the fragment (`:143`); only a broken link exits 1 (`:182-184`). | Probes: a PLAN without a MADR, a misplaced record, a reused number exit 0 with a warning; `23-MADR-x.md`, `0023-madr-x.md` and a missing anchor exit 0 silently; with a gap made, `--next` gives the gap. The tree has no gap (0001 to 0022), no non-record file in the record directories, and 11 anchor links, all resolving. |
| E12 | `.golangci.yml` excludes `^example/` for linters and formatters; CI (`ci.yml:40`) and `make test` (`Makefile:37`) only build the examples. | The repo's configuration without the exclusion: 50 issues over `./example/...`, the same for Windows (errcheck 17, gosec 13, errorlint 9, revive 5, gocritic 3, govet 2, nilerr 1). |
| E13 | `Makefile:177`: "then tag v$(VERSION)", `VERSION` the schema version (`schema/version` is `1.25.0`). | Read; `RELEASING.md:51-52`. |
| E14 | `docs/architecture.md:266-267`: "an object with `result` or `error` is a response"; the code also requires no `method` and takes an id matching a pending call (`connection.go:1113`, `isPending` `:1289`). `README.md:35`: the client "connects to a running agent"; it starts `go run ../agent` from its source file's directory (`example/client/main.go:189-207`) and ignores `StopReason` (`:264`). | Read. |
| E15 | `scripts/go-precheck.sh:210-211` (offline heuristic) and `:219-220` (`GO_PRECHECK_SKIP_VULN=1`) skip without failing; `:229-231` always names govulncheck in the success line. | Read. |
| E16 | `ci.yml:58-59` pins govulncheck v1.8.0; the precheck reads that pin (`go-precheck.sh:200`). | `go list -m -versions golang.org/x/vuln` ends at v1.9.0. |
| E17 | `mise.lock` is tracked; there is no `mise.toml`. It pins golangci-lint 2.12.2 (`:142-143`), Go 1.26.3 (`:74-75`), actionlint 1.7.12. | CI, `Makefile:51`, `go-precheck.sh:91` and `RELEASING.md:12` pin golangci-lint v2.14.0; `go.mod` says 1.27.2. |
| I12 | 654 exported identifiers in the root package have no doc comment: `Agent`, `Client`, `RequestIdNull`; 147 `Validate`, 316 `UnmarshalJSON`, 71 `MarshalJSON`; the 40 typed outbound methods; 76 enum constants in 19 blocks; `(*RequestError).Error`. The emitter has the method docs (`model/methods.go:209-219`) and does not attach them to the outbound methods (`emit/dispatch.go:658-739`); `addValidate` (`emit/types.go:960`) and the codecs (`:361`, `:457`, `:1149`, `:1153`) attach none; `model.EnumValue` (`model/model.go:232-235`) has no `Doc`, so a value's schema description is dropped; `sdkInterfaces` gives `Agent` and `Client` no `Doc` (`model/protocol.go:174`, `:177`); the null variant is built without one (`model/fields.go:589-590`). | A `go/parser` count over the package. No lint gate sees it: the generated header suppresses revive's `exported` rule. |

## Scope

In scope: the examples under `example/` and the three `Example` files; root tests; the generator (`cmd/generate/internal/model`,
`internal/emit`) for doc comments only, and the regenerated files; `scripts/check_records.py` and its template; `scripts/go-precheck.sh`;
`Makefile`; `.golangci.yml`; `.github/workflows/ci.yml`; `mise.lock`; `README.md`, `docs/architecture.md`; the release notes for
`v1.15.1`; these records.

Outside the repository, by decision 11: the workspace template's `templates/scripts/check_records.py` and its `references/gates.md`, in
the `workspace-scaffolding` skill. That directory is not under version control; step 2 saves both files before changing them.

Out of scope: the modernizing linters and lint over generated code, which move to phase 5 with the fixes for what they flag (the
MADR's amendment of 2026-10-16); `go fix -diff`, phase 5; rewording the 369 type docs that are schema descriptions not starting with
the type's name; confining the example clients' reads (E7 names writes); `AGENTS.md`'s "same slug" sentence, which numbers 0011 and
0019 already depart from with several PLANs each, as the MADR and PLAN skill allows; other repositories' copies of the template.

## Design

### E11. The records script, template first (decision 11)

The template's script changes, then is copied byte for byte into `scripts/check_records.py`:

* Every structural finding is an error that sets exit 1: a PLAN without a MADR, a GATES without a PLAN, two MADRs with one number, a
  record outside its directory. Several PLANs and REPORTs for one number stay allowed.
* A Markdown file in a `docs/decisions` or `docs/reports` directory whose name `RECORD_RE` does not match, and any Markdown file whose
  name looks like a record in another letter case or width (`^\d+-(madr|plan|report|gates)-`, case-insensitive), is an error.
* A link with a fragment to a Markdown file, or to the same file, must name a heading's anchor, computed as GitHub computes it
  (lowercase, punctuation but `-` removed, spaces to `-`, a repeated heading suffixed `-1`, `-2`; headings in fenced code ignored).
* `--next` returns one more than the highest number in use, so a gap stays a gap (Q2).

The template's `references/gates.md` gains a plant for each. Probes in scratch copies of the repository, before and after.

### Tooling: E13, E15, the pins of E16, E17

* E13: `make release` ends "Schema $(VERSION) is ready. Review the changes and commit; the module's tag follows RELEASING.md, not the
  schema version."
* E15: the success line names govulncheck only when it ran; when skipped it reads "govulncheck skipped (<reason>)", the reason being
  `GO_PRECHECK_SKIP_VULN=1` or "offline". The offline heuristic is unchanged.
* E16, the pins: `ci.yml` pins `golang.org/x/vuln/cmd/govulncheck@v1.9.0`, which the precheck reads. Its run on both modules must be
  clean; a finding it reports is a deviation.
* E17: `mise.lock` is removed. Tool versions stay in `ci.yml`, the `Makefile`, the precheck and `RELEASING.md`.

### Examples: E5, E6, E7, E12, E14

* E5: `example/claude-code` runs `npx -y @agentclientprotocol/claude-agent-acp@latest`; `example/gemini` and `Example_gemini` pass
  `--acp`; the README's lines follow.
* E6: the example agent's `Prompt` runs under the context the SDK gives it, and its own cancel map, the cancel-the-previous-turn code
  and the context it makes go; a cancelled permission ends the turn `cancelled`. The turn then stops on `session/cancel` and on the
  connection's end, as the README says the SDK provides.
* E7: `example/client` and `example/gemini` stop advertising `terminal`; the five terminal methods of all three clients answer
  `acp.NewMethodNotFound` with their method's name. `WriteTextFile` records each session's working directory when the session is made
  and refuses, with `acp.NewInvalidParams`, a path whose `filepath.Rel` from it is absolute or starts with `..`.
* E12: `.golangci.yml` stops excluding `^example/`, and the 50 issues are fixed in the examples, none suppressed. A root test,
  `TestExamplePair`, builds `./example/agent` and `./example/client` into a temporary directory, runs the client with the agent's
  binary as its argument and scripted input, and checks that it exits 0 and prints the turn's stop reason. `go test ./...` already runs
  on all three CI runners, so the pair runs on Windows too (E4's regression).
* E14: `docs/architecture.md` states the classification as the code has it; the README says the client starts the agent from a source
  checkout, or runs the command given as its arguments, and prints the stop reason, which the client then does.

### E8. The `Example` functions

The three files become `package acp_test`, with every error checked. `Example_agent` and `Example_client` each run a client and an
agent in process over two `io.Pipe`s and end in a `// Output:` block: the streamed text and the stop reason, deterministic. Paths come
from `os.TempDir` through `filepath.Abs`, not `testAbsPath`. `Example_gemini` needs the `gemini` binary, so it keeps no `// Output:`;
it passes `--acp`, checks its errors, and stops and waits for its child.

### E9 and E10. Tests

E9, one test per documented behaviour the profile shows unrun: `SendRequestNoResult` after a successful write (a result, an error
response, the connection's end); `(*AgentSideConnection).NotifyExtension` sent and a bad name refused, and the client's bad name;
`answerUnlessCancelled` when the context ends for another cause, and when the handler returns first under `ErrPromptCancelled`;
`waitNotificationsUpTo` when the context ends and when the connection ends; `$/cancel_request` with bad params, an empty
`requestId`, an id that does not canonicalise (`1e5000`, cancelled by its raw bytes) and an unknown id; `handleInbound` with a nil
handler, for a request and a notification, and with a result that does not marshal; a response and a request whose id does not
canonicalise; `NewAuthRequired`, `Ptr`, `WithStartRawOutput`; `(*RequestError).Error` and `Unwrap` on a nil receiver and `Error`'s
fallback. The invariant panics (`connection.go:1390`, `:1395`, `:1762`, `:1785`, `:1364`, `:1425`) are named, not run.

E10: the three fixed sleeps go. Each test runs in a `testing/synctest` bubble, with its pipes and goroutines made inside it, or, where
a goroutine outside the bubble is involved, waits on the event it asserts. `TestConformanceConstructors` fills each nested required
union the constructor leaves empty with that union's first instance from the instance builder, so all 67 subtests encode, decode and
compare, and none skips. Neither design is prototyped yet; a step that finds either does not hold stops as a deviation.

### I12. A doc comment on every exported identifier (Q1)

The generator attaches:

* to a typed outbound method: "`<Name>` sends the `<method>` request to the `<agent|client>` and returns its response." or "… sends
  the `<method>` notification to the …", then a blank comment line and the method's doc as the interface method has it;
* to `Validate`: "Validate returns an error if v does not satisfy the `<Definition>` schema definition.";
* to `MarshalJSON` and `UnmarshalJSON`: "MarshalJSON encodes v as the `<Definition>` schema definition's JSON." and "UnmarshalJSON
  decodes the `<Definition>` schema definition's JSON into v.";
* to an enum constant: "`<Const>` is the `"<value>"` value.", then the value's schema description when it has one (`EnumValue` gains
  `Doc`, from the variant's `description`);
* to `Agent` and `Client`: a doc in `sdkInterfaces` naming the side, the methods it implements and the side connection that serves it;
* to `RequestIdNull`: the null variant's schema description.

`(*RequestError).Error` gets a hand-written doc. A root test, `TestExportedDeclarationsDocumented`, parses the package's non-test
files, generated ones included, and fails naming each exported type, function, method, constant or variable without a doc comment (a
constant or variable is documented by its own doc or its group's). A model test pins `EnumValue.Doc`. The regenerated files differ
from the committed ones in comments only, which a scratch script checks by comparing the files' syntax trees with comments removed;
the API listing against `v1.15.0` is unchanged.

## Implementation Steps

Each step ends with its gates and an execution record; the owner commits it. Agents do not commit on `main`. Every new test is seen
to fail on a planted break in a scratch copy, never in the tree, before it is relied on.

1. **Records.** This PLAN and its `docs/README.md` row; the MADR's amendment of 2026-10-16 and the notes on outcome items 4 and 5.
   When the owner approves this PLAN, it becomes `in-progress`, in the same records commit. Before step 2, a script asserts every
   code fact above, snippet at line, on the commit step 2 starts from. Gates: `make check-records`, `make markdownlint`, the
   identifier scan.
2. **E11.** Save the template's script and `references/gates.md` outside the tree; change the template; probe it in scratch copies
   of the repository (each new error exits 1 naming its file; the tree as committed exits 0; `--next` after a planted gap gives the
   number after the highest); then `cp` it to `scripts/check_records.py`, `cmp` exit 0.
3. **Tooling: E13, E15, the pins of E16, E17.** `Makefile`, `scripts/go-precheck.sh`, `ci.yml`, `mise.lock` removed. govulncheck
   v1.9.0 run on both modules. E15 checked by running the precheck with `GO_PRECHECK_SKIP_VULN=1` in a scratch copy and reading its
   last line.
4. **Examples: E5, E6, E7, E12, E14.** The four example programs, `.golangci.yml`, `TestExamplePair`, the README and
   `docs/architecture.md` passages. Lint over `./example/...` for the host and Windows: 0 issues.
5. **E8.** The three `Example` files; `go test -run Example -v .` lists and passes `Example_agent` and `Example_client`.
6. **E9 and E10.** The tests; the coverage profile before and after; the E10 tests run `-count=50 -race`.
7. **I12.** Model, emitter, regeneration, `(*RequestError).Error`'s doc, `TestExportedDeclarationsDocumented`, the model test;
   `docs/architecture.md` on generated docs.
8. **Release notes.** `docs/reports/0019-REPORT-release-notes-v1-15-1.md` and its `docs/README.md` row: the examples, the docs, the
   tooling, and that no exported API or SDK behaviour changes.
9. **Verification and close.** The gates over the tree; the re-check of each finding re-run (each fixed); the API listing against
   `v1.15.0`, unchanged; Windows. After CI passes on the pushed commits, this PLAN and its row become `complete`, and the owner tags
   `v1.15.1` and publishes its release from the report.

## Verification

Per code step, through the Python runner outside the tree: regenerate, and the diff of the generated files is exactly the step's
(none before step 7; comments only in step 7); `make pre-add-check`; `scripts/go-precheck.sh` on each new `.go` file;
`GOEXPERIMENT=nojsonv2 go test ./...`; `GOOS=windows go vet ./...` in both modules; each new test seen to fail on its planted break in
a scratch copy; `make check-records`, `make markdownlint`, the identifier scan; Windows `go test -count=1 -shuffle=on ./...` in both
modules; the API listing against `v1.15.0`.

Planted breaks, at least: E11, each structural error back to a warning, the fragment dropped, `--next` back to the lowest gap; E7, a
write outside the working directory allowed; E6, a cancelled permission ending `end_turn`; E12, the client ignoring the stop reason;
E8, a changed line of streamed text; E9, each branch's handling changed so its test's assertion fails; E10, the conformance fill
removed (a subtest skips, which the test then refuses); I12, one generated doc dropped, and an enum's description dropped.

Expected API listing difference against `v1.15.0`: none.

Acceptance: every step's checks pass and its record is written; each finding's re-check shows it fixed; root coverage rises, each
branch E9 lists is run by the profile; `TestExportedDeclarationsDocumented` passes with 0 undocumented; `go test -list Example .`
lists `Example_agent` and `Example_client`; lint over the examples is clean; CI passes on the pushed commits.

## Rollout and Rollback

Each step is one commit, reverted by reverting it. No exported API or SDK behaviour changes, so `v1.15.1` is a patch; a consumer sees
better documentation on pkg.go.dev and nothing else. The example programs change behaviour (E5 to E7), which the release notes name.
The template change of step 2 is reverted by restoring the files saved before it; a copy of the template in another repository is
untouched until that repository refreshes it.

## Execution record

### Step 1, records, 2026-10-16

The owner approved this PLAN on 2026-10-16 ("proceed"), and it is `in-progress`. Written: this PLAN and its `docs/README.md` row;
the MADR's "Amended while planning phase 4, 2026-10-16", which records the owner's four decisions of that day (E16's linters to phase
5, I12 over every exported identifier, E17 and the removal of `mise.lock`, E7's example clients) and E5's answer on gemini's flag, and
the notes on outcome items 4 and 5.

Before this PLAN was written, each finding was re-checked on `0acc8e1` in scratch copies of the tree, read-only: E5 to E17 and I12
each still hold, with the facts above. Writing the facts table, two references were off and corrected before this record: the
response classification is at `connection.go:1113` and `isPending` at `:1289`.

The code facts, asserted by a Python script outside the tree, snippet at line, on `0acc8e1` (this step changes no code, so step 2
starts from the same code): 75 of 75 hold. The check was seen to fail on a planted break in a scratch copy: with `Makefile:177` and
`example/agent/main.go:193` changed, it named both and exited 1.

Gates, through the Python runner: `make check-records` and `make markdownlint` exit 0; the identifier scan finds nothing. No Go file
changed in this step.
