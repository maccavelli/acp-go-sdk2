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
* E12: `.golangci.yml` stops excluding `^example/`, and the 50 issues are fixed in the examples, none suppressed. Amended
  2026-10-16: but the four gosec G204 launches of a command the user names, each marked `//nolint:gosec` with its reason
  ("Deviation, 2026-10-16, during step 4"). A root test,
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

The owner committed step 1's record as `2906c21`.

### Step 2, E11, the records script, 2026-10-16

Decision 11, template first. Before any change, the workspace template's `templates/scripts/check_records.py` and
`references/gates.md` (the `workspace-scaffolding` skill, not under version control) were saved outside the tree; `cmp` of the
template's script against `scripts/check_records.py` exited 0, so the copy was byte-identical when the step began.

The template's script, changed:

* `check_structure` reports a PLAN without a MADR, a GATES without a PLAN, two MADRs on one number and a record outside its
  directory as errors, each setting exit 1; several PLANs and REPORTs on one number stay allowed.
* `check_names`, new: a Markdown file in a `docs/decisions` or `docs/reports` directory that `RECORD_RE` does not match, or any
  Markdown file named like a record in another case or width (`^\d+-(madr|plan|report|gates)-`, case-insensitive), is a "misnamed
  record".
* `check_links` checks a link's fragment, to another Markdown file or to the same one, against the target's anchors: its ATX headings
  as GitHub computes them (inline markup removed, lowercased, characters other than letters, digits, `_`, space and `-` removed,
  spaces to `-`, a repeated heading suffixed `-1`, `-2`; fenced code ignored) and its HTML `id` and `name` anchors.
* `next_number` returns one more than the highest number in use.
* The summary line is "N error(s)"; the docstring states every error.

Its `references/gates.md` gains a plant for each. Then `cp` to `scripts/check_records.py`: `cmp` exit 0.

Probes, a Python script outside the tree, each case in a fresh copy of `git archive HEAD`, the script under test copied in:

| Case | Planted | New script | Old script |
| :--- | :--- | :--- | :--- |
| baseline | nothing | exit 0, `--next` `0023` | the same |
| PLAN without a MADR | `docs/decisions/0023-PLAN-orphan.md` | exit 1, "number 0023 has a PLAN but no MADR" | exit 0, a warning |
| GATES without a PLAN | `docs/reports/0023-GATES-orphan.md` | exit 1, "has a GATES but no PLAN" | exit 0, a warning |
| a reused number | `docs/decisions/0022-MADR-dup.md` | exit 1, "is claimed by 2 MADRs" | exit 0, a warning |
| misplaced | a 0023 MADR and PLAN in `docs/reports` | exit 1, "record outside its directory" | exit 0, a warning |
| not a record | `docs/decisions/notes.md` | exit 1, "misnamed record" | exit 0, silent |
| misnamed | `docs/decisions/23-MADR-x.md`; `docs/decisions/0023-madr-x.md`; `docs/0023-madr-x.md` | exit 1, "misnamed record", each | exit 0, silent |
| missing anchor | a link to `0019-MADR-…#no-such-heading` in a record; `[x](#no-such-heading)` in `README.md` | exit 1, "broken anchor", each | exit 0, silent |
| a repeated heading | `## Same` twice, links to `#same` and `#same-1`; then to `#same-2` | exit 0; exit 1 | exit 0; exit 0 |
| a heading with markup | ``## The `Agent` *type*``, a link to `#the-agent-type` | exit 0 | exit 0 |
| missing file | `[x](missing.md)` in `README.md` | exit 1, "broken relative link" | the same |
| a gap | the 0016 report deleted | `--next` `0023` | `--next` `0016` |

The old script is the planted break for each new check: structural errors back to warnings, the fragment dropped, `--next` back to the
lowest gap. It missed 12 of the 16 cases; the new one matched all 16.

On the tree: `make check-records` exit 0 (it runs `python3`, the host's 3.9, which the script supports), `--next` `0023`. Gates,
through the Python runner: `make check-records` and `make markdownlint` exit 0; the identifier scan finds nothing. No Go file changed
in this step.

The owner committed step 2's record as `ca166fb`. The workspace template's two files changed in that step are not in any repository.

### Step 3, tooling: E13, E15, the pins of E16, E17, 2026-10-16

What changed, on `ca166fb`:

* E13: `make release` ends "Schema $(VERSION) is ready. Review the changes and commit; the module's tag follows RELEASING.md, not
  the schema version.", where it said "then tag v$(VERSION)".
* E15: `scripts/go-precheck.sh` records why govulncheck was skipped (`GO_PRECHECK_SKIP_VULN=1`, or "offline" when the offline
  heuristic matched for either module), and the success line then reads "clean (gofmt, golangci-lint, go vet, go test); govulncheck
  skipped (<reason>)."; it names govulncheck among the clean checks only when it ran. The heuristic itself is unchanged.
* E16, the pins: `.github/workflows/ci.yml` runs `golang.org/x/vuln/cmd/govulncheck@v1.9.0` in both modules, which the precheck and
  the `Makefile` read from it. govulncheck v1.9.0 on the tree: "No vulnerabilities found", exit 0, in both modules.
* E17: `mise.lock` removed. No file outside the records names mise.

E15, checked by running the precheck on one file in throwaway git repositories made from scratch copies, reading its last line:

| Case | Last line |
| :--- | :--- |
| the new script, `GO_PRECHECK_SKIP_VULN=1` | "go-precheck: 1 file(s) clean (gofmt, golangci-lint, go vet, go test); govulncheck skipped (GO_PRECHECK_SKIP_VULN=1)." |
| the new script, `GOPROXY=off` and an empty module cache | "… go test); govulncheck skipped (offline).", after "could not reach the vulnerability database; skipped." for both modules |
| the script at `ca166fb`, `GO_PRECHECK_SKIP_VULN=1` (the planted break) | "go-precheck: 1 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)." |

On the tree, where it ran, the line is "go-precheck: 116 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)."

Gates, through the Python runner, on the tree: regenerating changes no generated file; `make pre-add-check` exit 0; no new `.go`
file; `GOEXPERIMENT=nojsonv2 go test ./...` exit 0; `GOOS=windows go vet` exit 0 in both modules; `make check-records` and `make
markdownlint` exit 0; the identifier scan finds nothing; Windows `go test -count=1 -shuffle=on ./...` exit 0 at the root and in
`cmd/generate`; actionlint v1.7.12 on the workflow exit 0. The API listing: 2429 lines at `v1.15.0` and in the tree; 0 removed, 0
added.

The owner committed step 3's record as `394572f`.

### Deviation, 2026-10-16, during step 4

Found: four of the 50 example lint issues are gosec G204, "Subprocess launched with a potential tainted input or cmd arguments" (or
"with variable"), at `example/agent/main.go:436` (the client command the agent's arguments name), `example/client/main.go:188` (the
agent command the client's arguments name), `example/client/main.go:206` (the default `go run <agent dir>`) and
`example/gemini/main.go:218` (the `-gemini` binary). Launching the command the user names is what those programs do, and
`TestExamplePair` runs the client with the agent's binary as its argument; no change short of removing the feature clears them.
Decision (owner, 2026-10-16): each of the four is marked `//nolint:gosec // G204: <why>`, as `cmd/generate` already marks its own
launches, with `nolintlint` requiring each to name its linter and still be needed; the other 46 are fixed, none suppressed. "Design",
E12, is amended to say so. No decision or fact of the MADR changes.

### Step 4, examples: E5, E6, E7, E12, E14, 2026-10-16

What changed, on `394572f`:

* E5: `example/claude-code` runs `npx -y @agentclientprotocol/claude-agent-acp@latest`; `example/gemini` passes `--acp`.
  `Example_gemini` keeps its flag until step 5, which rewrites that file.
* E6: the example agent's `Prompt` runs under the SDK's context; its cancel map, the code that cancelled the previous turn, and the
  context it made are gone, and `Cancel` does nothing, its doc saying why. `simulateTurn` returns the turn's stop reason: `cancelled`
  when the permission request is answered cancelled, where the turn ended `end_turn`.
* E7: `example/client` and `example/gemini` no longer advertise `terminal`; the five terminal methods of all three clients answer
  `acp.NewMethodNotFound` with their method. Each client keeps the working directory of the session it opens, and `WriteTextFile`
  refuses with `acp.NewInvalidParams` a path whose `filepath.Rel` from it is not `filepath.IsLocal`, then writes through an `os.Root`
  of it, so a symbolic link cannot lead out either.
* E12: `.golangci.yml` no longer excludes `^example/`. The 50 issues: 46 fixed (errors checked, the child stopped by closing its
  input and waiting, `errors.AsType` for `*acp.RequestError`, `strconv.Atoi` for the option number, `0o750` and `0o600`, a line
  window without an integer conversion, `new(x)` for `acp.Ptr(x)`, a package doc each, one receiver renamed, the agent's cancel
  check without the `nilerr` pattern); the four G204 launches marked, as the deviation above records.
* E12: `TestExamplePair` (`example_pair_test.go`, new) builds `./example/agent` and `./example/client` and runs the client with the
  agent's binary: input `1` gives "stop reason end_turn", the end of input "stop reason cancelled"; the client exits 0 each time.
  `TestWriteTextFileStaysInCwd` and `TestTerminalMethodsNotFound` in each of the three clients (`example/*/main_test.go`, new).
* E14: `docs/architecture.md` states the response classification as `connection.go:1113` has it; the README's examples list says
  what the client starts, that it prints the stop reason (which it now does, and exits 1 on a failed call), the Claude Code adapter
  and `gemini --acp`, and that the example clients advertise no terminal and write only inside the working directory.

Planted breaks, each in a fresh scratch copy (11 of 11 seen to fail):

| Break | What failed |
| :--- | :--- |
| a cancelled permission ends the turn `end_turn` | `TestExamplePair/input_ended`: "the client's output lacks \"stop reason cancelled\"" |
| the client prints no stop reason | `TestExamplePair`, both cases |
| a write outside the working directory not refused, in each client | `TestWriteTextFileStaysInCwd`: both outside paths not answered invalid params (the `os.Root` still refused them) |
| writes not through an `os.Root`, in each client | `TestWriteTextFileStaysInCwd`: "a write through a link out of the working directory succeeded" |
| `CreateTerminal` answering success, in each client | `TestTerminalMethodsNotFound`: "CreateTerminal: <nil>, want method not found" |

On a host that cannot make a symbolic link, the test logs that and checks the rest; macOS made one. Whether the Windows host did is
not established: its run reports the packages `ok`, not the log line.

Found, not changed: the example clients print a tool call update's status with `%v` of a pointer (`updated: 0x369dbacca890`), as they
did before this step. It is outside E5 to E17 and is left for the owner.

Gates, through the Python runner, on the tree: regenerating changes no generated file; `make pre-add-check` exit 0, which now lints
the examples for the host and for Windows; the four new test files clean through `scripts/go-precheck.sh`; `GOEXPERIMENT=nojsonv2 go
test ./...` exit 0; `GOOS=windows go vet` exit 0 in both modules; `make check-records` and `make markdownlint` exit 0; the identifier
scan finds nothing; Windows `go test -count=1 -shuffle=on ./...` exit 0 at the root (the example packages and `TestExamplePair`
among them) and in `cmd/generate`. The API listing: 2429 lines at `v1.15.0` and in the tree; 0 removed, 0 added.

The owner committed step 4's record as `fdbdff6`.

### Step 5, E8, the `Example` functions, 2026-10-16

What changed, on `fdbdff6`: the three files are `package acp_test`, importing the package as a user does, and none calls
`testAbsPath` or drops an error.

* `example_agent_test.go`: `demoAgent`, an `Agent` that streams a message, starts a tool call, asks permission and completes or fails
  the call by the answer, with the session's working directory in its paths and every method it does not offer answering
  method-not-found; `Example_agent` serves it and drives it with `demoClient` over two `io.Pipe`s, prints the stop reason, closes the
  client's connection and waits for the agent's to end. Its doc shows the stdio form a real agent uses.
* `example_client_test.go`: `demoClient`, a `Client` that prints the turn's text and tool calls (a status by value, not the
  pointer), allows each permission with the first option, prints nothing from that request (it runs on its own goroutine, so its
  output could interleave), and answers the file-system, terminal and elicitation requests method-not-found, advertising none of
  them; `Example_client` runs it against `demoAgent` and also prints the protocol version.
* `example_gemini_test.go`: `Example_gemini` starts `gemini --acp` (E5) with `exec.CommandContext`, checks every error, uses
  `demoClient`, and closes gemini's input and waits for it when done. It needs the binary, so it has no `// Output:`.

Each runnable example ends in `// Output:`: the streamed text, the tool call started and completed, and `stop reason: end_turn`.
`go test -list Example .` lists `Example_agent` and `Example_client`; `go test -count=1 -run Example -v .` passes both; `go test
-race -count=50 -run '^Example_(agent|client)$' .` passes.

Planted breaks, each in a fresh scratch copy (3 of 3 seen to fail):

| Break | What failed |
| :--- | :--- |
| a line of the agent's streamed text changed | `Example_agent` and `Example_client`, their output |
| the client rejects the change (the last option) | `Example_agent` and `Example_client`: the tool call is reported failed |
| the client prints the status pointer | the build: `go vet`'s printf check refuses `%s` for `*ToolCallStatus`, before the examples run |

Gates, through the Python runner, on the tree: regenerating changes no generated file; `make pre-add-check` exit 0;
`GOEXPERIMENT=nojsonv2 go test ./...` exit 0; `GOOS=windows go vet` exit 0 in both modules; `make check-records` and `make
markdownlint` exit 0; the identifier scan finds nothing; Windows `go test -count=1 -shuffle=on ./...` exit 0 at the root, where the
two examples run, and in `cmd/generate`. The API listing: 2429 lines at `v1.15.0` and in the tree; 0 removed, 0 added.
