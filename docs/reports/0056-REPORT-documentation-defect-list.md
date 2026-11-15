# Documentation defect list

Observed 2026-11-15 on `e67216c` (`v1.36.0-3-ge67216c`), for phase 1 of
[0056-PLAN-bring-the-docs-current-and-keep-them-current.md](../decisions/0056-PLAN-bring-the-docs-current-and-keep-them-current.md).
It applies the standard of
[0056-MADR-bring-the-docs-current-and-keep-them-current.md](../decisions/0056-MADR-bring-the-docs-current-and-keep-them-current.md),
D1, and lists every defect found, each with its evidence and the phase that fixes it.

This report records what was read and run. It decides nothing, corrects no doc, and amends no record.

## Base

- `git rev-parse HEAD`: `e67216c7ae19f04ee9a7c8c0259ae88407ac502f`, on `main`. `git status --short` was empty.
- `git describe --tags --abbrev=0`: `v1.36.0`.
- `git diff --stat v1.36.0..HEAD -- '*.go' Makefile go.mod` printed nothing. No code has changed since `77a81cb`, the
  commit the MADR read, so each MADR row was re-checked against the same code.

## How it was read

- **`README.md`, lines 1 to 1927, in full**, in the ranges 1-260, 260-510, 510-760, 760-989, 989-1218, 1218-1467,
  1467-1706 and 1706-1927.
- **`docs/architecture.md`, lines 1 to 1469, in full**, in the ranges 1-260, 260-500, 500-760, 758-958, 957-1217 and
  1216-1469. Every path of its "Tree" section was checked against the disk and its description read.
- **In full:** `RELEASING.md`, `tools/README.md`, `AGENTS.md`, the prose of `docs/README.md` and its "I want to…"
  matrix, the package comment of every package (`doc.go` of the root, `unstable`, `remote`, `acptest` and `clienthost`;
  the comment in `agentproc/agentproc.go`; the internal packages' and commands'), the `flag` definitions of
  `example/*`, `cmd/generate` and `cmd/generate/internal/mutate`, and the `##` text of every Make target.
- **Statements were checked** against the code that decides them, by reading it or its doc comment, by a constant, a
  count or a test name, or by running a program. The evidence is the `file:line` beside each row below.
- **Three one-off scripts were run over the whole of `README.md`, `docs/architecture.md`, `RELEASING.md`,
  `tools/README.md` and `AGENTS.md`**, from a scratch directory outside the tree:
  - every qualified Go name a doc cites in backticks, such as `acp.PresentNotice` or `Negotiation.AllowsValue`, is
    declared in the tree;
  - every bare identifier in backticks occurs somewhere in the tracked code;
  - every file, directory and engine case named in backticks exists.

  Each was also run on a planted line naming something that does not exist, and reported it. On the docs they found
  nothing beyond W1's `output.go`. The rest of their output was false positives, which were read one by one: file
  stems, dot-files their path rule stripped, the placeholder `acp.New<Union><Variant>`, `exec.Cmd`'s fields, and the
  engine cases, which are `.txtar` files.
- **Each count the docs state was compared with the code by hand**: 28 gated methods, 13 of them unstable, 36 capability
  rules, 17 absolute paths, 12 patch fields, 10 doc notes, the test counts, and the size and time constants.
- **What a reading could not settle** is listed under "Not verified". It is not counted as holding.

## Counts

| Class | Rows | Items | Phase |
| :--- | ---: | ---: | :--- |
| Wrong | 5 | 5 | 2 (4), 4a (1) |
| Missing feature doc | 13 | 46 | 3 (11), 5a (2) |
| Missing doc comment | 2 | 87 members | 4a (34), 4b (53) |
| Stale | 2 | 2 | 6 |

Two leads are refuted, W2 and M5. Four MADR rows changed on re-checking: M2, M3, M8 and S1. One item needs the owner
(below).

## Wrong

| Id | Doc | What it says | What the code says | Phase |
| :--- | :--- | :--- | :--- | :--- |
| W1 | `docs/architecture.md:1205` | "The output (`output.go`) is kept to the request's `outputByteLimit` and `MaxOutputBytes`" | `clienthost/` holds no `output.go`. The limit is set in `clienthost/terminals.go:113`-`:117`, and the cut from the front, 64 KiB past the limit, is `internal/tailbuf/tailbuf.go:19` (`compactAfter = 64 << 10`) | 2 |
| W4 | `README.md:85`-`:86` | "The example clients advertise the file-system capability and not the terminal one, and write only inside the session's working directory" | `example/remote`'s client, which the list above it names, advertises no capability (`example/remote/main.go:235` and `:271`, an `InitializeRequest` with no `ClientCapabilities`) and embeds `acp.ClientRefusals` (`:303`-`:304`), so it serves no file method. The sentence holds for `example/client`, `example/gemini` and `example/claude-code` (`example/client/main.go:254`, `example/gemini/main.go:246`, `example/claude-code/main.go:230`) | 2 |
| W5 | `docs/architecture.md:1170`-`:1171` | "the stderr tail's six in `stderr_test.go`" | `agentproc/stderr_test.go` has seven tests, `:16` to `:158` | 2 |
| W6 | `docs/architecture.md:1336`, `:1345` | "`remote`'s 63 tests", "`ws_test.go`'s sixteen" | `remote/ws_test.go` has 17 tests. `TestDefaultMaxMessageSize` was added in `8e7caad`, 2026-11-10. With frame 3, route 4, handler 8, httpclient 16 and httpserver 16, the package has 64, not counting `TestMain` | 2 |
| W7 | `doc.go:36`-`:38` | "The connection's errors wrap sentinels, for `errors.Is`:" followed by six names | The connection also returns errors that wrap `ErrShutdown`, `ErrHandlersRunning`, `ErrSendingMiddleware` and `ErrNotificationQueueOverflow` (`errors.go:205`, `:214`, `:225`, `:251`), and `README.md:896`-`:915`, `:1048`-`:1052` and `:1222`-`:1223` describe each. The list reads as the whole set, and names six of ten | 4a (a Go file) |

## Missing feature doc

The inventory of step 6 counts 223 items: 21 packages, 28 commands, 67 flags, 19 environment and Make variables, 65
settings keys and 23 Make targets. 32 are named in no reference doc. The audit is described under "The inventory audit".
Phase 3 starts from this list.

| Id | Item | Where it is decided | Where it goes | Phase |
| :--- | :--- | :--- | :--- | :--- |
| M1 | Make targets `all`, `vet`, `lint`, `tidy`, `vuln`, `bench`, `help` | `Makefile:69`, `:83`, `:88`, `:103`, `:107`, `:138`, `:163` | `docs/architecture.md` | 3 |
| M2 | Make and environment variables `GOLANGCI_LINT` (`Makefile:50`, `tools/platforms.py:31`, `scripts/go-precheck.sh:102`), `MUTANTS`, `SEED`, `JOBS` (`Makefile:145`-`:147`) and `GIT_BASH` (`Makefile:6`) | as named | `docs/architecture.md` | 3 |
| M3a | `cmd/generate`'s `-schema` | `cmd/generate/main.go:41` | `docs/architecture.md` | 3 |
| M3b | `example/claude-code`'s `-yolo` | `example/claude-code/main.go:209` | `README.md`, "Try the Examples" | 3 |
| M3c | 13 flags of the tools and scripts: `scripts/check_ci_filters.py --root` (`:339`), `scripts/check_disclosure.py --root` (`:100`), `scripts/check_records.py --check` (`:256`), `scripts/push_guard.py --guard` (`:40`), `tools/ci_jobs.py --runs`, `--created`, `--markdown` (`:278`-`:280`) and `--attempt` (`:283`, `:288`), `tools/leftovers.py --name` (`:39`), `tools/plants.py --keep` (`:136`), `tools/remote.py --goarch` (`:269`), `tools/watch_ci.py --minutes` and `--more` (`:86`-`:87`) | as named | `tools/README.md` for `tools/`; `docs/architecture.md` for `scripts/` | 3 |
| M4 | the connection's diagnostic logging, `WithLogger` and `Connection.SetLogger` | `connection.go:906`-`:911`, `:1440`-`:1444` | `README.md` | 3 |
| M6 | `CHANGELOG.md` | none exists | `CHANGELOG.md` | 5a |
| M7 | the release tools in `RELEASING.md`: `tools/release_body.py`, `tools/watch_ci.py`, `tools/publication.py`, `tools/consumer.py`, `tools/consumer_probe.py` | `grep -n 'release_body\|watch_ci\|publication\|consumer' RELEASING.md` exits 1; `tools/README.md:38`-`:43` describes each | `RELEASING.md` | 5a |
| M9 | `opencode.json`'s `instructions` key, an agent harness's pointer to `.opencode/rules.md` | `opencode.json:3` | `docs/architecture.md`, "Tree" | 3 |
| M10 | `WithNotificationDropHandler`, called for each notification `OverflowDropNewest` drops | `connection.go:892`-`:899` | `README.md`, beside `OverflowDropNewest` | 3 |
| M11 | `agentproc.Command.ConnectionOptions` and `agentproc.InProcess.ConnectionOptions`, the options of the client's connection to the agent | `agentproc/agentproc.go:52`, `agentproc/inprocess.go:54`-`:55` | `README.md`, beside `agentproc` | 3 |
| M12 | the error constructors `NewParseError`, `NewInvalidRequest`, `NewInvalidParams`, `NewRequestCancelled` and `NewAuthRequired`. `README.md:1549` names `NewResourceNotFound`, and its examples use `NewMethodNotFound` and `NewInternalError` | `errors.go:310`-`:347` | `README.md`, "Errors on the wire" | 3 |
| M13 | `ToolCallStream.Entity`, the stream's state as a `ToolCallEntity`, and `ErrToolCallStreamFinished`, which every method returns after `Finish` | `toolstream.go:216`-`:218`, `errors.go:227`-`:231` | `README.md`, "Streaming a tool call" | 3 |

M9 is a judgment the MADR left to this phase. `opencode.json` is a tracked configuration file with one key, read by
an agent harness, and no doc names it. It is counted as a settings key, and the fix is one line in "Tree". The owner
can overrule this, which drops the row.

Nothing is missing among the packages and commands. Every package has a package comment, and every command is named
in a doc.

## Missing doc comment

The `go/ast` audit of step 8 measures the scope the owner set for Open Question 4: the public packages `acp`,
`unstable`, `acptest`, `agentproc`, `clienthost` and `remote`. It counts exported top-level declarations, exported
methods on exported receivers, and the exported fields and interface methods of exported types, `type ( … )` groups
included. A member counts as documented when it has its own comment, or when its type's doc comment names it as a
word.

| Package | Source | Top-level | Undocumented | Methods | Undocumented | Members | Own comment | Named by type | Undocumented |
| :--- | :--- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `acp` | generated | 576 | 0 | 627 | 0 | 1,203 | 1,133 | 19 | 51 |
| `acp` | hand-written | 168 | 0 | 84 | 0 | 43 | 6 | 15 | 22 |
| `acptest` | hand-written | 20 | 0 | 31 | 0 | 20 | 5 | 5 | 10 |
| `agentproc` | hand-written | 10 | 0 | 17 | 0 | 14 | 12 | 0 | 2 |
| `clienthost` | hand-written | 5 | 0 | 8 | 0 | 4 | 3 | 1 | 0 |
| `remote` | hand-written | 11 | 0 | 5 | 0 | 11 | 11 | 0 | 0 |
| `unstable` | generated | 46 | 0 | 39 | 0 | 56 | 54 | 0 | 2 |

The type-comment rule clears 40 members: 21 hand-written and 19 generated. The 19 generated ones are fields named
`SessionUpdate` of `SessionUpdate`'s variants. Their type's comment names the union, as in "the `user_message_chunk`
variant of the SessionUpdate schema definition", so the rule matches the field's name, not a description of it.
Phase 4b's sentence goes on every variant type, so it describes these fields as well.

| Id | Members | Phase |
| :--- | :--- | :--- |
| D1 | 34 hand-written. `compaction.go:12`-`:15`: `CompactionEntity.Status`, `Summary`, `Error`, `Meta`. `elicitation.go:44`-`:46`: `ElicitationField.Name`, `Required`, `Schema`. `elicitation.go:388`-`:389`: `ElicitationURL.Host`, `Secure`. `errors.go:19`-`:21`: `RequestError.Code`, `Message`, `Data`. `errors.go:297`: `ProtocolVersionError.Requested`. `observer.go:71`: `Observation.Kind`. `tooltimeline.go:14`-`:21`: `ToolCallEntity.Title`, `Kind`, `Status`, `Name`, `Content`, `Locations`, `RawInput`, `RawOutput`. `acptest/client.go:42`-`:46`: `TerminalHost`'s five methods. `acptest/pair.go:18`-`:19`: `Pair.Agent`, `Client`. `acptest/recorder.go:29`-`:30`: `Record.Direction`, `Frame`. `acptest/script.go:16`: `Turn.Updates`. `agentproc/agentproc.go:84`: `Command.StderrTailLines`. `agentproc/inprocess.go:60`: `InProcess.InputTap` | 4a |
| D2 | 53 generated. 49 discriminator fields in `types_gen.go`: 29 `Type`, 7 `StopReason`, 4 `State`, 4 `Kind`, 3 `Action`, 2 `Mode`, 2 `Outcome`, such as `types_gen.go:761` `AuthMethodTerminalInline.Type`. 2 in `unstable/unstable_gen.go:526` and `:1599`. `types_gen.go:25584`-`:25585` `Negotiation.Request` and `Response`, which are not discriminators (see "For the owner") | 4b |

Phase 4a's file list at `77a81cb` named 14 files and 52 members. After the type-comment rule it is 11 files and 34
members. `connection.go`, `extensions.go` and `acptest/replay.go` drop out. `doc.go` joins for W7. The lexical scan in
the MADR counted 13 `acptest` members and 52 hand-written in all. The `go/ast` count is 15 without a comment of its own,
because the lexical scan could not see grouped types.

## Stale

| Id | Doc | Finding | Evidence | Phase |
| :--- | :--- | :--- | :--- | :--- |
| S1 | `docs/README.md`, "I want to…" | 32 of the 48 MADRs have no row: the MADR's 31, and `0056-MADR-bring-the-docs-current-and-keep-them-current.md` | a script compared the matrix's links with `docs/decisions/*-MADR-*.md`: 0013 to 0015, 0017 to 0024, 0026 to 0031, 0033, 0034, 0036 to 0038, 0042, 0045 to 0050, 0052, 0055 and 0056 | 6 |
| S4 | `docs/README.md`, "I want to…" | rows whose task is now history, phrased as current: "know why the next ACP feature is the remote transport, and what it would take" (package `remote` shipped; `docs/architecture.md:1230`), and the Go-floor rows "see whether a Go 1.27.1 floor fits this SDK" and "apply the Go 1.27.2 and schema 1.25.0 pin" (the floor is `1.26.11`, `go.mod:3`) | the rows, read against the tree | 6 |

The Records table is current. A script compared its 161 links with the 161 record files, both ways, and each MADR
and PLAN row's status with the file's `status`. It found no difference. Run on a copy with one row removed and one
status changed, it reported both.

## The MADR's rows and the survey's

The survey itself is not in the tree. Its rows are known here as the MADR carries them, and each maps to a MADR row.
Its A3 is W3, its B2 to B6 and B8 are M8, and its C1 and C2 are the index check above. Its `acpext` row was an error
of its brief and is not a finding.

| Row | Status | Evidence |
| :--- | :--- | :--- |
| W1 | verified | above |
| W2 | refuted again | `cmd/generate/internal/model/acp.go:35`-`:106`: 28 method rules carry a `Capability`, 13 of them `providers/`, `nes/` or `document/`. `:203`-`:247` holds 36 capability rules. `docs/architecture.md:246`-`:249` holds |
| W3 | done | both files read in full; it yielded W4 to W6 here, and W1 |
| M1 | verified | above |
| M2 | verified, and widened | `GIT_BASH` added; `FUZZTIME`, `BASE`, `VERSION`, `MDSH`, `GOCACHE` and `FILES` are named in docs |
| M3 | partly changed | `-schema` and `-yolo` verified missing. The mutation tool's ten flags are named, each with what it takes, in its command's package comment (`cmd/generate/internal/mutate/main.go:10`-`:12`), which D1 counts as a reference, so they are not counted missing. The `-skip`, `-only` and `-timeout` hits are that line and `Makefile:151`. `-out` is named at `docs/architecture.md:1004`-`:1005`. 13 flags of the tools and scripts were not in the MADR (M3c) |
| M4 | verified | above |
| M5 | refuted | `tools/_common.py` is described at `docs/architecture.md:1364`-`:1369`, and its variables at `tools/README.md:10`-`:23`. It is a module, not a command |
| M6 | verified | `ls CHANGELOG.md` fails |
| M7 | verified | above |
| M8 | turned into features | The 72 hand-written names that no doc names as a word belong to features that are described, but for M4, M10, M12 and M13. Examples: `AgentRefusals`' and `ClientRefusals`' protocol methods (`README.md:98`-`:102`, `:133`-`:138`), the `Rule…` constants under `ValidationError.Rule` (`README.md:354`), `UpdateUserMessageText` and `UpdateAgentThoughtText` ("each with a `Text` form", `README.md:210`-`:211`), and `MemoryClient`'s setters (`README.md:1829`-`:1833`) |
| M9 | judged in scope | above |
| S1 | verified, changed | 32 rows now, with 0056 |
| S2 | verified | 0 of 48 reports have frontmatter; the owner chose to leave them (Open Question 3) |
| S3 | verified | 38 release-notes reports. Of 53 `v*` tags, 16 have no report under their name: the 15 tags `v0.4.3` to `v0.14.0`, and `v1.14.0`, filed as `0019-REPORT-release-notes-v0-18-0.md`. `go.mod:7` retracts `v1.13.0`, which has no tag |
| survey C1, C2 | hold | the index and status check above |

## The inventory audit

The audit is a script in the session's scratch directory, not committed (MADR D3). It rebuilds the inventory of step 6
from the code. Its sources are `go list ./...` in both modules; every `package main`; every `tools/*.py` and
`scripts/*.py` with a `__main__`; every `flag` definition in non-test Go; every `add_argument`; `os.Getenv` in Go;
`os.environ` and `_common.py`'s `HOST_VARS` in Python; the `?=` and `$(…)` variables of the `Makefile`; the `${…}`
reads of `scripts/go-precheck.sh`; the root's `With…` functions; the exported fields of the option and configuration
structs (`remote.DialOptions` and `HandlerOptions`, `agentproc.Command` and `InProcess`, `clienthost.Terminals` and
`Files`, `acptest.ScriptAgent`, `Turn` and `MemoryClient`); `opencode.json`'s keys; and the Make targets.

It searches the D1 docs for each item: `README.md`, `docs/architecture.md`, `RELEASING.md`, `tools/README.md`,
`AGENTS.md` and every package comment. A flag counts as found only within three lines of its command's name.

- **Judged by reading.** Eight items are recorded in the audit with the line that describes them in other words.
  They are `WithStartStatus`, `WithStartContent`, `WithStartLocations`, `WithUpdateTitle`, `WithUpdateKind` and
  `WithUpdateStatus` (`README.md:227`, "and their options"); `InProcess.OutputTap` and `InputTap`
  (`docs/architecture.md:1133`-`:1135`); `ScriptAgent.Turns` (`README.md:1858`); and `cmd/generate -out`
  (`docs/architecture.md:1004`-`:1005`).
- **Out of scope.** Make's own variables (`CURDIR`, `MAKE`, `MAKEFILE_LIST`, `MAKE_VERSION`, `OS`) and the operating
  system's (`TMPDIR`, `LOCALAPPDATA`, `XDG_CACHE_HOME`) are listed and not counted.
- **On the tree:** exit 1, 32 items, exactly the rows M1, M2, M3a, M3b, M3c, M4, M9, M10 and M11.
- **Planted breaks**, on a scratch clone:
  - `ACPTEST_UPDATE` removed from `README.md` and `docs/architecture.md`: exit 1, and one more line,
    `UNDESCRIBED env: ACPTEST_UPDATE (acptest/golden.go)`;
  - every `make api` and `` `api` `` removed from the docs and the Go package comments: exit 1, and one more line,
    `UNDESCRIBED target: api`.

  A first plant that left `remote/doc.go:10` in place was not reported. That was correct, since that package comment
  names `make api`.

## The doc-comment audit

It is a `go/ast` program in the session's scratch directory, using the standard library alone. It is not committed. It
parses the tracked non-test `.go` files of the public packages with `parser.ParseComments`, and marks a file generated
by `ast.IsGenerated`. A grouped constant or variable counts as documented by its group's comment. Each name of a
multi-name field and each embedded field is counted.

- **On the tree:** exit 1, 87 undocumented, as tabled.
- **Planted breaks**, four of them on a scratch clone, each asserted to land:
  - the comment above `ClientRefusals.CompleteElicitation` removed;
  - the comment above the generated `AgentAuthCapabilities` removed;
  - the own comment of `remote.DialOptions.Header` removed, a field its type's comment does not name;
  - the own comment of `clienthost.Files.MkdirAll` removed, a field its type's comment names ("unless MkdirAll").

  It exited 1 with 90. The three new lines were `refusals.go … method ClientRefusals.CompleteElicitation`,
  `types_gen.go … type AgentAuthCapabilities` and `remote/client.go … member DialOptions.Header`. It did not report
  `MkdirAll`.

## For the owner

These items need a decision, not a fix. The pair is not amended for them in this phase.

1. **`Negotiation.Request` and `Negotiation.Response`** (`types_gen.go:25584`-`:25585`) have no comment of their own.
   - Their type's comment describes them in lower case, as "the request the client sent and the response the agent
     answered", so the rule's word match does not count them.
   - They are generated, so only phase 4b can change them. Phase 4b's step 1 covers the discriminator sentence alone.
   - Recommended: amend phase 4b's step 1 to have the emitter name both fields in `Negotiation`'s comment. This is
     generator text of its own, not schema text, and the files are already in phase 4b's list.
   - The other choice is to count the lower-case description as naming them, and record that as the reading of the
     rule.
   - Doing nothing leaves phase 4b's acceptance, "the same audit reports 0", impossible to meet.

Two judgments of this phase follow. The owner can overrule either; neither blocks a phase.

- **M9.** `opencode.json` is counted as a settings key (above).
- **What counts as describing a flag.** A flag named in a reference counts as described. That covers the mutation
  tool's usage line and `README.md:79`'s list of `example/gemini`'s flags. The stricter reading, a meaning and a
  default for each, adds 15 rows to phase 3: the mutation tool's 10 and `example/gemini`'s 5.

No defect lies in a file that no phase lists.

## Not verified

Each range below was read through. Its names, paths, numbers and test names were checked as above. Its behavioural
claims were not each traced through the code that decides them, and they are not counted as holding:

- `README.md:253`-`:363`, the codec rules. Each was matched to a test or an error string (`types_gen.go`,
  `enum_closed_test.go:109`, `union_wire_test.go:193`, `raw_value_test.go:8`), not traced through the generator. The
  `fmt` behaviour of `:310`-`:314` was run: `null` on go1.27.4, and `[]`, empty and `[]` on go1.26.11 and under
  `GOEXPERIMENT=nojsonv2`.
- `README.md:529`-`:666`, the order of the `initialize` record, the hold and the hooks. The gates were checked against
  the protocol table.
- `README.md:740`-`:804`, `:881`-`:915`, `:1040`-`:1136` and `:1170`-`:1324`: cancellation, closing, the end of input,
  the reader's pause, the middleware and the observer. Their constants and option names were checked.
- `README.md:1345`-`:1462` and `docs/architecture.md:1238`-`:1332`, the transport's behaviour. Its bounds were checked
  against `remote/handler.go:27`, `:127`, `:138`-`:147`, `remote/client.go:14`, `:49` and `remote/httpclient.go:26`.
- `README.md:1531`-`:1600`, the JSON-RPC answers, but for the error codes (`errors.go:310`-`:359`).
- `docs/architecture.md:136`-`:197`, `:199`-`:499`, `:501`-`:756` and `:953`-`:1029`: the generator's model and its
  refusal rules, the connection's internals, and the unstable boundary. Every identifier named exists, and every count
  matches, but the rules themselves were not traced.
- `docs/architecture.md:1033`-`:1155`, the internals of `agentproc`. Its 54 test names exist, and its four test counts
  were counted (W5).
- The `help` text of each `argparse` option, against what the tool does.

None of these ranges showed a contradiction where it was checked. The PLAN's later phases re-run the evidence for their
own rows before editing, as its "Ordering" step 2 requires.
