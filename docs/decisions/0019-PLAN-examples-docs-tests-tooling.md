---
status: in-progress
date: 2026-10-18
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
`v1.15.1`; these records. Amended 2026-10-18: and `scripts/push_guard.py`, its `make push-guard` target, and `AGENTS.md`'s paragraph
on the disclosure guard ("Deviation, 2026-10-18, after step 9").

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
  decodes the `<Definition>` schema definition's JSON into v."; amended 2026-10-17: `<Definition>` is the definition's own key, and a
  type that is not itself a definition names where it sits, as "the `<value>` variant of the `<Definition>` schema definition" or
  "the `<key>` property of the `<Definition>` schema definition" ("Deviation, 2026-10-17, during step 7");
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

The owner committed step 5's record as `4da5878`.

### Step 6, E9 and E10, the tests, 2026-10-17

What changed, on `4da5878`: a new `branches_test.go`, and edits to `acp_test.go`, `connection_cancel_test.go` and
`conformance_test.go`. No SDK source changes.

E9, `branches_test.go`, one test per documented branch the profile showed unrun:

* `TestSendRequestNoResultAnswers`: after the write, a result returns nil, an error response returns the peer's error, and the
  peer's end returns -32603 wrapping `ErrPeerDisconnected`.
* `TestSendRequestNoResultHeldBehindNotification`, in a `synctest` bubble: an answer that arrives while an earlier notification's
  handler still runs is held. When the caller's context ends first, the call returns -32800 wrapping `context.Canceled`.
* `TestNotifyExtensionNames`: both sides refuse `""` and `x/ping` and write nothing; the agent's `_x/ping` is sent as a notification.
* `TestAnswerUnlessCancelledHandlerFirst`: the handler returns first under a context whose cause is already `ErrPromptCancelled`; the
  answer is cancelled. The context withholds its `Done` channel to stage that race, the race of a `session/cancel` with a handler's
  return.
* `TestPermissionCancelRequestWaitsForHandler`, in a `synctest` bubble: a permission request cancelled by `$/cancel_request` is
  answered with what its handler returns.
* `TestWaitNotificationsEnds`: the wait for earlier notifications returns the caller's cause when its context ends, and the
  connection's end when the connection ends.
* `TestCancelRequestFaults`: params that do not decode (`[1]`), no `requestId`, and two unknown ids (`1e4999`, `2`) cancel nothing,
  the first two logged. A request with id `1e5000`, which does not canonicalise, is keyed by its bytes, and `$/cancel_request` naming
  `1e5000` cancels it (-32800, id `1e5000`).
* `TestNilHandlerAnswers`: a connection without a handler answers a request -32601 and leaves a notification unanswered.
* `TestUnmarshalableResultAnswered`: a result that does not encode is answered -32603, naming the encoding error.
* `TestUncanonicalIDs`: a response whose id does not canonicalise is logged and dropped. An object with such an id and no method,
  result or error is answered -32600 with that id.
* `TestSmallConstructors` (`NewAuthRequired`, `Ptr`, `WithStartRawOutput`) and `TestRequestErrorNilAndFallback` (nil receivers,
  and `Error`'s plain-text fallback when the data does not encode).

`Ptr` carries `//go:fix inline`, so govet's `inline` analyzer reports any call to it, with or without a type argument. Its own test
has to call it, so that line carries `//nolint:govet // inline: …`, the repository's justified-nolint form.

E10, as designed; both designs held on their first prototype in a scratch copy:

* The three fixed sleeps are gone. `TestConnectionHandlesNotifications`, `TestPromptCancellationSendsCancelAndAllowsNewSession` and
  `TestRequestWithEndedContextWritesNothing` each run their body in `synctest.Test`. The body makes its pipes and goroutines and closes
  the pipes in `t.Cleanup`, and `synctest.Wait()` replaces the sleep. No goroutine outside a bubble is involved, so no event wait was
  needed.
* `TestConformanceConstructors` calls `fillEmptyUnions` before encoding and fails, rather than skips, a value that still does not
  encode.
  * Two of the four were a union field left empty (`SessionConfigOption_Select`'s `SessionConfigSelectOptions`,
    `ElicitationPropertySchema_Array`'s `MultiSelectItems`). Each is filled with its type's first instance from the instance builder.
  * In the other two (`CreateElicitationRequest_Form`, `CreateElicitationRequest_Url`), the scope union is folded into the variant's
    own struct, as members of which exactly one is set, so it has no type of its own to take an instance of. When none is set and the
    struct does not encode, the first member is filled with its type's first instance (`ElicitationSessionScope`).
  * All 67 subtests encode, decode and compare.

Coverage, `go test -count=1 -coverprofile` at the root: 88.8% before, 89.3% after.

| Function | Before | After |
| :--- | ---: | ---: |
| `answerUnlessCancelled` | 81.2% | 100.0% |
| `handleCancelRequest` | 58.8% | 100.0% |
| `handleInbound` | 76.9% | 96.2% |
| `waitNotificationsUpTo` | 55.9% | 85.3% |
| `SendRequestNoResult` | 59.1% | 95.5% |
| `(*AgentSideConnection).NotifyExtension` | 0.0% | 100.0% |
| `(*ClientSideConnection).NotifyExtension` | 66.7% | 100.0% |
| `(*RequestError).Unwrap` | 66.7% | 100.0% |
| `(*RequestError).Error` | 60.0% | 90.0% |
| `NewAuthRequired`, `Ptr`, `WithStartRawOutput` | 0.0% | 100.0% |

Each line range the facts table names for E9 is run, apart from the five lines below. None is a documented branch.

* `connection.go:1474`: the method-not-found answer of a connection without a handler fails to send. It returns early, the same as
  falling through.
* `connection.go:1797`: the context and the connection end at once. A same-instant race; each alone is tested.
* `connection.go:1802-1803` and `:1807`: a done context with no cause, which cannot occur; `context.Cause` gives `Err` at worst.
* `errors.go:62`: `Error`'s fallback without data. The view without data always encodes, so it cannot be reached.

The other unrun line in `SendRequestNoResult` (`connection.go:1869`, params that do not encode) is before the write, outside E9.

`go test -race -count=50 -v` over the three E10 tests, `TestConformanceConstructors` and the six E9 tests with a context or a bubble:
50 PASS for each of the ten, no SKIP.

Planted breaks, each in a fresh scratch copy (28 of 28 seen to fail):

| Break | What failed |
| :--- | :--- |
| `SendRequestNoResult` drops the error response | `TestSendRequestNoResultAnswers/an_error`: nil, want -32602 |
| `SendRequestNoResult` does not wait for notifications | `TestSendRequestNoResultHeldBehindNotification`: returned while the handler ran |
| the agent's `NotifyExtension` does not check the name | `TestNotifyExtensionNames`: nil for `""` and `x/ping`; the wrong first line |
| the client's `NotifyExtension` does not check the name | `TestNotifyExtensionNames`: nil for `""` and `x/ping` |
| a handler's result is kept under `ErrPromptCancelled` | `TestAnswerUnlessCancelledHandlerFirst`: the handler's result |
| every cause answers cancelled at once | `TestPermissionCancelRequestWaitsForHandler`: the cancelled outcome |
| the wait ignores the context's end | `TestWaitNotificationsEnds/context`: nil |
| the wait ignores the connection's end | `TestWaitNotificationsEnds/connection`: nil |
| bad `$/cancel_request` params not logged | `TestCancelRequestFaults`: the log line missing |
| a missing `requestId` not logged | `TestCancelRequestFaults`: the log line missing |
| the `requestId` fallback to its bytes removed | `TestCancelRequestFaults`: `1e5000` never cancelled |
| the inbound request id fallback to its bytes removed | `TestCancelRequestFaults`: `1e5000` never cancelled |
| an unknown id cancels another request | `TestCancelRequestFaults`: `1e4999` cancelled `1e5000` |
| no handler answers -32603 | `TestNilHandlerAnswers`: -32603, want -32601 |
| no handler answers a notification | `TestNilHandlerAnswers`: answered before the probe |
| a result that does not encode is sent as null | `TestUnmarshalableResultAnswered`: no error |
| the response id's canonicalisation failure not logged | `TestUncanonicalIDs`: the log line missing |
| an id that does not canonicalise counts as pending | `TestUncanonicalIDs`: the -32600 answer never came |
| `NewAuthRequired` gives -32001 | `TestSmallConstructors` |
| `Ptr` returns a pointer to a zero value | `TestSmallConstructors` |
| `WithStartRawOutput` sets `rawInput` | `TestSmallConstructors` |
| a nil `Error()` gives `""` | `TestRequestErrorNilAndFallback` |
| a nil `Unwrap()` gives `ErrConnectionClosed` | `TestRequestErrorNilAndFallback` |
| `Error`'s fallback drops the data | `TestRequestErrorNilAndFallback` |
| the conformance fill removed | `TestConformanceConstructors`: the four subtests fail, not skip |
| a request with an ended context queues a `$/cancel_request` | `TestRequestWithEndedContextWritesNothing`: 142 bytes written |
| the agent does not call `Cancel` for `session/cancel` | `TestConnectionHandlesNotifications`: the cancel entry missing |
| the client does not send `session/cancel` when a prompt's context ends | `TestPromptCancellationSendsCancelAndAllowsNewSession`: timeout |

Gates, through the Python runner, on the tree:

* regenerating changes no generated file;
* `make pre-add-check` exit 0;
* `GOEXPERIMENT=nojsonv2 go test ./...` exit 0;
* `GOOS=windows go vet` exit 0 in both modules;
* `make check-records` and `make markdownlint` exit 0;
* the identifier scan finds nothing;
* Windows `go test -count=1 -shuffle=on ./...` exit 0 at the root, where the `synctest` tests run, and in `cmd/generate`;
* the API listing: 2429 lines at `v1.15.0` and in the tree; 0 removed, 0 added.

The owner committed step 6's record as `d13d2d5`.

### Deviation, 2026-10-17, during step 7

Found: the sentence "Design", I12, gives `Validate`, `MarshalJSON` and `UnmarshalJSON` names "the `<Definition>` schema definition",
but 208 of the 534 such methods have no definition of that name. Those 208 are 128 `UnmarshalJSON`, 28 `MarshalJSON` and 52 `Validate`.
124 are on inline union variants, such as `RequestPermissionOutcomeSelected`. The other 84 are on 48 definitions whose Go name differs
from the definition's key: `UnstableDidChangeDocumentNotification` is the `DidChangeDocumentNotification` definition. The structs of
inline object properties have the same gap, but in the current schema none of them has a codec or `Validate`. The 208 are a `go/parser`
pass over the package, checked against the keys of the two schema files; the split is counted from the regenerated docs.

Decision (owner, 2026-10-17): each doc names where the type's schema sits.

* A definition is named by its own key.
* A variant is "the `<value>` variant of the `<Definition>` schema definition": its discriminator value, else its Go field name.
* A nested struct is "the `<key>` property of the `<Definition>` schema definition".

The emitter hands that phrase to the code that emits a struct's and a union's methods. "Design", I12, is annotated to say so. No
decision or fact of the MADR changes.

### Step 7, I12, doc comments on every exported identifier, 2026-10-17

What changed, on `d13d2d5`, as "Design", I12, sets it and as the deviation above amends it:

- The model (`cmd/generate/internal/model`):
  - `EnumValue` gains `Doc`, the value's schema description, for a string-const union, an open enum and an integer enum; a plain
    `enum` has no per-value description.
  - `Struct` gains `Key`, the property its object is.
  - The null variant keeps its description, so `RequestIdNull` is documented.
  - `sdkInterfaces` gives `Agent` and `Client` their docs.
- The emitter (`cmd/generate/internal/emit`):
  - `Validate`, `MarshalJSON` and `UnmarshalJSON` are documented by where their type's schema sits.
  - `emitStruct`, `emitObjectCodec`, `emitValidate`, `addValidate` and `emitInlineVariant` take that phrase. `defPlace` names a
    definition by its own key, and `schemaVariantPlace` a variant.
  - Each enum constant's spec carries its doc, and the layout places a grouped constant's doc on the lines above it.
  - Each typed outbound method gets `outboundDoc`, its peer taken from the side table.
- The regenerated `types_gen.go`, `agent_gen.go` and `client_gen.go`; the hand-written doc on `(*RequestError).Error`.
- Tests:
  - the root test `TestExportedDeclarationsDocumented`;
  - the model's `TestEnumValueKinds` pins `EnumValue.Doc` for the three kinds that carry one, and a plain enum's none;
  - the existing comparison in `model_test.go` follows the new field.
- `docs/architecture.md`, on the generated docs and the test.

The comments use none of the backticks the PLAN uses to mark placeholders, like every other generated comment. Identifiers are plain
and wire values in double quotes: `StopReasonEndTurn is the "end_turn" value.`, `UnmarshalJSON decodes the "selected" variant of the
RequestPermissionOutcome schema definition's JSON into v.`, `Prompt sends the session/prompt request to the agent and returns its
response.`

The first draft of the `Agent` doc named `AgentLoader`. `TestDispatchFollowsTheModel`, which runs the generator on a protocol of its
own, refused it: `sdkInterfaces` serves every protocol, and only ACP's table defines `AgentLoader`. Both docs now name only what every
generated SDK has.

Two branches produce no output under the current schema. The property phrase is unused, since no inline object's struct has a codec or
`Validate`. The wording for a request without a result is unused, since every request has one.

Results:

- **Undocumented declarations:** 654 before, 0 after (`TestExportedDeclarationsDocumented`).
- **Codec and `Validate` docs:** 534. 326 name a definition under the type's own name, 84 a definition under another name (48 types,
  all `Unstable`-prefixed), and 124 a union's variant.
- **Enum constants:** 76 documented.
- **Typed outbound methods:** 40 documented.
- **Code unchanged:** the five generated files match `d13d2d5`'s in code. A scratch Go program parses each file at the commit and in
  the tree without comments and compares their syntax trees, positions left out: equal for all five.
- **Why positions are left out:** a first version reprinted each tree and compared the text. That differed in 170 lines of
  `types_gen.go`, all layout: with docs between them, the printer aligns and separates grouped constants differently.
- **API:** unchanged, as the API listing below shows.

Planted breaks, each in a fresh scratch copy; those in the generator regenerate in the copy (12 of 12 seen to fail):

| Break | What failed |
| :--- | :--- |
| `addValidate` attaches no doc | `TestExportedDeclarationsDocumented`: 147 `Validate`s listed |
| an object's `UnmarshalJSON` gets no doc | `TestExportedDeclarationsDocumented`: 291 `UnmarshalJSON`s listed |
| the outbound methods get no doc | `TestExportedDeclarationsDocumented`: 40 methods listed |
| the layout drops a grouped constant's doc | `TestExportedDeclarationsDocumented`: 76 constants listed |
| `Client` has no doc in `sdkInterfaces` | `TestExportedDeclarationsDocumented`: `Client` |
| the null variant's description dropped | `TestExportedDeclarationsDocumented`: `RequestIdNull` |
| `(*RequestError).Error`'s doc removed | `TestExportedDeclarationsDocumented`: `RequestError.Error` |
| an exported constant added without a doc | `TestExportedDeclarationsDocumented`: the constant |
| an exported method added without a doc | `TestExportedDeclarationsDocumented`: the method |
| a string-const union value's description dropped | `TestEnumValueKinds`: `Consts`'s `Doc` empty |
| an open enum value's description dropped | `TestEnumValueKinds`: `Open`'s `Doc` empty |
| an integer enum value's description dropped | `TestEnumValueKinds`: `Codes`'s `Doc` empty |

The first form of the `(*RequestError).Error` plant removed only the doc's last line, left the method documented, and passed. It was
corrected to remove the whole doc, and then failed.

The comments-only check, on scratch copies of the tree:

- changing an enum constant's value was refused;
- adding a statement to a generated method was refused;
- changing a comment was accepted.

Gates, through the Python runner, on the tree:

- regenerating changes no generated file;
- `make pre-add-check` exit 0, both modules, the host and `GOOS=windows`;
- `GOEXPERIMENT=nojsonv2 go test ./...` exit 0;
- `GOOS=windows go vet` exit 0 in both modules;
- `make check-records` and `make markdownlint` exit 0;
- the identifier scan finds nothing;
- Windows `go test -count=1 -shuffle=on ./...` exit 0 at the root and in `cmd/generate`;
- the API listing: 2429 lines at `v1.15.0` and in the tree; 0 removed, 0 added.

The first full run failed `make markdownlint` on `docs/architecture.md`: MD004, the new list's `*` in a file whose lists use `-`. After
the markers were changed, the records, Markdown and identifier gates passed again; no other file changed.

The owner committed step 7's record as `9a70c6a`.

### Step 8, release notes, 2026-10-18

What changed, on `9a70c6a`:

- [0019-REPORT-release-notes-v1-15-1.md](../reports/0019-REPORT-release-notes-v1-15-1.md), new: the text of the GitHub release for
  `v1.15.1` and the sources behind it, in the form of the `v1.15.0` notes.
- Its row in `docs/README.md`.

The release text has three sections.

- **Documentation:** I12, E8, E14.
- **Example programs:** E5, E6, E7, E12, E14. These change behaviour, as "Rollout and Rollback" says the notes must name.
- **For contributors:** E9 to E11, E13, E15 to E17.

Each statement cites its commit and finding.

Facts it rests on, re-read for this step:

- `git log v1.15.0..HEAD`: nine commits.
- `git log v1.15.0..HEAD -- '*.go' ':!cmd/generate'`: `fdbdff6`, `4da5878`, `d13d2d5`, `9a70c6a`.
- `git diff --stat v1.15.0 HEAD` over the non-test, non-example Go files of the root: `agent_gen.go`, `client_gen.go`,
  `errors.go` and `types_gen.go`, 1091 lines added and 56 removed.
- `git diff v1.15.0 -- schema/`: empty.
- The API listing: 0 removed, 0 added (step 7's gates).

Decision 12 of the MADR, read at its line 208, says phases 3 to 5 ride the release after each lands.

Gates on the tree: `make check-records` and `make markdownlint` exit 0; the identifier scan finds nothing. No Go file changed in this
step.

The owner committed step 8's record as `93655d1`.

### Step 9, verification, 2026-10-18

Run over the finished tree at `93655d1`, with a clean working tree, through the Python runner.

#### Gates

- regenerating changes no generated file;
- `make pre-add-check` exit 0;
- `GOEXPERIMENT=nojsonv2 go test ./...` exit 0;
- `GOOS=windows go vet` exit 0 in both modules;
- `make check-records` and `make markdownlint` exit 0;
- the identifier scan finds nothing;
- Windows `go test -count=1 -shuffle=on ./...` exit 0 at the root and in `cmd/generate`.

The exported API listing has 2429 lines at `v1.15.0` and at `93655d1`: 0 removed and 0 added, exactly "Verification"'s expectation.

#### Each finding re-checked

A script outside the tree (`recheck.py`) re-runs each finding's re-check on the tree. 21 of 21 checks hold:

| Finding | Re-check | Result |
| :--- | :--- | :--- |
| E5 | `example/claude-code` runs `@agentclientprotocol/claude-agent-acp@latest`; `--acp` in `example/gemini`; no `experimental-acp` in the examples, `Example_gemini` or the README | fixed |
| E6 | the agent's `Prompt` takes the SDK's context and its session holds no cancel; `TestExamplePair` passes (`end_turn` and `cancelled`) | fixed |
| E7 | no example advertises `Terminal: true`; `TestWriteTextFileStaysInCwd` and `TestTerminalMethodsNotFound` pass in all three clients | fixed |
| E8 | `go test -list Example .` lists `Example_agent` and `Example_client`; the three files are `package acp_test`, with no `testAbsPath`, and two `// Output:` blocks | fixed |
| E9 | root coverage 89.3% (88.8% before); every range the facts table names runs, apart from the two step 6 names as run in part | fixed |
| E10 | the three fixed sleeps gone, `synctest.Wait()` in their place; `TestConformanceConstructors` runs 67 subtests, none skipped | fixed |
| E11 | `scripts/check_records.py` against step 2's probes: every case as expected | fixed |
| E12 | `.golangci.yml` no longer excludes `^example/`; the examples are linted by the precheck above | fixed |
| E13 | `make release` no longer says "then tag v$(VERSION)" | fixed |
| E14 | `docs/architecture.md` classifies a response as the code does; the README says what the client starts and that it prints the stop reason | fixed |
| E15 | step 3's two cases: the precheck names the skip and its reason, for `GO_PRECHECK_SKIP_VULN=1` and offline | fixed |
| E16 | `ci.yml` runs `govulncheck@v1.9.0` in both modules | fixed |
| E17 | `mise.lock` is gone | fixed |
| I12 | `TestExportedDeclarationsDocumented` passes: 0 undocumented | fixed |

**E9's line numbers.** Step 7 added `(*RequestError).Error`'s four-line doc above three of the `errors.go` ranges, so step 6's line list
was shifted by four for those ranges. The first run used the unshifted list and reported 19 of 22 ranges run; the shifted list gives
step 6's 20 of 22.

**The re-check, broken on purpose.** It was seen to fail on planted breaks in a scratch copy:

- `mise.lock` restored;
- `Terminal: true` advertised by `example/client`;
- `^example/` excluded again;
- the old `make release` message;
- `Example_client`'s `// Output:` removed.

Each break missed its check. The last also missed the listing check, since an example without output is not runnable. The script
exited 1 with 15 of 21 holding.

**Scratch tools.** Between steps 8 and 9, a cleanup of the host's temporary directory removed some of the scratch tools these records
name. `scratch_copy.py` was rewritten to the same behaviour for this step. `plant.py`, the push guard script and the CI watcher are
not used here.

#### Acceptance

Every acceptance criterion in "Verification" is met on the tree, apart from CI on the pushed commits. After the push, this PLAN and its
`docs/README.md` row become `complete`, and the owner tags `v1.15.1` and publishes its release from
[0019-REPORT-release-notes-v1-15-1.md](../reports/0019-REPORT-release-notes-v1-15-1.md).

The pre-push disclosure guard runs over the commits not yet pushed, `2906c21` to this record's commit, when the owner is asked to push.

The owner committed step 9's record as `e4707cb`.

### Deviation, 2026-10-18, after step 9

**Found.** `AGENTS.md` has agents run the global pre-push disclosure guard over the outgoing commits before asking the owner to push.
It says to do this with a shell pipeline, and agents have kept a wrapper for it outside the tree. That wrapper and other scratch
tools were lost overnight to a cleanup of the host's temporary directory, as step 9 records. The guard run was rebuilt for step 9, and
it passed over `0acc8e1..e4707cb`, 9 commits, exit 0. The owner then pushed `e4707cb`. CI run `39129122998` on it passed on
`validate (ubuntu-24.04)`, `validate (macos-15)` and `validate (windows-2025)`.

**Decision (owner, 2026-10-18).** The wrapper lives in the repository, as this phase's last change, before its close:

- `scripts/push_guard.py`: standard library only, with paths relative to the repository and no machine paths. Remote and branch
  default to `origin` and `main`.
- `make push-guard`, which runs it.
- `AGENTS.md`'s paragraph on the guard points at it in place of the pipeline.
- A line in the release notes' list for contributors.

These files join "Scope", which is annotated to say so. The script is seen to fail on a planted disclosure in a scratch clone. The
gates run again, then the guard runs through the new script over the one unpushed commit, this one. The close waits on CI for that
commit. No decision or fact of the MADR changes.

The workspace template's `AGENTS.md` keeps the pipeline. Its other repositories have no such script, and decision 11 changes only the
records script there. So this repository's paragraph is a named difference from the template.

### The push guard, as the deviation of 2026-10-18 decides

What changed, on `e4707cb`:

- **`scripts/push_guard.py`, new.**
  - It gives the global guard the line git's pre-push hook would, for `--branch` (default `main`) against `--remote`'s
    remote-tracking ref (default `origin`), and exits with the guard's status.
  - It exits 1, saying that nothing was checked, when the guard is missing; and 1 when the branch or the ref does not exist.
  - It says when the remote is not on github.com, since the guard then checks nothing.
  - It uses the standard library only and runs under the system `python3` too.
  - The repository's Python ignore rules already cover `scripts/`.
- **`make push-guard`**, with its help line.
- **`AGENTS.md`'s guard paragraph** names `make push-guard` in place of the pipeline.
- **The release notes** gain a line under "For contributors".

Planted breaks, each in a fresh scratch clone with one commit on `e4707cb`. Each commit is built with plumbing, so no hook ran, and a
throwaway deny list with one entry is passed to the guard through `DISCLOSURE_DENY`. All five ran as expected:

| Case | Result |
| :--- | :--- |
| the commit carries the deny list's entry | exit 1, "REFUSED the push" |
| the commit is clean | exit 0, "the guard passed" |
| `--guard` names a missing file | exit 1, "is missing; nothing was checked" |
| the remote is not on github.com | exit 0, "is not on github.com" |
| the wrapper broken to send the local commit as the remote's, so no commit is outgoing | exit 0: the planted entry passes, which the first case refuses |

On the tree, `make push-guard` after a fetch: 0 outgoing commits, `e4707cb..e4707cb`, the guard passed. The 9 commits were already
pushed, as the deviation records.
